package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	Email    string `json:"email"`
	DeviceID string `json:"device_id,omitempty"`
	jwt.RegisteredClaims
}
type TokenManager struct {
	secret     []byte
	issuer     string
	audience   string
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

func NewTokenManager(secret, issuer string, accessTTL, refreshTTL time.Duration) *TokenManager {
	return NewTokenManagerWithAudience(secret, issuer, "bookflow-web", accessTTL, refreshTTL)
}
func NewTokenManagerWithAudience(secret, issuer, audience string, accessTTL, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), issuer: issuer, audience: audience, accessTTL: accessTTL, refreshTTL: refreshTTL, now: time.Now}
}

func (m *TokenManager) Access(userID uuid.UUID, email string, deviceID uuid.UUID) (string, time.Time, error) {
	now := m.now().UTC()
	expires := now.Add(m.accessTTL)
	claims := Claims{Email: email, DeviceID: deviceID.String(), RegisteredClaims: jwt.RegisteredClaims{Issuer: m.issuer, Subject: userID.String(), Audience: jwt.ClaimStrings{m.audience}, ExpiresAt: jwt.NewNumericDate(expires), NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)), IssuedAt: jwt.NewNumericDate(now), ID: uuid.NewString()}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	return token, expires, err
}

func (m *TokenManager) ParseAccess(raw string) (Claims, error) {
	var claims Claims
	token, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected JWT signing method")
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithAudience(m.audience), jwt.WithExpirationRequired(), jwt.WithLeeway(10*time.Second), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return Claims{}, ErrInvalidToken
	}
	if _, err := uuid.Parse(claims.Subject); err != nil {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}

func (m *TokenManager) Refresh() (raw string, hash []byte, expires time.Time, err error) {
	b := make([]byte, 48)
	if _, err = rand.Read(b); err != nil {
		return "", nil, time.Time{}, fmt.Errorf("refresh entropy: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:], m.now().UTC().Add(m.refreshTTL), nil
}
func HashRefresh(raw string) []byte { sum := sha256.Sum256([]byte(raw)); return sum[:] }
