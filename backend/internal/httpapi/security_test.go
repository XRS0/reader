package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/XRS0/reader/backend/internal/auth"
	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/XRS0/reader/backend/internal/observability"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSafeAuthResponseNeverSerializesTokens(t *testing.T) {
	response := safeAuthResponse(auth.TokenPair{AccessToken: "access-secret", RefreshToken: "refresh-secret", User: model.User{ID: uuid.New(), Email: "person@example.com"}})
	raw, err := json.Marshal(response)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "access-secret")
	require.NotContains(t, string(raw), "refresh-secret")
}

func TestCookieMutationRequiresSignedDoubleSubmitCSRF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		Environment: "development",
		CORSOrigins: []string{"http://frontend.test"},
		Cookie:      config.Cookie{Name: "bookflow_session", Path: "/", CSRFCookieName: "bookflow_csrf", CSRFSecret: strings.Repeat("s", 32), SameSite: "lax", HTTPOnly: true},
	}
	tokens := auth.NewTokenManagerWithAudience(strings.Repeat("j", 32), "bookflow", "bookflow-web", time.Minute, time.Hour)
	server := &Server{cfg: cfg, services: Services{Tokens: tokens}, logger: observability.DiscardLogger()}
	userID, deviceID := uuid.New(), uuid.New()
	access, accessExpiry, err := tokens.Access(userID, "person@example.com", deviceID)
	require.NoError(t, err)
	pair := auth.TokenPair{AccessToken: access, RefreshToken: "a-valid-refresh-token-value-for-cookie-testing", AccessExpiresAt: accessExpiry, RefreshExpiresAt: time.Now().Add(time.Hour), User: model.User{ID: userID}, Device: model.Device{ID: deviceID}}

	router := gin.New()
	router.GET("/login", func(c *gin.Context) { server.setAuthCookies(c, pair); c.Status(http.StatusNoContent) })
	router.POST("/mutate", server.authenticate(), server.csrfForCookieAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	httpServer := httptest.NewServer(router)
	defer httpServer.Close()
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar}

	loginRequest, err := http.NewRequestWithContext(context.Background(), http.MethodGet, httpServer.URL+"/login", nil)
	require.NoError(t, err)
	loginResponse, err := client.Do(loginRequest)
	require.NoError(t, err)
	csrf := loginResponse.Header.Get("X-CSRF-Token")
	require.NotEmpty(t, csrf)
	_ = loginResponse.Body.Close()

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, httpServer.URL+"/mutate", nil)
	require.NoError(t, err)
	request.Header.Set("Origin", "http://frontend.test")
	request.Header.Set("X-CSRF-Token", csrf)
	response, err := client.Do(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, response.StatusCode)
	_ = response.Body.Close()

	missing, err := http.NewRequestWithContext(context.Background(), http.MethodPost, httpServer.URL+"/mutate", nil)
	require.NoError(t, err)
	missing.Header.Set("Origin", "http://frontend.test")
	response, err = client.Do(missing)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, response.StatusCode)
	_ = response.Body.Close()
}

func TestErrorEnvelopeIncludesRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(requestID())
	router.GET("/failure", func(c *gin.Context) {
		fail(c, http.StatusBadRequest, "TEST_ERROR", "failure", map[string]any{"field": "x"})
	})
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/failure", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var body apiError
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "TEST_ERROR", body.Error.Code)
	require.NotEmpty(t, body.Error.RequestID)
	require.Equal(t, recorder.Header().Get("X-Request-ID"), body.Error.RequestID)
}
