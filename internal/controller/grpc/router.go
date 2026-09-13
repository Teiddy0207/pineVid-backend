package grpc

import (
	v1 "github.com/evrone/go-clean-template/internal/controller/grpc/v1"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
	pbgrpc "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// NewRouter -.
func NewRouter(app *pbgrpc.Server, u usecase.User, l logger.Interface) {
	{
		v1.NewAuthRoutes(app, u, l)
	}

	reflection.Register(app)
}
