package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/internal/usecase/user"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

var errInternalServErr = errors.New("internal server error")

func newUserUseCase(t *testing.T) (usecase.User, *MockUserRepo) {
	t.Helper()

	uc, repo, _ := newUserUseCaseWithTokens(t)

	return uc, repo
}

func newUserUseCaseWithTokens(t *testing.T) (usecase.User, *MockUserRepo, *MockRefreshTokenRepo) {
	t.Helper()

	ctrl := gomock.NewController(t)

	repo := NewMockUserRepo(ctrl)
	tokens := NewMockRefreshTokenRepo(ctrl)
	jwtManager := jwt.New("test-secret", time.Hour)
	useCase := user.New(repo, tokens, jwtManager, 7*24*time.Hour)

	return useCase, repo, tokens
}

func TestRegister(t *testing.T) {
	t.Parallel()

	t.Run("register success", func(t *testing.T) {
		t.Parallel()

		uc, repo := newUserUseCase(t)
		repo.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)

		u, err := uc.Register(context.Background(), "testuser", "test@example.com", "password123")

		require.NoError(t, err)
		assert.NotEmpty(t, u.ID)
		assert.Equal(t, "testuser", u.Username)
		assert.Equal(t, "test@example.com", u.Email)
	})

	t.Run("register duplicate", func(t *testing.T) {
		t.Parallel()

		uc, repo := newUserUseCase(t)
		repo.EXPECT().Store(gomock.Any(), gomock.Any()).Return(entity.ErrUserAlreadyExists)

		_, err := uc.Register(context.Background(), "testuser", "test@example.com", "password123")

		require.ErrorIs(t, err, entity.ErrUserAlreadyExists)
	})
}

func TestLogin(t *testing.T) {
	t.Parallel()

	t.Run("login success", func(t *testing.T) {
		t.Parallel()

		uc, repo, tokens := newUserUseCaseWithTokens(t)
		hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		require.NoError(t, err)

		storedUser := entity.User{
			ID: "user-id-123", Username: "testuser",
			Email: "test@example.com", PasswordHash: string(hash),
		}
		repo.EXPECT().GetByEmail(gomock.Any(), "test@example.com").Return(storedUser, nil)
		tokens.EXPECT().Store(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rt *entity.RefreshToken) error {
			assert.Equal(t, "user-id-123", rt.UserID)
			assert.Len(t, rt.TokenHash, 64) // hex SHA-256, never the raw token
			return nil
		})

		token, err := uc.Login(context.Background(), "test@example.com", "password123")

		require.NoError(t, err)
		assert.NotEmpty(t, token.Token)
		assert.NotEmpty(t, token.RefreshToken)
		assert.Equal(t, int64(3600), token.ExpiresIn)
	})

	t.Run("login banned user", func(t *testing.T) {
		t.Parallel()

		uc, repo := newUserUseCase(t)
		hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
		require.NoError(t, err)

		repo.EXPECT().GetByEmail(gomock.Any(), "b@example.com").Return(entity.User{
			ID: "u", Email: "b@example.com", PasswordHash: string(hash), IsBanned: true,
		}, nil)

		_, err = uc.Login(context.Background(), "b@example.com", "password123")

		require.ErrorIs(t, err, entity.ErrUserBanned)
	})

	t.Run("login wrong password", func(t *testing.T) {
		t.Parallel()

		uc, repo := newUserUseCase(t)
		hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		require.NoError(t, err)

		storedUser := entity.User{
			ID: "user-id-123", Username: "testuser",
			Email: "test@example.com", PasswordHash: string(hash),
		}
		repo.EXPECT().GetByEmail(gomock.Any(), "test@example.com").Return(storedUser, nil)

		token, err := uc.Login(context.Background(), "test@example.com", "wrongpassword")

		require.ErrorIs(t, err, entity.ErrInvalidCredentials)
		assert.Empty(t, token.Token)
	})

	t.Run("login user not found", func(t *testing.T) {
		t.Parallel()

		uc, repo := newUserUseCase(t)
		repo.EXPECT().GetByEmail(gomock.Any(), "notfound@example.com").Return(entity.User{}, entity.ErrUserNotFound)

		token, err := uc.Login(context.Background(), "notfound@example.com", "password123")

		require.ErrorIs(t, err, entity.ErrInvalidCredentials)
		assert.Empty(t, token.Token)
	})
}

func TestGetUser(t *testing.T) {
	t.Parallel()

	expectedUser := entity.User{
		ID:       "user-id-123",
		Username: "testuser",
		Email:    "test@example.com",
	}

	t.Run("get user success", func(t *testing.T) {
		t.Parallel()

		uc, repo := newUserUseCase(t)
		repo.EXPECT().GetByID(gomock.Any(), "user-id-123").Return(expectedUser, nil)

		u, err := uc.GetUser(context.Background(), "user-id-123")

		require.NoError(t, err)
		assert.Equal(t, expectedUser, u)
	})

	t.Run("get user not found", func(t *testing.T) {
		t.Parallel()

		uc, repo := newUserUseCase(t)
		repo.EXPECT().GetByID(gomock.Any(), "missing-id").Return(entity.User{}, entity.ErrUserNotFound)
		repo.EXPECT().GetByUsername(gomock.Any(), "missing-id").Return(entity.User{}, entity.ErrUserNotFound)

		_, err := uc.GetUser(context.Background(), "missing-id")

		require.Error(t, err)
	})
}

func TestGetUser_GenericError(t *testing.T) {
	t.Parallel()

	uc, repo := newUserUseCase(t)

	repo.EXPECT().GetByID(gomock.Any(), "user-id-123").Return(entity.User{}, errInternalServErr)
	repo.EXPECT().GetByUsername(gomock.Any(), "user-id-123").Return(entity.User{}, errInternalServErr)

	_, err := uc.GetUser(context.Background(), "user-id-123")

	require.Error(t, err)
}

func TestRefreshToken(t *testing.T) {
	t.Parallel()

	t.Run("unknown refresh token", func(t *testing.T) {
		t.Parallel()

		uc, _, tokens := newUserUseCaseWithTokens(t)
		tokens.EXPECT().GetByHash(gomock.Any(), gomock.Any()).Return(entity.RefreshToken{}, entity.ErrRefreshTokenNotFound)

		_, err := uc.RefreshToken(context.Background(), "invalid-token")
		require.ErrorIs(t, err, entity.ErrInvalidCredentials)
	})

	t.Run("an access token is not a refresh token", func(t *testing.T) {
		t.Parallel()

		uc, _, tokens := newUserUseCaseWithTokens(t)
		access, err := jwt.New("test-secret", time.Hour).GenerateToken("user-1")
		require.NoError(t, err)
		tokens.EXPECT().GetByHash(gomock.Any(), gomock.Any()).Return(entity.RefreshToken{}, entity.ErrRefreshTokenNotFound)

		_, err = uc.RefreshToken(context.Background(), access)
		require.ErrorIs(t, err, entity.ErrInvalidCredentials)
	})

	t.Run("rotation revokes the old token and issues a new pair", func(t *testing.T) {
		t.Parallel()

		uc, repo, tokens := newUserUseCaseWithTokens(t)
		tokens.EXPECT().GetByHash(gomock.Any(), gomock.Any()).Return(entity.RefreshToken{
			ID: "old-id", UserID: "user-1", ExpiresAt: time.Now().Add(time.Hour),
		}, nil)
		repo.EXPECT().GetByID(gomock.Any(), "user-1").Return(entity.User{ID: "user-1"}, nil)
		var newID string
		tokens.EXPECT().Store(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rt *entity.RefreshToken) error {
			newID = rt.ID
			return nil
		})
		tokens.EXPECT().Revoke(gomock.Any(), "old-id", gomock.Any()).DoAndReturn(func(_ context.Context, _, replacedBy string) (bool, error) {
			assert.Equal(t, newID, replacedBy)
			return true, nil
		})

		res, err := uc.RefreshToken(context.Background(), "some-refresh-token")

		require.NoError(t, err)
		assert.NotEmpty(t, res.Token)
		assert.NotEmpty(t, res.RefreshToken)
		assert.NotEqual(t, "some-refresh-token", res.RefreshToken)
	})

	t.Run("reusing a revoked token revokes every session", func(t *testing.T) {
		t.Parallel()

		uc, _, tokens := newUserUseCaseWithTokens(t)
		revokedAt := time.Now().Add(-time.Minute)
		tokens.EXPECT().GetByHash(gomock.Any(), gomock.Any()).Return(entity.RefreshToken{
			ID: "old-id", UserID: "user-1", ExpiresAt: time.Now().Add(time.Hour), RevokedAt: &revokedAt,
		}, nil)
		tokens.EXPECT().RevokeAllForUser(gomock.Any(), "user-1").Return(nil)

		_, err := uc.RefreshToken(context.Background(), "stolen-token")
		require.ErrorIs(t, err, entity.ErrInvalidCredentials)
	})

	t.Run("expired token", func(t *testing.T) {
		t.Parallel()

		uc, _, tokens := newUserUseCaseWithTokens(t)
		tokens.EXPECT().GetByHash(gomock.Any(), gomock.Any()).Return(entity.RefreshToken{
			ID: "old-id", UserID: "user-1", ExpiresAt: time.Now().Add(-time.Second),
		}, nil)

		_, err := uc.RefreshToken(context.Background(), "expired")
		require.ErrorIs(t, err, entity.ErrInvalidCredentials)
	})

	t.Run("banned user cannot refresh", func(t *testing.T) {
		t.Parallel()

		uc, repo, tokens := newUserUseCaseWithTokens(t)
		tokens.EXPECT().GetByHash(gomock.Any(), gomock.Any()).Return(entity.RefreshToken{
			ID: "old-id", UserID: "user-1", ExpiresAt: time.Now().Add(time.Hour),
		}, nil)
		repo.EXPECT().GetByID(gomock.Any(), "user-1").Return(entity.User{ID: "user-1", IsBanned: true}, nil)

		_, err := uc.RefreshToken(context.Background(), "t")
		require.ErrorIs(t, err, entity.ErrUserBanned)
	})
}

func TestLogout(t *testing.T) {
	t.Parallel()

	uc, _, tokens := newUserUseCaseWithTokens(t)
	tokens.EXPECT().GetByHash(gomock.Any(), gomock.Any()).Return(entity.RefreshToken{ID: "rt-1", UserID: "user-1"}, nil)
	tokens.EXPECT().Revoke(gomock.Any(), "rt-1", "").Return(true, nil)

	require.NoError(t, uc.Logout(context.Background(), "token"))
}

func TestIsBanned_CachedAndInvalidated(t *testing.T) {
	t.Parallel()

	uc, repo := newUserUseCase(t)

	repo.EXPECT().GetByID(gomock.Any(), "user-1").Return(entity.User{ID: "user-1"}, nil).Times(1)
	banned, err := uc.IsBanned(context.Background(), "user-1")
	require.NoError(t, err)
	assert.False(t, banned)

	// Served from cache: no second DB call.
	banned, err = uc.IsBanned(context.Background(), "user-1")
	require.NoError(t, err)
	assert.False(t, banned)

	// After an admin ban, invalidation forces a fresh read.
	uc.InvalidateBanCache("user-1")
	repo.EXPECT().GetByID(gomock.Any(), "user-1").Return(entity.User{ID: "user-1", IsBanned: true}, nil).Times(1)
	banned, err = uc.IsBanned(context.Background(), "user-1")
	require.NoError(t, err)
	assert.True(t, banned)
}
