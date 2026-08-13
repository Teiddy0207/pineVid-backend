// Package usecase implements application business logic. Each logic group in own file.
package usecase

import (
	"context"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
)

//go:generate mockgen -source=contracts.go -destination=./mocks_usecase_test.go -package=usecase_test

type (
	// Translation -.
	Translation interface {
		Translate(ctx context.Context, userID string, t entity.Translation) (entity.Translation, error)
		History(ctx context.Context, userID string) (entity.TranslationHistory, error)
	}

	// User -.
	User interface {
		Register(ctx context.Context, username, email, password string) (entity.User, error)
		Login(ctx context.Context, email, password string) (string, error)
		GetUser(ctx context.Context, userID string) (entity.User, error)
		UpdateUser(ctx context.Context, userID, username, email, avatar string) (entity.User, error)
	}

	// Task -.
	Task interface {
		Create(ctx context.Context, userID, title, description string) (entity.Task, error)
		Get(ctx context.Context, userID, taskID string) (entity.Task, error)
		List(ctx context.Context, userID string, status *entity.TaskStatus, limit, offset int) ([]entity.Task, int, error)
		Transition(ctx context.Context, userID, taskID string, newStatus entity.TaskStatus) (entity.Task, error)
		Update(ctx context.Context, userID, taskID, title, description string) (entity.Task, error)
		Delete(ctx context.Context, userID, taskID string) error
	}

	// Video -.
	Video interface {
		CreateUpload(ctx context.Context, userID string, req request.CreateVideoUpload) (response.UploadUrlResponse, error)
		ConfirmUpload(ctx context.Context, userID string, req request.ConfirmUpload) (response.VideoResponse, error)
		GetByID(ctx context.Context, id string) (response.VideoResponse, error)
		ListPublicVideos(ctx context.Context, userID, category string, page, limit int) (response.PageResponse[response.VideoResponse], error)
		ListStudioVideos(ctx context.Context, userID string, page, limit int) (response.PageResponse[response.VideoResponse], error)
		PublishVideo(ctx context.Context, userID, videoID string) (response.VideoResponse, error)
		UpdateVideo(ctx context.Context, userID, videoID string, req request.UpdateVideo) (response.VideoResponse, error)
		DeleteVideo(ctx context.Context, userID, videoID string) error
		HandleTranscodeCallback(ctx context.Context, videoID, status, hlsMasterURL string) error
		RecordView(ctx context.Context, videoID, clientIP, deviceID string) (bool, int64, error)
	}

	// Livestream -.
	Livestream interface {
		GetStreamKey(ctx context.Context, userID string) (response.StreamKeyResponse, error)
		ResetStreamKey(ctx context.Context, userID string) (response.StreamKeyResponse, error)
		AuthenticateStreamKey(ctx context.Context, req request.StreamKeyAuth) (bool, error)
		UnpublishStream(ctx context.Context, streamKey string) error
		GetStreamByID(ctx context.Context, id string) (response.LivestreamResponse, error)
		ListActiveStreams(ctx context.Context, category string, page, limit int) (response.PageResponse[response.LivestreamResponse], error)
		UpdateStreamInfo(ctx context.Context, userID string, req request.UpdateLivestreamInfo) (response.LivestreamResponse, error)
		SendChatMessage(ctx context.Context, streamID string, req request.SendChatMessage) (response.ChatMessageResponse, error)
		SubscribeChat(streamID string) (<-chan response.ChatMessageResponse, func(), error)
	}

	// History -.
	History interface {
		RecordWatch(ctx context.Context, userID, videoID string, watchSeconds int) error
		ListHistory(ctx context.Context, userID string, page, limit int) (response.PageResponse[response.VideoResponse], error)
	}

	// Admin -.
	Admin interface {
		GetDashboard(ctx context.Context) (response.SystemDashboardResponse, error)
		GetWorkersStatus(ctx context.Context) ([]response.WorkerStatusResponse, error)
		BanStream(ctx context.Context, streamID string) error
		BanVideo(ctx context.Context, videoID string) error
	}

	// Like -.
	Like interface {
		ToggleLikeVideo(ctx context.Context, userID, videoID string) (response.LikeResponse, error)
		HeartStream(ctx context.Context, streamID string) (response.HeartResponse, error)
	}

	// Comment -.
	Comment interface {
		CreateComment(ctx context.Context, videoID, userID, userName, userAvatar string, req request.CreateCommentRequest) (response.CommentResponse, error)
		ListVideoComments(ctx context.Context, videoID string, page, limit int) (response.PageResponse[response.CommentResponse], error)
	}

	// Recommendation -.
	Recommendation interface {
		GetPersonalizedFeed(ctx context.Context, userID string, page, limit int) (response.PageResponse[response.RecommendedVideoItem], error)
	}
)
