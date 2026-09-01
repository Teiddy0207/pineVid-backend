package integration_test

import (
	"bytes"
	"net/http"
	"testing"
)

// TestAdminEndpoints_RejectRegularUser is the regression test for the RBAC
// gap found in BUSINESS_REQUIREMENTS.md review: previously any authenticated
// user — not just admins — could call /v1/admin/* (ban users, view the
// dashboard...). RequireAdmin must reject a normal, freshly-registered user.
func TestAdminEndpoints_RejectRegularUser(t *testing.T) {
	token := registerAndLogin(t)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"dashboard", http.MethodGet, basePathV1 + "/admin/dashboard"},
		{"workers", http.MethodGet, basePathV1 + "/admin/workers"},
		{"list users", http.MethodGet, basePathV1 + "/admin/users"},
		{"ban user", http.MethodPost, basePathV1 + "/admin/users/some-id/ban"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := doAuthenticatedRequest(t.Context(), tc.method, tc.path, http.NoBody, token)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("expected 403 for non-admin caller, got %d", resp.StatusCode)
			}
		})
	}
}

// TestAdminEndpoints_RejectAnonymousCaller confirms the admin group still
// requires a valid token at all (RequireAdmin runs after Auth, so a missing
// token must be rejected before the role check even runs).
func TestAdminEndpoints_RejectAnonymousCaller(t *testing.T) {
	resp, err := http.Get(basePathV1 + "/admin/dashboard") //nolint:noctx // simple anonymous GET is fine for this check
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous caller, got %d", resp.StatusCode)
	}
}

// TestVideoLikeComment_RequireAuth is the regression test for the
// "mock-user-123" bug: liking or commenting used to silently fall back to a
// shared fake identity for anyone without a token. Now both routes must
// reject unauthenticated callers outright instead of degrading like that.
func TestVideoLikeComment_RequireAuth(t *testing.T) {
	const fakeVideoID = "00000000-0000-0000-0000-000000000000"

	t.Run("like without token", func(t *testing.T) {
		resp, err := http.Post(basePathV1+"/videos/"+fakeVideoID+"/like", "application/json", http.NoBody) //nolint:noctx
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 for anonymous like, got %d", resp.StatusCode)
		}
	})

	t.Run("comment without token", func(t *testing.T) {
		resp, err := http.Post( //nolint:noctx
			basePathV1+"/videos/"+fakeVideoID+"/comments",
			"application/json",
			bytes.NewBufferString(`{"content":"anonymous comment"}`),
		)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 for anonymous comment, got %d", resp.StatusCode)
		}
	})

	t.Run("like with token is accepted by auth layer", func(t *testing.T) {
		token := registerAndLogin(t)

		resp, err := doAuthenticatedRequest(t.Context(), http.MethodPost, basePathV1+"/videos/"+fakeVideoID+"/like", http.NoBody, token)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// The video doesn't exist, so this isn't expected to succeed — the
		// point is that it must get past authentication (not 401) and fail
		// for a video-not-found reason instead.
		if resp.StatusCode == http.StatusUnauthorized {
			t.Fatalf("expected authenticated caller to pass the auth gate, got 401")
		}
	})
}
