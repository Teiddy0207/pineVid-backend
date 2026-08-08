package video

import (
	"context"
	"fmt"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/google/uuid"
)

type UseCase struct {
	repo repo.VideoRepo
}

func New(r repo.VideoRepo) *UseCase {
	return &UseCase{repo: r}
}

func (u *UseCase) CreateUpload(ctx context.Context, userID string, req request.CreateVideoUpload) (response.UploadUrlResponse, error) {
	videoID := uuid.New().String()
	s3Key := fmt.Sprintf("raw-uploads/%s/%s", videoID, req.FileName)

	v := mapper.ToVideoEntity(userID, req, videoID, s3Key)
	v.CreatedAt = time.Now().UTC()
	v.UpdatedAt = time.Now().UTC()

	if err := u.repo.Store(ctx, &v); err != nil {
		return response.UploadUrlResponse{}, fmt.Errorf("VideoUseCase - CreateUpload - Store: %w", err)
	}

	// Presigned S3 upload URL mock (In production, generate via minio/s3 SDK)
	presignedUrl := fmt.Sprintf("https://s3.pipevid.com/upload/%s", s3Key)

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

	// TODO: Publish NATS JetStream event 'video.transcode' for Transcode Worker

	return mapper.ToVideoResponse(v), nil
}

func (u *UseCase) GetByID(ctx context.Context, id string) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return response.VideoResponse{}, err
	}
	return mapper.ToVideoResponse(v), nil
}

func (u *UseCase) ListPublicVideos(ctx context.Context, category string, page, limit int) (response.PageResponse[response.VideoResponse], error) {
	offset := (page - 1) * limit
	status := entity.VideoStatusComplete
	visibility := entity.VideoVisibilityPublic

	filter := repo.VideoFilter{
		Category:   category,
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

	v.Visibility = entity.VideoVisibilityPublic
	v.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &v); err != nil {
		return response.VideoResponse{}, fmt.Errorf("VideoUseCase - PublishVideo - Update: %w", err)
	}

	return mapper.ToVideoResponse(v), nil
}

func (u *UseCase) UpdateVideo(ctx context.Context, userID, videoID string, req request.UpdateVideo) (response.VideoResponse, error) {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return response.VideoResponse{}, err
	}

	if req.Title != "" {
		v.Title = req.Title
	}
	if req.Description != "" {
		v.Description = req.Description
	}
	if req.Category != "" {
		v.Category = req.Category
	}
	if req.Visibility != "" {
		v.Visibility = entity.VideoVisibility(req.Visibility)
	}
	v.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &v); err != nil {
		return response.VideoResponse{}, fmt.Errorf("VideoUseCase - UpdateVideo - Update: %w", err)
	}

	return mapper.ToVideoResponse(v), nil
}

func (u *UseCase) DeleteVideo(ctx context.Context, userID, videoID string) error {
	return u.repo.Delete(ctx, videoID)
}

func (u *UseCase) HandleTranscodeCallback(ctx context.Context, videoID, status, hlsMasterURL string) error {
	v, err := u.repo.GetByID(ctx, videoID)
	if err != nil {
		return fmt.Errorf("VideoUseCase - HandleTranscodeCallback - GetByID: %w", err)
	}

	if status == "complete" {
		v.Status = entity.VideoStatusComplete
		if hlsMasterURL != "" {
			v.HLSUrl = hlsMasterURL
		}
	} else if status == "failed" {
		v.Status = entity.VideoStatusFailed
	}

	v.UpdatedAt = time.Now().UTC()
	return u.repo.Update(ctx, &v)
}

