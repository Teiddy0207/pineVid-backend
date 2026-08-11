package entity

type UserVideoInteraction struct {
	UserID  string  `json:"user_id"`
	VideoID string  `json:"video_id"`
	Rating  float64 `json:"rating"`
}

type RecommendedVideo struct {
	Video          Video   `json:"video"`
	PredictedScore float64 `json:"predicted_score"`
}
