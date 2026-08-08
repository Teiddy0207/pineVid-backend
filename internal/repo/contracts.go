// Package repo implements application outer layer logic. Each logic group in own file.
package repo

import (
	"context"

	"github.com/evrone/go-clean-template/internal/entity"
)

//go:generate mockgen -source=contracts.go -destination=../usecase/mocks_repo_test.go -package=usecase_test

type (
	// TranslationRepo -.
	TranslationRepo interface {
		Store(ctx context.Context, userID string, t entity.Translation) error
		GetHistory(ctx context.Context, userID string) ([]entity.Translation, error)
	}

	// TranslationWebAPI -.
	TranslationWebAPI interface {
		Translate(ctx context.Context, t entity.Translation) (entity.Translation, error)
	}

	// UserRepo -.
	UserRepo interface {
		Store(ctx context.Context, user *entity.User) error
		GetByID(ctx context.Context, id string) (entity.User, error)
		GetByEmail(ctx context.Context, email string) (entity.User, error)
	}

	// TaskRepo -.
	TaskRepo interface {
		Store(ctx context.Context, task *entity.Task) error
		GetByID(ctx context.Context, userID, taskID string) (entity.Task, error)
		List(ctx context.Context, userID string, filter TaskFilter) ([]entity.Task, int, error)
		Update(ctx context.Context, task *entity.Task) error
		Delete(ctx context.Context, userID, taskID string) error
	}

	// TaskFilter -.
	TaskFilter struct {
		Status *entity.TaskStatus
		Limit  uint64
		Offset uint64
	}

	// VideoRepo -.
	VideoRepo interface {
		Store(ctx context.Context, video *entity.Video) error
		GetByID(ctx context.Context, id string) (entity.Video, error)
		List(ctx context.Context, filter VideoFilter) ([]entity.Video, int, error)
		Update(ctx context.Context, video *entity.Video) error
		Delete(ctx context.Context, id string) error
	}

	// VideoFilter -.
	VideoFilter struct {
		UserID     string
		Category   string
		Status     *entity.VideoStatus
		Visibility *entity.VideoVisibility
		Limit      uint64
		Offset     uint64
	}

	// LivestreamRepo -.
	LivestreamRepo interface {
		Store(ctx context.Context, ls *entity.Livestream) error
		GetByID(ctx context.Context, id string) (entity.Livestream, error)
		GetByUserID(ctx context.Context, userID string) (entity.Livestream, error)
		GetByStreamKey(ctx context.Context, streamKey string) (entity.Livestream, error)
		ListActive(ctx context.Context, category string, limit, offset int) ([]entity.Livestream, int, error)
		Update(ctx context.Context, ls *entity.Livestream) error
	}
)
