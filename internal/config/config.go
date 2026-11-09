package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL        string
	Address            string
	AppBaseURL         string
	EmailFrom          string
	ResendAPIKey       string
	EmailEncryptionKey []byte
	EmailHMACKey       []byte
	CookieSecure       bool
	TrustProxy         bool
	S3Bucket           string
	S3Region           string
	SightengineUser    string
	SightengineSecret  string
	MapboxAccessToken  string
}

func Load() (Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Host == "" || parsed.Path == "" || parsed.Path == "/" ||
		(parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return Config{}, errors.New("DATABASE_URL must be a valid PostgreSQL URL")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return Config{}, errors.New("PORT must be between 1 and 65535")
	}

	cfg := Config{DatabaseURL: databaseURL, Address: ":" + port}
	if err := loadAuthConfig(&cfg); err != nil {
		return Config{}, err
	}
	if bucket := os.Getenv("S3_BUCKET"); bucket != "" {
		region := os.Getenv("AWS_REGION")
		if region == "" {
			return Config{}, errors.New("AWS_REGION is required when S3_BUCKET is set")
		}
		cfg.S3Bucket = bucket
		cfg.S3Region = region
	}
	moderationUser := os.Getenv("SIGHTENGINE_API_USER")
	moderationSecret := os.Getenv("SIGHTENGINE_API_SECRET")
	if (moderationUser == "") != (moderationSecret == "") {
		return Config{}, errors.New("SIGHTENGINE_API_USER and SIGHTENGINE_API_SECRET must be set together")
	}
	cfg.SightengineUser = moderationUser
	cfg.SightengineSecret = moderationSecret
	cfg.MapboxAccessToken = strings.TrimSpace(os.Getenv("MAPBOX_ACCESS_TOKEN"))
	return cfg, nil
}

func loadAuthConfig(cfg *Config) error {
	baseURL, err := url.Parse(os.Getenv("APP_BASE_URL"))
	if err != nil || baseURL.Host == "" || baseURL.User != nil || baseURL.RawQuery != "" ||
		baseURL.Fragment != "" || baseURL.ForceQuery || (baseURL.Path != "" && baseURL.Path != "/") ||
		(baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return errors.New("APP_BASE_URL must be an HTTP(S) origin without credentials or path")
	}
	cfg.AppBaseURL = strings.TrimSuffix(baseURL.String(), "/")

	cfg.EmailFrom = strings.TrimSpace(os.Getenv("EMAIL_FROM"))
	address, err := mail.ParseAddress(cfg.EmailFrom)
	if err != nil || address.Address == "" || len(cfg.EmailFrom) > 320 {
		return errors.New("EMAIL_FROM must be a valid sender address")
	}
	cfg.ResendAPIKey = strings.TrimSpace(os.Getenv("RESEND_API_KEY"))
	if cfg.ResendAPIKey == "" {
		return errors.New("RESEND_API_KEY is required")
	}

	if cfg.EmailEncryptionKey, err = decodeAuthKey("EMAIL_ENCRYPTION_KEY"); err != nil {
		return err
	}
	if cfg.EmailHMACKey, err = decodeAuthKey("EMAIL_HMAC_KEY"); err != nil {
		return err
	}
	if cfg.CookieSecure, err = authBoolEnv("COOKIE_SECURE", true); err != nil {
		return err
	}
	if cfg.TrustProxy, err = authBoolEnv("TRUST_PROXY_HEADERS", false); err != nil {
		return err
	}
	return nil
}

func decodeAuthKey(name string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(os.Getenv(name))
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("%s must be a base64-encoded 32-byte key", name)
	}
	return key, nil
}

func authBoolEnv(name string, defaultValue bool) (bool, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return value, nil
}
