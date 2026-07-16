package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/XRS0/reader/backend/internal/annotations"
	"github.com/XRS0/reader/backend/internal/auth"
	"github.com/XRS0/reader/backend/internal/books"
	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/dictionary"
	"github.com/XRS0/reader/backend/internal/observability"
	"github.com/XRS0/reader/backend/internal/preferences"
	"github.com/XRS0/reader/backend/internal/reading"
	"github.com/XRS0/reader/backend/internal/statistics"
	"github.com/XRS0/reader/backend/internal/storage"
	"github.com/XRS0/reader/backend/internal/translation"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Services struct {
	Auth         *auth.Service
	Tokens       *auth.TokenManager
	Books        *books.Service
	Progress     *reading.ProgressService
	Sessions     *reading.SessionService
	Preferences  *preferences.Service
	Translations *translation.Service
	Dictionary   *dictionary.Service
	Annotations  *annotations.Service
	Statistics   *statistics.Service
	Storage      storage.ObjectStore
}
type Server struct {
	cfg      config.Config
	db       *bun.DB
	services Services
	metrics  *observability.Metrics
	logger   *slog.Logger
}

func New(cfg config.Config, db *bun.DB, services Services, metrics *observability.Metrics, logger *slog.Logger) *gin.Engine {
	s := &Server{cfg: cfg, db: db, services: services, metrics: metrics, logger: logger}
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	if err := r.SetTrustedProxies(cfg.HTTP.TrustedProxies); err != nil {
		logger.Warn("invalid trusted proxy configuration; disabling trusted proxies", "error", err)
		_ = r.SetTrustedProxies(nil)
	}
	r.Use(requestID(), secureHeaders(), s.recovery(), otelgin.Middleware(cfg.ServiceName), s.metricsMiddleware(), cors.New(cors.Config{AllowOrigins: cfg.CORSOrigins, AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}, AllowHeaders: []string{"Authorization", "Content-Type", "Idempotency-Key", "If-Match", "X-CSRF-Token", "X-Request-ID"}, ExposeHeaders: []string{"X-Request-ID", "X-CSRF-Token", "ETag"}, AllowCredentials: true, MaxAge: 12 * time.Hour}))
	r.GET("/health/live", s.live)
	r.GET("/health/ready", s.ready)
	r.GET("/metrics", gin.WrapH(metrics.Handler()))
	v1 := r.Group("/api/v1")
	loginLimit := newRateLimiter(cfg.RateLimit.LoginPerMinute, time.Minute)
	registerLimit := newRateLimiter(cfg.RateLimit.RegisterPerMinute, time.Minute)
	translationLimit := newRateLimiter(cfg.RateLimit.TranslationPerMinute, time.Minute)
	authRoutes := v1.Group("/auth")
	authRoutes.POST("/register", registerLimit.middleware(), s.register)
	authRoutes.POST("/login", loginLimit.middleware(), s.login)
	authRoutes.POST("/refresh", loginLimit.middleware(), s.refresh)
	protectedAuth := authRoutes.Group("")
	protectedAuth.Use(s.authenticate(), s.csrfForCookieAuth())
	protectedAuth.POST("/logout", s.logout)
	protectedAuth.POST("/logout-all", s.logoutAll)
	protectedAuth.GET("/me", s.me)
	protected := v1.Group("")
	protected.Use(s.authenticate(), s.csrfForCookieAuth())
	s.registerBookRoutes(protected)
	s.registerReaderRoutes(protected)
	s.registerTranslationRoutes(protected, translationLimit.middleware())
	s.registerDictionaryRoutes(protected)
	s.registerAnnotationRoutes(protected)
	s.registerStatisticsRoutes(protected)
	protected.GET("/devices", s.devices)
	protected.DELETE("/devices/:deviceId", s.revokeDevice)
	r.NoRoute(func(c *gin.Context) { fail(c, http.StatusNotFound, "ROUTE_NOT_FOUND", "Route was not found", nil) })
	return r
}
func (s *Server) live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC()})
}
func (s *Server) ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.PingContext(ctx); err != nil {
		fail(c, http.StatusServiceUnavailable, "NOT_READY", "PostgreSQL is not ready", nil)
		return
	}
	if err := s.services.Storage.Ready(ctx, s.cfg.S3.OriginalBucket); err != nil {
		fail(c, http.StatusServiceUnavailable, "NOT_READY", "Object storage is not ready", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
func parsePage(c *gin.Context) (int, int) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if cursor := c.Query("cursor"); cursor != "" {
		if parsed, err := strconv.Atoi(cursor); err == nil {
			offset = parsed
		}
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
