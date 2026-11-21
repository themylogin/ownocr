package objectstore

import "testing"

func TestNewWithSettings(t *testing.T) {
	store := New(Settings{
		Endpoint:        "http://minio:9000",
		Region:          "us-east-1",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		Bucket:          "ownocr",
	})

	if store.Bucket() != "ownocr" {
		t.Errorf("Bucket = %q", store.Bucket())
	}
	if !store.client.Options().UsePathStyle {
		t.Error("a custom endpoint needs path style addressing")
	}
}

func TestNewWithoutEndpoint(t *testing.T) {
	store := New(Settings{
		Region:          "eu-central-1",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		Bucket:          "ownocr",
	})

	if store.client.Options().BaseEndpoint != nil {
		t.Error("an absent endpoint URL still set a base endpoint")
	}
}
