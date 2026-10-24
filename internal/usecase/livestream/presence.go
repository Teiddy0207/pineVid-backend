package livestream

import (
	"context"
	"errors"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
)

var ErrPresenceUnavailable = errors.New("live presence unavailable")
var ErrStreamOffline = errors.New("stream is offline")
var ErrStaleBroadcast = errors.New("broadcast has changed")

func (u *UseCase) WithPresence(store repo.LivePresence) *UseCase { u.presence = store; return u }
func broadcastKey(ls entity.Livestream) string {
	return ls.ID + ":" + ls.StartedAt.UTC().Format(time.RFC3339Nano)
}

func (u *UseCase) ViewerPresence(ctx context.Context, id, action, tab, identity, broadcast string) (response.LiveViewersResponse, error) {
	ls, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return response.LiveViewersResponse{}, err
	}
	if ls.IsBanned || ls.OwnerBanned {
		return response.LiveViewersResponse{}, entity.ErrStreamBanned
	}
	if !ls.IsLive || ls.StartedAt == nil {
		return response.LiveViewersResponse{}, ErrStreamOffline
	}
	key := broadcastKey(ls)
	if broadcast != "" && broadcast != key {
		return response.LiveViewersResponse{}, ErrStaleBroadcast
	}
	if u.presence == nil {
		return response.LiveViewersResponse{}, ErrPresenceUnavailable
	}
	count, err := u.presence.Apply(ctx, key, action, tab, identity)
	if err != nil {
		return response.LiveViewersResponse{}, ErrPresenceUnavailable
	}
	return response.LiveViewersResponse{StreamID: id, Broadcast: key, ViewersCount: count}, nil
}

// Redis is authoritative; legacy DB counts are a best-effort fallback only.
func (u *UseCase) presenceCount(ctx context.Context, id string, startedAt *time.Time, live bool, fallback int64) int64 {
	if !live {
		return 0
	}
	if u.presence != nil && startedAt != nil {
		key := id + ":" + startedAt.UTC().Format(time.RFC3339Nano)
		if count, err := u.presence.Apply(ctx, key, "count", "", ""); err == nil {
			return count
		}
	}
	return fallback
}
