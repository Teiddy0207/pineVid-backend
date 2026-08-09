// Package app configures and runs application.
package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/evrone/go-clean-template/config"
	"github.com/evrone/go-clean-template/internal/controller/grpc"
	grpcmw "github.com/evrone/go-clean-template/internal/controller/grpc/middleware"
	natsrpc "github.com/evrone/go-clean-template/internal/controller/nats_rpc"
	"github.com/evrone/go-clean-template/internal/controller/restapi"
	"github.com/evrone/go-clean-template/internal/events"
	adminusecase "github.com/evrone/go-clean-template/internal/usecase/admin"
	livestreamusecase "github.com/evrone/go-clean-template/internal/usecase/livestream"
	videousecase "github.com/evrone/go-clean-template/internal/usecase/video"
	persistLivestreamRepo "github.com/evrone/go-clean-template/internal/repo/persistent/livestream"
	persistTaskRepo "github.com/evrone/go-clean-template/internal/repo/persistent/task"
	persistTranslationRepo "github.com/evrone/go-clean-template/internal/repo/persistent/translation"
	persistUserRepo "github.com/evrone/go-clean-template/internal/repo/persistent/user"
	persistVideoRepo "github.com/evrone/go-clean-template/internal/repo/persistent/video"
	"github.com/evrone/go-clean-template/internal/repo/webapi"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/internal/usecase/task"
	"github.com/evrone/go-clean-template/internal/usecase/translation"
	"github.com/evrone/go-clean-template/internal/usecase/user"
	"github.com/evrone/go-clean-template/pkg/grpcserver"
	"github.com/evrone/go-clean-template/pkg/httpserver"
	"github.com/evrone/go-clean-template/pkg/nats"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	natsRPCServer "github.com/evrone/go-clean-template/pkg/nats/nats_rpc/server"
	"github.com/evrone/go-clean-template/pkg/postgres"
	"github.com/evrone/go-clean-template/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	pbgrpc "google.golang.org/grpc"
)

type useCases struct {
	translation usecase.Translation
	user        usecase.User
	task        usecase.Task
	video       usecase.Video
	livestream  usecase.Livestream
	admin       usecase.Admin
}

type servers struct {
	nats *natsRPCServer.Server
	grpc *grpcserver.Server
	http *httpserver.Server
}

func initUseCases(pg *postgres.Postgres, jwtManager *jwt.Manager, chatHub *events.ChatHub) useCases {
	translationRepo := persistTranslationRepo.New(pg)
	taskRepo := persistTaskRepo.New(pg)
	userRepo := persistUserRepo.New(pg)
	videoRepo := persistVideoRepo.New(pg)
	livestreamRepo := persistLivestreamRepo.New(pg)

	natsPub, _ := nats.NewPublisher()
	videoUc := videousecase.New(videoRepo, natsPub)
	livestreamUc := livestreamusecase.New(livestreamRepo, chatHub)
	adminUc := adminusecase.New(livestreamRepo, videoRepo)

	return useCases{
		user:        user.New(userRepo, jwtManager),
		task:        task.New(taskRepo),
		translation: translation.New(translationRepo, webapi.New()),
		video:       videoUc,
		livestream:  livestreamUc,
		admin:       adminUc,
	}
}

func initServers(cfg *config.Config, uc useCases, chatHub *events.ChatHub, jwtManager *jwt.Manager, l logger.Interface) servers {
	// NATS RPC Server
	var natsServer *natsRPCServer.Server
	var err error
	if cfg.NATS.URL != "" {
		natsRouter := natsrpc.NewRouter(uc.translation, uc.user, uc.task, jwtManager, l)
		natsServer, err = natsRPCServer.New(cfg.NATS.URL, cfg.NATS.ServerExchange, natsRouter, l)
		if err != nil {
			l.Error(fmt.Errorf("app - Run - natsServer: %w", err))
		}
	}

	// gRPC Server
	grpcServer := grpcserver.New(
		l,
		grpcserver.Port(cfg.GRPC.Port),
		grpcserver.ServerOptions(
			pbgrpc.UnaryInterceptor(grpcmw.AuthInterceptor(jwtManager)),
			pbgrpc.StatsHandler(otelgrpc.NewServerHandler()),
		),
	)
	grpc.NewRouter(grpcServer.App, uc.translation, uc.user, uc.task, l)

	// HTTP Server
	videoEventHub := events.NewHub()
	httpServer := httpserver.New(l, httpserver.Port(cfg.HTTP.Port), httpserver.Prefork(cfg.HTTP.UsePreforkMode))
	restapi.NewRouter(httpServer.App, cfg, uc.translation, uc.user, uc.task, uc.video, uc.livestream, uc.admin, videoEventHub, chatHub, jwtManager, l)

	return servers{
		nats: natsServer,
		grpc: grpcServer,
		http: httpServer,
	}
}

func (s *servers) startServers() {
	if s.nats != nil {
		s.nats.Start()
	}
	s.grpc.Start()
	s.http.Start()
}

func (s *servers) waitForShutdown(l logger.Interface) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	var err error

	select {
	case sig := <-interrupt:
		l.Info("app - Run - signal: %s", sig.String())
	case err = <-s.http.Notify():
		l.Error(fmt.Errorf("app - Run - httpServer.Notify: %w", err))
	case err = <-s.grpc.Notify():
		l.Error(fmt.Errorf("app - Run - grpcServer.Notify: %w", err))
	}

	s.shutdownServers(l)
}

func (s *servers) shutdownServers(l logger.Interface) {
	if err := s.http.Shutdown(); err != nil {
		l.Error(fmt.Errorf("app - Run - httpServer.Shutdown: %w", err))
	}

	if err := s.grpc.Shutdown(); err != nil {
		l.Error(fmt.Errorf("app - Run - grpcServer.Shutdown: %w", err))
	}

	if s.nats != nil {
		if err := s.nats.Shutdown(); err != nil {
			l.Error(fmt.Errorf("app - Run - natsServer.Shutdown: %w", err))
		}
	}
}

// Run creates objects via constructors.
func Run(cfg *config.Config) {
	l := logger.New(cfg.Log.Level)

	ctx := context.Background()

	// Tracing
	shutdownTracing, err := tracing.New(ctx, tracing.Config{
		Enabled:     cfg.Tracing.Enabled,
		ServiceName: cfg.App.Name,
		Version:     cfg.App.Version,
		Endpoint:    cfg.Tracing.OTLPEndpoint,
		Insecure:    cfg.Tracing.OTLPInsecure,
		SampleRate:  cfg.Tracing.SampleRate,
	})
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - tracing.New: %w", err))
	}
	defer func() {
		if err := shutdownTracing(ctx); err != nil {
			l.Error(fmt.Errorf("app - Run - shutdownTracing: %w", err))
		}
	}()

	// Repository
	pg, err := postgres.New(cfg.PG.URL, postgres.MaxPoolSize(cfg.PG.PoolMax))
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - postgres.New: %w", err))
	}
	defer pg.Close()

	// JWT
	jwtManager := jwt.New(cfg.JWT.Secret, cfg.JWT.TokenExpiry)

	chatHub := events.NewChatHub()
	uc := initUseCases(pg, jwtManager, chatHub)
	s := initServers(cfg, uc, chatHub, jwtManager, l)
	s.startServers()
	s.waitForShutdown(l)
}
