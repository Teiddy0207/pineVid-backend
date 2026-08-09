package response

// Error -.
type Error struct {
	Success bool   `json:"success" example:"false"`
	Code    int    `json:"code" example:"400"`
	Error   string `json:"error" example:"message"`
} // @name v1.Error
