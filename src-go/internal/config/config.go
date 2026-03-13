package config

import (
	"os"
	"strconv"
)

type Config struct {
	// HTTP
	Port              int
	VerificationToken string

	// Redis - cache
	RedisURL  string
	RedisCert string

	// Redis - queue (BullMQ)
	MQRedisURL  string
	MQRedisCert string

	// Browser
	ChromiumPath   string
	FirefoxPath    string
	UseFirefox     bool
	LaunchHeadless bool

	// Object storage
	// BlobStorageURL is a gocloud.dev blob URL that selects the backend:
	//   gs://bucket                          → GCS (Application Default Credentials)
	//   s3://bucket?region=us-east-1         → AWS S3
	//   s3://bucket?endpoint=http://minio:9000&use_path_style=true&disable_https=true&region=us-east-1
	//                                         → MinIO
	// When empty, a gs:// URL is constructed from GCSUploadBucket (backward compat).
	BlobStorageURL string

	// Legacy GCS settings kept for backward compatibility.
	// Prefer BLOB_STORAGE_URL for new deployments.
	GCSUploadBucket    string
	GCSKeyFilePath     string
	SkipUploadOriginal bool

	// Analytics (PostHog)
	PostHogAPIKey  string
	SendAnalytics  bool
	APIEnv         string

	// Import metrics
	ImporterMetricsCollectorURL string
	JWTSecret                   string

	// Domain blocking
	MaxFeedFetchFailures int

	// --- API server ---

	// Database
	PGHost     string
	PGPort     int
	PGUser     string
	PGPassword string
	PGDatabase string
	PGPoolMax  int
	// Optional read replica; falls back to primary when empty.
	PGReplicaHost     string
	PGReplicaPort     int
	PGReplicaUser     string
	PGReplicaPassword string
	PGReplicaDatabase string

	// HTTP
	APIPort   int    // default 4000
	ClientURL string // allowed CORS origin

	// Content fetcher (internal)
	ContentFetchURL string

	// Image proxy
	ImageProxyURL    string
	ImageProxySecret string

	// Email
	SendgridAPIKey string

	// Feature flags
	AutoVerify bool // skip email verification in dev
}

func Load() *Config {
	cfg := &Config{
		Port:              envInt("PORT", 3002),
		VerificationToken: os.Getenv("VERIFICATION_TOKEN"),

		RedisURL:  os.Getenv("REDIS_URL"),
		RedisCert: os.Getenv("REDIS_CERT"),

		MQRedisURL:  os.Getenv("MQ_REDIS_URL"),
		MQRedisCert: os.Getenv("MQ_REDIS_CERT"),

		ChromiumPath:   envDefault("CHROMIUM_PATH", "/usr/bin/chromium"),
		FirefoxPath:    envDefault("FIREFOX_PATH", "/usr/bin/firefox"),
		UseFirefox:     os.Getenv("USE_FIREFOX") == "true",
		LaunchHeadless: os.Getenv("LAUNCH_HEADLESS") == "true",

		BlobStorageURL: os.Getenv("BLOB_STORAGE_URL"),

		GCSUploadBucket:    envDefault("GCS_UPLOAD_BUCKET", "omnivore-files"),
		GCSKeyFilePath:     os.Getenv("GCS_UPLOAD_SA_KEY_FILE_PATH"),
		SkipUploadOriginal: os.Getenv("SKIP_UPLOAD_ORIGINAL") == "true",

		PostHogAPIKey: envDefault("POSTHOG_API_KEY", "test"),
		SendAnalytics: os.Getenv("SEND_ANALYTICS") != "",
		APIEnv:        os.Getenv("API_ENV"),

		ImporterMetricsCollectorURL: os.Getenv("IMPORTER_METRICS_COLLECTOR_URL"),
		JWTSecret:                   os.Getenv("JWT_SECRET"),

		MaxFeedFetchFailures: envInt("MAX_FEED_FETCH_FAILURES", 10),

		PGHost:     envDefault("PG_HOST", "localhost"),
		PGPort:     envInt("PG_PORT", 5432),
		PGUser:     envDefault("PG_USER", "app_user"),
		PGPassword: os.Getenv("PG_PASSWORD"),
		PGDatabase: envDefault("PG_DB", "omnivore"),
		PGPoolMax:  envInt("PG_POOL_MAX", 10),

		PGReplicaHost:     os.Getenv("PG_REPLICA_HOST"),
		PGReplicaPort:     envInt("PG_REPLICA_PORT", 5432),
		PGReplicaUser:     os.Getenv("PG_REPLICA_USER"),
		PGReplicaPassword: os.Getenv("PG_REPLICA_PASSWORD"),
		PGReplicaDatabase: os.Getenv("PG_REPLICA_DB"),

		APIPort:   envInt("PORT", 4000),
		ClientURL: os.Getenv("CLIENT_URL"),

		ContentFetchURL: os.Getenv("CONTENT_FETCH_URL"),

		ImageProxyURL:    os.Getenv("IMAGE_PROXY_URL"),
		ImageProxySecret: os.Getenv("IMAGE_PROXY_SECRET"),

		SendgridAPIKey: os.Getenv("SENDGRID_API_KEY"),

		AutoVerify: os.Getenv("AUTO_VERIFY") == "true",
	}

	return cfg
}

// BlobURL returns the effective gocloud.dev blob URL to open.
// If BLOB_STORAGE_URL is set it is returned as-is.
// Otherwise a gs:// URL is constructed from GCS_UPLOAD_BUCKET for backward
// compatibility with existing GCS deployments.
func (c *Config) BlobURL() string {
	if c.BlobStorageURL != "" {
		return c.BlobStorageURL
	}
	return "gs://" + c.GCSUploadBucket
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
