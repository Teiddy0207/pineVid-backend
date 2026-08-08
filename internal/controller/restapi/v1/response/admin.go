package response

type SystemDashboardResponse struct {
	TotalVideos      int64 `json:"total_videos" example:"1420"`
	ActiveLivestreams int64 `json:"active_livestreams" example:"12"`
	ActiveWorkers    int64 `json:"active_workers" example:"4"`
	TotalViewers     int64 `json:"total_viewers" example:"35480"`
	BandwidthUsageGb float64 `json:"bandwidth_usage_gb" example:"128.4"`
}

type WorkerStatusResponse struct {
	WorkerID string  `json:"worker_id" example:"worker-node-1"`
	Status   string  `json:"status" example:"processing"`
	CurrentJob string `json:"current_job,omitempty" example:"job_vid_9421"`
	CpuUsage float64 `json:"cpu_usage" example:"42.5"`
	RamUsage float64 `json:"ram_usage" example:"68.1"`
}
