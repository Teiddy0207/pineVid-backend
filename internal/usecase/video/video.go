package video

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/repo/persistent/view"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/nats"
	"github.com/google/uuid"
)

type UseCase struct {
	repo          repo.VideoRepo
	viewRepo      *view.Repo
	natsPublisher *nats.Publisher
	notifUc       usecase.Notification
}

func New(r repo.VideoRepo, vRepo *view.Repo, natsPub *nats.Publisher, notifUc usecase.Notification) *UseCase {
	return &UseCase{
		repo:          r,
		viewRepo:      vRepo,
		natsPublisher: natsPub,
		notifUc:       notifUc,
	}
}

func (u *UseCase) RecordView(ctx context.Context, videoID, clientIP, deviceID string) (bool, int64, error) {
	if u.viewRepo == nil {
		return false, 0, nil
	}
	return u.viewRepo.RecordView(ctx, videoID, clientIP, deviceID)
}

func (u *UseCase) CreateUpload(ctx context.Context, userID string, req request.CreateVideoUpload) (response.UploadUrlResponse, error) {
	ext := strings.ToLower(filepath.Ext(req.FileName))
	if ext == "" {
		ext = ".mp4"
	}
	if !entity.AllowedVideoExtensions[ext] {
		return response.UploadUrlResponse{}, entity.ErrUnsupportedVideoFormat
	}

	videoID := uuid.New().String()
	s3Key := fmt.Sprintf("raw-uploads/%s/raw%s", videoID, ext)

	v := mapper.ToVideoEntity(userID, req, videoID, s3Key)
	v.CreatedAt = time.Now().UTC()
	v.UpdatedAt = time.Now().UTC()

	if err := u.repo.Store(ctx, &v); err != nil {
		return response.UploadUrlResponse{}, fmt.Errorf("VideoUseCase - CreateUpload - Store: %w", err)
	}

	// Presigned S3 upload URL for local MinIO S3 bucket raw-videos
	presignedUrl := fmt.Sprintf("http://localhost:9000/raw-videos/%s", s3Key)

	return response.UploadUrlResponse{
		VideoID:   videoID,
		UploadUrl: presignedUrl,
		RawS3Key:  s3Key,
	}, nil
}

func (u *UseCase) ConfirmUpload(ctx context.Context, userID string, req request.ConfirmUpload) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, req.VideoID)
	if err != nil {
		return response.VideoResponse{}, err
	}

	v.Status = entity.VideoStatusProcessing
	v.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &v); err != nil {
		return response.VideoResponse{}, fmt.Errorf("VideoUseCase - ConfirmUpload - Update: %w", err)
	}

	// Publish NATS JetStream event 'video.transcode' for Transcode Worker
	if u.natsPublisher != nil {
		_ = u.natsPublisher.PublishTranscodeJob(v.ID, v.RawS3Key, userID)
	}

	return mapper.ToVideoResponse(v), nil
}

func (u *UseCase) GetByID(ctx context.Context, id string) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return response.VideoResponse{}, err
	}
	if u.viewRepo != nil {
		v.Views += u.viewRepo.GetPendingViewsForVideo(ctx, id)
	}
	return mapper.ToVideoResponse(v), nil
}

func (u *UseCase) ListPublicVideos(ctx context.Context, userID, category, query string, page, limit int) (response.PageResponse[response.VideoResponse], error) {
	offset := (page - 1) * limit
	status := entity.VideoStatusComplete
	visibility := entity.VideoVisibilityPublic

	filter := repo.VideoFilter{
		UserID:     userID,
		Category:   category,
		Query:      query,
		Status:     &status,
		Visibility: &visibility,
		Limit:      uint64(limit),
		Offset:     uint64(offset),
	}

	videos, total, err := u.repo.List(ctx, filter)
	if err != nil {
		return response.PageResponse[response.VideoResponse]{}, err
	}

	return mapper.ToVideoPageResponse(videos, total, page, limit), nil
}

func (u *UseCase) ListStudioVideos(ctx context.Context, userID string, page, limit int) (response.PageResponse[response.VideoResponse], error) {
	offset := (page - 1) * limit

	filter := repo.VideoFilter{
		UserID: userID,
		Limit:  uint64(limit),
		Offset: uint64(offset),
	}

	videos, total, err := u.repo.List(ctx, filter)
	if err != nil {
		return response.PageResponse[response.VideoResponse]{}, err
	}

	return mapper.ToVideoPageResponse(videos, total, page, limit), nil
}

func (u *UseCase) PublishVideo(ctx context.Context, userID, videoID string) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return response.VideoResponse{}, err
	}
	if v.UserID != userID {
		return response.VideoResponse{}, entity.ErrVideoForbidden
	}

	wasAlreadyPublic := v.Visibility == entity.VideoVisibilityPublic

	v.Visibility = entity.VideoVisibilityPublic
	v.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &v); err != nil {
		return response.VideoResponse{}, fmt.Errorf("VideoUseCase - PublishVideo - Update: %w", err)
	}

	// Notify followers of the new video — but only the first time it goes
	// public, so re-saving an already-public video doesn't spam followers.
	if u.notifUc != nil && !wasAlreadyPublic {
		go func() {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = u.notifUc.NotifyFollowers(
				notifyCtx,
				v.UserID, v.UserName, v.UserAvatar,
				entity.NotificationTypeNewVideo,
				fmt.Sprintf("%s posted a new video", v.UserName),
				v.Title,
				fmt.Sprintf("/watch/%s", v.ID),
			)
		}()
	}

	return mapper.ToVideoResponse(v), nil
}

func (u *UseCase) UpdateVideo(ctx context.Context, userID, videoID string, req request.UpdateVideo) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return response.VideoResponse{}, err
	}
	if v.UserID != userID {
		return response.VideoResponse{}, entity.ErrVideoForbidden
	}

	mapper.ApplyVideoUpdate(&v, req)
	v.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &v); err != nil {
		return response.VideoResponse{}, fmt.Errorf("VideoUseCase - UpdateVideo - Update: %w", err)
	}

	return mapper.ToVideoResponse(v), nil
}

func (u *UseCase) UpdateThumbnail(ctx context.Context, userID, videoID string, req request.UpdateThumbnail) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return response.VideoResponse{}, err
	}
	if v.UserID != userID {
		return response.VideoResponse{}, entity.ErrVideoForbidden
	}

	mapper.ApplyThumbnailUpdate(&v, req)
	v.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &v); err != nil {
		return response.VideoResponse{}, fmt.Errorf("VideoUseCase - UpdateThumbnail - Update: %w", err)
	}

	return mapper.ToVideoResponse(v), nil
}

func (u *UseCase) DeleteVideo(ctx context.Context, userID, videoID string) error {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return err
	}
	if v.UserID != userID {
		return entity.ErrVideoForbidden
	}
	return u.repo.Delete(ctx, videoID)
}


func (u *UseCase) HandleTranscodeCallback(ctx context.Context, videoID, status, hlsMasterURL string) error {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return fmt.Errorf("VideoUseCase - HandleTranscodeCallback - GetByID: %w", err)
	}

	wasComplete := v.Status == entity.VideoStatusComplete

	if status == "complete" {
		v.Status = entity.VideoStatusComplete
		if hlsMasterURL != "" {
			v.HLSUrl = hlsMasterURL
		}
	} else if status == "failed" {
		v.Status = entity.VideoStatusFailed
	}

	v.UpdatedAt = time.Now().UTC()
	if err := u.repo.Update(ctx, &v); err != nil {
		return err
	}

	// Notify followers once a public video is actually watchable — not at
	// upload time (PublishVideo's own notify only fires when a private/
	// unlisted video is switched to public, which doesn't cover the default
	// public-on-upload flow this transcode completion always goes through).
	if u.notifUc != nil && status == "complete" && !wasComplete && v.Visibility == entity.VideoVisibilityPublic {
		go func() {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = u.notifUc.NotifyFollowers(
				notifyCtx,
				v.UserID, v.UserName, v.UserAvatar,
				entity.NotificationTypeNewVideo,
				fmt.Sprintf("%s posted a new video", v.UserName),
				v.Title,
				fmt.Sprintf("/watch/%s", v.ID),
			)
		}()
	}

	return nil
}

