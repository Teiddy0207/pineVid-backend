package hlslink_test

import (
	"crypto/md5" //nolint:gosec // matches nginx secure_link
	"encoding/base64"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/pkg/hlslink"
	"github.com/stretchr/testify/assert"
)

func TestSign(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_800_000_100, 0) // 100 s into an hour
	s := hlslink.New("k3y", 6*time.Hour).WithClock(func() time.Time { return now })

	const vid = "38c9efb4-5c96-4cf5-b1b2-e510e37c480f"
	got := s.Sign("/hls-streams/videos/" + vid + "/master.m3u8")

	exp := int64(1_800_000_000 + 3600 + 6*3600) // next full hour + ttl
	sum := md5.Sum([]byte("1800025200" + vid + " k3y")) //nolint:gosec
	want := "/s/" + base64.RawURLEncoding.EncodeToString(sum[:]) + "/1800025200/" + vid + "/master.m3u8"
	assert.Equal(t, int64(1800025200), exp)
	assert.Equal(t, want, got)

	// Same hour → same URL; the token does not depend on the file name.
	now = time.Unix(1_800_003_500, 0)
	assert.Equal(t, got, s.Sign("/hls-streams/videos/"+vid+"/master.m3u8"))
	assert.Equal(t, hlslink.Token("k3y", exp, vid), hlslink.Token("k3y", exp, vid))
	assert.NotEqual(t, hlslink.Token("k3y", exp, vid), hlslink.Token("other", exp, vid))

	// Not a stored VOD path → unchanged.
	for _, p := range []string{"", "/live/abc.m3u8", "/v1/live/streams/x/master.m3u8", "https://cdn/x.m3u8", got, "/hls-streams/videos/master.m3u8"} {
		assert.Equal(t, p, s.Sign(p), p)
	}
}
