package v1

import (
	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	"github.com/evrone/go-clean-template/internal/events"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// NewRoutes -.
func NewRoutes(apiV1Group fiber.Router, t usecase.Translation, u usecase.User, tk usecase.Task, vd usecase.Video, ls usecase.Livestream, ad usecase.Admin, hub *events.Hub, chatHub *events.ChatHub, jwtManager *jwt.Manager, l logger.Interface) {
	r := &V1{t: t, u: u, tk: tk, vd: vd, ls: ls, ad: ad, hub: hub, chatHub: chatHub, l: l, v: validator.New(validator.WithRequiredStructEnabled())}

	// Public routes
	authGroup := apiV1Group.Group("/auth")
	{
		authGroup.Post("/register", r.register)
		authGroup.Post("/login", r.login)
	}

	videosPublicGroup := apiV1Group.Group("/videos")
	{
		videosPublicGroup.Get("/", r.listPublicVideos)
		videosPublicGroup.Get("/:id", r.getVideo)
		videosPublicGroup.Post("/:id/views", r.recordVideoView)
	}

	apiV1Group.Post("/transcode/callback", r.transcodeCallback)
	apiV1Group.Get("/events/videos", r.sseVideoEvents)
	apiV1Group.Get("/events/chat/:id", r.sseChatEvents)

	livePublicGroup := apiV1Group.Group("/live")
	{
		livePublicGroup.Get("/streams", r.listActiveStreams)
		livePublicGroup.Get("/streams/:id", r.getStream)
		livePublicGroup.Post("/streams/:id/chat", r.sendChatMessage)
		livePublicGroup.Post("/auth", r.authenticateRTMPStreamKey) // Internal Webhook for SRS
	}

	// Protected routes
	protected := apiV1Group.Group("", middleware.Auth(jwtManager))

	userGroup := protected.Group("/user")
	{
		userGroup.Get("/profile", r.profile)
		userGroup.Put("/profile", r.updateProfile)
	}

	studioGroup := protected.Group("/studio")
	{
		studioGroup.Get("/videos", r.listStudioVideos)
		studioGroup.Post("/upload-url", r.createVideoUpload)
		studioGroup.Post("/confirm-upload", r.confirmVideoUpload)
		studioGroup.Post("/videos/:id/publish", r.publishVideo)
		studioGroup.Get("/live/key", r.getStreamKey)
		studioGroup.Post("/live/reset-key", r.resetStreamKey)
	}

	adminGroup := protected.Group("/admin")
	{
		adminGroup.Get("/dashboard", r.getAdminDashboard)
		adminGroup.Get("/workers", r.getAdminWorkers)
		adminGroup.Post("/streams/:id/ban", r.banStream)
		adminGroup.Post("/videos/:id/ban", r.banVideo)
	}

	taskGroup := protected.Group("/tasks")
	{
		taskGroup.Post("/", r.createTask)
		taskGroup.Get("/", r.listTasks)
		taskGroup.Get("/:id", r.getTask)
		taskGroup.Put("/:id", r.updateTask)
		taskGroup.Patch("/:id/status", r.transitionTask)
		taskGroup.Delete("/:id", r.deleteTask)
	}

	translationGroup := protected.Group("/translation")
	{
		translationGroup.Get("/history", r.history)
		translationGroup.Post("/do-translate", r.doTranslate)
	}
}
