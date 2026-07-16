//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/XRS0/reader/backend/internal/auth"
	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/httpapi"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/XRS0/reader/backend/internal/observability"
	"github.com/XRS0/reader/backend/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRefreshTokenRotationDetectsReuseAndRevokesFamily(t *testing.T) {
	resetDatabase(t)
	tokens := auth.NewTokenManagerWithAudience(
		"integration-test-signing-secret-at-least-32-characters",
		"bookflow-integration",
		"bookflow-integration-web",
		15*time.Minute,
		24*time.Hour,
	)
	service := auth.NewService(
		integrationDB,
		auth.NewPasswordHasher(8*1024, 1, 1),
		tokens,
	)

	registered, err := service.Register(testContext(t), auth.RegisterInput{
		Email:       "  Reader@Example.Test ",
		Password:    "correct horse battery staple",
		DisplayName: "Reader",
		Timezone:    "Asia/Yekaterinburg",
		Locale:      "ru",
		DeviceKey:   "integration-browser",
		DeviceName:  "Integration browser",
		UserAgent:   "BookFlow integration test",
	})
	require.NoError(t, err)
	require.Equal(t, "reader@example.test", registered.User.Email)
	require.Equal(t, "ru", registered.User.Locale)
	require.Equal(t, "Asia/Yekaterinburg", registered.User.Timezone)
	require.NotEmpty(t, registered.AccessToken)
	require.NotEmpty(t, registered.RefreshToken)

	claims, err := tokens.ParseAccess(registered.AccessToken)
	require.NoError(t, err)
	require.Equal(t, registered.User.ID.String(), claims.Subject)
	require.Equal(t, registered.Device.ID.String(), claims.DeviceID)
	require.Equal(t, registered.User.Email, claims.Email)

	preferenceCount, err := integrationDB.NewSelect().Table("reader_preferences").
		Where("user_id=?", registered.User.ID).
		Count(testContext(t))
	require.NoError(t, err)
	require.Equal(t, 1, preferenceCount)

	gin.SetMode(gin.TestMode)
	api := httpapi.New(
		config.Config{
			Environment: "test",
			ServiceName: "bookflow-integration",
			Cookie: config.Cookie{
				Name:           "bookflow_session",
				Path:           "/",
				HTTPOnly:       true,
				SameSite:       "lax",
				CSRFSecret:     "integration-csrf-secret-at-least-32-characters",
				CSRFCookieName: "bookflow_csrf",
			},
			RateLimit: config.RateLimit{
				LoginPerMinute:       100,
				RegisterPerMinute:    100,
				TranslationPerMinute: 100,
			},
			CORSOrigins: []string{"http://example.test"},
		},
		integrationDB,
		httpapi.Services{Auth: service, Tokens: tokens, Storage: storage.NewMemoryStore()},
		observability.NewMetrics(),
		observability.DiscardLogger(),
	)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{
      "email":"unknown-field@example.test",
      "password":"correct horse battery staple",
      "display_name":"Unknown field",
      "timezone":"UTC",
      "locale":"en",
      "device_key":"unknown-field-device",
      "unexpected_admin":true
    }`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	api.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var errorResponse struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &errorResponse))
	require.Equal(t, "INVALID_JSON", errorResponse.Error.Code, "register must reject unknown JSON fields")
	unknownUserCount, err := integrationDB.NewSelect().Table("users").
		Where("email=?", "unknown-field@example.test").
		Count(testContext(t))
	require.NoError(t, err)
	require.Zero(t, unknownUserCount)

	_, err = service.Register(testContext(t), auth.RegisterInput{
		Email:      "unsupported-locale@example.test",
		Password:   "correct horse battery staple",
		Timezone:   "UTC",
		Locale:     "de",
		DeviceKey:  "unsupported-locale-device",
		DeviceName: "Unsupported locale",
	})
	require.EqualError(t, err, "invalid locale")
	unsupportedLocaleCount, err := integrationDB.NewSelect().Table("users").
		Where("email=?", "unsupported-locale@example.test").
		Count(testContext(t))
	require.NoError(t, err)
	require.Zero(t, unsupportedLocaleCount)

	request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{
      "email":"unsupported-locale-http@example.test",
      "password":"correct horse battery staple",
      "display_name":"Unsupported locale",
      "timezone":"UTC",
      "locale":"de",
      "device_key":"unsupported-locale-http-device"
    }`))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	api.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code, "invalid locale is a client validation error, not a server failure")
	errorResponse = struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &errorResponse))
	require.Equal(t, "VALIDATION_ERROR", errorResponse.Error.Code)

	_, err = service.Register(testContext(t), auth.RegisterInput{
		Email:      "reader@example.test",
		Password:   "another secure password",
		DeviceKey:  "another-device",
		DeviceName: "Duplicate",
	})
	require.ErrorIs(t, err, auth.ErrEmailExists)

	rotated, err := service.Rotate(testContext(t), registered.RefreshToken)
	require.NoError(t, err)
	require.NotEqual(t, registered.RefreshToken, rotated.RefreshToken)
	require.NotEqual(t, registered.AccessToken, rotated.AccessToken)
	require.Equal(t, registered.User.ID, rotated.User.ID)
	require.Equal(t, registered.Device.ID, rotated.Device.ID)

	var family []model.RefreshToken
	err = integrationDB.NewSelect().Model(&family).
		Where("family_id=(SELECT family_id FROM refresh_tokens WHERE token_hash=?)", auth.HashRefresh(registered.RefreshToken)).
		Order("created_at ASC").
		Scan(testContext(t))
	require.NoError(t, err)
	require.Len(t, family, 2)
	require.NotNil(t, family[0].RevokedAt)
	require.NotNil(t, family[0].LastUsedAt)
	require.NotNil(t, family[0].ReplacedBy)
	require.Equal(t, family[1].ID, *family[0].ReplacedBy)
	require.Nil(t, family[1].RevokedAt)
	require.Equal(t, family[0].FamilyID, family[1].FamilyID)

	_, err = service.Rotate(testContext(t), registered.RefreshToken)
	require.ErrorIs(t, err, auth.ErrRefreshReuse)

	family = nil
	err = integrationDB.NewSelect().Model(&family).
		Where("family_id=?", familyIDForRefresh(t, registered.RefreshToken)).
		Order("created_at ASC").
		Scan(testContext(t))
	require.NoError(t, err)
	require.Len(t, family, 2)
	for _, item := range family {
		require.NotNil(t, item.RevokedAt, "reuse of an ancestor refresh token must revoke the entire token family")
	}

	_, err = service.Rotate(testContext(t), rotated.RefreshToken)
	require.ErrorIs(t, err, auth.ErrRefreshReuse, "the newest descendant is unusable after its family is compromised")

	freshFamily, err := service.Login(testContext(t), auth.LoginInput{
		Email:      "reader@example.test",
		Password:   "correct horse battery staple",
		DeviceKey:  "integration-browser",
		DeviceName: "Integration browser",
		UserAgent:  "BookFlow integration test after reuse",
	})
	require.NoError(t, err)
	require.NotEmpty(t, freshFamily.RefreshToken)
	refreshedFreshFamily, err := service.Rotate(testContext(t), freshFamily.RefreshToken)
	require.NoError(t, err, "a separately authenticated token family must not be revoked by reuse in an older family")
	require.NotEmpty(t, refreshedFreshFamily.RefreshToken)

	require.NoError(t, service.LogoutAll(testContext(t), registered.User.ID))
	_, err = service.Rotate(testContext(t), refreshedFreshFamily.RefreshToken)
	require.ErrorIs(t, err, auth.ErrRefreshReuse)

	_, err = service.Rotate(testContext(t), "not-a-valid-refresh-token")
	require.ErrorIs(t, err, auth.ErrInvalidToken)
}

func familyIDForRefresh(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	var token model.RefreshToken
	err := integrationDB.NewSelect().Model(&token).
		Where("token_hash=?", auth.HashRefresh(raw)).
		Scan(testContext(t))
	require.NoError(t, err)
	return token.FamilyID
}
