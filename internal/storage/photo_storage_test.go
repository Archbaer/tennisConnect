package storage

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestS3PresignURLIsShortLivedAndScoped(t *testing.T) {
	client := s3.New(s3.Options{
		Region:       "eu-west-2",
		Credentials:  credentials.NewStaticCredentialsProvider("AKIAEXAMPLE", "secret", ""),
		UsePathStyle: true,
	})
	store := newS3(client, "private-bucket")
	rawURL, err := store.PresignPut(context.Background(), "photos/example-key", 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignPut: %v", err)
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

func TestS3PresignGetIsShortLivedAndScoped(t *testing.T) {
	client := s3.New(s3.Options{
		Region:       "eu-west-2",
		Credentials:  credentials.NewStaticCredentialsProvider("AKIAEXAMPLE", "secret", ""),
		UsePathStyle: true,
	})
	store := newS3(client, "private-bucket")
	rawURL, err := store.PresignGet(context.Background(), "photos/example-key", 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse presigned URL: %v", err)
	}
	if parsed.Host != "s3.eu-west-2.amazonaws.com" || parsed.Path != "/private-bucket/photos/example-key" {
		t.Fatalf("presigned URL = %q", rawURL)
	}
	if got := parsed.Query().Get("X-Amz-Expires"); got != "300" {
		t.Fatalf("X-Amz-Expires = %q, want 300", got)
	}
	lower := strings.ToLower(rawURL)
	for _, forbidden := range []string{"x-amz-acl", "public-read"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("presigned URL contains %q", forbidden)
		}
	}
}
