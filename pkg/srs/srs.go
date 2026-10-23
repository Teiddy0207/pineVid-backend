// Package srs is a thin client for SRS's own read-only HTTP management API
// (http_api, distinct from the http_hooks callbacks SRS calls into us).
// It exists purely so the backend can ask SRS "what is actually being
// published right now?" instead of relying solely on the on_publish/
// on_unpublish webhooks ever arriving — see ReconcileLiveStreams in
// internal/usecase/livestream/livestream.go for why that matters.
package srs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	hlsURL       string
	publicHLSURL string
	baseURL      string
	httpClient   *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }},
	}
}

type streamsResponse struct {
	Code    int      `json:"code"`
	Streams []stream `json:"streams"`
}

type stream struct {
	App   string `json:"app"`
	Video *struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"video"`
	Name    string  `json:"name"` // the stream key, e.g. "sk_live_..."
	Publish publish `json:"publish"`
}

type publish struct {
	Active bool   `json:"active"`
	CID    string `json:"cid"` // SRS client ID of the publishing connection
}

func (c *Client) listStreams(ctx context.Context) ([]stream, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/streams/", nil)
	if err != nil {
		return nil, fmt.Errorf("NewRequestWithContext: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var body streamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("Decode: %w", err)
	}
	if body.Code != 0 {
		return nil, fmt.Errorf("SRS API error: %d", body.Code)
	}
	return body.Streams, nil
}

// KickPublisher disconnects the source and any active ABR outputs of streamKey
// (SRS DELETE /api/v1/clients/{cid}). Not publishing is not an error.
func (c *Client) KickPublisher(ctx context.Context, streamKey string) error {
	streams, err := c.listStreams(ctx)
	if err != nil {
		return fmt.Errorf("srs.KickPublisher - %w", err)
	}

	for _, s := range streams {
		if (s.Name != streamKey && s.Name != streamKey+"_720" && s.Name != streamKey+"_480") || !s.Publish.Active || s.Publish.CID == "" {
			continue
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/v1/clients/"+s.Publish.CID, nil)
		if err != nil {
			return fmt.Errorf("srs.KickPublisher - NewRequestWithContext: %w", err)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("srs.KickPublisher - Do: %w", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("srs.KickPublisher - unexpected status %d", resp.StatusCode)
		}
	}
	return nil
}

// ActiveStreamKeys returns the set of stream keys SRS is currently actually
// receiving RTMP data for (publish.active == true), keyed for O(1) lookup.
func (c *Client) ActiveStreamKeys(ctx context.Context) (map[string]bool, error) {
	streams, err := c.listStreams(ctx)
	if err != nil {
		return nil, fmt.Errorf("srs.ActiveStreamKeys - %w", err)
	}

	keys := make(map[string]bool, len(streams))
	for _, s := range streams {
		if s.Publish.Active && (s.App == "live" || s.App == "") {
			keys[s.Name] = true
		}
	}
	return keys, nil
}
