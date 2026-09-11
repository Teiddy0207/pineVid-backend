package recommendation

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/evrone/go-clean-template/internal/repo"
	persistRecRepo "github.com/evrone/go-clean-template/internal/repo/persistent/recommendation"
)

type UseCase struct {
	repo      *persistRecRepo.Repo
	videoRepo repo.VideoRepo
	prefRepo  repo.UserPreferenceRepo
	factors   int
	gamma     float64 // Learning rate
	lambda    float64 // Regularization
	epochs    int

	mu             sync.RWMutex
	trainedIsEmpty bool                 // true if the last training pass had zero interactions (cold start)
	userVec        map[string][]float64 // trained P, keyed by userID
	videoVec       map[string][]float64 // trained Q, keyed by videoID
}

func New(r *persistRecRepo.Repo, vr repo.VideoRepo, pr repo.UserPreferenceRepo) *UseCase {
	return &UseCase{
		repo:      r,
		videoRepo: vr,
		prefRepo:  pr,
		factors:   10,
		gamma:     0.01,
		lambda:    0.02,
		epochs:    50,
	}
}

// categoryBoost is added to a video's score when its category is one of the
// caller's preferred categories. It only ever gets applied in cold-start
// paths (see GetPersonalizedFeed) — once a user has real interactions, the
// trained model's dot-product score alone decides ranking. The constant is
// picked large enough to dominate the views-based baseline for any
// realistic view count, so a preferred-category video always outranks a
// non-preferred one regardless of view count.
const categoryBoost = 1_000_000.0

// StartBackgroundTraining trains the model once immediately (so the cache is
// warm before the server starts taking traffic), then keeps retraining on a
// fixed interval in the background for as long as ctx is alive. No HTTP
// request ever waits on training — GetPersonalizedFeed only ever reads
// whatever is currently cached.
func (u *UseCase) StartBackgroundTraining(ctx context.Context, interval time.Duration) {
	_ = u.train(ctx)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = u.train(ctx)
			}
		}
	}()
}

// train runs Matrix Factorization with SGD over the latest interactions and
// publishes the result into u.userVec/u.videoVec.
func (u *UseCase) train(ctx context.Context) error {
	interactions, err := u.repo.FetchInteractions(ctx)
	if err != nil {
		return fmt.Errorf("RecommendationUseCase - train - FetchInteractions: %w", err)
	}

	if len(interactions) == 0 {
		u.mu.Lock()
		u.userVec = nil
		u.videoVec = nil
		u.trainedIsEmpty = true
		u.mu.Unlock()
		return nil
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
			// With very few users/videos (e.g. a single user with a handful
			// of interactions), unclamped SGD can overshoot and blow up to
			// +/-Inf over many epochs; clamp the error term so updates stay
			// bounded instead of ever reaching Inf/NaN.
			errVal = clampFloat(errVal, -maxSGDError, maxSGDError)

			// Update latent factors
			for f := 0; f < k; f++ {
				pOld := P[uIdx][f]
				qOld := Q[vIdx][f]

				P[uIdx][f] += u.gamma * (errVal*qOld - u.lambda*pOld)
				Q[vIdx][f] += u.gamma * (errVal*pOld - u.lambda*qOld)
			}
		}
	}

	if !allFinite(P) || !allFinite(Q) {
		// Training diverged (shouldn't happen with the clamp above, but this
		// is cheap insurance): keep serving whatever was cached before rather
		// than publishing vectors that would make every response 500 with
		// "json: unsupported value: NaN".
		return fmt.Errorf("RecommendationUseCase - train: model diverged (non-finite values), keeping previous cache")
	}

	// 3. Publish the trained vectors keyed by real ID, so serving can look up
	// any user/video directly without redoing the index mapping above.
	userVec := make(map[string][]float64, uCount)
	for id, idx := range userMap {
		userVec[id] = P[idx]
	}
	videoVec := make(map[string][]float64, vCount)
	for id, idx := range videoMap {
		videoVec[id] = Q[idx]
	}

	u.mu.Lock()
	u.userVec = userVec
	u.videoVec = videoVec
	u.trainedIsEmpty = false
	u.mu.Unlock()

	return nil
}

const maxSGDError = 50.0

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func allFinite(matrix [][]float64) bool {
	for _, row := range matrix {
		for _, v := range row {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return false
			}
		}
	}
	return true
}

// GetPersonalizedFeed returns top-ranked videos for a user, reading whatever
// model StartBackgroundTraining has most recently published. The view-count
// baseline is always computed fresh from the DB.
func (u *UseCase) GetPersonalizedFeed(ctx context.Context, userID string, page, limit int) (response.PageResponse[response.RecommendedVideoItem], error) {
	status := entity.VideoStatusComplete
	visibility := entity.VideoVisibilityPublic
	allVideos, totalVideos, err := u.videoRepo.List(ctx, repo.VideoFilter{Status: &status, Visibility: &visibility, Limit: 50})
	if err != nil {
		return response.PageResponse[response.RecommendedVideoItem]{}, fmt.Errorf("RecommendationUseCase - List: %w", err)
	}

	// Best-effort: an anonymous caller (empty userID) or a lookup failure
	// just means no boost is applied, falling back to plain trending — never
	// fail the whole feed over a preferences lookup.
	preferredCategories := make(map[string]bool)
	if userID != "" && u.prefRepo != nil {
		if categories, prefErr := u.prefRepo.GetCategories(ctx, userID); prefErr == nil {
			for _, c := range categories {
				preferredCategories[c] = true
			}
		}
	}

	u.mu.RLock()
	defer u.mu.RUnlock()

	if u.trainedIsEmpty || u.userVec == nil {
		// Cold-start fallback: no interactions exist anywhere yet (or the
		// background trainer hasn't run for the first time). Still honor
		// explicit category preference so onboarding isn't wasted even
		// before the model has ever trained.
		recs := make([]entity.RecommendedVideo, len(allVideos))
		for i, v := range allVideos {
			score := 1.0
			if preferredCategories[v.Category] {
				score += categoryBoost
			}
			recs[i] = entity.RecommendedVideo{Video: v, PredictedScore: score}
		}
		sort.Slice(recs, func(i, j int) bool {
			return recs[i].PredictedScore > recs[j].PredictedScore
		})
		return mapper.ToPersonalizedFeedPageResponse(paginate(recs, page, limit), totalVideos, page, limit), nil
	}

	userFactors, hasUser := u.userVec[userID]

	recs := make([]entity.RecommendedVideo, 0, len(allVideos))
	for _, v := range allVideos {
		score := float64(v.Views) * 0.1 // Base baseline, always fresh
		if hasUser {
			if videoFactors, found := u.videoVec[v.ID]; found {
				pred := 0.0
				for f := 0; f < u.factors; f++ {
					pred += userFactors[f] * videoFactors[f]
				}
				// Defense in depth: never let a single non-finite value (from
				// a future/edge-case model) poison the whole JSON response.
				if !math.IsNaN(pred) && !math.IsInf(pred, 0) {
					score += pred
				}
			}
		} else if preferredCategories[v.Category] {
			// Cold-start for this specific user: the model has real signal
			// for other users, but not this one yet. Bootstrap with their
			// explicit onboarding preference until they generate real
			// interactions — at which point `hasUser` becomes true and this
			// boost stops applying, handing ranking fully back to the
			// trained model.
			score += categoryBoost
		}
		recs = append(recs, entity.RecommendedVideo{Video: v, PredictedScore: score})
	}

	// Sort recommendations by predicted score descending
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].PredictedScore > recs[j].PredictedScore
	})

	return mapper.ToPersonalizedFeedPageResponse(paginate(recs, page, limit), len(recs), page, limit), nil
}

// paginate slices an already-sorted recommendation list down to the
// requested page/limit window. Without this, callers asking for e.g. 12
// items would get back everything List() fetched (up to 50) — the
// PaginationMeta would claim a small page while Data held dozens of items.
func paginate(recs []entity.RecommendedVideo, page, limit int) []entity.RecommendedVideo {
	if limit <= 0 {
		return recs
	}
	if page < 1 {
		page = 1
	}
	start := (page - 1) * limit
	if start >= len(recs) {
		return []entity.RecommendedVideo{}
	}
	end := start + limit
	if end > len(recs) {
		end = len(recs)
	}
	return recs[start:end]
}
