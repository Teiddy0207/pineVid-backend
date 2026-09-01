package vocabulary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/google/uuid"
)

type UseCase struct {
	repo repo.VocabularyRepo
}

func New(r repo.VocabularyRepo) *UseCase {
	return &UseCase{repo: r}
}

func (uc *UseCase) SaveWord(ctx context.Context, userID string, req request.SaveWordRequest) (response.VocabularyResponse, error) {
	item := entity.Vocabulary{
		ID:           uuid.New().String(),
		UserID:       userID,
		Word:         strings.TrimSpace(req.Word),
		IPA:          req.IPA,
		PartOfSpeech: req.PartOfSpeech,
		Meaning:      req.Meaning,
		Example:      req.Example,
		VideoID:      req.VideoID,
		VideoTitle:   req.VideoTitle,
		CreatedAt:    time.Now().UTC(),
	}

	if err := uc.repo.SaveWord(ctx, &item); err != nil {
		return response.VocabularyResponse{}, fmt.Errorf("VocabularyUseCase - SaveWord - repo.SaveWord: %w", err)
	}

	return response.VocabularyResponse{
		ID:           item.ID,
		UserID:       item.UserID,
		Word:         item.Word,
		IPA:          item.IPA,
		PartOfSpeech: item.PartOfSpeech,
		Meaning:      item.Meaning,
		Example:      item.Example,
		VideoID:      item.VideoID,
		VideoTitle:   item.VideoTitle,
		CreatedAt:    item.CreatedAt,
	}, nil
}

func (uc *UseCase) ListWords(ctx context.Context, userID string) ([]response.VocabularyResponse, error) {
	items, err := uc.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("VocabularyUseCase - ListWords - repo.ListByUserID: %w", err)
	}

	res := make([]response.VocabularyResponse, 0, len(items))
	for _, item := range items {
		res = append(res, response.VocabularyResponse{
			ID:           item.ID,
			UserID:       item.UserID,
			Word:         item.Word,
			IPA:          item.IPA,
			PartOfSpeech: item.PartOfSpeech,
			Meaning:      item.Meaning,
			Example:      item.Example,
			VideoID:      item.VideoID,
			VideoTitle:   item.VideoTitle,
			CreatedAt:    item.CreatedAt,
		})
	}
	return res, nil
}

func (uc *UseCase) DeleteWord(ctx context.Context, id, userID string) error {
	if err := uc.repo.DeleteWord(ctx, id, userID); err != nil {
		return fmt.Errorf("VocabularyUseCase - DeleteWord - repo.DeleteWord: %w", err)
	}
	return nil
}

type FreeDictEntry struct {
	Word      string `json:"word"`
	Phonetic  string `json:"phonetic"`
	Phonetics []struct {
		Text string `json:"text"`
	} `json:"phonetics"`
	Meanings []struct {
		PartOfSpeech string `json:"partOfSpeech"`
		Definitions  []struct {
			Definition string `json:"definition"`
			Example    string `json:"example"`
		} `json:"definitions"`
	} `json:"meanings"`
}

type MyMemoryResp struct {
	ResponseData struct {
		TranslatedText string `json:"translatedText"`
	} `json:"responseData"`
}

func translateText(ctx context.Context, text, langPair string) string {
	apiURL := fmt.Sprintf("https://api.mymemory.translated.net/get?q=%s&langpair=%s", url.QueryEscape(text), langPair)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return ""
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer resp.Body.Close()
	var res MyMemoryResp
	if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
		return strings.TrimSpace(res.ResponseData.TranslatedText)
	}
	return ""
}

func (uc *UseCase) LookupWord(ctx context.Context, word string) (response.DictionaryLookupResponse, error) {
	clean := strings.TrimSpace(word)
	if clean == "" {
		return response.DictionaryLookupResponse{}, fmt.Errorf("empty word")
	}

	lower := strings.ToLower(clean)

	// Perform 100% Live Dynamic Multi-Language Translations via MyMemory API
	trans := response.MultiLangTranslations{}

	var wg sync.WaitGroup
	wg.Add(8)

	go func() {
		defer wg.Done()
		trans.EN = translateText(ctx, clean, "AUTODETECT|en")
	}()

	go func() {
		defer wg.Done()
		trans.VI = translateText(ctx, clean, "AUTODETECT|vi")
		if trans.VI == "" {
			trans.VI = fmt.Sprintf("Từ vựng: \"%s\"", clean)
		}
	}()

	go func() {
		defer wg.Done()
		trans.JA = translateText(ctx, clean, "AUTODETECT|ja")
	}()

	go func() {
		defer wg.Done()
		trans.FR = translateText(ctx, clean, "AUTODETECT|fr")
	}()

	go func() {
		defer wg.Done()
		trans.ES = translateText(ctx, clean, "AUTODETECT|es")
	}()

	go func() {
		defer wg.Done()
		trans.DE = translateText(ctx, clean, "AUTODETECT|de")
	}()

	go func() {
		defer wg.Done()
		trans.ZH = translateText(ctx, clean, "AUTODETECT|zh")
	}()

	go func() {
		defer wg.Done()
		trans.KO = translateText(ctx, clean, "AUTODETECT|ko")
	}()

	wg.Wait()

	cleanTrans := func(val string) string {
		if val == "" || strings.Contains(strings.ToUpper(val), "PLEASE SELECT TWO DISTINCT LANGUAGES") || strings.Contains(val, "MYMEMORY") {
			return clean
		}
		return val
	}

	trans.EN = cleanTrans(trans.EN)
	trans.VI = cleanTrans(trans.VI)
	trans.JA = cleanTrans(trans.JA)
	trans.FR = cleanTrans(trans.FR)
	trans.ES = cleanTrans(trans.ES)
	trans.DE = cleanTrans(trans.DE)
	trans.ZH = cleanTrans(trans.ZH)
	trans.KO = cleanTrans(trans.KO)

	// Try Free Dictionary API for English entries (IPA phonetics and example sentence)
	ipa := fmt.Sprintf("/%s/", clean)
	partOfSpeech := "vocabulary"
	example := fmt.Sprintf("Context sentence featuring the word \"%s\".", clean)

	apiURL := fmt.Sprintf("https://api.dictionaryapi.dev/api/v2/entries/en/%s", url.QueryEscape(lower))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err == nil {
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			var entries []FreeDictEntry
			if err := json.NewDecoder(resp.Body).Decode(&entries); err == nil && len(entries) > 0 {
				entry := entries[0]
				if entry.Phonetic != "" {
					ipa = entry.Phonetic
				} else if len(entry.Phonetics) > 0 {
					for _, p := range entry.Phonetics {
						if p.Text != "" {
							ipa = p.Text
							break
						}
					}
				}

				if len(entry.Meanings) > 0 {
					m := entry.Meanings[0]
					partOfSpeech = m.PartOfSpeech
					if len(m.Definitions) > 0 && m.Definitions[0].Example != "" {
						example = m.Definitions[0].Example
					}
				}
			}
			resp.Body.Close()
		}
	}

	return response.DictionaryLookupResponse{
		Word:         clean,
		IPA:          ipa,
		PartOfSpeech: partOfSpeech,
		Meaning:      trans.VI,
		Example:      example,
		Translations: trans,
	}, nil
}
