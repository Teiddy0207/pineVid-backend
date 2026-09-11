// Package repo implements application outer layer logic. Each logic group in own file.
package repo

import (
	"context"
	"time"

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
		GetByUsername(ctx context.Context, username string) (entity.User, error)
		Update(ctx context.Context, user *entity.User) error
		List(ctx context.Context, page, limit int) ([]entity.User, int, error)
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
		Query      string
		Status     *entity.VideoStatus
		Visibility *entity.VideoVisibility
		Limit      uint64
		Offset     uint64
	}

	// WatchHistoryRepo -.
	WatchHistoryRepo interface {
		Upsert(ctx context.Context, userID, videoID string, watchSeconds int) error
		ListByUser(ctx context.Context, userID string, limit, offset int) ([]entity.Video, int, error)
	}

	// FollowRepo -.
	FollowRepo interface {
		Follow(ctx context.Context, followerID, channelID string) error
		Unfollow(ctx context.Context, followerID, channelID string) error
		IsFollowing(ctx context.Context, followerID, channelID string) (bool, error)
		CountFollowers(ctx context.Context, channelID string) (int64, error)
		ListFollowedChannels(ctx context.Context, followerID string, page, limit int) ([]entity.User, int, error)
		ListFollowers(ctx context.Context, channelID string, limit int) ([]entity.User, error)
	}

	// LivestreamRepo -.
	LivestreamRepo interface {
		Store(ctx context.Context, ls *entity.Livestream) error
		GetByID(ctx context.Context, id string) (entity.Livestream, error)
		GetByUserID(ctx context.Context, userID string) (entity.Livestream, error)
		GetByStreamKey(ctx context.Context, streamKey string) (entity.Livestream, error)
		ListActive(ctx context.Context, category string, limit, offset int) ([]entity.Livestream, int, error)
		Update(ctx context.Context, ls *entity.Livestream) error
		CountActive(ctx context.Context) (int64, error)
		SumActiveViewers(ctx context.Context) (int64, error)
	}

	// SubtitleRepo -.
	SubtitleRepo interface {
		ReplaceCues(ctx context.Context, videoID string, cues []entity.SubtitleCue) error
		GetByVideoID(ctx context.Context, videoID string) ([]entity.SubtitleCue, error)
	}

	// WorkerRepo -.
	WorkerRepo interface {
		UpsertHeartbeat(ctx context.Context, hb entity.WorkerHeartbeat) error
		ListActive(ctx context.Context, staleAfter time.Duration) ([]entity.WorkerHeartbeat, error)
	}

	// NotificationRepo -.
	NotificationRepo interface {
		Store(ctx context.Context, notif *entity.Notification) error
		ListByUserID(ctx context.Context, userID string, limit, offset int) ([]entity.Notification, int, error)
		MarkAsRead(ctx context.Context, id, userID string) error
		MarkAllAsRead(ctx context.Context, userID string) error
		CountUnread(ctx context.Context, userID string) (int, error)
	}

	// VocabularyRepo -.
	VocabularyRepo interface {
		SaveWord(ctx context.Context, item *entity.Vocabulary) error
		ListByUserID(ctx context.Context, userID string) ([]entity.Vocabulary, error)
		DeleteWord(ctx context.Context, id, userID string) error
	}

	// AdminStatsRepo -.
	AdminStatsRepo interface {
		ChannelStats(ctx context.Context) (entity.ChannelStats, error)
		CategoryBreakdown(ctx context.Context) ([]entity.CategoryStat, error)
	}

	// CommentRepo -.
	CommentRepo interface {
		Store(ctx context.Context, c *entity.Comment) error
		GetByID(ctx context.Context, id string) (entity.Comment, error)
		ListByVideoID(ctx context.Context, videoID string, limit, offset uint64) ([]entity.Comment, int, error)
		ListRepliesByParentID(ctx context.Context, parentID string, limit, offset uint64) ([]entity.Comment, int, error)
		CountAllByVideoID(ctx context.Context, videoID string) (int, error)
	}

	// UserPreferenceRepo -.
	UserPreferenceRepo interface {
		SetCategories(ctx context.Context, userID string, categories []string) error
		GetCategories(ctx context.Context, userID string) ([]string, error)
	}

	// LikeRepo -.
	LikeRepo interface {
		ToggleLike(ctx context.Context, l *entity.VideoLike) (bool, int64, error)
		IncrementHeart(ctx context.Context, streamID string) (int64, error)
		GetLikeCount(ctx context.Context, videoID string) (int64, error)
		IsLikedByUser(ctx context.Context, videoID, userID string) (bool, error)
	}

	// SavedVideoRepo -.
	SavedVideoRepo interface {
		ToggleSave(ctx context.Context, userID, videoID string) (bool, error)
		IsSavedByUser(ctx context.Context, videoID, userID string) (bool, error)
		ListSavedVideos(ctx context.Context, userID string, limit, offset int) ([]entity.Video, int, error)
	}

	// CommentLikeRepo -.
	CommentLikeRepo interface {
		ToggleLike(ctx context.Context, commentID, userID string) (bool, int64, error)
		GetLikeCount(ctx context.Context, commentID string) (int64, error)
		IsLikedByUser(ctx context.Context, commentID, userID string) (bool, error)
	}
)
