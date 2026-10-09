package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
)

// workerStaleAfter is how long a transcode worker can go without sending a
// heartbeat before it's dropped from the "active" list (e.g. crashed/killed).
const workerStaleAfter = 30 * time.Second

// StreamKicker force-disconnects a live RTMP publisher (livestream usecase).
type StreamKicker interface {
	KickStream(ctx context.Context, streamKey string) error
}

// SessionManager makes an account ban take effect immediately (user usecase):
// drop the cached ban state read by the auth middleware and revoke refresh
// tokens so the account can't mint new access tokens.
type SessionManager interface {
	InvalidateBanCache(userID string)
	RevokeSessions(ctx context.Context, userID string) error
}

type UseCase struct {
	lsRepo    repo.LivestreamRepo
	vRepo     repo.VideoRepo
	uRepo     repo.UserRepo
	wRepo     repo.WorkerRepo
	statsRepo repo.AdminStatsRepo
	kicker    StreamKicker
	sessions  SessionManager
}

// New -. kicker and sessions may be nil (tests); the corresponding side
// effects are then skipped.
func New(ls repo.LivestreamRepo, v repo.VideoRepo, u repo.UserRepo, w repo.WorkerRepo, stats repo.AdminStatsRepo, kicker StreamKicker, sessions SessionManager) *UseCase {
	return &UseCase{lsRepo: ls, vRepo: v, uRepo: u, wRepo: w, statsRepo: stats, kicker: kicker, sessions: sessions}
}

func (u *UseCase) GetDashboard(ctx context.Context) (response.SystemDashboardResponse, error) {
	_, totalVideos, err := u.vRepo.List(ctx, repo.VideoFilter{Limit: 1})
	if err != nil {
		return response.SystemDashboardResponse{}, fmt.Errorf("AdminUseCase - GetDashboard - vRepo.List: %w", err)
	}

	activeLivestreams, err := u.lsRepo.CountActive(ctx)
	if err != nil {
		return response.SystemDashboardResponse{}, fmt.Errorf("AdminUseCase - GetDashboard - CountActive: %w", err)
	}

	totalViewers, err := u.lsRepo.SumActiveViewers(ctx)
	if err != nil {
		return response.SystemDashboardResponse{}, fmt.Errorf("AdminUseCase - GetDashboard - SumActiveViewers: %w", err)
	}

	activeWorkers, err := u.wRepo.ListActive(ctx, workerStaleAfter)
	if err != nil {
		return response.SystemDashboardResponse{}, fmt.Errorf("AdminUseCase - GetDashboard - wRepo.ListActive: %w", err)
	}

	channelStats, err := u.statsRepo.ChannelStats(ctx)
	if err != nil {
		return response.SystemDashboardResponse{}, fmt.Errorf("AdminUseCase - GetDashboard - statsRepo.ChannelStats: %w", err)
	}

	categoryBreakdown, err := u.statsRepo.CategoryBreakdown(ctx)
	if err != nil {
		return response.SystemDashboardResponse{}, fmt.Errorf("AdminUseCase - GetDashboard - statsRepo.CategoryBreakdown: %w", err)
	}

	categoryResp := make([]response.CategoryStatResponse, len(categoryBreakdown))
	for i, c := range categoryBreakdown {
		categoryResp[i] = response.CategoryStatResponse{Category: c.Category, VideoCount: c.VideoCount}
	}

	return response.SystemDashboardResponse{
		TotalVideos:       int64(totalVideos),
		ActiveLivestreams: activeLivestreams,
		ActiveWorkers:     int64(len(activeWorkers)),
		TotalViewers:      totalViewers,
		TotalChannels:     channelStats.TotalChannels,
		ActiveChannels:    channelStats.ActiveChannels,
		BannedChannels:    channelStats.BannedChannels,
		CategoryBreakdown: categoryResp,
	}, nil
}

func (u *UseCase) GetWorkersStatus(ctx context.Context) ([]response.WorkerStatusResponse, error) {
	workers, err := u.wRepo.ListActive(ctx, workerStaleAfter)
	if err != nil {
		return nil, fmt.Errorf("AdminUseCase - GetWorkersStatus - wRepo.ListActive: %w", err)
	}

	res := make([]response.WorkerStatusResponse, len(workers))
	for i, w := range workers {
		res[i] = response.WorkerStatusResponse{
			WorkerID:   w.WorkerID,
			Status:     w.Status,
			CurrentJob: w.CurrentJob,
			CpuUsage:   w.CPUUsage,
			RamUsage:   w.RAMUsageMB,
		}
	}
	return res, nil
}

// RecordHeartbeat is called by transcode worker processes (see
// transcode/internal/worker/heartbeat.go) to self-report their live status.
func (u *UseCase) RecordHeartbeat(ctx context.Context, hb entity.WorkerHeartbeat) error {
	return u.wRepo.UpsertHeartbeat(ctx, hb)
}

// BanStream takes a channel's stream down: it is marked banned (so on_publish
// rejects the key from now on) and the current RTMP publisher is kicked.
func (u *UseCase) BanStream(ctx context.Context, streamID string) error {
	return u.setStreamBanned(ctx, streamID, true)
}

// UnbanStream lets the channel go live again.
func (u *UseCase) UnbanStream(ctx context.Context, streamID string) error {
	return u.setStreamBanned(ctx, streamID, false)
}

func (u *UseCase) setStreamBanned(ctx context.Context, streamID string, banned bool) error {
	ls, err := u.lsRepo.GetByID(ctx, streamID)
	if err != nil {
		return err
	}
	ls.IsBanned = banned
	if banned {
		ls.IsLive = false
	}
	ls.UpdatedAt = time.Now().UTC()
	if err := u.lsRepo.Update(ctx, &ls); err != nil {
		return err
	}

	if banned {
		u.kickStream(ctx, ls.StreamKey)
	}
	return nil
}

func (u *UseCase) kickStream(ctx context.Context, streamKey string) {
	if u.kicker == nil || streamKey == "" {
		return
	}
	// Best-effort: the DB ban already blocks reconnects even if SRS is
	// unreachable right now.
	_ = u.kicker.KickStream(ctx, streamKey)
}

// BanVideo takes a video down (status = banned). It vanishes from every
// listing and returns 404 to everyone but its owner; a later transcode
// callback cannot revive it.
func (u *UseCase) BanVideo(ctx context.Context, videoID string) error {
	if _, err := u.vRepo.GetByID(ctx, videoID); err != nil {
		return err
	}
	return u.vRepo.SetStatus(ctx, videoID, entity.VideoStatusBanned, time.Now().UTC())
}

func (u *UseCase) ListUsers(ctx context.Context, page, limit int) (response.PageResponse[response.UserResponse], error) {
	users, total, err := u.uRepo.List(ctx, page, limit)
	if err != nil {
		return response.PageResponse[response.UserResponse]{}, err
	}
	return mapper.ToUserPageResponse(users, total, page, limit), nil
}

func (u *UseCase) BanUser(ctx context.Context, userID string) error {
	return u.setUserBanned(ctx, userID, true)
}

func (u *UseCase) UnbanUser(ctx context.Context, userID string) error {
	return u.setUserBanned(ctx, userID, false)
}

func (u *UseCase) setUserBanned(ctx context.Context, userID string, banned bool) error {
	usr, err := u.uRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	usr.IsBanned = banned
	usr.UpdatedAt = time.Now().UTC()
	if err := u.uRepo.Update(ctx, &usr); err != nil {
		return err
	}

	if u.sessions != nil {
		u.sessions.InvalidateBanCache(userID)
		if banned {
			_ = u.sessions.RevokeSessions(ctx, userID)
		}
	}

	// A banned user who is live right now gets cut off immediately
	// (on_publish then rejects them via OwnerBanned).
	if banned {
		if ls, err := u.lsRepo.GetByUserID(ctx, userID); err == nil && ls.IsLive {
			ls.IsLive = false
			ls.UpdatedAt = time.Now().UTC()
			_ = u.lsRepo.Update(ctx, &ls)
			u.kickStream(ctx, ls.StreamKey)
		}
	}
	return nil
}
