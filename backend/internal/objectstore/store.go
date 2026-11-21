package objectstore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Settings struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

type Store struct {
	client *s3.Client
	bucket string
}

func New(settings Settings) *Store {
	awsConfig := aws.Config{
		Region: settings.Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			settings.AccessKeyID, settings.SecretAccessKey, ""),
		// S3 compatible stores reject the checksum trailer that newer SDKs add
		// to every upload. Only send one where the API demands it.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
	}

	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		if settings.Endpoint != "" {
			o.BaseEndpoint = aws.String(settings.Endpoint)
			o.UsePathStyle = true
		}
	})

	return &Store{client: client, bucket: settings.Bucket}
}

func (s *Store) Bucket() string { return s.bucket }

func (s *Store) Put(ctx context.Context, key string, body io.ReadSeeker) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

func (s *Store) PutFile(ctx context.Context, key, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return s.Put(ctx, key, file)
}

func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	return out.Body, nil
}

// GetFile writes an object to a local path. The caller owns the file.
func (s *Store) GetFile(ctx context.Context, key, path string) error {
	body, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	defer body.Close()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, body); err != nil {
		return fmt.Errorf("download %s: %w", key, err)
	}
	return file.Close()
}

func (s *Store) List(ctx context.Context, prefix string) ([]string, error) {
	keys := make([]string, 0)
	pages := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", prefix, err)
		}
		for _, object := range page.Contents {
			if object.Key != nil {
				keys = append(keys, *object.Key)
			}
		}
	}

	return keys, nil
}

// DeletePrefix removes objects one by one. The batch call needs a Content-MD5
// header that S3 compatible stores disagree about, and one key per request
// works everywhere.
func (s *Store) DeletePrefix(ctx context.Context, prefix string) error {
	keys, err := s.List(ctx, prefix)
	if err != nil {
		return err
	}

	for _, key := range keys {
		_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return fmt.Errorf("delete %s: %w", key, err)
		}
	}

	return nil
}
