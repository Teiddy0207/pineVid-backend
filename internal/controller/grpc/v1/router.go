package v1

import (
	v1 "github.com/evrone/go-clean-template/docs/proto/v1"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/go-playground/validator/v10"
	pbgrpc "google.golang.org/grpc"
)

// NewAuthRoutes -.
func NewAuthRoutes(app *pbgrpc.Server, u usecase.User, l logger.Interface) {
	r := &AuthController{u: u, l: l, v: validator.New(validator.WithRequiredStructEnabled())}

	v1.RegisterAuthServiceServer(app, r)
}
