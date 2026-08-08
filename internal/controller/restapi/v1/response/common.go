package response

// Response is the standard generic response structure for single objects.
type Response[T any] struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message,omitempty" example:"Success"`
	Data    T      `json:"data"`
}

// PaginationMeta contains pagination metadata.
type PaginationMeta struct {
	TotalItems  int `json:"total_items" example:"142"`
	TotalPages  int `json:"total_pages" example:"15"`
	CurrentPage int `json:"current_page" example:"1"`
	Limit       int `json:"limit" example:"10"`
}

// PageResponse is the standard generic response structure for paginated lists.
type PageResponse[T any] struct {
	Success    bool           `json:"success" example:"true"`
	Data       []T            `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}
