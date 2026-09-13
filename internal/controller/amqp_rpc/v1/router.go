package v1

import (
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/evrone/go-clean-template/pkg/rabbitmq/rmq_rpc/server"
	"github.com/go-playground/validator/v10"
)

// NewRoutes -.
func NewRoutes(routes map[string]server.CallHandler, u usecase.User, j *jwt.Manager, l logger.Interface) {
	r := &V1{u: u, j: j, l: l, v: validator.New(validator.WithRequiredStructEnabled())}

	routes["v1.auth.register"] = r.register()
	routes["v1.auth.login"] = r.login()
}
