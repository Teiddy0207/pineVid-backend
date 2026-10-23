package srs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrHLSNotReady = errors.New("live HLS is not ready")
var safeStreamKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// ConfigureHLS separates private probe URLs from browser-visible media URLs.
func (c *Client) ConfigureHLS(privateURL, publicURL string) {
	c.hlsURL = strings.TrimRight(privateURL, "/")
	c.publicHLSURL = strings.TrimRight(publicURL, "/")
}

type variant struct {
	path                     string
	width, height, bandwidth int
}

// Master only advertises active, fresh renditions whose latest segment exists.
// Requests are bounded and parallel: an unavailable encoder cannot stall playback.
func (c *Client) Master(ctx context.Context, key string, startedAt *time.Time) (string, error) {
	if !safeStreamKey.MatchString(key) {
		return "", ErrHLSNotReady
	}
	streams, err := c.listStreams(ctx)
	if err != nil {
		return "", err
	}
	var source *stream
	for i := range streams {
		s := &streams[i]
		if s.App == "live" && s.Name == key && s.Publish.Active {
			source = s
			break
		}
	}
	if source == nil {
		return "", ErrHLSNotReady
	}
	candidates := []variant{}
	for _, s := range streams {
		if !s.Publish.Active || s.Video == nil || s.Video.Width <= 0 || s.Video.Height <= 0 {
			continue
		}
		if s.App == "live" && s.Name == key {
			candidates = append(candidates, variant{"live/" + key + ".m3u8", s.Video.Width, s.Video.Height, 8000000})
		}
		if s.App == "abr" && (s.Name == key+"_720" || s.Name == key+"_480") {
			bandwidth := 2800000
			if strings.HasSuffix(s.Name, "_480") {
				bandwidth = 1400000
			}
			candidates = append(candidates, variant{"abr/" + s.Name + ".m3u8", s.Video.Width, s.Video.Height, bandwidth})
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	var mu sync.Mutex
	var ready []variant
	var wg sync.WaitGroup
	for _, v := range candidates {
		wg.Add(1)
		go func(v variant) {
			defer wg.Done()
			if c.ready(ctx, v.path, startedAt) {
				mu.Lock()
				ready = append(ready, v)
				mu.Unlock()
			}
		}(v)
	}
	wg.Wait()
	if len(ready) == 0 {
		return "", ErrHLSNotReady
	}
	sort.Slice(ready, func(i, j int) bool {
		if ready[i].height == ready[j].height {
			return ready[i].bandwidth > ready[j].bandwidth
		}
		return ready[i].height > ready[j].height
	})
	var out strings.Builder
	out.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	seen := map[int]bool{}
	for _, v := range ready {
		if seen[v.height] {
			continue
		}
		seen[v.height] = true
		fmt.Fprintf(&out, "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n%s/%s\n", v.bandwidth, v.width, v.height, c.publicHLSURL, v.path)
	}
	return out.String(), nil
}

func (c *Client) ready(ctx context.Context, path string, startedAt *time.Time) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.hlsURL+"/"+path, nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if err != nil {
		return false
	}
	text := string(body)
	if !strings.HasPrefix(text, "#EXTM3U") || strings.Contains(text, "#EXT-X-ENDLIST") {
		return false
	}
	var segment string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			segment = line
		}
	}
	if segment == "" {
		return false
	}
	// SRS does not emit Last-Modified. Timestamped segment names are created
	// by our HLS config; require a fresh segment from this broadcast.
	parts := strings.Split(strings.TrimSuffix(strings.Split(segment, "?")[0], ".ts"), "-")
	if len(parts) < 3 {
		return false
	}
	stamp, err := strconv.ParseInt(parts[len(parts)-2], 10, 64)
	if err != nil {
		return false
	}
	created := time.UnixMilli(stamp)
	if time.Since(created) > 20*time.Second || time.Until(created) > 5*time.Second || (startedAt != nil && created.Before(*startedAt)) {
		return false
	}

	base, _ := url.Parse(c.hlsURL + "/" + path)
	ref, err := url.Parse(segment)
	if err != nil {
		return false
	}
	target := base.ResolveReference(ref)
	// Never follow an arbitrary origin supplied by a playlist.
	if target.Host != base.Host || target.Scheme != base.Scheme {
		return false
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodHead, target.String(), nil)
	if err != nil {
		return false
	}
	seg, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	seg.Body.Close()
	return seg.StatusCode == 200
}
