package response

type RecommendedVideoItem struct {
	Video          VideoResponse `json:"video"`
	PredictedScore float64       `json:"predicted_score"`
}
