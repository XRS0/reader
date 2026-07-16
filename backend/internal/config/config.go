package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment   string
	ServiceName   string
	HTTP          HTTP
	Database      Database
	JWT           JWT
	Cookie        Cookie
	S3            S3
	Upload        Upload
	Worker        Worker
	Reading       Reading
	Translation   Translation
	Observability Observability
	RateLimit     RateLimit
	CORSOrigins   []string
}

type HTTP struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	TrustedProxies  []string
}

type Database struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	PingTimeout     time.Duration
}

type JWT struct {
	Secret       string
	Issuer       string
	Audience     string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	ArgonMemory  uint32
	ArgonTime    uint32
	ArgonThreads uint8
}

type Cookie struct {
	Name           string
	Path           string
	Domain         string
	Secure         bool
	HTTPOnly       bool
	SameSite       string
	CSRFSecret     string
	CSRFCookieName string
}

type S3 struct {
	Endpoint       string
	PublicEndpoint string
	Region         string
	AccessKey      string
	SecretKey      string
	UsePathStyle   bool
	OperationTTL   time.Duration
	PresignTTL     time.Duration
	MaxRetries     int
	OriginalBucket string
	ContentBucket  string
	AssetsBucket   string
	CoversBucket   string
	ExportsBucket  string
}

type Upload struct {
	MaxBytes             int64
	MaxEPUBFiles         int
	MaxEPUBUnpackedBytes int64
	MaxCompressionRatio  float64
}

type Worker struct {
	ID             string
	PollInterval   time.Duration
	JobTimeout     time.Duration
	Concurrency    int
	MaxAttempts    int
	StaleLockAfter time.Duration
}

type Reading struct {
	HeartbeatInterval time.Duration
	HeartbeatMaxGap   time.Duration
	IdleThreshold     time.Duration
	StaleAfter        time.Duration
}

type Translation struct {
	Provider       string
	APIKey         string
	Endpoint       string
	MaxRetries     int
	MaxWordRunes   int
	MaxTextRunes   int
	CacheTTL       time.Duration
	ProviderModel  string
	PromptVersion  string
	RequestTimeout time.Duration
}

type Observability struct {
	OTLPEndpoint string
	OTELInsecure bool
	TraceRatio   float64
}

type RateLimit struct {
	LoginPerMinute       int
	RegisterPerMinute    int
	TranslationPerMinute int
}

func Load() (Config, error) {
	c := Config{
		Environment: envAny([]string{"APP_ENV", "BOOKFLOW_ENV"}, "development"),
		ServiceName: envAny([]string{"OTEL_SERVICE_NAME", "APP_NAME", "SERVICE_NAME"}, "bookflow-api"),
		HTTP: HTTP{
			Addr: env("HTTP_ADDR", ":8080"), ReadTimeout: duration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: duration("HTTP_WRITE_TIMEOUT", 30*time.Second), IdleTimeout: duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: duration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second), TrustedProxies: csv("HTTP_TRUSTED_PROXIES"),
		},
		Database:      Database{URL: env("DATABASE_URL", "postgres://bookflow:bookflow@localhost:5432/bookflow?sslmode=disable"), MaxOpenConns: integerAny([]string{"DATABASE_MAX_OPEN_CONNS", "DB_MAX_OPEN_CONNS"}, 20), MaxIdleConns: integerAny([]string{"DATABASE_MAX_IDLE_CONNS", "DB_MAX_IDLE_CONNS"}, 10), ConnMaxLifetime: durationAny([]string{"DATABASE_CONN_MAX_LIFETIME", "DB_CONN_MAX_LIFETIME"}, 30*time.Minute), PingTimeout: duration("DB_PING_TIMEOUT", 5*time.Second)},
		JWT:           JWT{Secret: envAny([]string{"JWT_SIGNING_KEY", "JWT_SECRET"}, ""), Issuer: env("JWT_ISSUER", "bookflow"), Audience: env("JWT_AUDIENCE", "bookflow-web"), AccessTTL: duration("JWT_ACCESS_TTL", 15*time.Minute), RefreshTTL: duration("JWT_REFRESH_TTL", 30*24*time.Hour), ArgonMemory: uint32(integer("ARGON2_MEMORY_KIB", 64*1024)), ArgonTime: uint32(integer("ARGON2_ITERATIONS", 3)), ArgonThreads: uint8(integer("ARGON2_PARALLELISM", 2))},
		Cookie:        Cookie{Name: env("COOKIE_NAME", "bookflow_session"), Path: env("COOKIE_PATH", "/"), Domain: os.Getenv("COOKIE_DOMAIN"), Secure: boolean("COOKIE_SECURE", false), HTTPOnly: boolean("COOKIE_HTTP_ONLY", true), SameSite: env("COOKIE_SAME_SITE", "lax"), CSRFSecret: os.Getenv("CSRF_SECRET"), CSRFCookieName: env("CSRF_COOKIE_NAME", "bookflow_csrf")},
		S3:            S3{Endpoint: envAny([]string{"RUSTFS_ENDPOINT", "S3_ENDPOINT"}, "http://localhost:9000"), PublicEndpoint: envAny([]string{"RUSTFS_PUBLIC_ENDPOINT", "S3_PUBLIC_ENDPOINT", "RUSTFS_ENDPOINT", "S3_ENDPOINT"}, "http://localhost:9000"), Region: envAny([]string{"RUSTFS_REGION", "S3_REGION"}, "us-east-1"), AccessKey: envAny([]string{"RUSTFS_ACCESS_KEY", "S3_ACCESS_KEY"}, "bookflow"), SecretKey: envAny([]string{"RUSTFS_SECRET_KEY", "S3_SECRET_KEY"}, "bookflow-development-only"), UsePathStyle: booleanAny([]string{"RUSTFS_USE_PATH_STYLE", "S3_USE_PATH_STYLE"}, true), OperationTTL: durationAny([]string{"RUSTFS_OPERATION_TIMEOUT", "S3_OPERATION_TIMEOUT"}, 30*time.Second), PresignTTL: durationAny([]string{"RUSTFS_PRESIGN_TTL", "S3_PRESIGN_TTL"}, 10*time.Minute), MaxRetries: integer("RUSTFS_MAX_RETRIES", 3), OriginalBucket: envAny([]string{"RUSTFS_BUCKET_BOOKS_ORIGINAL", "S3_BUCKET_ORIGINAL"}, "books-original"), ContentBucket: envAny([]string{"RUSTFS_BUCKET_BOOKS_CONTENT", "S3_BUCKET_CONTENT"}, "books-content"), AssetsBucket: envAny([]string{"RUSTFS_BUCKET_BOOKS_ASSETS", "S3_BUCKET_ASSETS"}, "books-assets"), CoversBucket: envAny([]string{"RUSTFS_BUCKET_BOOKS_COVERS", "S3_BUCKET_COVERS"}, "books-covers"), ExportsBucket: envAny([]string{"RUSTFS_BUCKET_USER_EXPORTS", "S3_BUCKET_EXPORTS"}, "user-exports")},
		Upload:        Upload{MaxBytes: int64(integer("UPLOAD_MAX_BYTES", 100*1024*1024)), MaxEPUBFiles: integer("EPUB_MAX_FILES", 5000), MaxEPUBUnpackedBytes: int64(integerAny([]string{"EPUB_MAX_UNCOMPRESSED_BYTES", "EPUB_MAX_UNPACKED_BYTES"}, 500*1024*1024)), MaxCompressionRatio: floating("EPUB_MAX_COMPRESSION_RATIO", 100)},
		Worker:        Worker{ID: env("WORKER_ID", "worker-local"), PollInterval: duration("WORKER_POLL_INTERVAL", time.Second), JobTimeout: duration("WORKER_JOB_TIMEOUT", 5*time.Minute), Concurrency: integer("WORKER_CONCURRENCY", 2), MaxAttempts: integer("WORKER_MAX_ATTEMPTS", 5), StaleLockAfter: duration("WORKER_STALE_LOCK_AFTER", 10*time.Minute)},
		Reading:       Reading{HeartbeatInterval: duration("READING_HEARTBEAT_INTERVAL", 15*time.Second), HeartbeatMaxGap: durationAny([]string{"READING_MAX_CREDIT_INTERVAL", "READING_HEARTBEAT_MAX_GAP"}, 45*time.Second), IdleThreshold: duration("READING_IDLE_THRESHOLD", 2*time.Minute), StaleAfter: duration("READING_STALE_AFTER", 2*time.Minute)},
		Translation:   Translation{Provider: env("TRANSLATION_PROVIDER", "mock"), APIKey: os.Getenv("TRANSLATION_API_KEY"), Endpoint: env("TRANSLATION_ENDPOINT", "https://api.openai.com/v1/responses"), MaxRetries: integer("TRANSLATION_MAX_RETRIES", 2), MaxWordRunes: integer("TRANSLATION_MAX_WORD_RUNES", 128), MaxTextRunes: integer("TRANSLATION_MAX_TEXT_RUNES", 2000), CacheTTL: duration("TRANSLATION_CACHE_TTL", 30*24*time.Hour), ProviderModel: envAny([]string{"TRANSLATION_MODEL", "TRANSLATION_PROVIDER_MODEL"}, "mock-v1"), PromptVersion: env("TRANSLATION_PROMPT_VERSION", "v1"), RequestTimeout: duration("TRANSLATION_TIMEOUT", 15*time.Second)},
		Observability: Observability{OTLPEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), OTELInsecure: boolean("OTEL_EXPORTER_OTLP_INSECURE", true), TraceRatio: floating("OTEL_TRACE_RATIO", 0.1)},
		RateLimit:     RateLimit{LoginPerMinute: integerAny([]string{"RATE_LIMIT_LOGIN", "RATE_LIMIT_LOGIN_PER_MINUTE"}, 10), RegisterPerMinute: integerAny([]string{"RATE_LIMIT_REGISTER", "RATE_LIMIT_REGISTER_PER_MINUTE"}, 5), TranslationPerMinute: integerAny([]string{"RATE_LIMIT_TRANSLATION", "RATE_LIMIT_TRANSLATION_PER_MINUTE"}, 30)},
		CORSOrigins:   csvDefault("CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173", "http://127.0.0.1:5173"}),
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) Validate() error {
	var errs []error
	if _, err := url.ParseRequestURI(c.Database.URL); err != nil || !strings.HasPrefix(c.Database.URL, "postgres") {
		errs = append(errs, errors.New("DATABASE_URL must be a PostgreSQL URL"))
	}
	if len(c.JWT.Secret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET must contain at least 32 characters"))
	}
	if len(c.Cookie.CSRFSecret) < 32 {
		errs = append(errs, errors.New("CSRF_SECRET must contain at least 32 characters"))
	}
	if c.JWT.AccessTTL <= 0 || c.JWT.RefreshTTL <= c.JWT.AccessTTL {
		errs = append(errs, errors.New("JWT TTL values are invalid"))
	}
	if c.Upload.MaxBytes <= 0 || c.Upload.MaxEPUBFiles <= 0 || c.Upload.MaxEPUBUnpackedBytes < c.Upload.MaxBytes {
		errs = append(errs, errors.New("upload/EPUB limits are invalid"))
	}
	if c.Worker.Concurrency < 1 || c.Worker.MaxAttempts < 1 {
		errs = append(errs, errors.New("worker settings are invalid"))
	}
	if c.Worker.StaleLockAfter <= c.Worker.JobTimeout {
		errs = append(errs, errors.New("WORKER_STALE_LOCK_AFTER must exceed WORKER_JOB_TIMEOUT"))
	}
	if c.Reading.HeartbeatInterval <= 0 || c.Reading.HeartbeatMaxGap < c.Reading.HeartbeatInterval || c.Reading.StaleAfter < c.Reading.HeartbeatInterval {
		errs = append(errs, errors.New("reading heartbeat settings are invalid"))
	}
	if c.S3.Endpoint == "" || c.S3.AccessKey == "" || c.S3.SecretKey == "" {
		errs = append(errs, errors.New("S3 endpoint and credentials are required"))
	}
	if c.Cookie.SameSite != "lax" && c.Cookie.SameSite != "strict" && c.Cookie.SameSite != "none" {
		errs = append(errs, errors.New("COOKIE_SAME_SITE must be lax, strict, or none"))
	}
	for _, origin := range c.CORSOrigins {
		if origin == "*" {
			errs = append(errs, errors.New("wildcard CORS origin is not allowed with credentials"))
		}
	}
	if c.Environment == "production" {
		if !c.Cookie.Secure || !c.Cookie.HTTPOnly {
			errs = append(errs, errors.New("production cookies must be Secure and HttpOnly"))
		}
		publicURL, err := url.Parse(c.S3.PublicEndpoint)
		if err != nil || publicURL.Scheme != "https" {
			errs = append(errs, errors.New("production RUSTFS_PUBLIC_ENDPOINT must use HTTPS"))
		}
		if c.S3.AccessKey == "bookflow" || c.S3.SecretKey == "bookflow-development-only" {
			errs = append(errs, errors.New("development RustFS credentials cannot be used in production"))
		}
	}
	if c.Translation.Provider == "openai" && c.Translation.APIKey == "" {
		errs = append(errs, errors.New("TRANSLATION_API_KEY is required for the openai provider"))
	}
	return errors.Join(errs...)
}

func env(k, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return fallback
}
func envAny(keys []string, fallback string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return fallback
}
func integer(k string, fallback int) int {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
func integerAny(keys []string, fallback int) int {
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return integer(key, fallback)
		}
	}
	return fallback
}
func floating(k string, fallback float64) float64 {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}
func boolean(k string, fallback bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
func booleanAny(keys []string, fallback bool) bool {
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return boolean(key, fallback)
		}
	}
	return fallback
}
func duration(k string, fallback time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
func durationAny(keys []string, fallback time.Duration) time.Duration {
	for _, key := range keys {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return duration(key, fallback)
		}
	}
	return fallback
}
func csv(k string) []string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return nil
	}
	return split(v)
}
func csvDefault(k string, fallback []string) []string {
	if v := csv(k); len(v) > 0 {
		return v
	}
	return fallback
}
func split(v string) []string {
	raw := strings.Split(v, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func (c Config) String() string {
	return fmt.Sprintf("env=%s http=%s database_configured=%t s3=%s", c.Environment, c.HTTP.Addr, c.Database.URL != "", c.S3.Endpoint)
}
