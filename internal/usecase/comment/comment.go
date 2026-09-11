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
	likeRepo      repo.CommentLikeRepo
	natsPublisher *nats.Publisher
}

func New(r repo.CommentRepo, likeRepo repo.CommentLikeRepo, natsPub *nats.Publisher) *UseCase {
	return &UseCase{
		repo:          r,
		likeRepo:      likeRepo,
		natsPublisher: natsPub,
	}
}

// withLikeInfo populates LikeCount/IsLiked on each comment in place — done
// as a post-mapping pass (like video.GetByID does for likes_count/is_liked)
// since the mapper package has no DB access of its own.
func (u *UseCase) withLikeInfo(ctx context.Context, comments []response.CommentResponse, userID string) []response.CommentResponse {
	if u.likeRepo == nil {
		return comments
	}
	for i := range comments {
		if count, err := u.likeRepo.GetLikeCount(ctx, comments[i].ID); err == nil {
			comments[i].LikeCount = count
		}
		if liked, err := u.likeRepo.IsLikedByUser(ctx, comments[i].ID, userID); err == nil {
			comments[i].IsLiked = liked
		}
	}
	return comments
}

func (u *UseCase) ToggleLikeComment(ctx context.Context, commentID, userID string) (response.CommentLikeResponse, error) {
	liked, total, err := u.likeRepo.ToggleLike(ctx, commentID, userID)
	if err != nil {
		return response.CommentLikeResponse{}, fmt.Errorf("CommentUseCase - ToggleLikeComment: %w", err)
	}
	return response.CommentLikeResponse{CommentID: commentID, Liked: liked, TotalLikes: total}, nil
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

func (u *UseCase) ListVideoComments(ctx context.Context, videoID, userID string, page, limit int) (response.CommentPageResponse, error) {
	offset := uint64((page - 1) * limit)
	comments, total, err := u.repo.ListByVideoID(ctx, videoID, uint64(limit), offset)
	if err != nil {
		return response.CommentPageResponse{}, fmt.Errorf("CommentUseCase - ListVideoComments: %w", err)
	}

	totalAll, err := u.repo.CountAllByVideoID(ctx, videoID)
	if err != nil {
		return response.CommentPageResponse{}, fmt.Errorf("CommentUseCase - ListVideoComments - CountAllByVideoID: %w", err)
	}

	pageRes := mapper.ToCommentPageResponseWithTotal(comments, total, totalAll, page, limit)
	pageRes.Data = u.withLikeInfo(ctx, pageRes.Data, userID)
	return pageRes, nil
}

func (u *UseCase) ListReplies(ctx context.Context, parentID, userID string, page, limit int) (response.PageResponse[response.CommentResponse], error) {
	offset := uint64((page - 1) * limit)
	replies, total, err := u.repo.ListRepliesByParentID(ctx, parentID, uint64(limit), offset)
	if err != nil {
		return response.PageResponse[response.CommentResponse]{}, fmt.Errorf("CommentUseCase - ListReplies: %w", err)
	}

	pageRes := mapper.ToCommentPageResponse(replies, total, page, limit)
	pageRes.Data = u.withLikeInfo(ctx, pageRes.Data, userID)
	return pageRes, nil
}
