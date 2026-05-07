// Package blobstore provides a small abstraction over an S3-compatible object
// store (RustFS / MinIO) for storing user-facing download blobs.
package blobstore

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
)

// ErrNotFound is returned when a requested blob does not exist.
var ErrNotFound = errors.New("blob not found")

type Client interface {
	// EnsureBucket creates the configured bucket if it does not already exist
	// and applies a lifecycle rule that expires objects after the configured
	// number of days. Idempotent.
	EnsureBucket(ctx context.Context) error
	// Upload stores the given reader's contents under key. filename is
	// preserved as user metadata for later retrieval at download time.
	Upload(ctx context.Context, key string, reader io.Reader, size int64, filename, contentType string) error
	// GetObject opens the object identified by key. The caller must Close the
	// returned ReadCloser. Returns ErrNotFound if the object does not exist.
	GetObject(ctx context.Context, key string) (rc io.ReadCloser, length int64, contentType, filename string, err error)
}

type MinioClient struct {
	client *minio.Client
	bucket string
	ttl    int
}

const (
	filenameMetaKey = "filename"
	lifecycleRuleID = "expire-old-blobs"
)

// NewMinioClient constructs a MinioClient from the given RustFS config.
func NewMinioClient(cfg *config.RustFSConfig) (*MinioClient, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	return &MinioClient{
		client: client,
		bucket: cfg.WorldDownloadsBucket,
		ttl:    cfg.BlobTTLDays,
	}, nil
}

func (c *MinioClient) EnsureBucket(ctx context.Context) error {
	exists, err := c.client.BucketExists(ctx, c.bucket)
	if err != nil {
		return err
	}

	if !exists {
		if err := c.client.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}

	lc := lifecycle.NewConfiguration()
	lc.Rules = []lifecycle.Rule{
		{
			ID:     lifecycleRuleID,
			Status: "Enabled",
			Expiration: lifecycle.Expiration{
				Days: lifecycle.ExpirationDays(c.ttl),
			},
		},
	}

	return c.client.SetBucketLifecycle(ctx, c.bucket, lc)
}

func (c *MinioClient) Upload(ctx context.Context, key string, reader io.Reader, size int64, filename, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := c.client.PutObject(ctx, c.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
		UserMetadata: map[string]string{
			filenameMetaKey: url.QueryEscape(filename),
		},
	})

	return err
}

func (c *MinioClient) GetObject(ctx context.Context, key string) (io.ReadCloser, int64, string, string, error) {
	obj, err := c.client.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, "", "", err
	}

	stat, err := obj.Stat()
	if err != nil {
		_ = obj.Close()

		errResp := minio.ToErrorResponse(err)
		if errResp.StatusCode == http.StatusNotFound || errResp.Code == "NoSuchKey" {
			return nil, 0, "", "", ErrNotFound
		}

		return nil, 0, "", "", err
	}

	return obj, stat.Size, stat.ContentType, decodeFilenameMeta(stat.UserMetadata), nil
}

// decodeFilenameMeta retrieves the URL-encoded filename metadata in a
// case-insensitive manner — different S3-compatible servers normalize header
// case differently.
func decodeFilenameMeta(meta map[string]string) string {
	for k, v := range meta {
		if http.CanonicalHeaderKey(k) == http.CanonicalHeaderKey(filenameMetaKey) {
			if decoded, err := url.QueryUnescape(v); err == nil {
				return decoded
			}

			return v
		}
	}

	return ""
}
