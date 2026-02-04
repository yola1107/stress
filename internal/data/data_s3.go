package data

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"stress/internal/conf"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-kratos/kratos/v2/log"
)

const (
	maxRetries    = 3                // 最大重试次数
	retryDelay    = time.Second      // 基础重试延迟
	uploadTimeout = 30 * time.Second // 单次上传超时
)

// S3Bucket S3上传器实现
type S3Bucket struct {
	client *s3.Client
	bucket string
	logger *log.Helper
}

// NewS3Bucket 创建S3上传器
func NewS3Bucket(c *conf.Data, logger log.Logger) (*S3Bucket, func(), error) {
	l := log.NewHelper(logger)

	if c.S3 == nil {
		return nil, nil, fmt.Errorf("s3 config is nil")
	}

	// 配置选项
	configOptions := []func(*config.LoadOptions) error{
		config.WithRegion(c.S3.Region),
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     c.S3.AccessKeyId,
				SecretAccessKey: c.S3.SecretAccessKey,
			}, nil
		})),
	}

	// 如果配置了自定义 endpoint，添加 endpoint resolver
	if c.S3.Endpoint != "" {
		configOptions = append(configOptions, config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               c.S3.Endpoint,
					HostnameImmutable: true,
					Source:            aws.EndpointSourceCustom,
				}, nil
			}),
		))
		l.Infof("Using custom S3 endpoint: %s", c.S3.Endpoint)
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), configOptions...)
	if err != nil {
		l.Errorf("failed loading AWS config: %v", err)
		return nil, nil, err
	}

	client := s3.NewFromConfig(cfg)
	uploader := &S3Bucket{
		client: client,
		bucket: c.S3.Bucket,
		logger: l,
	}

	cleanup := func() {
		l.Info("S3 uploader closed")
	}

	return uploader, cleanup, nil
}

// UploadFile 上传文件到S3（带重试机制）
func (r *dataRepo) UploadFile(ctx context.Context, bucket, key, contentType string, body io.Reader) (string, error) {
	s := r.data.s3Bucket
	if bucket == "" {
		bucket = s.bucket
	}

	// 读取到内存以支持重试
	data, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(retryDelay * time.Duration(i))
			s.logger.Infof("Retry upload %d/%d: %s", i, maxRetries-1, key)
		}

		uploadCtx, cancel := context.WithTimeout(ctx, uploadTimeout)
		uploader := manager.NewUploader(s.client)
		result, err := uploader.Upload(uploadCtx, &s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(key),
			Body:        bytes.NewReader(data),
			ContentType: aws.String(contentType),
		})
		cancel()

		if err == nil {
			s.logger.Infof("S3 upload success: bucket=%s, key=%s, url=%s", bucket, key, result.Location)
			return result.Location, nil
		}

		lastErr = err
		s.logger.Warnf("Upload attempt %d/%d failed: %v", i+1, maxRetries, err)
	}

	return "", fmt.Errorf("upload failed after %d attempts: %w", maxRetries, lastErr)
}

// UploadBytes 上传字节数组到S3
func (r *dataRepo) UploadBytes(ctx context.Context, bucket, key, contentType string, data []byte) (string, error) {
	return r.UploadFile(ctx, bucket, key, contentType, bytes.NewReader(data))
}
