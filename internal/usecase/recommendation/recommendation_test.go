package recommendation

import (
	"context"
	"testing"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// White-box tests (same package) so GetPersonalizedFeed's cold-start
// branches can be exercised by constructing UseCase directly with
// pre-populated userVec/videoVec/trainedIsEmpty, instead of going through
// train() (which needs a real Postgres via the concrete *persistRecRepo.Repo
// field).

type mockVideoRepo struct{ mock.Mock }

func (m *mockVideoRepo) Store(ctx context.Context, v *entity.Video) error {
	return m.Called(ctx, v).Error(0)
}
func (m *mockVideoRepo) GetByID(ctx context.Context, id string) (entity.Video, error) {
	a := m.Called(ctx, id)
	return a.Get(0).(entity.Video), a.Error(1)
}
func (m *mockVideoRepo) List(ctx context.Context, filter repo.VideoFilter) ([]entity.Video, int, error) {
	a := m.Called(ctx, filter)
	return a.Get(0).([]entity.Video), a.Int(1), a.Error(2)
}
func (m *mockVideoRepo) Update(ctx context.Context, v *entity.Video) error {
	return m.Called(ctx, v).Error(0)
}
func (m *mockVideoRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

type mockPrefRepo struct{ mock.Mock }

func (m *mockPrefRepo) SetCategories(ctx context.Context, userID string, categories []string) error {
	return m.Called(ctx, userID, categories).Error(0)
}
func (m *mockPrefRepo) GetCategories(ctx context.Context, userID string) ([]string, error) {
	a := m.Called(ctx, userID)
	return a.Get(0).([]string), a.Error(1)
}

func videosFixture() []entity.Video {
	return []entity.Video{
		{ID: "gaming-low-views", Category: "Gaming", Views: 100},
		{ID: "music-high-views", Category: "Music", Views: 100000},
	}
}

func TestGetPersonalizedFeed_NeverTrained_PreferredCategoryOutranksHigherViews(t *testing.T) {
	t.Parallel()

	vRepo := new(mockVideoRepo)
	vRepo.On("List", mock.Anything, mock.Anything).Return(videosFixture(), 2, nil)
	prefRepo := new(mockPrefRepo)
	prefRepo.On("GetCategories", mock.Anything, "user-1").Return([]string{"Gaming"}, nil)

	uc := &UseCase{videoRepo: vRepo, prefRepo: prefRepo, factors: 10, trainedIsEmpty: true}

	page, err := uc.GetPersonalizedFeed(context.Background(), "user-1", 1, 10)

	require.NoError(t, err)
	require.Equal(t, "gaming-low-views", page.Data[0].Video.ID)
}

func TestGetPersonalizedFeed_ColdStartUser_PreferredCategoryOutranksHigherViews(t *testing.T) {
	t.Parallel()

	vRepo := new(mockVideoRepo)
	vRepo.On("List", mock.Anything, mock.Anything).Return(videosFixture(), 2, nil)
	prefRepo := new(mockPrefRepo)
	prefRepo.On("GetCategories", mock.Anything, "user-1").Return([]string{"Gaming"}, nil)

	uc := &UseCase{
		videoRepo: vRepo, prefRepo: prefRepo, factors: 10,
		trainedIsEmpty: false,
		userVec:        map[string][]float64{"other-user": {0.1, 0.2}}, // model trained, but not for user-1
		videoVec:       map[string][]float64{},
	}

	page, err := uc.GetPersonalizedFeed(context.Background(), "user-1", 1, 10)

	require.NoError(t, err)
	require.Equal(t, "gaming-low-views", page.Data[0].Video.ID)
}

func TestGetPersonalizedFeed_ColdStartUser_NoPreference_FallsBackToViews(t *testing.T) {
	t.Parallel()

	vRepo := new(mockVideoRepo)
	vRepo.On("List", mock.Anything, mock.Anything).Return(videosFixture(), 2, nil)
	prefRepo := new(mockPrefRepo)
	prefRepo.On("GetCategories", mock.Anything, "user-1").Return([]string{}, nil)

	uc := &UseCase{
		videoRepo: vRepo, prefRepo: prefRepo, factors: 10,
		trainedIsEmpty: false,
		userVec:        map[string][]float64{"other-user": {0.1, 0.2}},
		videoVec:       map[string][]float64{},
	}

	page, err := uc.GetPersonalizedFeed(context.Background(), "user-1", 1, 10)

	require.NoError(t, err)
	require.Equal(t, "music-high-views", page.Data[0].Video.ID) // plain trending, unchanged behavior
}

func TestGetPersonalizedFeed_RespectsLimit(t *testing.T) {
	t.Parallel()

	// videoRepo.List can return up to 50 videos regardless of the requested
	// page size — GetPersonalizedFeed must still only hand back `limit` of
	// them, not everything it fetched internally.
	videos := make([]entity.Video, 20)
	for i := range videos {
		videos[i] = entity.Video{ID: string(rune('a' + i)), Category: "Music", Views: int64(20 - i)}
	}

	vRepo := new(mockVideoRepo)
	vRepo.On("List", mock.Anything, mock.Anything).Return(videos, len(videos), nil)
	prefRepo := new(mockPrefRepo)
	prefRepo.On("GetCategories", mock.Anything, "user-1").Return([]string{}, nil)

	uc := &UseCase{videoRepo: vRepo, prefRepo: prefRepo, factors: 10, trainedIsEmpty: true}

	page, err := uc.GetPersonalizedFeed(context.Background(), "user-1", 1, 5)

	require.NoError(t, err)
	require.Len(t, page.Data, 5)
	require.Equal(t, 20, page.Pagination.TotalItems)
	require.Equal(t, 4, page.Pagination.TotalPages)
}

func TestGetPersonalizedFeed_TrainedUser_ModelScoreOverridesPreference(t *testing.T) {
	t.Parallel()

	vRepo := new(mockVideoRepo)
	vRepo.On("List", mock.Anything, mock.Anything).Return(videosFixture(), 2, nil)
	prefRepo := new(mockPrefRepo)
	// user prefers Gaming, but has real interactions now — the trained
	// dot-product must decide, not the onboarding preference.
	prefRepo.On("GetCategories", mock.Anything, "user-1").Return([]string{"Gaming"}, nil)

	uc := &UseCase{
		videoRepo: vRepo, prefRepo: prefRepo, factors: 2,
		trainedIsEmpty: false,
		userVec:        map[string][]float64{"user-1": {1.0, 1.0}},
		videoVec: map[string][]float64{
			"gaming-low-views": {0.01, 0.01},  // low dot-product
			"music-high-views": {10.0, 10.0}, // high dot-product
		},
	}

	page, err := uc.GetPersonalizedFeed(context.Background(), "user-1", 1, 10)

	require.NoError(t, err)
	require.Equal(t, "music-high-views", page.Data[0].Video.ID)
}
