package mapper

import (
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
)

func ToSubtitleCueResponses(cues []entity.SubtitleCue) []response.SubtitleCueResponse {
	res := make([]response.SubtitleCueResponse, len(cues))
	for i, c := range cues {
		res[i] = response.SubtitleCueResponse{
			ID:       c.ID,
			StartSec: c.StartSec,
			EndSec:   c.EndSec,
			TextEN:   c.TextEN,
			TextVI:   c.TextVI,
		}
	}
	return res
}
