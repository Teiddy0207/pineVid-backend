package like

import (
	"context"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHeartWithoutCounterReturnsError(t *testing.T) {
	r := &Repo{}
	total, err := r.IncrementHeart(context.Background(), "room")
	require.ErrorIs(t, err, entity.ErrLiveEventsUnavailable)
	require.Zero(t, total)
}
