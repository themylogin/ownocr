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
	defaultSidecarURL     = "http://sidecar:8000"
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
	Dewarp         Endpoint
	OCR            Endpoint
	OpenRouter     OpenRouter
	SidecarURL     string
}

// OpenRouter names the metered API the Ultra rescan reads on.
//
// APIURL is empty for the API the vendor runs, which is what a deployment uses.
// An end to end run points it at a stand-in, so a test reads pages without
// spending money.
type OpenRouter struct {
	APIURL   string
	APIToken string
}

type S3 struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

// Endpoint names a RunPod serverless endpoint and the token that opens it.
type Endpoint struct {
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
		Dewarp: Endpoint{
			APIURL:   strings.TrimRight(e.require("DEWARP_API_URL"), "/"),
			APIToken: e.require("DEWARP_API_TOKEN"),
		},
		OCR: Endpoint{
			APIURL:   strings.TrimRight(e.require("PADDLEOCR_VL_API_URL"), "/"),
			APIToken: e.require("PADDLEOCR_VL_API_TOKEN"),
		},
		OpenRouter: OpenRouter{
			APIURL:   strings.TrimRight(e.orElse("OPENROUTER_API_URL", ""), "/"),
			APIToken: e.require("OPENROUTER_API_KEY"),
		},
		SidecarURL: strings.TrimRight(e.orElse("SIDECAR_URL", defaultSidecarURL), "/"),
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
