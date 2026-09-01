package entity

import "time"

type SubtitleCue struct {
	ID        string  `json:"id"`
	StartSec  float64 `json:"start_sec"`
	EndSec    float64 `json:"end_sec"`
	TextEN    string  `json:"text_en"`
	TextVI    string  `json:"text_vi"`
}

type SubtitleTrack struct {
	ID        string        `json:"id"`
	VideoID   string        `json:"video_id"`
	Language  string        `json:"language"`
	Label     string        `json:"label"`
	VTTUrl    string        `json:"vtt_url"`
	Cues      []SubtitleCue `json:"cues,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}
