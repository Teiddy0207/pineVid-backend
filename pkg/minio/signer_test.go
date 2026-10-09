package minio_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	pkgminio "github.com/evrone/go-clean-template/pkg/minio"
	"github.com/stretchr/testify/require"
)

func TestSigner_PresignPut(t *testing.T) {
	t.Parallel()

	s, err := pkgminio.NewSigner("http://203.0.113.7:9000", "access", "secret")
	require.NoError(t, err)

	raw, err := s.PresignPut(context.Background(), "raw-videos", "raw-uploads/abc/raw.mp4", 2*time.Hour)
	require.NoError(t, err)

	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "203.0.113.7:9000", u.Host, "signature must be for the address the browser uses")
	require.Equal(t, "/raw-videos/raw-uploads/abc/raw.mp4", u.Path)
	require.Equal(t, "7200", u.Query().Get("X-Amz-Expires"))
	require.NotEmpty(t, u.Query().Get("X-Amz-Signature"))
}

func TestNewSigner_RejectsBadPublicURL(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{"", "localhost:9000", "ftp://x", "https://x/prefix"} {
		_, err := pkgminio.NewSigner(bad, "a", "b")
		require.Error(t, err, bad)
	}
}
