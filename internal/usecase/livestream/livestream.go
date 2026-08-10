package livestream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/events"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/google/uuid"
)

type UseCase struct {
	repo    repo.LivestreamRepo
	chatHub *events.ChatHub
}

func New(r repo.LivestreamRepo, chatHub *events.ChatHub) *UseCase {
	return &UseCase{repo: r, chatHub: chatHub}
}

func (u *UseCase) GetStreamKey(ctx context.Context, userID string) (response.StreamKeyResponse, error) {
	ls, err := u.repo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrLivestreamNotFound) {
			// Auto create livestream record for user if not exists
			newStreamKey := fmt.Sprintf("sk_live_%s", uuid.New().String()[:18])
			newLs := entity.Livestream{
				ID:        uuid.New().String(),
				UserID:    userID,
				StreamKey: newStreamKey,
				Title:     "My Livestream",
				Category:  "General",
				IsLive:    false,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}
			if err := u.repo.Store(ctx, &newLs); err != nil {
				return response.StreamKeyResponse{}, fmt.Errorf("LivestreamUseCase - GetStreamKey - Store: %w", err)
			}
			ls = newLs
		} else {
			return response.StreamKeyResponse{}, err
		}
	}

	return response.StreamKeyResponse{
		ServerUrl: "rtmp://live.pipevid.com/live",
		StreamKey: ls.StreamKey,
	}, nil
}

func (u *UseCase) ResetStreamKey(ctx context.Context, userID string) (response.StreamKeyResponse, error) {
	ls, err := u.repo.GetByUserID(ctx, userID)
	if err != nil {
		return response.StreamKeyResponse{}, err
	}

	ls.StreamKey = fmt.Sprintf("sk_live_%s", uuid.New().String()[:18])
	ls.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &ls); err != nil {
		return response.StreamKeyResponse{}, fmt.Errorf("LivestreamUseCase - ResetStreamKey - Update: %w", err)
	}

	return response.StreamKeyResponse{
		ServerUrl: "rtmp://live.pipevid.com/live",
		StreamKey: ls.StreamKey,
	}, nil
}

func (u *UseCase) AuthenticateStreamKey(ctx context.Context, req request.StreamKeyAuth) (bool, error) {
	ls, err := u.repo.GetByStreamKey(ctx, req.StreamKey)
	if err != nil {
		return false, entity.ErrInvalidStreamKey
	}

	// Mark stream as live
	now := time.Now().UTC()
	ls.IsLive = true
	ls.StartedAt = &now
	ls.HLSUrl = fmt.Sprintf("http://localhost:9000/hls-streams/live/%s/master.m3u8", ls.StreamKey)
	ls.UpdatedAt = now

	_ = u.repo.Update(ctx, &ls)

	return true, nil
}

func (u *UseCase) GetStreamByID(ctx context.Context, id string) (response.LivestreamResponse, error) {
	ls, err := u.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, entity.ErrLivestreamNotFound) {
			now := time.Now().UTC()
			mockLs := entity.Livestream{
				ID:           id,
				UserID:       "usr_demo",
				Title:        "Exploring the edge of the universe",
				Category:     "Science",
				IsLive:       true,
				HLSUrl:       "http://localhost:8082/live/sk_live_8h2k_92md_71px.m3u8",
				ViewersCount: 0,
				StartedAt:    &now,
			}
			if id == "2" {
				mockLs.Title = "Ranked grind · road to Radiant"
				mockLs.Category = "Gaming"
				mockLs.ViewersCount = 0
			} else if id == "3" {
				mockLs.Title = "Late night studio session"
				mockLs.Category = "Music"
				mockLs.ViewersCount = 0
			}
			return mapper.ToLivestreamResponse(mockLs), nil
		}
		return response.LivestreamResponse{}, err
	}
	return mapper.ToLivestreamResponse(ls), nil
}

func (u *UseCase) ListActiveStreams(ctx context.Context, category string, page, limit int) (response.PageResponse[response.LivestreamResponse], error) {
	offset := (page - 1) * limit
	streams, total, err := u.repo.ListActive(ctx, category, limit, offset)
	if err != nil {
		return response.PageResponse[response.LivestreamResponse]{}, err
	}

	return mapper.ToLivestreamPageResponse(streams, total, page, limit), nil
}

func (u *UseCase) UpdateStreamInfo(ctx context.Context, userID string, req request.UpdateLivestreamInfo) (response.LivestreamResponse, error) {
	ls, err := u.repo.GetByUserID(ctx, userID)
	if err != nil {
		return response.LivestreamResponse{}, err
	}

	if req.Title != "" {
		ls.Title = req.Title
	}
	if req.Category != "" {
		ls.Category = req.Category
	}
	ls.UpdatedAt = time.Now().UTC()

	if err := u.repo.Update(ctx, &ls); err != nil {
		return response.LivestreamResponse{}, fmt.Errorf("LivestreamUseCase - UpdateStreamInfo - Update: %w", err)
	}

	return mapper.ToLivestreamResponse(ls), nil
}

func (u *UseCase) SendChatMessage(ctx context.Context, streamID string, req request.SendChatMessage) (response.ChatMessageResponse, error) {
	if req.Username == "" {
		req.Username = "Guest"
	}
	if req.Avatar == "" {
		req.Avatar = req.Username[0:1]
	}

	msg := events.ChatMessage{
		StreamID:  streamID,
		Username:  req.Username,
		Avatar:    req.Avatar,
		Text:      req.Text,
		CreatedAt: time.Now().Format("15:04:05"),
	}

	if u.chatHub != nil {
		u.chatHub.Broadcast(streamID, msg)
	}

	return mapper.ToChatMessageResponse(msg), nil
}

func (u *UseCase) SubscribeChat(streamID string) (<-chan events.ChatMessage, func(), error) {
	if u.chatHub == nil {
		return nil, nil, errors.New("chat hub unavailable")
	}
	ch, unsub := u.chatHub.Subscribe(streamID)
	return ch, unsub, nil
}
