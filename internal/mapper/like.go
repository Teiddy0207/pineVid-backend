package mapper

import (
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
)

func ToLikeEntity(userID, videoID string) entity.VideoLike {
	return entity.VideoLike{
		ID:      userID + ":" + videoID,
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
