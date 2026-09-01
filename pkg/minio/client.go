// Package minio wraps the minio-go SDK for the one thing the backend itself
// needs to upload directly (as opposed to handing the browser a presigned
// URL): pushing a livestream's DVR recording into the raw-videos bucket so it
// can flow through the exact same transcode pipeline as a normal upload.
package minio

import (
	"context"
	"fmt"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	mc *minio.Client
}

func New(endpoint, accessKey, secretKey string, useSSL bool) (*Client, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio.New: %w", err)
	}
	return &Client{mc: mc}, nil
}

// UploadFile streams localPath's contents into bucket/objectKey, creating the
// bucket first if it doesn't already exist.
func (c *Client) UploadFile(ctx context.Context, bucket, objectKey, localPath string) error {
	exists, err := c.mc.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("MinioClient - UploadFile - BucketExists: %w", err)
	}
	if !exists {
		if err := c.mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("MinioClient - UploadFile - MakeBucket: %w", err)
		}
	}

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("MinioClient - UploadFile - Open: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("MinioClient - UploadFile - Stat: %w", err)
	}

	_, err = c.mc.PutObject(ctx, bucket, objectKey, f, info.Size(), minio.PutObjectOptions{
		ContentType: "video/x-flv",
	})
	if err != nil {
		return fmt.Errorf("MinioClient - UploadFile - PutObject: %w", err)
	}

	return nil
}
