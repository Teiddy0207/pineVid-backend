package webhooksig_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/pkg/webhooksig"
	"github.com/stretchr/testify/assert"
)

func TestSignVerify(t *testing.T) {
	t.Parallel()

	const secret = "s3cret"
	body := []byte(`{"job_id":"j1","status":"COMPLETED"}`)
	now := time.Unix(1_700_000_000, 0)
	ts := strconv.FormatInt(now.Unix(), 10)
	sig := webhooksig.Sign(secret, now.Unix(), body)

	assert.NoError(t, webhooksig.Verify(secret, ts, sig, body, now))
	assert.NoError(t, webhooksig.Verify(secret, ts, sig, body, now.Add(4*time.Minute)))

	tests := []struct {
		name      string
		secret    string
		ts, sig   string
		body      []byte
		now       time.Time
		wantError error
	}{
		{"tampered body", secret, ts, sig, []byte(`{"job_id":"j1","status":"FAILED"}`), now, webhooksig.ErrBadSignature},
		{"wrong secret", "other", ts, sig, body, now, webhooksig.ErrBadSignature},
		{"replayed too late", secret, ts, sig, body, now.Add(6 * time.Minute), webhooksig.ErrExpired},
		{"from the future", secret, ts, sig, body, now.Add(-6 * time.Minute), webhooksig.ErrExpired},
		{"missing signature", secret, ts, "", body, now, webhooksig.ErrMissing},
		{"garbage timestamp", secret, "abc", sig, body, now, webhooksig.ErrMissing},
		{"no secret configured", "", ts, sig, body, now, webhooksig.ErrNoSecret},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.ErrorIs(t, webhooksig.Verify(tc.secret, tc.ts, tc.sig, tc.body, tc.now), tc.wantError)
		})
	}
}

// Pins the wire format so the worker's copy of Sign can be checked against it.
// Reference value: printf '1700000000.{}' | openssl dgst -sha256 -hmac secret
func TestSignKnownVector(t *testing.T) {
	t.Parallel()
	assert.Equal(t,
		"b8569b78799ff9e3cbff0fc2d63a33a2b57f3282abd07c37ae5e8e7d79a5f163",
		webhooksig.Sign("secret", 1700000000, []byte("{}")),
	)
}
