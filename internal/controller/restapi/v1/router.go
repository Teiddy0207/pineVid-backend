package v1

import (
	"time"

	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	"github.com/evrone/go-clean-template/internal/events"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

// NewInternalRoutes registers the signed service webhooks under /internal
// (docs/message-contract.md §4): POST /internal/webhooks/transcode.
func NewInternalRoutes(internalGroup fiber.Router, vd usecase.Video, hub *events.Hub, webhookSecret string, l logger.Interface) {
	r := &V1{vd: vd, hub: hub, l: l, v: validator.New(validator.WithRequiredStructEnabled())}

	internalGroup.Post("/webhooks/transcode", middleware.RequireWebhookSignature(webhookSecret, nil), r.transcodeWebhook)
}

// NewRoutes -.
func NewRoutes(apiV1Group fiber.Router, u usecase.User, vd usecase.Video, ls usecase.Livestream, ad usecase.Admin, lk usecase.Like, cm usecase.Comment, ps usecase.Post, rc usecase.Recommendation, hs usecase.History, fw usecase.Follow, nt usecase.Notification, vb usecase.Vocabulary, sub usecase.Subtitle, up usecase.UserPreference, sv usecase.SavedVideo, hub *events.Hub, chatHub *events.ChatHub, viewHub *events.ViewHub, jwtManager *jwt.Manager, internalSecret string, rl *middleware.RateLimiter, l logger.Interface) {
	r := &V1{u: u, vd: vd, ls: ls, ad: ad, lk: lk, cm: cm, ps: ps, rc: rc, hs: hs, fw: fw, notif: nt, vocab: vb, sub: sub, up: up, sv: sv, hub: hub, chatHub: chatHub, viewHub: viewHub, l: l, v: validator.New(validator.WithRequiredStructEnabled())}

	auth := middleware.Auth(jwtManager, u)
	optionalAuth := middleware.OptionalAuth(jwtManager, u)
	internalOnly := middleware.RequireInternalSecret(internalSecret)

	// Public routes
	authGroup := apiV1Group.Group("/auth")
	{
		authGroup.Post("/register", rl.Limit("register", 5, time.Minute), r.register)
		authGroup.Post("/login", rl.Limit("login", 10, time.Minute), r.login)
		authGroup.Post("/refresh", rl.Limit("refresh", 30, time.Minute), r.refreshToken)
		authGroup.Post("/logout", r.logout)
	}

	apiV1Group.Get("/dictionary/lookup", r.lookupDictionary)

	videosPublicGroup := apiV1Group.Group("/videos", optionalAuth)
	{
		videosPublicGroup.Get("/", r.listPublicVideos)
		videosPublicGroup.Get("/trending", r.listTrendingVideos)
		videosPublicGroup.Get("/reels", r.listReels)
		videosPublicGroup.Get("/feed/personalized", r.getPersonalizedFeed)
		videosPublicGroup.Get("/:id", r.getVideo)
		videosPublicGroup.Get("/:id/subtitles", r.getVideoSubtitles)
		videosPublicGroup.Post("/:id/view-session", rl.Limit("view-session", 60, time.Minute), r.startViewSession)
		videosPublicGroup.Post("/:id/views", rl.Limit("views", 60, time.Minute), r.recordVideoView)
		videosPublicGroup.Get("/:id/comments", r.listVideoComments)
		videosPublicGroup.Get("/:id/replay-chat", r.getVideoReplayChat)
	}

	commentsPublicGroup := apiV1Group.Group("/comments", optionalAuth)
	{
		commentsPublicGroup.Get("/:id/replies", r.listCommentReplies)
	}

	// Liking and commenting require a real logged-in identity — an
	// OptionalAuth caller with no token would otherwise be indistinguishable
	// from every other anonymous visitor, corrupting per-user like/comment
	// state (see BUSINESS_REQUIREMENTS.md).
	videosProtectedGroup := apiV1Group.Group("/videos", auth)
	{
		videosProtectedGroup.Post("/:id/like", rl.Limit("like", 60, time.Minute), r.toggleLikeVideo)
		videosProtectedGroup.Post("/:id/comments", rl.Limit("comment", 10, time.Minute), r.createComment)
		videosProtectedGroup.Post("/:id/save", rl.Limit("save", 60, time.Minute), r.toggleSaveVideo)
	}

	commentsProtectedGroup := apiV1Group.Group("/comments", auth)
	{
		commentsProtectedGroup.Post("/:id/like", rl.Limit("comment-like", 60, time.Minute), r.toggleLikeComment)
	}

	postsGroup := apiV1Group.Group("/posts", auth)
	{
		postsGroup.Post("/", rl.Limit("post", 5, time.Minute), r.createPost)
		postsGroup.Post("/:id/like", rl.Limit("post-like", 60, time.Minute), r.togglePostLike)
		postsGroup.Delete("/:id", r.deletePost)
	}

	channelsPublicGroup := apiV1Group.Group("/channels", optionalAuth)
	{
		channelsPublicGroup.Get("/:id", r.getChannelDetails)
		channelsPublicGroup.Get("/:id/posts", r.listUserPosts)
	}

	// Internal service-to-service webhooks: worker heartbeat + SRS http_hooks.
	// The transcode webhook lives under /internal (NewInternalRoutes).
	apiV1Group.Post("/admin/workers/heartbeat", internalOnly, r.recordWorkerHeartbeat)

	// SSE streams. Video status and notifications are per-user, so they need
	// the viewer's identity (passed as ?token= since EventSource can't set
	// headers); a live chat room is public to watch.
	apiV1Group.Get("/events/videos", middleware.SSEAuth(jwtManager, u), r.sseVideoEvents)
	apiV1Group.Get("/events/chat/:id", r.sseChatEvents)
	apiV1Group.Get("/events/notifications", middleware.SSEAuth(jwtManager, u), r.sseNotificationEvents)

	// Live view counter of a watch page (docs/message-contract.md §2.3).
	// Browsers can't set headers on a WebSocket, so it is anonymous: only
	// publicly viewable videos can be watched this way.
	apiV1Group.Get("/ws/videos/:id/views", r.wsViewsUpgrade, websocket.New(r.wsViews))

	livePublicGroup := apiV1Group.Group("/live")
	{
		livePublicGroup.Get("/streams", r.listActiveStreams)
		livePublicGroup.Get("/streams/:id", r.getStream)
		livePublicGroup.Post("/streams/:id/presence", optionalAuth, rl.Limit("live-presence", 120, time.Minute), r.liveHeartbeat)
		livePublicGroup.Delete("/streams/:id/presence", optionalAuth, rl.Limit("live-leave", 120, time.Minute), r.liveLeave)
		livePublicGroup.Get("/streams/:id/viewers", r.liveViewersEvents)
		livePublicGroup.Get("/streams/:id/master.m3u8", r.liveMaster)
		livePublicGroup.Get("/streams/:id/sessions", r.listLiveSessions)
		livePublicGroup.Get("/sessions/:sessionID/chat", r.getSessionChatHistory)
		livePublicGroup.Post("/streams/:id/chat", auth, rl.Limit("chat", 20, 30*time.Second), r.sendChatMessage)
		livePublicGroup.Post("/streams/:id/heart", optionalAuth, rl.Limit("heart", 120, time.Minute), r.heartStream)
		livePublicGroup.Post("/auth", internalOnly, r.authenticateRTMPStreamKey) // SRS on_publish
		livePublicGroup.Post("/unpublish", internalOnly, r.unpublishRTMPStream)  // SRS on_unpublish
		livePublicGroup.Post("/dvr", internalOnly, r.handleDVRComplete)          // SRS on_dvr
	}

	// Protected routes
	protected := apiV1Group.Group("", auth)

	vocabGroup := protected.Group("/vocabulary")
	{
		vocabGroup.Post("/", r.saveWord)
		vocabGroup.Get("/", r.listWords)
		vocabGroup.Delete("/:id", r.deleteWord)
	}

	notifGroup := protected.Group("/notifications")
	{
		notifGroup.Get("/", r.listNotifications)
		notifGroup.Post("/read-all", r.markAllNotificationsRead)
		notifGroup.Post("/:id/read", r.markNotificationRead)
	}

	userGroup := protected.Group("/user")
	{
		userGroup.Get("/profile", r.profile)
		userGroup.Put("/profile", r.updateProfile)
		userGroup.Get("/history", r.getWatchHistory)
		userGroup.Get("/following", r.listFollowedChannels)
		userGroup.Put("/preferences", r.setUserPreferences)
		userGroup.Get("/preferences", r.getUserPreferences)
		userGroup.Get("/saved", r.listSavedVideos)
	}

	protected.Post("/channels/:id/follow", rl.Limit("follow", 30, time.Minute), r.toggleFollowChannel)

	protected.Post("/uploads/image-url", rl.Limit("upload-image", 60, time.Hour), r.createImageUpload)

	studioGroup := protected.Group("/studio")
	{
		studioGroup.Get("/videos", r.listStudioVideos)
		studioGroup.Post("/upload-url", rl.Limit("upload", 20, time.Hour), r.createVideoUpload)
		studioGroup.Post("/confirm-upload", r.confirmVideoUpload)
		studioGroup.Post("/videos/:id/publish", r.publishVideo)
		studioGroup.Post("/videos/:id/subtitles", r.uploadVideoSubtitles)
		studioGroup.Put("/videos/:id", r.updateVideo)
		studioGroup.Put("/videos/:id/thumbnail", r.updateThumbnail)
		studioGroup.Delete("/videos/:id", r.deleteVideo)
		studioGroup.Post("/videos/:id/retry", r.retryTranscode)
		studioGroup.Get("/live/key", r.getStreamKey)
		studioGroup.Post("/live/reset-key", r.resetStreamKey)
		studioGroup.Put("/live/info", r.updateStreamInfo)
		studioGroup.Post("/live/end", r.endStream)
	}

	adminGroup := protected.Group("/admin", middleware.RequireAdmin(r.u))
	{
		adminGroup.Get("/dashboard", r.getAdminDashboard)
		adminGroup.Get("/workers", r.getAdminWorkers)
		adminGroup.Post("/streams/:id/ban", r.banStream)
		adminGroup.Post("/streams/:id/unban", r.unbanStream)
		adminGroup.Post("/videos/:id/ban", r.banVideo)
		adminGroup.Get("/users", r.listAdminUsers)
		adminGroup.Post("/users/:id/ban", r.banUser)
		adminGroup.Post("/users/:id/unban", r.unbanUser)
	}
}
