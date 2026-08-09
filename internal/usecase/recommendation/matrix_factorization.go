package recommendation

import (
	"math/rand"
	"sync"
	"time"
)

// Feedback represents a user interaction with a video
type Feedback struct {
	UserID  string
	VideoID string
	Rating  float64 // E.g. 1.0 (Watch), 2.0 (Like), 5.0 (Full Watch + Like)
}

// MatrixFactorization Model for Collaborative Filtering (FunkMF / GMF)
type MatrixFactorization struct {
	mu           sync.RWMutex
	K            int                  // Latent factors dimension (e.g. K=4)
	Gamma        float64              // Learning rate (e.g. 0.01)
	Lambda       float64              // Regularization factor (e.g. 0.02)
	UserFactors  map[string][]float64 // P_u vectors
	ItemFactors  map[string][]float64 // Q_i vectors
	UserIndexMap map[string]int
	ItemIndexMap map[string]int
}

// NewMatrixFactorization initializes the MF model
func NewMatrixFactorization(k int, gamma, lambda float64) *MatrixFactorization {
	return &MatrixFactorization{
		K:            k,
		Gamma:        gamma,
		Lambda:       lambda,
		UserFactors:  make(map[string][]float64),
		ItemFactors:  make(map[string][]float64),
		UserIndexMap: make(map[string]int),
		ItemIndexMap: make(map[string]int),
	}
}

// initVector creates a latent vector with small random values
func (mf *MatrixFactorization) initVector() []float64 {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	vec := make([]float64, mf.K)
	for i := 0; i < mf.K; i++ {
		vec[i] = r.Float64() * 0.1 // Small random initialization
	}
	return vec
}

// Predict calculates dot product P_u * Q_i
func (mf *MatrixFactorization) Predict(userID, videoID string) float64 {
	mf.mu.RLock()
	defer mf.mu.RUnlock()

	pU, existsP := mf.UserFactors[userID]
	qI, existsQ := mf.ItemFactors[videoID]

	if !existsP || !existsQ {
		return 0.0 // Default fallback for unseen user/item
	}

	score := 0.0
	for k := 0; k < mf.K; k++ {
		score += pU[k] * qI[k]
	}
	return score
}

// Train performs Stochastic Gradient Descent (SGD) over interaction feedbacks
func (mf *MatrixFactorization) Train(feedbacks []Feedback, epochs int) {
	mf.mu.Lock()
	defer mf.mu.Unlock()

	// Ensure latent vectors exist for all users and items
	for _, fb := range feedbacks {
		if _, ok := mf.UserFactors[fb.UserID]; !ok {
			mf.UserFactors[fb.UserID] = mf.initVector()
		}
		if _, ok := mf.ItemFactors[fb.VideoID]; !ok {
			mf.ItemFactors[fb.VideoID] = mf.initVector()
		}
	}

	// SGD Training Loop
	for epoch := 0; epoch < epochs; epoch++ {
		for _, fb := range feedbacks {
			pU := mf.UserFactors[fb.UserID]
			qI := mf.ItemFactors[fb.VideoID]

			// Calculate predicted score: R_hat = P_u * Q_i
			rHat := 0.0
			for k := 0; k < mf.K; k++ {
				rHat += pU[k] * qI[k]
			}

			// Calculate error: e_ui = R_ui - R_hat
			err := fb.Rating - rHat

			// Update vectors via SGD formulas
			for k := 0; k < mf.K; k++ {
				pOld := pU[k]
				qOld := qI[k]

				// P_u = P_u + Gamma * (e_ui * Q_i - Lambda * P_u)
				pU[k] += mf.Gamma * (err*qOld - mf.Lambda*pOld)
				// Q_i = Q_i + Gamma * (e_ui * P_u - Lambda * Q_i)
				qI[k] += mf.Gamma * (err*pOld - mf.Lambda*qOld)
			}
		}
	}
}
