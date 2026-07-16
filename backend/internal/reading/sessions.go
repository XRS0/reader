package reading

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type SessionService struct {
	db  *bun.DB
	cfg config.Reading
	now func() time.Time
}
type StartSessionInput struct {
	BookID          uuid.UUID       `json:"book_id"`
	DeviceID        *uuid.UUID      `json:"device_id"`
	Locator         json.RawMessage `json:"locator"`
	ProgressPercent float64         `json:"progress_percent"`
}
type HeartbeatInput struct {
	Locator           json.RawMessage `json:"locator"`
	ProgressPercent   float64         `json:"progress_percent"`
	Visible           bool            `json:"visible"`
	Focused           bool            `json:"focused"`
	UserActive        bool            `json:"user_active"`
	LastInteractionMS int64           `json:"last_interaction_ms"`
	ClientTimestamp   time.Time       `json:"client_timestamp"`
	IdempotencyKey    string          `json:"idempotency_key"`
	Sequence          int64           `json:"sequence"`
	CharactersRead    int64           `json:"characters_read"`
}
type FinishInput struct {
	Locator         json.RawMessage `json:"locator"`
	ProgressPercent float64         `json:"progress_percent"`
	CloseReason     string          `json:"close_reason"`
	IdempotencyKey  string          `json:"idempotency_key"`
	Sequence        int64           `json:"sequence"`
}

func NewSessionService(db *bun.DB, cfg config.Reading) *SessionService {
	return &SessionService{db: db, cfg: cfg, now: time.Now}
}

func (s *SessionService) Start(ctx context.Context, userID uuid.UUID, in StartSessionInput) (model.ReadingSession, error) {
	if err := ensureBook(ctx, s.db, userID, in.BookID); err != nil {
		return model.ReadingSession{}, err
	}
	if !validPercent(in.ProgressPercent) || !validLocator(in.Locator) {
		return model.ReadingSession{}, errors.New("invalid starting position")
	}
	if in.DeviceID != nil {
		var valid bool
		if err := s.db.NewSelect().ColumnExpr("EXISTS(SELECT 1 FROM devices WHERE id=? AND user_id=? AND revoked_at IS NULL)", *in.DeviceID, userID).Scan(ctx, &valid); err != nil {
			return model.ReadingSession{}, err
		}
		if !valid {
			return model.ReadingSession{}, errors.New("device does not belong to user")
		}
	}
	now := s.now().UTC()
	if len(in.Locator) == 0 {
		in.Locator = json.RawMessage(`{}`)
	}
	session := model.ReadingSession{ID: uuid.New(), UserID: userID, BookID: in.BookID, DeviceID: in.DeviceID, StartedAt: now, LastActivityAt: now, LastHeartbeatAt: now, StartLocator: in.Locator, EndLocator: in.Locator, StartProgressPercent: in.ProgressPercent, EndProgressPercent: in.ProgressPercent, Status: "active", LastWasActive: true, CreatedAt: now, UpdatedAt: now}
	event := model.ReadingEvent{ID: uuid.New(), SessionID: session.ID, UserID: userID, BookID: in.BookID, Type: "session_started", OccurredAt: now, ReceivedAt: now, IdempotencyKey: "start:" + session.ID.String(), SequenceNumber: 0, Metadata: json.RawMessage(`{}`)}
	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(&session).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewInsert().Model(&event).Exec(ctx)
		return err
	})
	return session, err
}

func (s *SessionService) Heartbeat(ctx context.Context, userID, sessionID uuid.UUID, in HeartbeatInput) (model.ReadingSession, error) {
	if len(in.IdempotencyKey) < 8 || len(in.IdempotencyKey) > 200 {
		return model.ReadingSession{}, errors.New("invalid idempotency key")
	}
	if in.Sequence <= 0 {
		return model.ReadingSession{}, errors.New("sequence must be positive")
	}
	if !validPercent(in.ProgressPercent) || !validLocator(in.Locator) || in.LastInteractionMS < 0 || in.CharactersRead < 0 {
		return model.ReadingSession{}, errors.New("invalid heartbeat")
	}
	var result model.ReadingSession
	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := tx.NewSelect().Model(&result).Where("id=? AND user_id=?", sessionID, userID).For("UPDATE").Scan(ctx); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		var duplicate bool
		if err := tx.NewSelect().ColumnExpr("EXISTS(SELECT 1 FROM reading_events WHERE session_id=? AND idempotency_key=?)", sessionID, in.IdempotencyKey).Scan(ctx, &duplicate); err != nil {
			return err
		}
		if duplicate {
			return nil
		}
		if result.Status == "finished" || result.Status == "stale" || result.Status == "finalized" {
			return ErrSessionFinished
		}
		if in.Sequence <= result.LastSequence {
			return ErrSequence
		}
		now := s.now().UTC()
		account := AccountInterval(result.LastHeartbeatAt, now, result.LastWasActive, ActivitySignals{Visible: in.Visible, Focused: in.Focused, UserActive: in.UserActive, SinceInteraction: time.Duration(in.LastInteractionMS) * time.Millisecond}, s.cfg.HeartbeatMaxGap, s.cfg.IdleThreshold)
		result.ActiveSeconds += account.ActiveSeconds
		result.IdleSeconds += account.IdleSeconds
		result.LastHeartbeatAt = now
		result.EndProgressPercent = in.ProgressPercent
		if len(in.Locator) > 0 {
			result.EndLocator = in.Locator
		}
		result.LastSequence = in.Sequence
		result.LastWasActive = account.CurrentActive
		result.Status = "idle"
		if account.CurrentActive {
			result.Status = "active"
			result.LastActivityAt = now
		}
		if in.CharactersRead > result.CharactersRead {
			result.CharactersRead = in.CharactersRead
			result.WordsReadEstimate = in.CharactersRead / 6
			result.PagesReadEstimate = float64(result.WordsReadEstimate) / 250
		}
		result.UpdatedAt = now
		res, err := tx.NewUpdate().Model(&result).WherePK().Where("user_id=?", userID).Exec(ctx)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return ErrNotFound
		}
		metadata, _ := json.Marshal(map[string]any{"visible": in.Visible, "focused": in.Focused, "user_active": in.UserActive, "active_seconds_added": account.ActiveSeconds, "idle_seconds_added": account.IdleSeconds, "client_timestamp": in.ClientTimestamp})
		event := model.ReadingEvent{ID: uuid.New(), SessionID: result.ID, UserID: userID, BookID: result.BookID, Type: "heartbeat", OccurredAt: now, ReceivedAt: now, IdempotencyKey: in.IdempotencyKey, SequenceNumber: in.Sequence, Metadata: metadata}
		_, err = tx.NewInsert().Model(&event).Exec(ctx)
		return err
	})
	return result, err
}

func (s *SessionService) Finish(ctx context.Context, userID, sessionID uuid.UUID, in FinishInput) (model.ReadingSession, error) {
	if in.CloseReason == "" {
		in.CloseReason = "unknown"
	}
	if !allowedCloseReason(in.CloseReason) || !validPercent(in.ProgressPercent) || !validLocator(in.Locator) {
		return model.ReadingSession{}, errors.New("invalid finish request")
	}
	var result model.ReadingSession
	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		err := tx.NewSelect().Model(&result).Where("id=? AND user_id=?", sessionID, userID).For("UPDATE").Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if result.Status == "finished" || result.Status == "stale" || result.Status == "finalized" {
			return nil
		}
		now := s.now().UTC()
		// A finish request contains no trustworthy activity signals. Conservatively
		// account the unobserved tail as idle rather than fabricating active time.
		account := AccountInterval(result.LastHeartbeatAt, now, result.LastWasActive, ActivitySignals{}, s.cfg.HeartbeatMaxGap, s.cfg.IdleThreshold)
		result.ActiveSeconds += account.ActiveSeconds
		result.IdleSeconds += account.IdleSeconds
		result.EndedAt = &now
		result.LastHeartbeatAt = now
		result.EndProgressPercent = in.ProgressPercent
		if len(in.Locator) > 0 {
			result.EndLocator = in.Locator
		}
		result.CloseReason = in.CloseReason
		result.Status = "finished"
		if in.Sequence > result.LastSequence {
			result.LastSequence = in.Sequence
		}
		result.UpdatedAt = now
		if _, err := tx.NewUpdate().Model(&result).WherePK().Where("user_id=?", userID).Exec(ctx); err != nil {
			return err
		}
		key := in.IdempotencyKey
		if key == "" {
			key = "finish:" + result.ID.String()
		}
		event := model.ReadingEvent{ID: uuid.New(), SessionID: result.ID, UserID: userID, BookID: result.BookID, Type: "session_finished", OccurredAt: now, ReceivedAt: now, IdempotencyKey: key, SequenceNumber: result.LastSequence, Metadata: json.RawMessage(`{}`)}
		_, err = tx.NewInsert().Model(&event).On("CONFLICT (session_id, idempotency_key) DO NOTHING").Exec(ctx)
		return err
	})
	return result, err
}
func (s *SessionService) Get(ctx context.Context, userID, id uuid.UUID) (model.ReadingSession, error) {
	var item model.ReadingSession
	err := s.db.NewSelect().Model(&item).Where("id=? AND user_id=?", id, userID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}
func (s *SessionService) List(ctx context.Context, userID uuid.UUID, bookID *uuid.UUID, limit, offset int) ([]model.ReadingSession, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	var items []model.ReadingSession
	q := s.db.NewSelect().Model(&items).Where("user_id=?", userID)
	if bookID != nil {
		q = q.Where("book_id=?", *bookID)
	}
	err := q.Order("started_at DESC").Limit(limit).Offset(offset).Scan(ctx)
	return items, err
}
func (s *SessionService) FinalizeStale(ctx context.Context) (int64, error) {
	cutoff := s.now().UTC().Add(-s.cfg.StaleAfter)
	res, err := s.db.NewUpdate().Model((*model.ReadingSession)(nil)).Set("status='stale'").Set("close_reason='stale_session_finalized'").Set("ended_at=last_heartbeat_at").Set("updated_at=now()").Where("status IN ('active','idle')").Where("last_heartbeat_at < ?", cutoff).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
func validPercent(v float64) bool         { return v >= 0 && v <= 100 }
func validLocator(v json.RawMessage) bool { return len(v) == 0 || (len(v) <= 16*1024 && json.Valid(v)) }
func allowedCloseReason(v string) bool {
	switch v {
	case "user_closed_reader", "switched_book", "app_backgrounded", "logout", "idle_timeout", "connection_lost", "stale_session_finalized", "book_finished", "server_shutdown", "unknown":
		return true
	}
	return false
}
