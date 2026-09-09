package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultWorkers        = 4
	defaultDataDir        = "/data"
	defaultMaxUploadBytes = 2 << 30
	defaultPDFDPI         = 300
	defaultRegion         = "us-east-1"
)

type Config struct {
	Addr           string
	DatabaseURL    string
	AllowedOrigins []string
	Workers        int
	DataDir        string
	MaxUploadBytes int64
	PDFDPI         int
	S3             S3
	Dewarp         Dewarp
}

type S3 struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

type Dewarp struct {
	APIURL   string
	APIToken string
}

func Load() (Config, error) {
	var e env

	cfg := Config{
		Addr:           e.orElse("ADDR", ":8080"),
		DatabaseURL:    e.require("DATABASE_URL"),
		AllowedOrigins: splitList(e.orElse("ALLOWED_ORIGINS", "*")),
		Workers:        e.positiveIntOrElse("WORKERS", defaultWorkers),
		DataDir:        e.orElse("DATA_DIR", defaultDataDir),
		MaxUploadBytes: e.positiveOrElse("MAX_UPLOAD_BYTES", defaultMaxUploadBytes),
		PDFDPI:         e.positiveIntOrElse("PDF_DPI", defaultPDFDPI),
		S3: S3{
			Endpoint:        e.orElse("AWS_ENDPOINT_URL", ""),
			Region:          e.orElse("AWS_REGION", defaultRegion),
			AccessKeyID:     e.require("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: e.require("AWS_SECRET_ACCESS_KEY"),
			Bucket:          e.require("AWS_BUCKET"),
		},
		Dewarp: Dewarp{
			APIURL:   strings.TrimRight(e.require("DEWARP_API_URL"), "/"),
			APIToken: e.require("DEWARP_API_TOKEN"),
		},
	}

	if err := e.err(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

type env struct {
	errs []error
}

func (e *env) require(key string) string {
	value := os.Getenv(key)
	if value == "" {
		e.errs = append(e.errs, fmt.Errorf("%s is not set", key))
	}
	return value
}

func (e *env) orElse(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func (e *env) positiveOrElse(key string, fallback int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 1 {
		e.errs = append(e.errs, fmt.Errorf("%s must be a positive integer, got %q", key, raw))
		return fallback
	}
	return value
}

func (e *env) positiveIntOrElse(key string, fallback int) int {
	return int(e.positiveOrElse(key, int64(fallback)))
}

func (e *env) err() error { return errors.Join(e.errs...) }

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	list := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			list = append(list, trimmed)
		}
	}
	return list
}
