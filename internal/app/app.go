// Package app configures and runs application.
package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/evrone/go-clean-template/config"
	"github.com/evrone/go-clean-template/internal/controller/grpc"
	grpcmw "github.com/evrone/go-clean-template/internal/controller/grpc/middleware"
	natsrpc "github.com/evrone/go-clean-template/internal/controller/nats_rpc"
	"github.com/evrone/go-clean-template/internal/controller/restapi"
	"github.com/evrone/go-clean-template/internal/events"
	adminusecase "github.com/evrone/go-clean-template/internal/usecase/admin"
	commentusecase "github.com/evrone/go-clean-template/internal/usecase/comment"
	postusecase "github.com/evrone/go-clean-template/internal/usecase/post"
	followusecase "github.com/evrone/go-clean-template/internal/usecase/follow"
	historyusecase "github.com/evrone/go-clean-template/internal/usecase/history"
	likeusecase "github.com/evrone/go-clean-template/internal/usecase/like"
	livestreamusecase "github.com/evrone/go-clean-template/internal/usecase/livestream"
	notifusecase "github.com/evrone/go-clean-template/internal/usecase/notification"
	recusecase "github.com/evrone/go-clean-template/internal/usecase/recommendation"
	videousecase "github.com/evrone/go-clean-template/internal/usecase/video"
	subtitleusecase "github.com/evrone/go-clean-template/internal/usecase/subtitle"
	vocabusecase "github.com/evrone/go-clean-template/internal/usecase/vocabulary"
	persistAdminStatsRepo "github.com/evrone/go-clean-template/internal/repo/persistent/adminstats"
	persistCommentRepo "github.com/evrone/go-clean-template/internal/repo/persistent/comment"
	persistCommentLikeRepo "github.com/evrone/go-clean-template/internal/repo/persistent/commentlike"
	persistPostRepo "github.com/evrone/go-clean-template/internal/repo/persistent/post"
	persistPostLikeRepo "github.com/evrone/go-clean-template/internal/repo/persistent/postlike"
	persistFollowRepo "github.com/evrone/go-clean-template/internal/repo/persistent/follow"
	persistHistoryRepo "github.com/evrone/go-clean-template/internal/repo/persistent/history"
	persistLikeRepo "github.com/evrone/go-clean-template/internal/repo/persistent/like"
	persistLivestreamRepo "github.com/evrone/go-clean-template/internal/repo/persistent/livestream"
	persistNotifRepo "github.com/evrone/go-clean-template/internal/repo/persistent/notification"
	persistRecRepo "github.com/evrone/go-clean-template/internal/repo/persistent/recommendation"
	persistSubtitleRepo "github.com/evrone/go-clean-template/internal/repo/persistent/subtitle"
	persistVocabRepo "github.com/evrone/go-clean-template/internal/repo/persistent/vocabulary"
	persistWorkerRepo "github.com/evrone/go-clean-template/internal/repo/persistent/worker"
	persistUserRepo "github.com/evrone/go-clean-template/internal/repo/persistent/user"
	persistUserPrefRepo "github.com/evrone/go-clean-template/internal/repo/persistent/userpreference"
	userprefusecase "github.com/evrone/go-clean-template/internal/usecase/userpreference"
	persistSavedVideoRepo "github.com/evrone/go-clean-template/internal/repo/persistent/savedvideo"
	savedvideousecase "github.com/evrone/go-clean-template/internal/usecase/savedvideo"
	persistVideoRepo "github.com/evrone/go-clean-template/internal/repo/persistent/video"
	persistViewRepo "github.com/evrone/go-clean-template/internal/repo/persistent/view"
	pkgminio "github.com/evrone/go-clean-template/pkg/minio"
	redispkg "github.com/evrone/go-clean-template/pkg/redis"
	pkgsrs "github.com/evrone/go-clean-template/pkg/srs"
	"github.com/evrone/go-clean-template/internal/usecase"
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
	user           usecase.User
	video          usecase.Video
	livestream     usecase.Livestream
	admin          usecase.Admin
	like           usecase.Like
	comment        usecase.Comment
	post           usecase.Post
	recommendation usecase.Recommendation
	history        usecase.History
	follow         usecase.Follow
	notification   usecase.Notification
	vocabulary     usecase.Vocabulary
	subtitle       usecase.Subtitle
	userPreference usecase.UserPreference
	savedVideo     usecase.SavedVideo
}

type servers struct {
	nats *natsRPCServer.Server
	grpc *grpcserver.Server
	http *httpserver.Server
}

func initUseCases(cfg *config.Config, pg *postgres.Postgres, jwtManager *jwt.Manager, chatHub *events.ChatHub, notifHub *events.NotificationHub, l logger.Interface) useCases {
	userRepo := persistUserRepo.New(pg)
	videoRepo := persistVideoRepo.New(pg)
	livestreamRepo := persistLivestreamRepo.New(pg)
	commentRepo := persistCommentRepo.New(pg)
	commentLikeRepo := persistCommentLikeRepo.New(pg)
	postRepo := persistPostRepo.New(pg)
	postLikeRepo := persistPostLikeRepo.New(pg)
	recRepo := persistRecRepo.New(pg)
	historyRepo := persistHistoryRepo.New(pg)
	followRepo := persistFollowRepo.New(pg)
	notifRepo := persistNotifRepo.New(pg)
	vocabRepo := persistVocabRepo.New(pg)
	subtitleRepo := persistSubtitleRepo.New(pg)
	userPrefRepo := persistUserPrefRepo.New(pg)
	savedVideoRepo := persistSavedVideoRepo.New(pg)

	redisClient, err := redispkg.New(cfg.Redis.URL, "", 0)
	if err != nil {
		l.Error(fmt.Errorf("app - initUseCases - redispkg.New: %w", err))
	}
	viewRepo := persistViewRepo.New(pg, redisClient)
	likeRepo := persistLikeRepo.New(pg, redisClient)

	natsPub, _ := nats.NewPublisher()
	notifUc := notifusecase.New(notifRepo, followRepo, notifHub)
	videoUc := videousecase.New(videoRepo, viewRepo, likeRepo, savedVideoRepo, natsPub, notifUc)

	minioClient, err := pkgminio.New(cfg.Minio.Endpoint, cfg.Minio.AccessKey, cfg.Minio.SecretKey, cfg.Minio.UseSSL)
	if err != nil {
		l.Error(fmt.Errorf("app - initUseCases - pkgminio.New: %w", err))
	}
	srsClient := pkgsrs.New(cfg.SRS.APIURL)
	livestreamUc := livestreamusecase.New(livestreamRepo, videoRepo, followRepo, chatHub, notifUc, minioClient, natsPub, srsClient, cfg.Minio.RawBucket, cfg.DVR.LocalDir, 30*time.Second)
	livestreamUc.StartReconciliationLoop(context.Background(), cfg.SRS.ReconcileInterval)
	workerRepo := persistWorkerRepo.New(pg)
	adminStatsRepo := persistAdminStatsRepo.New(pg)
	adminUc := adminusecase.New(livestreamRepo, videoRepo, userRepo, workerRepo, adminStatsRepo)
	likeUc := likeusecase.New(likeRepo, natsPub, livestreamUc)
	commentUc := commentusecase.New(commentRepo, commentLikeRepo, natsPub)
	postUc := postusecase.New(postRepo, postLikeRepo, notifUc)
	recUc := recusecase.New(recRepo, videoRepo, userPrefRepo)
	recUc.StartBackgroundTraining(context.Background(), 5*time.Minute)
	historyUc := historyusecase.New(historyRepo)
	followUc := followusecase.New(followRepo)
	vocabUc := vocabusecase.New(vocabRepo)
	subtitleUc := subtitleusecase.New(subtitleRepo, videoRepo)
	userPrefUc := userprefusecase.New(userPrefRepo)
	savedVideoUc := savedvideousecase.New(savedVideoRepo)

	return useCases{
		user:           user.New(userRepo, jwtManager),
		video:          videoUc,
		livestream:     livestreamUc,
		admin:          adminUc,
		like:           likeUc,
		comment:        commentUc,
		post:           postUc,
		recommendation: recUc,
		history:        historyUc,
		follow:         followUc,
		notification:   notifUc,
		vocabulary:     vocabUc,
		subtitle:       subtitleUc,
		userPreference: userPrefUc,
		savedVideo:     savedVideoUc,
	}
}

func initServers(cfg *config.Config, uc useCases, chatHub *events.ChatHub, jwtManager *jwt.Manager, l logger.Interface) servers {
	// NATS RPC Server
	var natsServer *natsRPCServer.Server
	var err error
	if cfg.NATS.URL != "" {
		natsRouter := natsrpc.NewRouter(uc.user, jwtManager, l)
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
	grpc.NewRouter(grpcServer.App, uc.user, l)

	// HTTP Server
	videoEventHub := events.NewHub()
	// WriteTimeout is unlimited: fasthttp applies it as a single deadline for
	// the connection's entire response, which fasthttp only surfaces as an
	// error on the *next* write attempted after it elapses (it doesn't
	// proactively close idle connections) — so with the previous 5s default,
	// every long-lived SSE stream (chat, hearts, notifications, transcode
	// events) silently stopped delivering anything sent more than ~5s after
	// the client connected, even though the connection looked "open".
	httpServer := httpserver.New(l, httpserver.Port(cfg.HTTP.Port), httpserver.Prefork(cfg.HTTP.UsePreforkMode), httpserver.WriteTimeout(0))
	restapi.NewRouter(httpServer.App, cfg, uc.user, uc.video, uc.livestream, uc.admin, uc.like, uc.comment, uc.post, uc.recommendation, uc.history, uc.follow, uc.notification, uc.vocabulary, uc.subtitle, uc.userPreference, uc.savedVideo, videoEventHub, chatHub, jwtManager, l)

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
	notifHub := events.NewNotificationHub()
	uc := initUseCases(cfg, pg, jwtManager, chatHub, notifHub, l)
	s := initServers(cfg, uc, chatHub, jwtManager, l)
	s.startServers()
	s.waitForShutdown(l)
}
