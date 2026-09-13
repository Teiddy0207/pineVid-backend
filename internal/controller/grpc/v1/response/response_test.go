package response_test

import (
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/grpc/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type userResponseFields struct {
	id        string
	username  string
	email     string
	createdAt string
	updatedAt string
}

type userResponseGetter interface {
	GetId() string
	GetUsername() string
	GetEmail() string
	GetCreatedAt() string
	GetUpdatedAt() string
}

func assertUserResponseFields(t *testing.T, f *userResponseFields, got userResponseGetter) {
	t.Helper()

	require.NotNil(t, got)
	assert.Equal(t, f.id, got.GetId())
	assert.Equal(t, f.username, got.GetUsername())
	assert.Equal(t, f.email, got.GetEmail())
	assert.Equal(t, f.createdAt, got.GetCreatedAt())
	assert.Equal(t, f.updatedAt, got.GetUpdatedAt())
}

func TestNewRegisterResponse(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	user := &entity.User{
		ID:        "user-id-123",
		Username:  "testuser",
		Email:     "test@example.com",
		CreatedAt: now,
		UpdatedAt: now,
	}

	resp := response.NewRegisterResponse(user)

	assertUserResponseFields(t, &userResponseFields{
		id:        user.ID,
		username:  user.Username,
		email:     user.Email,
		createdAt: "2026-01-01T00:00:00Z",
		updatedAt: "2026-01-01T00:00:00Z",
	}, resp)
}

func TestNewGetProfileResponse(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 12, 30, 0, 0, time.UTC)
	user := &entity.User{
		ID:        "user-id-456",
		Username:  "anotheruser",
		Email:     "another@example.com",
		CreatedAt: now,
		UpdatedAt: now,
	}

	resp := response.NewGetProfileResponse(user)

	assertUserResponseFields(t, &userResponseFields{
		id:        user.ID,
		username:  user.Username,
		email:     user.Email,
		createdAt: "2026-03-15T12:30:00Z",
		updatedAt: "2026-03-15T12:30:00Z",
	}, resp)
}
