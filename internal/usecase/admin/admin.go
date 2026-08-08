package admin

import (
	"context"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
)

type UseCase struct {
	lsRepo repo.LivestreamRepo
	vRepo  repo.VideoRepo
}

func New(ls repo.LivestreamRepo, v repo.VideoRepo) *UseCase {
	return &UseCase{lsRepo: ls, vRepo: v}
}

func (u *UseCase) GetDashboard(ctx context.Context) (response.SystemDashboardResponse, error) {
	return response.SystemDashboardResponse{
		TotalVideos:       1420,
		ActiveLivestreams: 12,
		ActiveWorkers:     4,
		TotalViewers:      35480,
		BandwidthUsageGb:  128.4,
	}, nil
}

func (u *UseCase) GetWorkersStatus(ctx context.Context) ([]response.WorkerStatusResponse, error) {
	return []response.WorkerStatusResponse{
		{WorkerID: "worker-node-1", Status: "processing", CurrentJob: "job_vid_9421", CpuUsage: 42.5, RamUsage: 68.1},
		{WorkerID: "worker-node-2", Status: "idle", CpuUsage: 5.2, RamUsage: 22.0},
		{WorkerID: "worker-node-3", Status: "processing", CurrentJob: "job_vid_9422", CpuUsage: 78.4, RamUsage: 81.5},
		{WorkerID: "worker-node-4", Status: "idle", CpuUsage: 4.1, RamUsage: 19.8},
	}, nil
}

func (u *UseCase) BanStream(ctx context.Context, streamID string) error {
	ls, err := u.lsRepo.GetByID(ctx, streamID)
	if err != nil {
		return err
	}
	ls.IsLive = false
	ls.UpdatedAt = time.Now().UTC()
	return u.lsRepo.Update(ctx, &ls)
}

func (u *UseCase) BanVideo(ctx context.Context, videoID string) error {
	v, err := u.vRepo.GetByID(ctx, videoID)
	if err != nil {
		return err
	}
	v.Visibility = entity.VideoVisibilityPrivate
	v.UpdatedAt = time.Now().UTC()
	return u.vRepo.Update(ctx, &v)
}
