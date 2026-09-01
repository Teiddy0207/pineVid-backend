package subtitle

import (
	"context"
	"fmt"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/google/uuid"
)

type UseCase struct {
	repo      repo.SubtitleRepo
	videoRepo repo.VideoRepo
}

func New(r repo.SubtitleRepo, videoRepo repo.VideoRepo) *UseCase {
	return &UseCase{repo: r, videoRepo: videoRepo}
}

func (u *UseCase) GetSubtitles(ctx context.Context, videoID string) (response.VideoSubtitlesResponse, error) {
	cues, err := u.repo.GetByVideoID(ctx, videoID)
	if err != nil {
		return response.VideoSubtitlesResponse{}, fmt.Errorf("SubtitleUseCase - GetSubtitles: %w", err)
	}
	return response.VideoSubtitlesResponse{
		VideoID: videoID,
		Cues:    mapper.ToSubtitleCueResponses(cues),
	}, nil
}

// UploadSubtitles parses one or two bilingual WebVTT tracks (English
// required, Vietnamese optional) and replaces the video's stored cues. When
// both are given, cues are paired up in order — this assumes the two tracks
// were authored with matching cue counts/timing, which is how bilingual VTT
// pairs are normally produced (e.g. one human/machine translation pass per
// English cue). Cues beyond the shorter track's length keep an empty
// translation rather than erroring, since a slight mismatch shouldn't block
// the whole upload.
func (u *UseCase) UploadSubtitles(ctx context.Context, userID, videoID, vttEN, vttVI string) error {
	video, err := u.videoRepo.GetByID(ctx, videoID)
	if err != nil {
		return err
	}
	if video.UserID != userID {
		return entity.ErrVideoForbidden
	}

	enCues, err := parseVTT(vttEN)
	if err != nil {
		return fmt.Errorf("SubtitleUseCase - UploadSubtitles - parse English VTT: %w", err)
	}
	if len(enCues) == 0 {
		return fmt.Errorf("SubtitleUseCase - UploadSubtitles: no cues found in English VTT")
	}

	var viCues []vttCue
	if vttVI != "" {
		viCues, err = parseVTT(vttVI)
		if err != nil {
			return fmt.Errorf("SubtitleUseCase - UploadSubtitles - parse Vietnamese VTT: %w", err)
		}
	}

	cues := make([]entity.SubtitleCue, len(enCues))
	for i, en := range enCues {
		cue := entity.SubtitleCue{
			ID:       uuid.New().String(),
			StartSec: en.StartSec,
			EndSec:   en.EndSec,
			TextEN:   en.Text,
		}
		if i < len(viCues) {
			cue.TextVI = viCues[i].Text
		}
		cues[i] = cue
	}

	if err := u.repo.ReplaceCues(ctx, videoID, cues); err != nil {
		return fmt.Errorf("SubtitleUseCase - UploadSubtitles - ReplaceCues: %w", err)
	}

	return nil
}
