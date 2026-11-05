package storage

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"tennisconnect/internal/config"
	"tennisconnect/internal/photos"
	"tennisconnect/internal/saferead"
)

// S3 stores photo objects in a private bucket.
type S3 struct {
	client *s3.Client
	bucket string
}

// New returns the configured bucket, or nil when S3 is not configured.
func New(ctx context.Context, cfg config.Config) (*S3, error) {
	if cfg.S3Bucket == "" {
		return nil, nil
	}
	loadContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	awsConfig, err := awsconfig.LoadDefaultConfig(loadContext, awsconfig.WithRegion(cfg.S3Region))
	if err != nil {
		return nil, errors.New("could not configure photo storage")
	}
	return newS3(s3.NewFromConfig(awsConfig), cfg.S3Bucket), nil
}

func newS3(client *s3.Client, bucket string) *S3 {
	return &S3{client: client, bucket: bucket}
}

func (s *S3) PresignPut(ctx context.Context, key string, expires time.Duration) (string, error) {
	request, err := s3.NewPresignClient(s.client).PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return request.URL, nil
}

func (s *S3) PresignGet(ctx context.Context, key string, expires time.Duration) (string, error) {
	request, err := s3.NewPresignClient(s.client).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return request.URL, nil
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isMissingObject(err) {
			return nil, photos.ErrObjectMissing
		}
		return nil, err
	}
	defer output.Body.Close()
	data, err := saferead.ReadBounded(output.Body, photos.MaxPhotoBytes)
	if errors.Is(err, saferead.ErrTooLarge) {
		return nil, photos.ErrObjectTooLarge
	}
	return data, err
}

func (s *S3) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("image/jpeg"),
	})
	return err
}

func (s *S3) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func isMissingObject(err error) bool {
	var notFound *types.NoSuchKey
	if errors.As(err, &notFound) {
		return true
	}
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		code := apiError.ErrorCode()
		return code == "NoSuchKey" || code == "NotFound"
	}
	return false
}
