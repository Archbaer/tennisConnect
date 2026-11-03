package main

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestNewPhotoObjectKeyIsRandomAndScoped(t *testing.T) {
	seen := make(map[string]bool, 200)
	for i := 0; i < 200; i++ {
		key, err := newPhotoObjectKey()
		if err != nil {
			t.Fatalf("newPhotoObjectKey: %v", err)
		}
		if !strings.HasPrefix(key, "photos/") {
			t.Fatalf("key %q does not use the photos/ prefix", key)
		}
		if len(key) != len("photos/")+43 {
			t.Fatalf("key %q length = %d, want %d", key, len(key), len("photos/")+43)
		}
		if seen[key] {
			t.Fatalf("duplicate key %q", key)
		}
		seen[key] = true
	}
}

func TestReadBoundedRejectsOversizedReader(t *testing.T) {
	exact := strings.Repeat("a", maxPhotoBytes)
	data, err := readBounded(strings.NewReader(exact), maxPhotoBytes)
	if err != nil || len(data) != maxPhotoBytes {
		t.Fatalf("readBounded at limit = %d bytes, err %v", len(data), err)
	}
	if _, err := readBounded(strings.NewReader(exact+"a"), maxPhotoBytes); !errors.Is(err, errPhotoObjectTooLarge) {
		t.Fatalf("readBounded over limit err = %v, want errPhotoObjectTooLarge", err)
	}
}

func TestS3PresignURLIsShortLivedAndScoped(t *testing.T) {
	client := s3.New(s3.Options{
		Region:       "eu-west-2",
		Credentials:  credentials.NewStaticCredentialsProvider("AKIAEXAMPLE", "secret", ""),
		UsePathStyle: true,
	})
	storage := s3PhotoStorage(client, "private-bucket")
	rawURL, err := storage.presignPut(context.Background(), "photos/example-key", photoUploadURLExpiry)
	if err != nil {
		t.Fatalf("presignPut: %v", err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse presigned URL: %v", err)
	}
	if parsed.Host != "s3.eu-west-2.amazonaws.com" {
		t.Fatalf("presigned host = %q", parsed.Host)
	}
	if parsed.Path != "/private-bucket/photos/example-key" {
		t.Fatalf("presigned path = %q", parsed.Path)
	}
	if got := parsed.Query().Get("X-Amz-Expires"); got != "300" {
		t.Fatalf("X-Amz-Expires = %q, want 300", got)
	}
	if parsed.Query().Get("X-Amz-Signature") == "" {
		t.Fatal("presigned URL has no signature")
	}
	lower := strings.ToLower(rawURL)
	for _, forbidden := range []string{"x-amz-acl", "public-read", "public-write"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("presigned URL contains %q", forbidden)
		}
	}
}
