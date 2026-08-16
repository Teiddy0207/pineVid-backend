package request

type WorkerHeartbeat struct {
	WorkerID   string  `json:"worker_id" validate:"required"`
	Status     string  `json:"status" validate:"required,oneof=idle processing"`
	CurrentJob string  `json:"current_job"`
	CPUUsage   float64 `json:"cpu_usage"`
	RAMUsageMB float64 `json:"ram_usage_mb"`
}
