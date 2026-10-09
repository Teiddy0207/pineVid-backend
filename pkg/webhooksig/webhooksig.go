// Package webhooksig signs and verifies service-to-service webhooks
// (docs/message-contract.md §4.1):
//
//	X-Timestamp: <unix seconds>
//	X-Signature: hex(HMAC-SHA256(secret, "<timestamp>." + body))
//
// The transcode worker keeps its own copy of Sign (separate Go module); the
// two must stay byte-for-byte compatible.
package webhooksig

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"time"
)

const (
	HeaderTimestamp = "X-Timestamp"
	HeaderSignature = "X-Signature"
	// MaxSkew bounds how old (or how far in the future) a signed request
	// may be, so a captured request can't be replayed later.
	MaxSkew = 5 * time.Minute
)

var (
	ErrNoSecret     = errors.New("webhooksig: no secret configured")
	ErrMissing      = errors.New("webhooksig: missing timestamp or signature")
	ErrExpired      = errors.New("webhooksig: timestamp outside the allowed window")
	ErrBadSignature = errors.New("webhooksig: signature mismatch")
)

// Sign returns the hex signature for body signed at unix time ts.
func Sign(secret string, ts int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks the timestamp/signature headers against body at time now.
func Verify(secret, timestamp, signature string, body []byte, now time.Time) error {
	if secret == "" {
		return ErrNoSecret
	}
	if timestamp == "" || signature == "" {
		return ErrMissing
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrMissing
	}
	skew := now.Sub(time.Unix(ts, 0))
	if skew > MaxSkew || skew < -MaxSkew {
		return ErrExpired
	}

	want := Sign(secret, ts, body)
	if !hmac.Equal([]byte(want), []byte(signature)) {
		return ErrBadSignature
	}
	return nil
}
