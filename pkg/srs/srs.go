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
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

type streamsResponse struct {
	Code    int      `json:"code"`
	Streams []stream `json:"streams"`
}

type stream struct {
	Name    string  `json:"name"` // the stream key, e.g. "sk_live_..."
	Publish publish `json:"publish"`
}

type publish struct {
	Active bool `json:"active"`
}

// ActiveStreamKeys returns the set of stream keys SRS is currently actually
// receiving RTMP data for (publish.active == true), keyed for O(1) lookup.
func (c *Client) ActiveStreamKeys(ctx context.Context) (map[string]bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/streams/", nil)
	if err != nil {
		return nil, fmt.Errorf("srs.ActiveStreamKeys - NewRequestWithContext: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("srs.ActiveStreamKeys - Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("srs.ActiveStreamKeys - unexpected status %d", resp.StatusCode)
	}

	var body streamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("srs.ActiveStreamKeys - Decode: %w", err)
	}

	keys := make(map[string]bool, len(body.Streams))
	for _, s := range body.Streams {
		if s.Publish.Active {
			keys[s.Name] = true
		}
	}
	return keys, nil
}
