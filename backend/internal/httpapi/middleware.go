package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/XRS0/reader/backend/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	userIDKey     = "bookflow.user_id"
	cookieAuthKey = "bookflow.cookie_auth"
	requestIDKey  = "bookflow.request_id"
)

type apiError struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Details   any    `json:"details"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

func fail(c *gin.Context, status int, code, message string, details any) {
	var body apiError
	body.Error.Code = code
	body.Error.Message = message
	body.Error.Details = details
	if value, ok := c.Get(requestIDKey); ok {
		body.Error.RequestID, _ = value.(string)
	}
	c.AbortWithStatusJSON(status, body)
}
func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.NewString()
		}
		c.Set(requestIDKey, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}
func secureHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		c.Next()
	}
}
func (s *Server) recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		s.logger.Error("panic recovered", "panic", fmt.Sprint(recovered), "request_id", c.GetString(requestIDKey))
		fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", nil)
	})
}
func (s *Server) metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		s.metrics.HTTPDuration.WithLabelValues(c.Request.Method, route, status).Observe(time.Since(start).Seconds())
		s.metrics.HTTPRequests.WithLabelValues(c.Request.Method, route, status).Inc()
		s.logger.Info("http request", "request_id", c.GetString(requestIDKey), "method", c.Request.Method, "route", route, "status", c.Writer.Status(), "duration_ms", time.Since(start).Milliseconds())
	}
}
func (s *Server) authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := ""
		cookieAuth := false
		if header := c.GetHeader("Authorization"); strings.HasPrefix(strings.ToLower(header), "bearer ") {
			raw = strings.TrimSpace(header[7:])
		} else if cookie, err := c.Cookie(s.cfg.Cookie.Name); err == nil {
			raw = cookie
			cookieAuth = true
		}
		if raw == "" {
			fail(c, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication is required", nil)
			return
		}
		claims, err := s.services.Tokens.ParseAccess(raw)
		if err != nil {
			fail(c, http.StatusUnauthorized, "INVALID_ACCESS_TOKEN", "Access token is invalid or expired", nil)
			return
		}
		userID, err := auth.UserIDFromClaims(claims)
		if err != nil {
			fail(c, http.StatusUnauthorized, "INVALID_ACCESS_TOKEN", "Access token is invalid", nil)
			return
		}
		c.Set(userIDKey, userID)
		c.Set(cookieAuthKey, cookieAuth)
		c.Next()
	}
}
func (s *Server) csrfForCookieAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		cookieAuth, _ := c.Get(cookieAuthKey)
		if cookieAuth != true {
			c.Next()
			return
		}
		cookie, err := c.Cookie(s.cfg.Cookie.CSRFCookieName)
		header := c.GetHeader("X-CSRF-Token")
		if err != nil || !s.validCSRF(cookie, header) || !s.validCookieOrigin(c) {
			fail(c, http.StatusForbidden, "CSRF_FAILED", "CSRF token is missing or invalid", nil)
			return
		}
		c.Next()
	}
}

func (s *Server) validCookieOrigin(c *gin.Context) bool {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" {
		return s.cfg.Environment != "production"
	}
	for _, allowed := range s.cfg.CORSOrigins {
		if subtle.ConstantTimeCompare([]byte(origin), []byte(allowed)) == 1 {
			return true
		}
	}
	return false
}

func (s *Server) signedCSRF() (string, error) {
	nonce, err := randomToken()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.Cookie.CSRFSecret))
	_, _ = mac.Write([]byte(nonce))
	return nonce + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Server) validCSRF(cookie, header string) bool {
	if cookie == "" || header == "" || len(cookie) != len(header) || subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) != 1 {
		return false
	}
	parts := strings.Split(cookie, ".")
	if len(parts) != 2 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.Cookie.CSRFSecret))
	_, _ = mac.Write([]byte(parts[0]))
	want, err := base64.RawURLEncoding.DecodeString(parts[1])
	return err == nil && hmac.Equal(want, mac.Sum(nil))
}
func userID(c *gin.Context) uuid.UUID {
	value, _ := c.Get(userIDKey)
	id, _ := value.(uuid.UUID)
	return id
}
func parseID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		fail(c, http.StatusBadRequest, "INVALID_ID", "Resource identifier is invalid", map[string]any{"field": name})
		return uuid.Nil, false
	}
	return id, true
}
func decodeJSON(c *gin.Context, dest any, max int64) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		code := "INVALID_JSON"
		message := "Request body is invalid"
		if errors.Is(err, io.EOF) {
			message = "Request body is required"
		}
		fail(c, http.StatusBadRequest, code, message, nil)
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		fail(c, http.StatusBadRequest, "INVALID_JSON", "Request body must contain one JSON document", nil)
		return false
	}
	return true
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateEntry
	limit   int
	window  time.Duration
}
type rateEntry struct {
	windowStart time.Time
	count       int
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	if limit < 1 {
		limit = 1
	}
	return &rateLimiter{entries: map[string]*rateEntry{}, limit: limit, window: window}
}
func (r *rateLimiter) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		now := time.Now()
		r.mu.Lock()
		entry := r.entries[key]
		if entry == nil || now.Sub(entry.windowStart) >= r.window {
			entry = &rateEntry{windowStart: now}
			r.entries[key] = entry
		}
		entry.count++
		allowed := entry.count <= r.limit
		retry := r.window - now.Sub(entry.windowStart)
		if len(r.entries) > 10000 {
			for k, v := range r.entries {
				if now.Sub(v.windowStart) > 2*r.window {
					delete(r.entries, k)
				}
			}
		}
		r.mu.Unlock()
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			fail(c, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests", nil)
			return
		}
		c.Next()
	}
}
