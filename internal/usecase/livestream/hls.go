package livestream

import (
	"context"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/pkg/srs"
)

func (u *UseCase) WithLiveHLS(enabled bool) *UseCase { u.liveHLS = enabled; return u }

func (u *UseCase) LiveMaster(ctx context.Context, id string) (string, error) {
	ls, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if ls.IsBanned || ls.OwnerBanned {
		return "", entity.ErrStreamBanned
	}
	if !ls.IsLive || !u.liveHLS || u.srsClient == nil {
		return "", srs.ErrHLSNotReady
	}
	return u.srsClient.Master(ctx, ls.StreamKey, ls.StartedAt)
}
