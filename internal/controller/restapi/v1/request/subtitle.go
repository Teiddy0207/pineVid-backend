package request

type UploadSubtitles struct {
	VTTEnglish   string `json:"vtt_en" validate:"required"`
	VTTVietnamese string `json:"vtt_vi"`
}
