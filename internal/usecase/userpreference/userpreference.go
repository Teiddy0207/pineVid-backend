package userpreference

import (
	"context"
	"fmt"

	"github.com/evrone/go-clean-template/internal/repo"
)

type UseCase struct {
	repo repo.UserPreferenceRepo
}

func New(r repo.UserPreferenceRepo) *UseCase {
	return &UseCase{repo: r}
}

func (u *UseCase) SetPreferredCategories(ctx context.Context, userID string, categories []string) error {
	if err := u.repo.SetCategories(ctx, userID, categories); err != nil {
		return fmt.Errorf("UserPreferenceUseCase - SetPreferredCategories: %w", err)
	}
	return nil
}

func (u *UseCase) GetPreferredCategories(ctx context.Context, userID string) ([]string, error) {
	categories, err := u.repo.GetCategories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("UserPreferenceUseCase - GetPreferredCategories: %w", err)
	}
	return categories, nil
}
