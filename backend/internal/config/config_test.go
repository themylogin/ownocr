package config

import (
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	setStorageEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DataDir != defaultDataDir {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, defaultDataDir)
	}
	if cfg.MaxUploadBytes != defaultMaxUploadBytes {
		t.Errorf("MaxUploadBytes = %d, want %d", cfg.MaxUploadBytes, defaultMaxUploadBytes)
	}
	if cfg.PDFDPI != defaultPDFDPI {
		t.Errorf("PDFDPI = %d, want %d", cfg.PDFDPI, defaultPDFDPI)
	}
	if cfg.Workers != defaultWorkers {
		t.Errorf("Workers = %d, want %d", cfg.Workers, defaultWorkers)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	setStorageEnv(t)
	t.Setenv("DATA_DIR", "/srv/books")
	t.Setenv("MAX_UPLOAD_BYTES", "1048576")
	t.Setenv("PDF_DPI", "150")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DataDir != "/srv/books" {
		t.Errorf("DataDir = %q", cfg.DataDir)
	}
	if cfg.MaxUploadBytes != 1048576 {
		t.Errorf("MaxUploadBytes = %d", cfg.MaxUploadBytes)
	}
	if cfg.PDFDPI != 150 {
		t.Errorf("PDFDPI = %d", cfg.PDFDPI)
	}
}

func TestLoadRejects(t *testing.T) {
	cases := []struct{ key, value string }{
		{"MAX_UPLOAD_BYTES", "0"},
		{"MAX_UPLOAD_BYTES", "-1"},
		{"MAX_UPLOAD_BYTES", "big"},
		{"PDF_DPI", "0"},
		{"PDF_DPI", "-300"},
		{"PDF_DPI", "high"},
		{"WORKERS", "0"},
		{"WORKERS", "many"},
	}

	for _, c := range cases {
		t.Run(c.key+"="+c.value, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://localhost/test")
			setStorageEnv(t)
			t.Setenv(c.key, c.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load accepted %s=%q", c.key, c.value)
			}
			if !strings.Contains(err.Error(), c.key) {
				t.Errorf("error %q does not name %s", err, c.key)
			}
		})
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	setStorageEnv(t)
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load accepted a missing DATABASE_URL")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("error %q does not name the missing variable", err)
	}
}

func setStorageEnv(t *testing.T) {
	t.Helper()
	clearStorageEnv(t)
	t.Setenv("AWS_ENDPOINT_URL", "http://minio:9000")
	t.Setenv("AWS_ACCESS_KEY_ID", "key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_BUCKET", "ownocr")
	t.Setenv("DEWARP_API_URL", "https://api.runpod.ai/v2/abc/")
	t.Setenv("DEWARP_API_TOKEN", "token")
}

// Makes the test independent of the environment the test runner happens to hold.
// Compose passes the real variables to the container.
func clearStorageEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"AWS_ENDPOINT_URL", "AWS_REGION", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"AWS_BUCKET", "DEWARP_API_URL", "DEWARP_API_TOKEN", "SIDECAR_URL",
	} {
		t.Setenv(name, "")
	}
}

func TestS3Complete(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	setStorageEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.S3.Bucket != "ownocr" {
		t.Errorf("Bucket = %q", cfg.S3.Bucket)
	}
	if cfg.S3.Endpoint != "http://minio:9000" {
		t.Errorf("Endpoint = %q", cfg.S3.Endpoint)
	}
	if cfg.S3.Region != defaultRegion {
		t.Errorf("Region = %q, want %q", cfg.S3.Region, defaultRegion)
	}

	t.Setenv("AWS_REGION", "eu-central-1")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.S3.Region != "eu-central-1" {
		t.Errorf("Region = %q", cfg.S3.Region)
	}
}

func TestDewarpComplete(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	setStorageEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Dewarp.APIToken != "token" {
		t.Errorf("APIToken = %q", cfg.Dewarp.APIToken)
	}
	if cfg.Dewarp.APIURL != "https://api.runpod.ai/v2/abc" {
		t.Errorf("APIURL = %q, want the trailing slash removed", cfg.Dewarp.APIURL)
	}
}

func TestSidecarURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	setStorageEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SidecarURL != defaultSidecarURL {
		t.Errorf("SidecarURL = %q, want %q", cfg.SidecarURL, defaultSidecarURL)
	}

	t.Setenv("SIDECAR_URL", "http://elsewhere:9000/")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SidecarURL != "http://elsewhere:9000" {
		t.Errorf("SidecarURL = %q, want the trailing slash removed", cfg.SidecarURL)
	}
}

func TestLoadReportsEveryProblem(t *testing.T) {
	setStorageEnv(t)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("AWS_BUCKET", "")
	t.Setenv("PDF_DPI", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("Load accepted three bad variables")
	}
	for _, name := range []string{"DATABASE_URL", "AWS_BUCKET", "PDF_DPI"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}

func TestLoadRequiresEveryStorageVariable(t *testing.T) {
	for _, name := range []string{
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_BUCKET",
		"DEWARP_API_URL", "DEWARP_API_TOKEN",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://localhost/test")
			setStorageEnv(t)
			t.Setenv(name, "")

			_, err := Load()
			if err == nil {
				t.Fatalf("Load accepted an empty %s", name)
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error %q does not name the missing variable", err)
			}
		})
	}
}

func TestS3EndpointAndRegionStayOptional(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	setStorageEnv(t)
	t.Setenv("AWS_ENDPOINT_URL", "")
	t.Setenv("AWS_REGION", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.S3.Endpoint != "" {
		t.Errorf("Endpoint = %q, want empty for real AWS", cfg.S3.Endpoint)
	}
	if cfg.S3.Region != defaultRegion {
		t.Errorf("Region = %q, want %q", cfg.S3.Region, defaultRegion)
	}
}
