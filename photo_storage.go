package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

const (
	photoUploadURLExpiry = 5 * time.Minute
	photoStaleAfter      = time.Hour
	photoUploadAttempts  = 3
)

var (
	errPhotoObjectMissing  = errors.New("photo object missing")
	errPhotoObjectTooLarge = errors.New("photo object too large")
)

// photoStorage isolates S3 behind function fields so handler tests never need
// AWS credentials or network access, matching the sendMagicLink pattern.
type photoStorage struct {
	presignPut func(context.Context, string, time.Duration) (string, error)
	presignGet func(context.Context, string, time.Duration) (string, error)
	get        func(context.Context, string) ([]byte, error)
	put        func(context.Context, string, []byte) error
	delete     func(context.Context, string) error
}

func (s photoStorage) configured() bool {
	return s.presignPut != nil && s.presignGet != nil && s.get != nil && s.put != nil && s.delete != nil
}

func newPhotoObjectKey() (string, error) {
	token, _, err := newToken()
	if err != nil {
		return "", err
	}
	return "photos/" + token, nil
}

func readBounded(reader io.Reader, limit int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, errPhotoObjectTooLarge
	}
	return data, nil
}

func newPhotoStorage(ctx context.Context, cfg config) (photoStorage, error) {
	if cfg.s3Bucket == "" {
		return photoStorage{}, nil
	}
	loadContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	awsConfig, err := awsconfig.LoadDefaultConfig(loadContext, awsconfig.WithRegion(cfg.s3Region))
	if err != nil {
		return photoStorage{}, errors.New("could not configure photo storage")
	}
	return s3PhotoStorage(s3.NewFromConfig(awsConfig), cfg.s3Bucket), nil
}

func s3PhotoStorage(client *s3.Client, bucket string) photoStorage {
	presigner := s3.NewPresignClient(client)
	return photoStorage{
		presignPut: func(ctx context.Context, key string, expires time.Duration) (string, error) {
			request, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			}, s3.WithPresignExpires(expires))
			if err != nil {
				return "", err
			}
			return request.URL, nil
		},
		presignGet: func(ctx context.Context, key string, expires time.Duration) (string, error) {
			request, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			}, s3.WithPresignExpires(expires))
			if err != nil {
				return "", err
			}
			return request.URL, nil
		},
		get: func(ctx context.Context, key string) ([]byte, error) {
			output, err := client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
			if err != nil {
				if isMissingObject(err) {
					return nil, errPhotoObjectMissing
				}
				return nil, err
			}
			defer output.Body.Close()
			return readBounded(output.Body, maxPhotoBytes)
		},
		put: func(ctx context.Context, key string, data []byte) error {
			_, err := client.PutObject(ctx, &s3.PutObjectInput{
				Bucket:      aws.String(bucket),
				Key:         aws.String(key),
				Body:        bytes.NewReader(data),
				ContentType: aws.String("image/jpeg"),
			})
			return err
		},
		delete: func(ctx context.Context, key string) error {
			_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
			return err
		},
	}
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
