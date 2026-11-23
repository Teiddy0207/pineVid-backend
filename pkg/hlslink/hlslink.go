// Package hlslink turns the stored path of a VOD's HLS output
// (/hls-streams/videos/<id>/master.m3u8) into an expiring signed path that
// nginx's secure_link module checks before proxying to MinIO:
//
//	/s/<token>/<expires>/<id>/master.m3u8
//	token = base64url(md5("<expires><id> <secret>")), no padding
//
// The matching nginx location (docs/DEPLOY_PROD_VPS.md) uses
//
//	secure_link     $tok,$exp;
//	secure_link_md5 "$exp$vid $hls_secret";
//
// The token covers the video id, not the file, so the relative URLs inside
// master.m3u8 (variant playlists, segments) keep working under the same
// prefix. Direct /hls-streams/... requests are refused by nginx, so a private
// or deleted video's media can only be fetched with a link the API handed out
// while the caller was allowed to watch it, and only until it expires.
package hlslink

import (
	"crypto/md5" //nolint:gosec // nginx secure_link only supports MD5
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

// StoredPrefix is how the transcode pipeline stores VOD HLS paths.
const StoredPrefix = "/hls-streams/videos/"

// Signer signs stored HLS paths. The zero value is unusable; use New.
type Signer struct {
	secret string
	ttl    time.Duration
	now    func() time.Time
}

// New returns a Signer. ttl is the minimum lifetime of a link.
func New(secret string, ttl time.Duration) *Signer {
	return &Signer{secret: secret, ttl: ttl, now: time.Now}
}

// WithClock is for tests.
func (s *Signer) WithClock(now func() time.Time) *Signer {
	s.now = now
	return s
}

// Sign rewrites a stored VOD HLS path into a signed one. Anything else (live
// paths, absolute URLs, empty strings, already signed paths) is returned
// unchanged, so Sign is safe to apply to every hls_url the API returns.
//
// The expiry is rounded up to the next full hour before adding ttl, so every
// response within the same hour carries the same URL (stable for caches and
// for clients comparing URLs), and a link is valid for between ttl and
// ttl+1h.
func (s *Signer) Sign(path string) string {
	if !strings.HasPrefix(path, StoredPrefix) {
		return path
	}
	rest := strings.TrimPrefix(path, StoredPrefix)
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 {
		return path
	}
	videoID, file := rest[:slash], rest[slash+1:]

	hour := s.now().Unix()/3600 + 1
	exp := hour*3600 + int64(s.ttl/time.Second)
	return "/s/" + Token(s.secret, exp, videoID) + "/" + strconv.FormatInt(exp, 10) + "/" + videoID + "/" + file
}

// Token is the nginx secure_link_md5 token for videoID valid until exp.
func Token(secret string, exp int64, videoID string) string {
	sum := md5.Sum([]byte(strconv.FormatInt(exp, 10) + videoID + " " + secret)) //nolint:gosec // see package doc
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
