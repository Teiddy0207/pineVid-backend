package recommendation

import (
	"context"
	"fmt"
	"math/rand"
	"sort"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
	persistRecRepo "github.com/evrone/go-clean-template/internal/repo/persistent/recommendation"
)

type UseCase struct {
	repo      *persistRecRepo.Repo
	videoRepo repo.VideoRepo
	factors   int
	gamma     float64 // Learning rate
	lambda    float64 // Regularization
	epochs    int
}

func New(r *persistRecRepo.Repo, vr repo.VideoRepo) *UseCase {
	return &UseCase{
		repo:      r,
		videoRepo: vr,
		factors:   10,
		gamma:     0.01,
		lambda:    0.02,
		epochs:    50,
	}
}

// GetPersonalizedFeed runs Matrix Factorization with SGD and returns top-ranked videos for user
func (u *UseCase) GetPersonalizedFeed(ctx context.Context, userID string, page, limit int) (response.PageResponse[response.RecommendedVideoItem], error) {
	interactions, err := u.repo.FetchInteractions(ctx)
	if err != nil {
		return response.PageResponse[response.RecommendedVideoItem]{}, fmt.Errorf("RecommendationUseCase - FetchInteractions: %w", err)
	}

	status := entity.VideoStatusComplete
	visibility := entity.VideoVisibilityPublic
	allVideos, totalVideos, err := u.videoRepo.List(ctx, repo.VideoFilter{Status: &status, Visibility: &visibility, Limit: 50})
	if err != nil {
		return response.PageResponse[response.RecommendedVideoItem]{}, fmt.Errorf("RecommendationUseCase - List: %w", err)
	}

	if len(interactions) == 0 {
		// Fallback for cold-start users
		recs := make([]entity.RecommendedVideo, len(allVideos))
		for i, v := range allVideos {
			recs[i] = entity.RecommendedVideo{Video: v, PredictedScore: 1.0}
		}
		return mapper.ToPersonalizedFeedPageResponse(recs, totalVideos, page, limit), nil
	}

	// 1. Index users and videos to matrix IDs
	userMap := make(map[string]int)
	videoMap := make(map[string]int)

	for _, inter := range interactions {
		if _, ok := userMap[inter.UserID]; !ok {
			userMap[inter.UserID] = len(userMap)
		}
		if _, ok := videoMap[inter.VideoID]; !ok {
			videoMap[inter.VideoID] = len(videoMap)
		}
	}

	uCount := len(userMap)
	vCount := len(videoMap)
	k := u.factors

	// Initialize Latent Vectors P (Users) and Q (Videos) with random small values
	P := make([][]float64, uCount)
	for i := range P {
		P[i] = make([]float64, k)
		for f := 0; f < k; f++ {
			P[i][f] = rand.Float64() * 0.1
		}
	}

	Q := make([][]float64, vCount)
	for j := range Q {
		Q[j] = make([]float64, k)
		for f := 0; f < k; f++ {
			Q[j][f] = rand.Float64() * 0.1
		}
	}

	// 2. Train Matrix Factorization using Stochastic Gradient Descent (SGD)
	for epoch := 0; epoch < u.epochs; epoch++ {
		for _, inter := range interactions {
			uIdx := userMap[inter.UserID]
			vIdx := videoMap[inter.VideoID]
			rating := inter.Rating

			// Predict score dot product P_u * Q_v
			pred := 0.0
			for f := 0; f < k; f++ {
				pred += P[uIdx][f] * Q[vIdx][f]
			}

			errVal := rating - pred

			// Update latent factors
			for f := 0; f < k; f++ {
				pOld := P[uIdx][f]
				qOld := Q[vIdx][f]

				P[uIdx][f] += u.gamma * (errVal*qOld - u.lambda*pOld)
				Q[vIdx][f] += u.gamma * (errVal*pOld - u.lambda*qOld)
			}
		}
	}

	// 3. Compute predicted scores for requested user
	uIdx, exists := userMap[userID]

	recs := make([]entity.RecommendedVideo, 0)
	for _, v := range allVideos {
		score := float64(v.Views) * 0.1 // Base baseline
		if exists {
			if vIdx, found := videoMap[v.ID]; found {
				pred := 0.0
				for f := 0; f < k; f++ {
					pred += P[uIdx][f] * Q[vIdx][f]
				}
				score += pred
			}
		}
		recs = append(recs, entity.RecommendedVideo{Video: v, PredictedScore: score})
	}

	// Sort recommendations by predicted score descending
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].PredictedScore > recs[j].PredictedScore
	})

	return mapper.ToPersonalizedFeedPageResponse(recs, len(recs), page, limit), nil
}
