package response

import "time"

type VocabularyResponse struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Word         string    `json:"word"`
	IPA          string    `json:"ipa"`
	PartOfSpeech string    `json:"part_of_speech"`
	Meaning      string    `json:"meaning"`
	Example      string    `json:"example"`
	VideoID      string    `json:"video_id"`
	VideoTitle   string    `json:"video_title"`
	CreatedAt    time.Time `json:"created_at"`
}

type MultiLangTranslations struct {
	EN string `json:"en"`
	VI string `json:"vi"`
	JA string `json:"ja"`
	FR string `json:"fr"`
	ES string `json:"es"`
	DE string `json:"de"`
	ZH string `json:"zh"`
	KO string `json:"ko"`
}

type DictionaryLookupResponse struct {
	Word         string                `json:"word"`
	IPA          string                `json:"ipa"`
	PartOfSpeech string                `json:"part_of_speech"`
	Meaning      string                `json:"meaning"`
	Example      string                `json:"example"`
	Translations MultiLangTranslations `json:"translations"`
}
