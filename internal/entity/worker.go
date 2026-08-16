package entity

import "time"

// WorkerHeartbeat is the last-known status a transcode worker process
// self-reported (see transcode/internal/worker heartbeat reporter).
type WorkerHeartbeat struct {
	WorkerID      string    `json:"worker_id"`
	Status        string    `json:"status"` // "idle" or "processing"
	CurrentJob    string    `json:"current_job"`
	CPUUsage      float64   `json:"cpu_usage"`
	RAMUsageMB    float64   `json:"ram_usage_mb"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}
