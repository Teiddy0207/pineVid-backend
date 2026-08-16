package response

// SystemDashboardResponse. Note: there's no bandwidth-usage field — this
// system has no instrumentation for measured network egress (no CDN/proxy
// byte-accounting), so rather than fabricate a number, it's simply omitted
// until real metering exists.
type SystemDashboardResponse struct {
	TotalVideos       int64 `json:"total_videos" example:"1420"`
	ActiveLivestreams int64 `json:"active_livestreams" example:"12"`
	ActiveWorkers     int64 `json:"active_workers" example:"4"`
	TotalViewers      int64 `json:"total_viewers" example:"35480"`
}

type WorkerStatusResponse struct {
	WorkerID string  `json:"worker_id" example:"worker-node-1"`
	Status   string  `json:"status" example:"processing"`
	CurrentJob string `json:"current_job,omitempty" example:"job_vid_9421"`
	CpuUsage float64 `json:"cpu_usage" example:"42.5"`
	RamUsage float64 `json:"ram_usage" example:"68.1"`
}
