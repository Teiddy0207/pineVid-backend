package post

import (
	"context"
	"fmt"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/google/uuid"
)

type UseCase struct {
	repo     repo.PostRepo
	likeRepo repo.PostLikeRepo
	notifUc  usecase.Notification
}

func New(r repo.PostRepo, likeRepo repo.PostLikeRepo, notifUc usecase.Notification) *UseCase {
	return &UseCase{
		repo:     r,
		likeRepo: likeRepo,
		notifUc:  notifUc,
	}
}

// withLikeInfo populates LikeCount/IsLiked on each post in place — a
// post-mapping pass, same as CommentUseCase.withLikeInfo, since the mapper
// package has no DB access of its own.
func (u *UseCase) withLikeInfo(ctx context.Context, posts []response.PostResponse, viewerID string) ([]response.PostResponse, error) {
	if u.likeRepo == nil {
		return posts, nil
	}
	for i := range posts {
		count, err := u.likeRepo.GetLikeCount(ctx, posts[i].ID)
		if err != nil {
			return nil, fmt.Errorf("PostUseCase - GetLikeCount: %w", err)
		}
		liked, err := u.likeRepo.IsLikedByUser(ctx, posts[i].ID, viewerID)
		if err != nil {
			return nil, fmt.Errorf("PostUseCase - IsLikedByUser: %w", err)
		}
		posts[i].LikeCount, posts[i].IsLiked = count, liked
	}
	return posts, nil
}

func (u *UseCase) CreatePost(ctx context.Context, userID, userName, userAvatar string, req request.CreatePostRequest) (response.PostResponse, error) {
	postID := uuid.New().String()
	p := mapper.ToPostEntity(userID, userName, userAvatar, req, postID)
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now

	if err := u.repo.Store(ctx, &p); err != nil {
		return response.PostResponse{}, fmt.Errorf("PostUseCase - CreatePost - Store: %w", err)
	}

	// Best-effort, fire-and-forget — never let a notification failure affect
	// the post itself. Mirrors VideoUseCase.PublishVideo's NotifyFollowers call.
	if u.notifUc != nil {
		go func() {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = u.notifUc.NotifyFollowers(
				notifyCtx,
				p.UserID, p.UserName, p.UserAvatar,
				entity.NotificationTypeNewPost,
				fmt.Sprintf("%s posted an update", p.UserName),
				p.Content,
				fmt.Sprintf("/channels/%s", p.UserID),
			)
		}()
	}

	return mapper.ToPostResponse(p), nil
}

func (u *UseCase) ListPostsByUser(ctx context.Context, userID, viewerID string, page, limit int) (response.PageResponse[response.PostResponse], error) {
	offset := uint64((page - 1) * limit)
	posts, total, err := u.repo.ListByUser(ctx, userID, uint64(limit), offset)
	if err != nil {
		return response.PageResponse[response.PostResponse]{}, fmt.Errorf("PostUseCase - ListPostsByUser: %w", err)
	}

	pageRes := mapper.ToPostPageResponse(posts, total, page, limit)
	pageRes.Data, err = u.withLikeInfo(ctx, pageRes.Data, viewerID)
	if err != nil {
		return response.PageResponse[response.PostResponse]{}, err
	}
	return pageRes, nil
}

func (u *UseCase) TogglePostLike(ctx context.Context, postID, userID string) (response.PostLikeResponse, error) {
	if _, err := u.repo.GetByID(ctx, postID); err != nil {
		return response.PostLikeResponse{}, err
	}
	liked, total, err := u.likeRepo.ToggleLike(ctx, postID, userID)
	if err != nil {
		return response.PostLikeResponse{}, fmt.Errorf("PostUseCase - TogglePostLike: %w", err)
	}
	return response.PostLikeResponse{PostID: postID, Liked: liked, TotalLikes: total}, nil
}

func (u *UseCase) DeletePost(ctx context.Context, userID, postID string) error {
	p, err := u.repo.GetByID(ctx, postID)
	if err != nil {
		return err
	}
	if p.UserID != userID {
		return entity.ErrPostForbidden
	}
	return u.repo.Delete(ctx, postID)
}
