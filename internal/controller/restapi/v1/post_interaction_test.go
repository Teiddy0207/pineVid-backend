package v1

import (
	"context"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

type missingPostStub struct{ usecase.Post }

func (missingPostStub) TogglePostLike(context.Context, string, string) (response.PostLikeResponse, error) {
	return response.PostLikeResponse{}, entity.ErrPostNotFound
}
func TestLikeMissingPostReturnsNotFound(t *testing.T) {
	r := &V1{ps: missingPostStub{}}
	app := fiber.New()
	app.Post("/posts/:id/like", r.togglePostLike)
	res, err := app.Test(httptest.NewRequest("POST", "/posts/missing/like", nil))
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, 404, res.StatusCode)
}
