package comment

import (
	"context"
	"fmt"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/pkg/nats"
	"github.com/google/uuid"
)

type UseCase struct {
	repo          repo.CommentRepo
	natsPublisher *nats.Publisher
}

func New(r repo.CommentRepo, natsPub *nats.Publisher) *UseCase {
	return &UseCase{
		repo:          r,
		natsPublisher: natsPub,
	}
}

func (u *UseCase) CreateComment(ctx context.Context, videoID, userID, userName, userAvatar string, req request.CreateCommentRequest) (response.CommentResponse, error) {
	if req.ParentID != nil {
		parent, err := u.repo.GetByID(ctx, *req.ParentID)
		if err != nil || parent.VideoID != videoID {
			return response.CommentResponse{}, entity.ErrInvalidParentID
		}
	}

	commentID := uuid.New().String()
	c := mapper.ToCommentEntity(videoID, userID, userName, userAvatar, req, commentID)
	c.CreatedAt = time.Now().UTC()

	if err := u.repo.Store(ctx, &c); err != nil {
		return response.CommentResponse{}, fmt.Errorf("CommentUseCase - CreateComment - Store: %w", err)
	}

	return mapper.ToCommentResponse(c), nil
}

func (u *UseCase) ListVideoComments(ctx context.Context, videoID string, page, limit int) (response.PageResponse[response.CommentResponse], error) {
	offset := uint64((page - 1) * limit)
	comments, total, err := u.repo.ListByVideoID(ctx, videoID, uint64(limit), offset)
	if err != nil {
		return response.PageResponse[response.CommentResponse]{}, fmt.Errorf("CommentUseCase - ListVideoComments: %w", err)
	}

	return mapper.ToCommentPageResponse(comments, total, page, limit), nil
}

func (u *UseCase) ListReplies(ctx context.Context, parentID string, page, limit int) (response.PageResponse[response.CommentResponse], error) {
	offset := uint64((page - 1) * limit)
	replies, total, err := u.repo.ListRepliesByParentID(ctx, parentID, uint64(limit), offset)
	if err != nil {
		return response.PageResponse[response.CommentResponse]{}, fmt.Errorf("CommentUseCase - ListReplies: %w", err)
	}

	return mapper.ToCommentPageResponse(replies, total, page, limit), nil
}
