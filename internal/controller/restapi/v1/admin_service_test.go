package v1

import (
	"context"
	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"testing"
	"time"
)

type serviceAdmin struct{ usecase.Admin }

func (serviceAdmin) ListUsers(context.Context, int, int) (response.PageResponse[response.UserResponse], error) {
	return response.PageResponse[response.UserResponse]{}, nil
}
func TestAdminRoutesUseServiceIdentity(t *testing.T) {
	f := fiber.New()
	NewRoutes(f.Group("/v1"),
		nil,
		nil,
		nil,
		serviceAdmin{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		jwt.New("jwt-secret", time.Hour),
		"service-secret",
		middleware.NewRateLimiter(false),
		logger.New("error"))
	for _, tc := range []struct {
		header, path string
		want         int
	}{
		{"service-secret", "/v1/admin/users", 200},
		{"", "/v1/admin/users", 401},
		{"", "/v1/admin/users?secret=service-secret", 401},
		{"wrong", "/v1/admin/users", 401},
	} {
		req := httptest.NewRequest("GET", tc.path, nil)
		req.Header.Set("X-Internal-Token", tc.header)
		res, err := f.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != tc.want {
			t.Fatalf("%s: got %d want %d", tc.path, res.StatusCode, tc.want)
		}
	}
}
