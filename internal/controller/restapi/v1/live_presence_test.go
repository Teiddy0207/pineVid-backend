package v1

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

type presenceService struct {
	usecase.Livestream
	identity string
	action   string
	err      error
}

func (s *presenceService) ViewerPresence(_ context.Context, id, action, tab, identity, broadcast string) (response.LiveViewersResponse, error) {
	s.identity = identity
	s.action = action
	return response.LiveViewersResponse{StreamID: id, ViewersCount: 1, Broadcast: "current"}, s.err
}
func TestHeartbeatUsesAuthenticatedIdentity(t *testing.T) {
	svc := &presenceService{}
	r := &V1{ls: svc}
	app := fiber.New()
	app.Post("/:id", func(c *fiber.Ctx) error { c.Locals("userID", "real-account"); return c.Next() }, r.liveHeartbeat)
	body := `{"tab_id":"550e8400-e29b-41d4-a716-446655440000","viewer_id":"attacker-controlled"}`
	req := httptest.NewRequest("POST", "/live", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, 200, res.StatusCode)
	require.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte("user:real-account"))), svc.identity)
	var out response.LiveViewersResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
	require.EqualValues(t, 1, out.ViewersCount)
}
func TestGuestValidationAndBannedRoom(t *testing.T) {
	svc := &presenceService{}
	r := &V1{ls: svc}
	app := fiber.New()
	app.Post("/:id", r.liveHeartbeat)
	app.Delete("/:id", r.liveLeave)
	for _, tc := range []struct {
		method, body string
		status       int
	}{
		{"POST", `{"tab_id":"bad","viewer_id":"bad"}`, 400},
		{"POST", `{"tab_id":"550e8400-e29b-41d4-a716-446655440000","viewer_id":"bad"}`, 400},
		{"DELETE", `{"tab_id":"550e8400-e29b-41d4-a716-446655440000","viewer_id":"550e8400-e29b-41d4-a716-446655440001"}`, 400},
	} {
		req := httptest.NewRequest(tc.method, "/live", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		require.NoError(t, err)
		res.Body.Close()
		require.Equal(t, tc.status, res.StatusCode)
	}
	svc.err = entity.ErrStreamBanned
	req := httptest.NewRequest("POST", "/live", strings.NewReader(`{"tab_id":"550e8400-e29b-41d4-a716-446655440000","viewer_id":"550e8400-e29b-41d4-a716-446655440001"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	require.NoError(t, err)
	res.Body.Close()
	require.Equal(t, 403, res.StatusCode)
}
