package entity

import (
	"errors"
	"time"
)

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

// RefreshToken is a server-side record of an issued refresh token. The raw
// token is only ever returned to the client once; TokenHash (hex SHA-256)
// is what's stored and looked up.
type RefreshToken struct {
	ID         string
	UserID     string
	TokenHash  string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *string
	CreatedAt  time.Time
}
