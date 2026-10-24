// Package repo implements application outer layer logic. Each logic group in own file.
package repo

import (
	"context"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
)

//go:generate mockgen -source=contracts.go -destination=../usecase/mocks_repo_test.go -package=usecase_test

type (
	// UserRepo -.
	UserRepo interface {
		Store(ctx context.Context, user *entity.User) error
		GetByID(ctx context.Context, id string) (entity.User, error)
		GetByEmail(ctx context.Context, email string) (entity.User, error)
		GetByUsername(ctx context.Context, username string) (entity.User, error)
		Update(ctx context.Context, user *entity.User) error
		List(ctx context.Context, page, limit int) ([]entity.User, int, error)
	}

	// RefreshTokenRepo -.
	RefreshTokenRepo interface {
		Store(ctx context.Context, t *entity.RefreshToken) error
		GetByHash(ctx context.Context, tokenHash string) (entity.RefreshToken, error)
		Revoke(ctx context.Context, id, replacedBy string) (bool, error)
		RevokeAllForUser(ctx context.Context, userID string) error
	}

	// VideoRepo -.
	VideoRepo interface {
		Store(ctx context.Context, video *entity.Video) error
		GetByID(ctx context.Context, id string) (entity.Video, error)
		GetByIDs(ctx context.Context, ids []string) ([]entity.Video, error)
		List(ctx context.Context, filter VideoFilter) ([]entity.Video, int, error)
		Update(ctx context.Context, video *entity.Video) error
		SetStatus(ctx context.Context, id string, status entity.VideoStatus, at time.Time) error
		MarkQueued(ctx context.Context, id, jobID string, from entity.VideoStatus, at time.Time) (bool, error)
		RevertQueued(ctx context.Context, id, jobID string, to entity.VideoStatus, at time.Time) error
		ApplyTranscodeEvent(ctx context.Context, ev entity.TranscodeEvent, at time.Time) (entity.TranscodeEventResult, error)
		Delete(ctx context.Context, id string) error
	}

	// VideoFilter -.
	VideoFilter struct {
		UserID     string
		Category   string
		Query      string
		Status     *entity.VideoStatus
		Visibility *entity.VideoVisibility
		IsReel     *bool
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

	// LiveSessionRepo manages per-broadcast session records. Each time a streamer
	// goes live a new LiveSession is created; the parent Livestream holds only
	// the stream-key config and channel identity.
	LiveSessionRepo interface {
		Store(ctx context.Context, sess *entity.LiveSession) error
		GetByID(ctx context.Context, id string) (entity.LiveSession, error)
		// GetCurrentByLivestreamID returns the open session (ended_at IS NULL) for
		// the given channel, or ErrLiveSessionNotFound if none is active.
		GetCurrentByLivestreamID(ctx context.Context, livestreamID string) (entity.LiveSession, error)
		// ListByLivestreamID returns sessions ordered newest-first.
		ListByLivestreamID(ctx context.Context, livestreamID string, limit, offset int) ([]entity.LiveSession, int, error)
		// EndSession stamps ended_at on the given session.
		EndSession(ctx context.Context, id string, endedAt time.Time) error
		// SetReplayVideo links a replay VOD to the session after DVR processing.
		SetReplayVideo(ctx context.Context, id, videoID string) error
	}

	// ChatMessageRepo persists chat lines for replay. Real-time delivery is
	// handled by ChatHub (in-memory SSE); this repo is the durable store.
	ChatMessageRepo interface {
		Store(ctx context.Context, msg *entity.LiveChatMessage) error
		ListBySessionID(ctx context.Context, sessionID string, limit, offset int) ([]entity.LiveChatMessage, int, error)
	}

	// LiveEventLog is the durable path for live chat and hearts: events are
	// appended to a Redis Stream and consumed by the live-agg worker.
	LiveEventLog interface {
		AppendChat(ctx context.Context, ev entity.LiveEvent) error
		// AppendHeart appends the event and bumps the room's running heart
		// total atomically, returning the new total for the realtime UI.
		AppendHeart(ctx context.Context, ev entity.LiveEvent) (int64, error)
		HeartTotal(ctx context.Context, livestreamID string) (int64, error)
		// ResetHearts zeroes the running total when a new broadcast starts.
		ResetHearts(ctx context.Context, livestreamID string) error
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
		GetLikeInfo(ctx context.Context, videoIDs []string, userID string) (counts map[string]int64, liked map[string]bool, err error)
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

	// PostRepo -.
	PostRepo interface {
		Store(ctx context.Context, p *entity.Post) error
		GetByID(ctx context.Context, id string) (entity.Post, error)
		ListByUser(ctx context.Context, userID string, limit, offset uint64) ([]entity.Post, int, error)
		Delete(ctx context.Context, id string) error
	}

	// PostLikeRepo -.
	PostLikeRepo interface {
		ToggleLike(ctx context.Context, postID, userID string) (bool, int64, error)
		GetLikeCount(ctx context.Context, postID string) (int64, error)
		IsLikedByUser(ctx context.Context, postID, userID string) (bool, error)
	}
)

// LivePresence tracks expiring tabs and deduplicates their viewer identities.
type LivePresence interface {
	Apply(ctx context.Context, broadcast, action, tab, identity string) (int64, error)
}
