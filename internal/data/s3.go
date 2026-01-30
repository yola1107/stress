package data

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"stress/internal/conf"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-kratos/kratos/v2/log"
)

const (
	maxRetries     = 3
	retryDelay     = time.Second
	uploadTimeout  = 30 * time.Second
	presignExpires = time.Hour * 24 * 3
)

type S3Bucket struct {
	client *s3.Client
	bucket string
	region string
	logger *log.Helper
}

func NewS3Bucket(c *conf.Data, logger log.Logger) (*S3Bucket, func(), error) {
	l := log.NewHelper(logger)

	if c.S3 == nil {
		return nil, nil, fmt.Errorf("s3 config is nil")
	}

	configOptions := []func(*config.LoadOptions) error{
		config.WithRegion(c.S3.Region),
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     c.S3.AccessKeyId,
				SecretAccessKey: c.S3.SecretAccessKey,
			}, nil
		})),
	}

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

	cleanup := func() {
		l.Info("S3 uploader closed")
	}

	return &S3Bucket{
		client: s3.NewFromConfig(cfg),
		bucket: c.S3.Bucket,
		region: c.S3.Region,
		logger: l,
	}, cleanup, nil
}

// UploadBytes 上传字节数组到S3
func (r *dataRepo) UploadBytes(ctx context.Context, bucket, key, contentType string, data []byte) (string, error) {
	s := r.data.s3Bucket
	if bucket == "" {
		bucket = s.bucket
	}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(retryDelay * time.Duration(i))
			s.logger.Infof("Retry upload %d/%d: %s", i, maxRetries-1, key)
		}

		uploadCtx, cancel := context.WithTimeout(ctx, uploadTimeout)

		presignClient := s3.NewPresignClient(s.client)
		presignPutResult, err := presignClient.PresignPutObject(
			uploadCtx,
			&s3.PutObjectInput{
				Bucket:      aws.String(bucket),
				Key:         aws.String(key),
				ContentType: aws.String(contentType),
			},
			s3.WithPresignExpires(presignExpires),
		)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("failed to generate presigned PUT URL: %w", err)
			s.logger.Warnf("Upload attempt %d/%d (presign PUT failed): %v", i+1, maxRetries, err)
			continue
		}

		req, err := http.NewRequestWithContext(uploadCtx, "PUT", presignPutResult.URL, bytes.NewReader(data))
		if err != nil {
			cancel()
			lastErr = err
			s.logger.Warnf("Upload attempt %d/%d (request creation failed): %v", i+1, maxRetries, err)
			continue
		}
		req.Header.Set("Content-Type", contentType)

		resp, err := http.DefaultClient.Do(req)
		cancel()

		if err != nil {
			lastErr = err
			s.logger.Warnf("Upload attempt %d/%d (HTTP request failed): %v", i+1, maxRetries, err)
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			lastErr = fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(respBody))
			s.logger.Warnf("Upload attempt %d/%d failed: %v", i+1, maxRetries, lastErr)
			continue
		}

		presignGetResult, err := presignClient.PresignGetObject(
			context.Background(),
			&s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			},
			s3.WithPresignExpires(presignExpires),
		)
		if err != nil {
			lastErr = fmt.Errorf("failed to generate presigned GET URL: %w", err)
			s.logger.Warnf("Failed to generate presigned GET URL: %v", err)
			continue
		}

		s.logger.Infof("S3 upload success: bucket=%s, key=%s, presigned_url=%s", bucket, key, presignGetResult.URL)
		return presignGetResult.URL, nil
	}

	return "", fmt.Errorf("upload failed after %d attempts: %w", maxRetries, lastErr)
}
