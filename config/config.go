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
		App     app
		HTTP    http
		Log     log
		PG      pg
		GRPC    grpc
		RMQ     rmq
		NATS    nats
		JWT     jwt
		Redis   redis
		Metrics metrics
		Swagger swagger
		Tracing tracing
		Minio   minio
		DVR     dvr
		SRS     srs
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
		Secret      string        `env:"JWT_SECRET,required"`
		TokenExpiry time.Duration `env:"JWT_TOKEN_EXPIRY" envDefault:"24h"`
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
)

// NewConfig returns app config.
func NewConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config error: %w", err)
	}

	return cfg, nil
}
