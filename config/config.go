package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	_ "github.com/joho/godotenv/autoload"
)

type (
	// Config -.
	Config struct {
		App       app
		HTTP      http
		Log       log
		PG        pg
		GRPC      grpc
		RMQ       rmq
		NATS      nats
		JWT       jwt
		Redis     redis
		Metrics   metrics
		Swagger   swagger
		Tracing   tracing
		Minio     minio
		DVR       dvr
		SRS       srs
		Security  security
		Transcode     transcode
		Benchmark     benchmark
		ViewCounter   viewCounter
		TranscodeSync transcodeSync
	}

	// App -.
	app struct {
		Name    string `env:"APP_NAME,required"`
		Version string `env:"APP_VERSION,required"`
	}

	// HTTP -.
	http struct {
		Port           string `env:"HTTP_PORT,required"`
		UsePreforkMode bool   `env:"HTTP_USE_PREFORK_MODE" envDefault:"false"`
		// CORSAllowOrigins is a comma-separated list of browser origins
		// allowed to call the API (the Next.js frontend).
		CORSAllowOrigins string `env:"CORS_ALLOW_ORIGINS" envDefault:"http://localhost:3000"`
		// TrustedProxies is a comma-separated list of reverse-proxy IPs/CIDRs
		// (e.g. nginx) whose X-Forwarded-For header is trusted for the real
		// client IP. Empty = no proxy: ctx.IP() is the TCP peer address and a
		// client-supplied X-Forwarded-For is ignored.
		TrustedProxies string `env:"HTTP_TRUSTED_PROXIES" envDefault:""`
	}

	// Log -.
	log struct {
		Level string `env:"LOG_LEVEL,required"`
	}

	// PG -.
	pg struct {
		PoolMax int    `env:"PG_POOL_MAX,required"`
		URL     string `env:"PG_URL,required"`
	}

	// GRPC -.
	grpc struct {
		Port string `env:"GRPC_PORT,required"`
	}

	// RMQ -.
	rmq struct {
		ServerExchange string `env:"RMQ_RPC_SERVER,required"`
		ClientExchange string `env:"RMQ_RPC_CLIENT,required"`
		URL            string `env:"RMQ_URL,required"`
	}

	// NATS -.
	nats struct {
		ServerExchange string `env:"NATS_RPC_SERVER,required"`
		URL            string `env:"NATS_URL,required"`
	}

	// JWT -.
	jwt struct {
		Secret             string        `env:"JWT_SECRET,required"`
		TokenExpiry        time.Duration `env:"JWT_TOKEN_EXPIRY" envDefault:"1h"`
		RefreshTokenExpiry time.Duration `env:"JWT_REFRESH_TOKEN_EXPIRY" envDefault:"168h"`
	}

	// Redis -.
	redis struct {
		URL string `env:"REDIS_URL" envDefault:"localhost:6379"`
	}

	// Metrics -.
	metrics struct {
		Enabled bool `env:"METRICS_ENABLED" envDefault:"true"`
	}

	// Swagger -.
	swagger struct {
		Enabled bool `env:"SWAGGER_ENABLED" envDefault:"false"`
	}

	// Tracing -.
	tracing struct {
		Enabled      bool    `env:"TRACING_ENABLED" envDefault:"false"`
		OTLPEndpoint string  `env:"TRACING_OTLP_ENDPOINT" envDefault:"localhost:4317"`
		OTLPInsecure bool    `env:"TRACING_OTLP_INSECURE" envDefault:"true"`
		SampleRate   float64 `env:"TRACING_SAMPLE_RATE" envDefault:"0.1"`
	}

	// Minio -.
	minio struct {
		Endpoint  string `env:"MINIO_ENDPOINT" envDefault:"localhost:9000"`
		AccessKey string `env:"MINIO_ACCESS_KEY" envDefault:"minioadmin"`
		SecretKey string `env:"MINIO_SECRET_KEY" envDefault:"minioadmin123"`
		UseSSL    bool   `env:"MINIO_USE_SSL" envDefault:"false"`
		RawBucket string `env:"MINIO_RAW_BUCKET" envDefault:"raw-videos"`
		// PublicBucket holds user-facing images (avatars, thumbnails, post
		// images). It must allow anonymous GET, unlike RawBucket.
		PublicBucket string `env:"MINIO_PUBLIC_BUCKET" envDefault:"public-assets"`
		// PublicURL is the MinIO address the BROWSER uploads to (the upload URL
		// handed out by POST /studio/upload-url). Endpoint above is the address
		// the backend itself uses, e.g. minio:9000 inside docker compose.
		PublicURL string `env:"MINIO_PUBLIC_URL" envDefault:"http://localhost:9000"`
		// PresignUploads hands the browser a signed, expiring PUT URL instead of
		// the bare object URL. Required whenever MinIO is reachable from the
		// internet (the bucket then stays private); the default keeps the dev
		// stack's anonymous-upload behaviour.
		PresignUploads bool `env:"MINIO_PRESIGN_UPLOADS" envDefault:"false"`
	}

	// DVR -. Local filesystem path where SRS writes livestream recordings
	// (bind-mounted into the SRS container at the same relative dvr_path).
	dvr struct {
		LocalDir string `env:"DVR_LOCAL_DIR" envDefault:"./dvr-data"`
	}

	// SRS -. SRS's own read-only http_api (not the http_hooks callback URLs,
	// which SRS calls into us on) — polled periodically to reconcile the
	// `is_live` DB flag in case an on_unpublish webhook never arrives.
	srs struct {
		APIURL            string        `env:"SRS_API_URL" envDefault:"http://localhost:1985"`
		ReconcileInterval time.Duration `env:"SRS_RECONCILE_INTERVAL" envDefault:"15s"`
	}

	// Security -.
	security struct {
		// InternalWebhookSecret authenticates service-to-service webhooks:
		// it is the HMAC key of the signed transcode webhook and the shared
		// token of the worker heartbeat / SRS http_hooks. Must match the
		// transcode worker's INTERNAL_WEBHOOK_SECRET and the ?secret= in the
		// SRS hook URLs.
		InternalWebhookSecret string `env:"INTERNAL_WEBHOOK_SECRET,required"`
		// RateLimitEnabled can be turned off for load tests (bot/).
		RateLimitEnabled bool `env:"RATE_LIMIT_ENABLED" envDefault:"true"`
	}

	// Transcode -. Renditions requested in every transcode job
	// (docs/message-contract.md §2.1).
	transcode struct {
		Renditions []string `env:"TRANSCODE_RENDITIONS" envSeparator:"," envDefault:"720p,480p"`
	}

	// Benchmark — lifts rate limits that would block load tests.
	benchmark struct {
		// Mode disables the per-IP hourly view cap and the 10-second ticket
		// wait so every request from a single k6 machine is counted.
		Mode bool `env:"BENCHMARK_MODE" envDefault:"false"`
	}

	// ViewCounter selects how an accepted view is counted. Only the control
	// variant of the Redis-batch vs direct-DB experiment should change it.
	viewCounter struct {
		// Mode: "redis" (default) = INCR in Redis, flushed in batches by the
		// view-sync worker; "direct_db" = UPDATE videos SET views = views + 1
		// on every accepted view.
		Mode string `env:"VIEW_COUNTER_MODE" envDefault:"redis"`
	}

	// TranscodeSync switches from the async NATS queue to a blocking
	// synchronous FFmpeg call in the same process. Only for the comparison
	// experiment required by the thesis (GĐ2).
	transcodeSync struct {
		// Mode: "async" (default) = NATS JetStream; "sync" = inline FFmpeg.
		Mode string `env:"TRANSCODE_MODE" envDefault:"async"`
		// FFmpegPath is the FFmpeg binary used in sync mode.
		FFmpegPath string `env:"FFMPEG_PATH" envDefault:"ffmpeg"`
		// FFprobePath is the FFprobe binary used in sync mode.
		FFprobePath string `env:"FFPROBE_PATH" envDefault:"ffprobe"`
		// TempDir is the working directory for sync transcodes.
		TempDir string `env:"TEMP_DIR" envDefault:"/tmp/transcode"`
		// MinioHLSBucket is the bucket where sync mode uploads HLS output.
		MinioHLSBucket string `env:"MINIO_HLS_BUCKET" envDefault:"hls-streams"`
	}
)

// NewConfig returns app config.
func NewConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	return cfg, nil
}
