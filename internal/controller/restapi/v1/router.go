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
func NewRoutes(apiV1Group fiber.Router, t usecase.Translation, u usecase.User, tk usecase.Task, vd usecase.Video, ls usecase.Livestream, ad usecase.Admin, lk usecase.Like, cm usecase.Comment, rc usecase.Recommendation, hs usecase.History, fw usecase.Follow, nt usecase.Notification, vb usecase.Vocabulary, sub usecase.Subtitle, hub *events.Hub, chatHub *events.ChatHub, jwtManager *jwt.Manager, l logger.Interface) {
	r := &V1{t: t, u: u, tk: tk, vd: vd, ls: ls, ad: ad, lk: lk, cm: cm, rc: rc, hs: hs, fw: fw, notif: nt, vocab: vb, sub: sub, hub: hub, chatHub: chatHub, l: l, v: validator.New(validator.WithRequiredStructEnabled())}

	// Public routes
	authGroup := apiV1Group.Group("/auth")
	{
		authGroup.Post("/register", r.register)
		authGroup.Post("/login", r.login)
		authGroup.Post("/refresh", r.refreshToken)
	}

	apiV1Group.Get("/dictionary/lookup", r.lookupDictionary)

	videosPublicGroup := apiV1Group.Group("/videos", middleware.OptionalAuth(jwtManager))
	{
		videosPublicGroup.Get("/", r.listPublicVideos)
		videosPublicGroup.Get("/feed/personalized", r.getPersonalizedFeed)
		videosPublicGroup.Get("/:id", r.getVideo)
		videosPublicGroup.Get("/:id/subtitles", r.getVideoSubtitles)
		videosPublicGroup.Post("/:id/views", r.recordVideoView)
		videosPublicGroup.Get("/:id/comments", r.listVideoComments)
	}

	commentsPublicGroup := apiV1Group.Group("/comments", middleware.OptionalAuth(jwtManager))
	{
		commentsPublicGroup.Get("/:id/replies", r.listCommentReplies)
	}

	// Liking and commenting require a real logged-in identity — an
	// OptionalAuth caller with no token would otherwise be indistinguishable
	// from every other anonymous visitor, corrupting per-user like/comment
	// state (see BUSINESS_REQUIREMENTS.md).
	videosProtectedGroup := apiV1Group.Group("/videos", middleware.Auth(jwtManager))
	{
		videosProtectedGroup.Post("/:id/like", r.toggleLikeVideo)
		videosProtectedGroup.Post("/:id/comments", r.createComment)
	}

	channelsPublicGroup := apiV1Group.Group("/channels", middleware.OptionalAuth(jwtManager))
	{
		channelsPublicGroup.Get("/:id", r.getChannelDetails)
	}

	apiV1Group.Post("/transcode/callback", r.transcodeCallback)
	apiV1Group.Post("/admin/workers/heartbeat", r.recordWorkerHeartbeat) // Internal webhook: transcode worker self-report
	apiV1Group.Get("/events/videos", r.sseVideoEvents)
	apiV1Group.Get("/events/chat/:id", r.sseChatEvents)
	apiV1Group.Get("/events/notifications", r.sseNotificationEvents)

	livePublicGroup := apiV1Group.Group("/live")
	{
		livePublicGroup.Get("/streams", r.listActiveStreams)
		livePublicGroup.Get("/streams/:id", r.getStream)
		livePublicGroup.Post("/streams/:id/chat", r.sendChatMessage)
		livePublicGroup.Post("/streams/:id/heart", r.heartStream)
		livePublicGroup.Post("/auth", r.authenticateRTMPStreamKey)      // Internal Webhook for SRS on_publish
		livePublicGroup.Post("/unpublish", r.unpublishRTMPStream)       // Internal Webhook for SRS on_unpublish
		livePublicGroup.Post("/dvr", r.handleDVRComplete)               // Internal Webhook for SRS on_dvr
	}

	// Protected routes
	protected := apiV1Group.Group("", middleware.Auth(jwtManager))

	vocabGroup := protected.Group("/vocabulary")
	{
		vocabGroup.Post("/", r.saveWord)
		vocabGroup.Get("/", r.listWords)
		vocabGroup.Delete("/:id", r.deleteWord)
	}

	notifGroup := protected.Group("/notifications")
	{
		notifGroup.Get("/", r.listNotifications)
		notifGroup.Post("/:id/read", r.markNotificationRead)
	}

	userGroup := protected.Group("/user")
	{
		userGroup.Get("/profile", r.profile)
		userGroup.Put("/profile", r.updateProfile)
		userGroup.Get("/history", r.getWatchHistory)
		userGroup.Get("/following", r.listFollowedChannels)
	}

	protected.Post("/channels/:id/follow", r.toggleFollowChannel)

	studioGroup := protected.Group("/studio")
	{
		studioGroup.Get("/videos", r.listStudioVideos)
		studioGroup.Post("/upload-url", r.createVideoUpload)
		studioGroup.Post("/confirm-upload", r.confirmVideoUpload)
		studioGroup.Post("/videos/:id/publish", r.publishVideo)
		studioGroup.Post("/videos/:id/subtitles", r.uploadVideoSubtitles)
		studioGroup.Put("/videos/:id", r.updateVideo)
		studioGroup.Put("/videos/:id/thumbnail", r.updateThumbnail)
		studioGroup.Delete("/videos/:id", r.deleteVideo)
		studioGroup.Get("/live/key", r.getStreamKey)
		studioGroup.Post("/live/reset-key", r.resetStreamKey)
	}


	adminGroup := protected.Group("/admin", middleware.RequireAdmin(r.u))
	{
		adminGroup.Get("/dashboard", r.getAdminDashboard)
		adminGroup.Get("/workers", r.getAdminWorkers)
		adminGroup.Post("/streams/:id/ban", r.banStream)
		adminGroup.Post("/videos/:id/ban", r.banVideo)
		adminGroup.Get("/users", r.listAdminUsers)
		adminGroup.Post("/users/:id/ban", r.banUser)
		adminGroup.Post("/users/:id/unban", r.unbanUser)
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
