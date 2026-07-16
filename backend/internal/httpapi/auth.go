package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/XRS0/reader/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

func (s *Server) register(c *gin.Context) {
	var in auth.RegisterInput
	if !decodeJSON(c, &in, 64*1024) {
		return
	}
	in.UserAgent = c.Request.UserAgent()
	pair, err := s.services.Auth.Register(c.Request.Context(), in)
	if err != nil {
		s.authError(c, err)
		return
	}
	s.setAuthCookies(c, pair)
	c.JSON(http.StatusCreated, safeAuthResponse(pair))
}
func (s *Server) login(c *gin.Context) {
	var in auth.LoginInput
	if !decodeJSON(c, &in, 64*1024) {
		return
	}
	in.UserAgent = c.Request.UserAgent()
	pair, err := s.services.Auth.Login(c.Request.Context(), in)
	if err != nil {
		s.authError(c, err)
		return
	}
	s.setAuthCookies(c, pair)
	c.JSON(http.StatusOK, safeAuthResponse(pair))
}
func (s *Server) refresh(c *gin.Context) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if c.Request.ContentLength > 0 && !decodeJSON(c, &in, 16*1024) {
		return
	}
	fromCookie := false
	if in.RefreshToken == "" {
		in.RefreshToken, _ = c.Cookie(s.cfg.Cookie.Name + "_refresh")
		fromCookie = true
	}
	if fromCookie {
		csrfCookie, _ := c.Cookie(s.cfg.Cookie.CSRFCookieName)
		if !s.validCSRF(csrfCookie, c.GetHeader("X-CSRF-Token")) || !s.validCookieOrigin(c) {
			fail(c, http.StatusForbidden, "CSRF_FAILED", "CSRF token is missing or invalid", nil)
			return
		}
	}
	pair, err := s.services.Auth.Rotate(c.Request.Context(), in.RefreshToken)
	if err != nil {
		s.clearAuthCookies(c)
		s.authError(c, err)
		return
	}
	s.setAuthCookies(c, pair)
	c.JSON(http.StatusOK, safeAuthResponse(pair))
}
func (s *Server) logout(c *gin.Context) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if c.Request.ContentLength > 0 && !decodeJSON(c, &in, 16*1024) {
		return
	}
	if in.RefreshToken == "" {
		in.RefreshToken, _ = c.Cookie(s.cfg.Cookie.Name + "_refresh")
	}
	if in.RefreshToken != "" {
		if err := s.services.Auth.Logout(c.Request.Context(), userID(c), in.RefreshToken); err != nil {
			fail(c, http.StatusInternalServerError, "LOGOUT_FAILED", "Could not log out", nil)
			return
		}
	}
	s.clearAuthCookies(c)
	c.Status(http.StatusNoContent)
}
func (s *Server) logoutAll(c *gin.Context) {
	if err := s.services.Auth.LogoutAll(c.Request.Context(), userID(c)); err != nil {
		fail(c, http.StatusInternalServerError, "LOGOUT_FAILED", "Could not log out all devices", nil)
		return
	}
	s.clearAuthCookies(c)
	c.Status(http.StatusNoContent)
}
func (s *Server) me(c *gin.Context) {
	u, err := s.services.Auth.User(c.Request.Context(), userID(c))
	if err != nil {
		s.authError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": u})
}
func (s *Server) devices(c *gin.Context) {
	items, err := s.services.Auth.Devices(c.Request.Context(), userID(c))
	if err != nil {
		fail(c, http.StatusInternalServerError, "DEVICES_FAILED", "Could not list devices", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
func (s *Server) revokeDevice(c *gin.Context) {
	id, ok := parseID(c, "deviceId")
	if !ok {
		return
	}
	if err := s.services.Auth.RevokeDevice(c.Request.Context(), userID(c), id); err != nil {
		fail(c, http.StatusNotFound, "DEVICE_NOT_FOUND", "Device was not found", nil)
		return
	}
	c.Status(http.StatusNoContent)
}
func (s *Server) authError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auth.ErrEmailExists):
		fail(c, http.StatusConflict, "EMAIL_EXISTS", "Email is already registered", nil)
	case errors.Is(err, auth.ErrWeakPassword):
		fail(c, http.StatusBadRequest, "WEAK_PASSWORD", err.Error(), nil)
	case errors.Is(err, auth.ErrRefreshReuse):
		fail(c, http.StatusUnauthorized, "REFRESH_TOKEN_REUSE", "Refresh token reuse was detected; this login session was revoked", nil)
	case errors.Is(err, auth.ErrUnauthorized), errors.Is(err, auth.ErrInvalidToken):
		fail(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Credentials or token are invalid", nil)
	default:
		if strings.Contains(err.Error(), "email") || strings.Contains(err.Error(), "display name") || strings.Contains(err.Error(), "locale") || strings.Contains(err.Error(), "timezone") {
			fail(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), nil)
		} else {
			s.logger.Error("auth operation", "error", err, "request_id", c.GetString(requestIDKey))
			fail(c, http.StatusInternalServerError, "AUTH_FAILED", "Authentication operation failed", nil)
		}
	}
}
func (s *Server) setAuthCookies(c *gin.Context, p auth.TokenPair) {
	same := s.sameSite()
	http.SetCookie(c.Writer, &http.Cookie{Name: s.cfg.Cookie.Name, Value: p.AccessToken, Path: s.cfg.Cookie.Path, Domain: s.cfg.Cookie.Domain, Expires: p.AccessExpiresAt, MaxAge: int(time.Until(p.AccessExpiresAt).Seconds()), HttpOnly: s.cfg.Cookie.HTTPOnly, Secure: s.cfg.Cookie.Secure, SameSite: same})
	http.SetCookie(c.Writer, &http.Cookie{Name: s.cfg.Cookie.Name + "_refresh", Value: p.RefreshToken, Path: "/api/v1/auth", Domain: s.cfg.Cookie.Domain, Expires: p.RefreshExpiresAt, MaxAge: int(time.Until(p.RefreshExpiresAt).Seconds()), HttpOnly: true, Secure: s.cfg.Cookie.Secure, SameSite: same})
	csrf, err := s.signedCSRF()
	if err != nil {
		s.logger.Error("generate CSRF token", "error", err)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: s.cfg.Cookie.CSRFCookieName, Value: csrf, Path: s.cfg.Cookie.Path, Domain: s.cfg.Cookie.Domain, Expires: p.RefreshExpiresAt, HttpOnly: false, Secure: s.cfg.Cookie.Secure, SameSite: same})
	c.Header("X-CSRF-Token", csrf)
}

func safeAuthResponse(p auth.TokenPair) gin.H {
	return gin.H{"user": p.User, "device": p.Device, "access_expires_at": p.AccessExpiresAt, "refresh_expires_at": p.RefreshExpiresAt}
}
func (s *Server) clearAuthCookies(c *gin.Context) {
	for _, cookie := range []struct {
		name, path string
		httpOnly   bool
	}{{s.cfg.Cookie.Name, s.cfg.Cookie.Path, true}, {s.cfg.Cookie.Name + "_refresh", "/api/v1/auth", true}, {s.cfg.Cookie.CSRFCookieName, s.cfg.Cookie.Path, false}} {
		http.SetCookie(c.Writer, &http.Cookie{Name: cookie.name, Value: "", Path: cookie.path, Domain: s.cfg.Cookie.Domain, MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: cookie.httpOnly, Secure: s.cfg.Cookie.Secure, SameSite: s.sameSite()})
	}
}
func (s *Server) sameSite() http.SameSite {
	switch strings.ToLower(s.cfg.Cookie.SameSite) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
