package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/XRS0/reader/backend/internal/model"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

var (
	ErrUnauthorized = errors.New("invalid email or password")
	ErrEmailExists  = errors.New("email is already registered")
	ErrWeakPassword = errors.New("password must be between 10 and 1024 bytes")
	ErrInvalidToken = errors.New("invalid or expired token")
	ErrRefreshReuse = errors.New("refresh token reuse detected")
)

type Service struct {
	db     *bun.DB
	hasher PasswordHasher
	tokens *TokenManager
	now    func() time.Time
}
type RegisterInput struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Timezone    string `json:"timezone"`
	Locale      string `json:"locale"`
	DeviceKey   string `json:"device_key"`
	DeviceName  string `json:"device_name"`
	UserAgent   string `json:"-"`
}
type LoginInput struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	DeviceKey  string `json:"device_key"`
	DeviceName string `json:"device_name"`
	UserAgent  string `json:"-"`
}
type TokenPair struct {
	AccessToken      string       `json:"access_token"`
	RefreshToken     string       `json:"refresh_token"`
	AccessExpiresAt  time.Time    `json:"access_expires_at"`
	RefreshExpiresAt time.Time    `json:"refresh_expires_at"`
	User             model.User   `json:"user"`
	Device           model.Device `json:"device"`
}

func NewService(db *bun.DB, hasher PasswordHasher, tokens *TokenManager) *Service {
	return &Service{db: db, hasher: hasher, tokens: tokens, now: time.Now}
}

func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 254 || !strings.Contains(email, ".") {
		return "", errors.New("invalid email")
	}
	return email, nil
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (TokenPair, error) {
	email, err := NormalizeEmail(in.Email)
	if err != nil {
		return TokenPair{}, err
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return TokenPair{}, err
	}
	if in.Timezone == "" {
		in.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return TokenPair{}, errors.New("invalid timezone")
	}
	if in.Locale == "" {
		in.Locale = "en"
	}
	if in.Locale != "en" && in.Locale != "ru" {
		return TokenPair{}, errors.New("invalid locale")
	}
	if len(in.DisplayName) > 100 {
		return TokenPair{}, errors.New("display name is too long")
	}
	now := s.now().UTC()
	user := model.User{ID: uuid.New(), Email: email, PasswordHash: hash, DisplayName: strings.TrimSpace(in.DisplayName), Timezone: in.Timezone, Locale: in.Locale, CreatedAt: now, UpdatedAt: now}
	device := newDevice(user.ID, in.DeviceKey, in.DeviceName, in.UserAgent, now)
	err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var exists bool
		if err := tx.NewSelect().ColumnExpr("EXISTS(SELECT 1 FROM users WHERE lower(email)=? AND deleted_at IS NULL)", email).Scan(ctx, &exists); err != nil {
			return err
		}
		if exists {
			return ErrEmailExists
		}
		if _, err := tx.NewInsert().Model(&user).Exec(ctx); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return ErrEmailExists
			}
			return err
		}
		if _, err := tx.NewInsert().Model(&device).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO reader_preferences(user_id) VALUES (?)`, user.ID)
		return err
	})
	if err != nil {
		return TokenPair{}, err
	}
	return s.issue(ctx, user, device, uuid.New())
}

func (s *Service) Login(ctx context.Context, in LoginInput) (TokenPair, error) {
	email, err := NormalizeEmail(in.Email)
	if err != nil {
		return TokenPair{}, ErrUnauthorized
	}
	var user model.User
	err = s.db.NewSelect().Model(&user).Where("lower(email)=?", email).Where("deleted_at IS NULL").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		s.fakePasswordWork(in.Password)
		return TokenPair{}, ErrUnauthorized
	}
	if err != nil {
		return TokenPair{}, err
	}
	ok, err := s.hasher.Verify(user.PasswordHash, in.Password)
	if err != nil || !ok {
		return TokenPair{}, ErrUnauthorized
	}
	now := s.now().UTC()
	device := newDevice(user.ID, in.DeviceKey, in.DeviceName, in.UserAgent, now)
	_, err = s.db.NewInsert().Model(&device).On("CONFLICT (user_id, device_key) DO UPDATE").Set("name=EXCLUDED.name").Set("user_agent=EXCLUDED.user_agent").Set("last_seen_at=EXCLUDED.last_seen_at").Set("revoked_at=NULL").Returning("id, user_id, device_key, name, user_agent, last_seen_at, revoked_at, created_at").Exec(ctx)
	if err != nil {
		return TokenPair{}, err
	}
	return s.issue(ctx, user, device, uuid.New())
}

func (s *Service) fakePasswordWork(password string) {
	fake, _ := s.hasher.Hash("not-a-real-user-password")
	_, _ = s.hasher.Verify(fake, password)
}

func newDevice(userID uuid.UUID, key, name, agent string, now time.Time) model.Device {
	if strings.TrimSpace(key) == "" {
		key = uuid.NewString()
	}
	if len(key) > 200 {
		key = key[:200]
	}
	if len(name) > 100 {
		name = name[:100]
	}
	if len(agent) > 500 {
		agent = agent[:500]
	}
	return model.Device{ID: uuid.New(), UserID: userID, DeviceKey: key, Name: strings.TrimSpace(name), UserAgent: agent, LastSeenAt: now, CreatedAt: now}
}

func (s *Service) issue(ctx context.Context, user model.User, device model.Device, family uuid.UUID) (TokenPair, error) {
	access, accessExp, err := s.tokens.Access(user.ID, user.Email, device.ID)
	if err != nil {
		return TokenPair{}, err
	}
	raw, refreshHash, refreshExp, err := s.tokens.Refresh()
	if err != nil {
		return TokenPair{}, err
	}
	token := model.RefreshToken{ID: uuid.New(), UserID: user.ID, DeviceID: device.ID, FamilyID: family, TokenHash: refreshHash, ExpiresAt: refreshExp, CreatedAt: s.now().UTC()}
	if _, err := s.db.NewInsert().Model(&token).Exec(ctx); err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: raw, AccessExpiresAt: accessExp, RefreshExpiresAt: refreshExp, User: user, Device: device}, nil
}

func (s *Service) Rotate(ctx context.Context, raw string) (TokenPair, error) {
	if len(raw) < 40 || len(raw) > 256 {
		return TokenPair{}, ErrInvalidToken
	}
	hash := HashRefresh(raw)
	var result TokenPair
	reuseDetected := false
	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var old model.RefreshToken
		err := tx.NewSelect().Model(&old).Where("token_hash=?", hash).For("UPDATE").Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidToken
		}
		if err != nil {
			return err
		}
		now := s.now().UTC()
		if old.RevokedAt != nil {
			if _, err := tx.NewUpdate().Model((*model.RefreshToken)(nil)).Set("revoked_at=COALESCE(revoked_at, ?)", now).Where("family_id=?", old.FamilyID).Exec(ctx); err != nil {
				return err
			}
			reuseDetected = true
			return nil
		}
		if now.After(old.ExpiresAt) {
			return ErrInvalidToken
		}
		var user model.User
		if err := tx.NewSelect().Model(&user).Where("id=?", old.UserID).Where("deleted_at IS NULL").Scan(ctx); err != nil {
			return ErrInvalidToken
		}
		var device model.Device
		if err := tx.NewSelect().Model(&device).Where("id=?", old.DeviceID).Where("revoked_at IS NULL").Scan(ctx); err != nil {
			return ErrInvalidToken
		}
		access, accessExp, err := s.tokens.Access(user.ID, user.Email, device.ID)
		if err != nil {
			return err
		}
		newRaw, newHash, refreshExp, err := s.tokens.Refresh()
		if err != nil {
			return err
		}
		newToken := model.RefreshToken{ID: uuid.New(), UserID: user.ID, DeviceID: device.ID, FamilyID: old.FamilyID, TokenHash: newHash, ExpiresAt: refreshExp, CreatedAt: now}
		if _, err := tx.NewInsert().Model(&newToken).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(&old).Set("revoked_at=?", now).Set("last_used_at=?", now).Set("replaced_by=?", newToken.ID).WherePK().Where("revoked_at IS NULL").Exec(ctx); err != nil {
			return err
		}
		result = TokenPair{AccessToken: access, RefreshToken: newRaw, AccessExpiresAt: accessExp, RefreshExpiresAt: refreshExp, User: user, Device: device}
		return nil
	})
	if err == nil && reuseDetected {
		return TokenPair{}, ErrRefreshReuse
	}
	return result, err
}

func (s *Service) Logout(ctx context.Context, userID uuid.UUID, raw string) error {
	hash := HashRefresh(raw)
	now := s.now().UTC()
	_, err := s.db.NewUpdate().Model((*model.RefreshToken)(nil)).Set("revoked_at=COALESCE(revoked_at, ?)", now).Where("user_id=?", userID).Where("token_hash=?", hash).Exec(ctx)
	return err
}
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	now := s.now().UTC()
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewUpdate().Model((*model.RefreshToken)(nil)).Set("revoked_at=COALESCE(revoked_at, ?)", now).Where("user_id=?", userID).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewUpdate().Model((*model.Device)(nil)).Set("revoked_at=COALESCE(revoked_at, ?)", now).Where("user_id=?", userID).Exec(ctx)
		return err
	})
}
func (s *Service) User(ctx context.Context, id uuid.UUID) (model.User, error) {
	var u model.User
	err := s.db.NewSelect().Model(&u).Where("id=?", id).Where("deleted_at IS NULL").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, ErrUnauthorized
	}
	return u, err
}
func (s *Service) Devices(ctx context.Context, userID uuid.UUID) ([]model.Device, error) {
	var items []model.Device
	err := s.db.NewSelect().Model(&items).Where("user_id=?", userID).Order("last_seen_at DESC").Limit(100).Scan(ctx)
	return items, err
}
func (s *Service) RevokeDevice(ctx context.Context, userID, deviceID uuid.UUID) error {
	now := s.now().UTC()
	res, err := s.db.NewUpdate().Model((*model.Device)(nil)).Set("revoked_at=COALESCE(revoked_at, ?)", now).Where("id=? AND user_id=?", deviceID, userID).Exec(ctx)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	_, err = s.db.NewUpdate().Model((*model.RefreshToken)(nil)).Set("revoked_at=COALESCE(revoked_at, ?)", now).Where("device_id=? AND user_id=?", deviceID, userID).Exec(ctx)
	return err
}

func UserIDFromClaims(c Claims) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("claims subject: %w", err)
	}
	return id, nil
}
