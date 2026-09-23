package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// banCacheTTL bounds how long IsBanned trusts a cached answer. Bans issued
// through the admin usecase invalidate the entry immediately (see
// InvalidateBanCache); the TTL only matters for changes made elsewhere
// (e.g. directly in the DB).
const banCacheTTL = 30 * time.Second

const refreshTokenBytes = 32

type banCacheEntry struct {
	banned  bool
	expires time.Time
}

// UseCase -.
type UseCase struct {
	repo          repo.UserRepo
	refreshRepo   repo.RefreshTokenRepo
	jwt           *jwt.Manager
	refreshExpiry time.Duration

	banCache sync.Map // userID -> banCacheEntry
}

// New returns a User usecase instrumented with OpenTelemetry tracing spans.
func New(r repo.UserRepo, rt repo.RefreshTokenRepo, j *jwt.Manager, refreshExpiry time.Duration) usecase.User {
	return newTraced(&UseCase{
		repo:          r,
		refreshRepo:   rt,
		jwt:           j,
		refreshExpiry: refreshExpiry,
	})
}

// Register -.
func (uc *UseCase) Register(ctx context.Context, username, email, password string) (entity.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return entity.User{}, fmt.Errorf("UserUseCase - Register - bcrypt.GenerateFromPassword: %w", err)
	}

	now := time.Now().UTC()

	user := entity.User{
		ID:           uuid.New().String(),
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         entity.UserRoleUser,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err = uc.repo.Store(ctx, &user)
	if err != nil {
		return entity.User{}, fmt.Errorf("UserUseCase - Register - uc.repo.Store: %w", err)
	}

	return user, nil
}

// Login verifies credentials and issues an access token plus a refresh token.
func (uc *UseCase) Login(ctx context.Context, email, password string) (response.Token, error) {
	user, err := uc.repo.GetByEmail(ctx, email)
	if err != nil {
		return response.Token{}, entity.ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return response.Token{}, entity.ErrInvalidCredentials
	}

	if user.IsBanned {
		return response.Token{}, entity.ErrUserBanned
	}

	return uc.issueTokens(ctx, user.ID)
}

// GetUser -.
func (uc *UseCase) GetUser(ctx context.Context, userID string) (entity.User, error) {
	user, err := uc.repo.GetByID(ctx, userID)
	if err == nil {
		return user, nil
	}

	userByUsername, errUsername := uc.repo.GetByUsername(ctx, userID)
	if errUsername == nil {
		return userByUsername, nil
	}

	return entity.User{}, fmt.Errorf("UserUseCase - GetUser: %w", err)
}

// UpdateUser persists changes to username, email, and avatar for the given user.
func (uc *UseCase) UpdateUser(ctx context.Context, userID, username, email, avatar string) (entity.User, error) {
	user, err := uc.repo.GetByID(ctx, userID)
	if err != nil {
		return entity.User{}, fmt.Errorf("UserUseCase - UpdateUser - uc.repo.GetByID: %w", err)
	}

	if username != "" {
		user.Username = username
	}
	if email != "" {
		user.Email = email
	}
	if avatar != "" {
		user.Avatar = avatar
	}
	user.UpdatedAt = time.Now().UTC()

	if err := uc.repo.Update(ctx, &user); err != nil {
		return entity.User{}, fmt.Errorf("UserUseCase - UpdateUser - uc.repo.Update: %w", err)
	}

	return user, nil
}

// RefreshToken exchanges a refresh token for a new access + refresh token
// pair (rotation). The presented token is revoked; presenting a token that
// was already revoked means it leaked (the legitimate client already rotated
// it), so every refresh token of that user is revoked.
func (uc *UseCase) RefreshToken(ctx context.Context, refreshToken string) (response.Token, error) {
	stored, err := uc.refreshRepo.GetByHash(ctx, hashToken(refreshToken))
	if err != nil {
		if errors.Is(err, entity.ErrRefreshTokenNotFound) {
			return response.Token{}, entity.ErrInvalidCredentials
		}
		return response.Token{}, fmt.Errorf("UserUseCase - RefreshToken - GetByHash: %w", err)
	}

	if stored.RevokedAt != nil {
		_ = uc.refreshRepo.RevokeAllForUser(ctx, stored.UserID)
		return response.Token{}, entity.ErrInvalidCredentials
	}
	if time.Now().After(stored.ExpiresAt) {
		return response.Token{}, entity.ErrInvalidCredentials
	}

	user, err := uc.repo.GetByID(ctx, stored.UserID)
	if err != nil {
		return response.Token{}, entity.ErrUserNotFound
	}
	if user.IsBanned {
		return response.Token{}, entity.ErrUserBanned
	}

	tokens, newID, err := uc.issueTokensWithID(ctx, user.ID)
	if err != nil {
		return response.Token{}, err
	}

	revoked, err := uc.refreshRepo.Revoke(ctx, stored.ID, newID)
	if err != nil {
		return response.Token{}, fmt.Errorf("UserUseCase - RefreshToken - Revoke: %w", err)
	}
	if !revoked {
		// Lost a race with a concurrent refresh using the same token.
		_, _ = uc.refreshRepo.Revoke(ctx, newID, "")
		return response.Token{}, entity.ErrInvalidCredentials
	}

	return tokens, nil
}

// Logout revokes the given refresh token. Unknown/already-revoked tokens are
// not an error — the client is logged out either way.
func (uc *UseCase) Logout(ctx context.Context, refreshToken string) error {
	stored, err := uc.refreshRepo.GetByHash(ctx, hashToken(refreshToken))
	if err != nil {
		if errors.Is(err, entity.ErrRefreshTokenNotFound) {
			return nil
		}
		return fmt.Errorf("UserUseCase - Logout - GetByHash: %w", err)
	}

	if _, err := uc.refreshRepo.Revoke(ctx, stored.ID, ""); err != nil {
		return fmt.Errorf("UserUseCase - Logout - Revoke: %w", err)
	}
	return nil
}

// IsBanned reports whether the account is banned, cached for banCacheTTL so
// the per-request check in the auth middleware doesn't hit the DB every time.
func (uc *UseCase) IsBanned(ctx context.Context, userID string) (bool, error) {
	if v, ok := uc.banCache.Load(userID); ok {
		entry := v.(banCacheEntry)
		if time.Now().Before(entry.expires) {
			return entry.banned, nil
		}
	}

	user, err := uc.repo.GetByID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("UserUseCase - IsBanned - GetByID: %w", err)
	}

	uc.banCache.Store(userID, banCacheEntry{banned: user.IsBanned, expires: time.Now().Add(banCacheTTL)})
	return user.IsBanned, nil
}

// InvalidateBanCache drops the cached ban state so the next IsBanned call
// reads the DB — called right after an admin bans/unbans an account.
func (uc *UseCase) InvalidateBanCache(userID string) {
	uc.banCache.Delete(userID)
}

// RevokeSessions revokes every refresh token of the user (used on ban).
func (uc *UseCase) RevokeSessions(ctx context.Context, userID string) error {
	return uc.refreshRepo.RevokeAllForUser(ctx, userID)
}

func (uc *UseCase) issueTokens(ctx context.Context, userID string) (response.Token, error) {
	tokens, _, err := uc.issueTokensWithID(ctx, userID)
	return tokens, err
}

func (uc *UseCase) issueTokensWithID(ctx context.Context, userID string) (response.Token, string, error) {
	accessToken, err := uc.jwt.GenerateToken(userID)
	if err != nil {
		return response.Token{}, "", fmt.Errorf("UserUseCase - issueTokens - GenerateToken: %w", err)
	}

	raw := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return response.Token{}, "", fmt.Errorf("UserUseCase - issueTokens - rand.Read: %w", err)
	}
	refreshToken := base64.RawURLEncoding.EncodeToString(raw)

	now := time.Now().UTC()
	record := entity.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    userID,
		TokenHash: hashToken(refreshToken),
		ExpiresAt: now.Add(uc.refreshExpiry),
		CreatedAt: now,
	}
	if err := uc.refreshRepo.Store(ctx, &record); err != nil {
		return response.Token{}, "", fmt.Errorf("UserUseCase - issueTokens - Store: %w", err)
	}

	return response.Token{
		Token:        accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(uc.jwt.Duration().Seconds()),
	}, record.ID, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
