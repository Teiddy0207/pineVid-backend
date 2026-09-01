package request

type SaveWordRequest struct {
	Word         string `json:"word"           validate:"required"`
	IPA          string `json:"ipa"`
	PartOfSpeech string `json:"part_of_speech"`
	Meaning      string `json:"meaning"        validate:"required"`
	Example      string `json:"example"`
	VideoID      string `json:"video_id"`
	VideoTitle   string `json:"video_title"`
}
