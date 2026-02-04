package data

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"stress/internal/conf"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-kratos/kratos/v2/log"
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

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(c.S3.Region),
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     c.S3.AccessKeyId,
				SecretAccessKey: c.S3.SecretAccessKey,
			}, nil
		})),
	)
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

// UploadFile 上传文件到S3
func (r *dataRepo) UploadFile(ctx context.Context, bucket, key, contentType string, body io.Reader) (string, error) {
	s := r.data.s3Bucket
	if bucket == "" {
		bucket = s.bucket
	}

	uploader := manager.NewUploader(s.client)
	result, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		s.logger.Errorf("failed uploading file to S3: %v", err)
		return "", err
	}

	return result.Location, nil
}

// UploadBytes 上传字节数组到S3
func (r *dataRepo) UploadBytes(ctx context.Context, bucket, key, contentType string, data []byte) (string, error) {
	return r.UploadFile(ctx, bucket, key, contentType, bytes.NewReader(data))
}
