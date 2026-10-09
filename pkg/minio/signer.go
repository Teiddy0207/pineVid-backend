package minio

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Signer hands out presigned PUT URLs that the BROWSER uses to upload straight
// to the object store. The signature covers the host, so the signer is built
// for the address the browser reaches (MINIO_PUBLIC_URL), not the in-cluster
// one the backend itself uses. Signing is offline: the fixed region means
// minio-go never contacts the server to look it up.
type Signer struct {
	mc *minio.Client
}

// NewSigner builds a signer for publicURL (e.g. https://media.example.com or
// http://203.0.113.7:9000).
func NewSigner(publicURL, accessKey, secretKey string) (*Signer, error) {
	u, err := url.Parse(publicURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("minio signer: invalid public URL %q", publicURL)
	}
	if u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("minio signer: public URL %q must not have a path prefix (the signature covers the path)", publicURL)
	}
	mc, err := minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: u.Scheme == "https",
		Region: "us-east-1",
	})
	if err != nil {
		return nil, fmt.Errorf("minio signer: %w", err)
	}
	return &Signer{mc: mc}, nil
}

// PresignPut returns a URL that accepts one PUT of bucket/objectKey until expiry.
func (s *Signer) PresignPut(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
	u, err := s.mc.PresignedPutObject(ctx, bucket, objectKey, expiry)
	if err != nil {
		return "", fmt.Errorf("minio signer - PresignedPutObject: %w", err)
	}
	return u.String(), nil
}
