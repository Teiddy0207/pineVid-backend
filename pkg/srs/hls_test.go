package srs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMasterFiltersStaleMissingAndDuplicateRenditions(t *testing.T) {
	now := time.Now()
	started := now.Add(-time.Minute)
	for _, tc := range []struct {
		name           string
		stale, missing bool
		want           int
	}{
		{"ready", false, false, 2}, {"stale", true, false, 1}, {"missing segment", false, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/api/v1/streams/":
					fmt.Fprint(w, `{"code":0,"streams":[{"app":"live","name":"key","publish":{"active":true},"video":{"width":1280,"height":720}},{"app":"abr","name":"key_720","publish":{"active":true},"video":{"width":1280,"height":720}},{"app":"abr","name":"key_480","publish":{"active":true},"video":{"width":854,"height":480}}]}`)
				case strings.HasSuffix(r.URL.Path, ".m3u8"):
					stamp := now.UnixMilli()
					if tc.stale && strings.Contains(r.URL.Path, "480") {
						stamp = now.Add(-time.Hour).UnixMilli()
					}
					fmt.Fprintf(w, "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\nseg-%d-1.ts\n", stamp)
				case strings.HasSuffix(r.URL.Path, ".ts"):
					if tc.missing && strings.HasPrefix(r.URL.Path, "/abr/") {
						w.WriteHeader(404)
					} else {
						w.WriteHeader(200)
					}
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			c := New(server.URL)
			c.ConfigureHLS(server.URL, "https://site.example/srs")
			master, err := c.Master(context.Background(), "key", &started)
			if err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(master, "#EXT-X-STREAM-INF:"); n != tc.want {
				t.Fatalf("want %d variants, got %s", tc.want, master)
			}
			if strings.Contains(master, server.URL) || !strings.Contains(master, "https://site.example/srs/live/key.m3u8") {
				t.Fatal(master)
			}
		})
	}
}

func TestMasterRejectsOfflineAndUnsafeKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"code":0,"streams":[]}`) }))
	defer server.Close()
	c := New(server.URL)
	c.ConfigureHLS(server.URL, server.URL)
	for _, key := range []string{"key", "../secret", "key?x=1"} {
		if _, err := c.Master(context.Background(), key, nil); err == nil {
			t.Fatalf("accepted %q", key)
		}
	}
}

// Optional test against the isolated SRS smoke-test container.
func TestLiveIntegration(t *testing.T) {
	api := os.Getenv("TEST_LIVE_SRS_API")
	if api == "" {
		t.Skip("TEST_LIVE_SRS_API not set")
	}
	media := os.Getenv("TEST_LIVE_SRS_HTTP")
	key := os.Getenv("TEST_LIVE_SRS_KEY")
	c := New(api)
	c.ConfigureHLS(media, media)
	master, err := c.Master(context.Background(), key, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, resolution := range []string{"1920x1080", "1280x720", "854x480"} {
		if !strings.Contains(master, resolution) {
			t.Fatalf("missing %s: %s", resolution, master)
		}
	}
	t.Log(master)
}
