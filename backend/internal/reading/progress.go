package reading

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/XRS0/reader/backend/internal/model"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

var (
	ErrNotFound         = errors.New("reading resource not found")
	ErrRevisionConflict = errors.New("reading progress revision conflict")
	ErrSequence         = errors.New("heartbeat sequence is stale")
	ErrSessionFinished  = errors.New("reading session is already finished")
)

type ConflictError struct{ Current model.ReadingProgress }

func (e *ConflictError) Error() string { return ErrRevisionConflict.Error() }
func (e *ConflictError) Unwrap() error { return ErrRevisionConflict }

type ProgressInput struct {
	ChapterID       *uuid.UUID      `json:"chapter_id"`
	LocatorType     string          `json:"locator_type"`
	Locator         json.RawMessage `json:"locator"`
	CharacterOffset int64           `json:"character_offset"`
	TextAnchor      string          `json:"text_anchor"`
	ChapterProgress float64         `json:"chapter_progress"`
	ProgressPercent float64         `json:"progress_percent"`
	ScrollPercent   float64         `json:"scroll_percent"`
	Revision        int64           `json:"revision"`
	ClientID        string          `json:"client_id"`
	DeviceID        *uuid.UUID      `json:"device_id"`
	ClientTimestamp time.Time       `json:"client_timestamp"`
}
type ProgressService struct {
	db  *bun.DB
	now func() time.Time
}

func NewProgressService(db *bun.DB) *ProgressService { return &ProgressService{db: db, now: time.Now} }
func (p *ProgressService) Get(ctx context.Context, userID, bookID uuid.UUID) (model.ReadingProgress, error) {
	if err := ensureBook(ctx, p.db, userID, bookID); err != nil {
		return model.ReadingProgress{}, err
	}
	var progress model.ReadingProgress
	err := p.db.NewSelect().Model(&progress).Where("user_id=? AND book_id=?", userID, bookID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		var chapterID uuid.UUID
		_ = p.db.NewSelect().Table("book_chapters").Column("id").Where("book_id=?", bookID).Order("version DESC, ordinal ASC").Limit(1).Scan(ctx, &chapterID)
		var chapter *uuid.UUID
		if chapterID != uuid.Nil {
			chapter = &chapterID
		}
		return model.ReadingProgress{UserID: userID, BookID: bookID, ChapterID: chapter, LocatorType: "chapter_offset", Locator: json.RawMessage(`{}`), Revision: 0, UpdatedAt: p.now().UTC()}, nil
	}
	return progress, err
}
func (p *ProgressService) Put(ctx context.Context, userID, bookID uuid.UUID, in ProgressInput) (model.ReadingProgress, error) {
	if len(in.Locator) == 0 {
		in.Locator = json.RawMessage(`{}`)
	}
	if err := validateProgress(in); err != nil {
		return model.ReadingProgress{}, err
	}
	var result model.ReadingProgress
	err := p.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var lockedBook uuid.UUID
		if err := tx.NewSelect().Table("books").Column("id").Where("id=? AND user_id=? AND deleted_at IS NULL", bookID, userID).For("UPDATE").Scan(ctx, &lockedBook); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if err := validateProgressReferences(ctx, tx, userID, bookID, in); err != nil {
			return err
		}
		var current model.ReadingProgress
		err := tx.NewSelect().Model(&current).Where("user_id=? AND book_id=?", userID, bookID).For("UPDATE").Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			if in.Revision != 0 {
				return &ConflictError{Current: model.ReadingProgress{UserID: userID, BookID: bookID, Revision: 0, Locator: json.RawMessage(`{}`)}}
			}
			result = progressFromInput(userID, bookID, in, 1, p.now().UTC())
			_, err = tx.NewInsert().Model(&result).Exec(ctx)
			if err != nil && stringsUnique(err) {
				var latest model.ReadingProgress
				if scanErr := tx.NewSelect().Model(&latest).Where("user_id=? AND book_id=?", userID, bookID).Scan(ctx); scanErr == nil {
					return &ConflictError{Current: latest}
				}
			}
			return err
		}
		if err != nil {
			return err
		}
		if in.Revision != current.Revision {
			return &ConflictError{Current: current}
		}
		result = progressFromInput(userID, bookID, in, current.Revision+1, p.now().UTC())
		result.ID = current.ID
		_, err = tx.NewUpdate().Model(&result).WherePK().Column("chapter_id", "locator_type", "locator", "character_offset", "text_anchor", "chapter_progress", "progress_percent", "scroll_percent", "revision", "client_id", "device_id", "updated_at").Exec(ctx)
		return err
	})
	return result, err
}
func validateProgressReferences(ctx context.Context, tx bun.Tx, userID, bookID uuid.UUID, in ProgressInput) error {
	if in.ChapterID != nil {
		var valid bool
		if err := tx.NewSelect().ColumnExpr("EXISTS(SELECT 1 FROM book_chapters WHERE id=? AND book_id=?)", *in.ChapterID, bookID).Scan(ctx, &valid); err != nil {
			return err
		}
		if !valid {
			return errors.New("chapter does not belong to book")
		}
	}
	if in.DeviceID != nil {
		var valid bool
		if err := tx.NewSelect().ColumnExpr("EXISTS(SELECT 1 FROM devices WHERE id=? AND user_id=? AND revoked_at IS NULL)", *in.DeviceID, userID).Scan(ctx, &valid); err != nil {
			return err
		}
		if !valid {
			return errors.New("device does not belong to user")
		}
	}
	return nil
}
func progressFromInput(userID, bookID uuid.UUID, in ProgressInput, revision int64, now time.Time) model.ReadingProgress {
	return model.ReadingProgress{ID: uuid.New(), UserID: userID, BookID: bookID, ChapterID: in.ChapterID, LocatorType: in.LocatorType, Locator: in.Locator, CharacterOffset: in.CharacterOffset, TextAnchor: in.TextAnchor, ChapterProgress: in.ChapterProgress, ProgressPercent: in.ProgressPercent, ScrollPercent: in.ScrollPercent, Revision: revision, ClientID: in.ClientID, DeviceID: in.DeviceID, UpdatedAt: now}
}
func validateProgress(in ProgressInput) error {
	if in.Revision < 0 {
		return errors.New("revision cannot be negative")
	}
	if len(in.LocatorType) == 0 || len(in.LocatorType) > 50 {
		return errors.New("invalid locator type")
	}
	if len(in.Locator) == 0 {
		in.Locator = json.RawMessage(`{}`)
	}
	if len(in.Locator) > 16*1024 || !json.Valid(in.Locator) {
		return errors.New("invalid locator")
	}
	if in.CharacterOffset < 0 {
		return errors.New("character offset cannot be negative")
	}
	if len(in.TextAnchor) > 512 || len(in.ClientID) > 200 {
		return errors.New("progress field is too long")
	}
	for _, v := range []float64{in.ChapterProgress, in.ProgressPercent, in.ScrollPercent} {
		if v < 0 || v > 100 {
			return errors.New("progress values must be between 0 and 100")
		}
	}
	return nil
}
func ensureBook(ctx context.Context, db *bun.DB, userID, bookID uuid.UUID) error {
	var exists bool
	err := db.NewSelect().ColumnExpr("EXISTS(SELECT 1 FROM books WHERE id=? AND user_id=? AND deleted_at IS NULL)", bookID, userID).Scan(ctx, &exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}
func stringsUnique(err error) bool {
	return err != nil && (fmt.Sprintf("%v", err) != "") && containsLower(err.Error(), "unique")
}
func containsLower(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		matched := true
		for j := range needle {
			c := value[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 32
			}
			if c != needle[j] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
