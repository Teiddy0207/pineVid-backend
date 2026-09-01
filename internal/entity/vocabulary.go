package entity

import (
	"errors"
	"time"
)

var ErrVocabularyNotFound = errors.New("vocabulary item not found")

type Vocabulary struct {
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
