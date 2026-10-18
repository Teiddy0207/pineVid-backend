package mapper

import (
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/google/uuid"
)

// ToLikeEntity builds a new video like. ID must be a plain UUID: likes.id is
// VARCHAR(64), so the old "userID:videoID" key (73 chars) made every INSERT
// fail. Uniqueness per user/video is enforced by unique_user_video_like.
func ToLikeEntity(userID, videoID string) entity.VideoLike {
	return entity.VideoLike{
		ID:      uuid.New().String(),
		VideoID: videoID,
		UserID:  userID,
	}
}

func ToLikeResponse(videoID string, liked bool, totalLikes int64) response.LikeResponse {
	return response.LikeResponse{
		VideoID:    videoID,
		Liked:      liked,
		TotalLikes: totalLikes,
	}
}

func ToHeartResponse(streamID string, totalHearts int64) response.HeartResponse {
	return response.HeartResponse{
		StreamID:    streamID,
		TotalHearts: totalHearts,
	}
}
