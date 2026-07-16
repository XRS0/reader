package dictionary

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/XRS0/reader/backend/internal/model"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

var ErrNotFound = errors.New("dictionary entry not found")

type Service struct {
	db  *bun.DB
	now func() time.Time
}
type CreateInput struct {
	SourceLanguage          string           `json:"source_language"`
	TargetLanguage          string           `json:"target_language"`
	OriginalWord            string           `json:"original_word"`
	NormalizedWord          string           `json:"normalized_word"`
	Lemma                   string           `json:"lemma"`
	Transcription           string           `json:"transcription"`
	PartOfSpeech            string           `json:"part_of_speech"`
	Translation             string           `json:"translation"`
	AlternativeTranslations []string         `json:"alternative_translations"`
	Definition              string           `json:"definition"`
	Note                    string           `json:"note"`
	Status                  string           `json:"status"`
	Occurrence              *OccurrenceInput `json:"occurrence,omitempty"`
}
type OccurrenceInput struct {
	BookID        *uuid.UUID      `json:"book_id"`
	ChapterID     *uuid.UUID      `json:"chapter_id"`
	Locator       json.RawMessage `json:"locator"`
	Sentence      string          `json:"sentence"`
	ContextBefore string          `json:"context_before"`
	ContextAfter  string          `json:"context_after"`
	EncounteredAt time.Time       `json:"encountered_at"`
}
type UpdateInput struct {
	Translation   *string    `json:"translation"`
	Transcription *string    `json:"transcription"`
	PartOfSpeech  *string    `json:"part_of_speech"`
	Definition    *string    `json:"definition"`
	Note          *string    `json:"note"`
	Status        *string    `json:"status"`
	NextReviewAt  *time.Time `json:"next_review_at"`
}

func NewService(db *bun.DB) *Service { return &Service{db: db, now: time.Now} }
func NormalizeWord(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	var b strings.Builder
	space := false
	for _, r := range v {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		space = false
	}
	return b.String()
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, in CreateInput) (model.DictionaryEntry, error) {
	normalized := NormalizeWord(in.OriginalWord)
	if err := validateCreate(in, normalized); err != nil {
		return model.DictionaryEntry{}, err
	}
	if in.Status == "" {
		in.Status = "unknown"
	}
	now := s.now().UTC()
	entry := model.DictionaryEntry{ID: uuid.New(), UserID: userID, SourceLanguage: strings.ToLower(in.SourceLanguage), TargetLanguage: strings.ToLower(in.TargetLanguage), OriginalWord: strings.TrimSpace(in.OriginalWord), NormalizedWord: normalized, Lemma: in.Lemma, Transcription: in.Transcription, PartOfSpeech: in.PartOfSpeech, Translation: strings.TrimSpace(in.Translation), AlternativeTranslations: in.AlternativeTranslations, Definition: in.Definition, Note: in.Note, Status: in.Status, EncounterCount: 1, FirstSeenAt: now, LastSeenAt: now, CreatedAt: now, UpdatedAt: now}
	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewInsert().Model(&entry).On("CONFLICT (user_id, source_language, target_language, normalized_word) DO UPDATE").Set("encounter_count=de.encounter_count+1").Set("last_seen_at=EXCLUDED.last_seen_at").Set("updated_at=EXCLUDED.updated_at").Set("deleted_at=NULL").Returning("*").Exec(ctx)
		if err != nil {
			return err
		}
		if in.Occurrence != nil {
			occ, err := s.makeOccurrence(ctx, tx, userID, entry.ID, *in.Occurrence, now)
			if err != nil {
				return err
			}
			_, err = tx.NewInsert().Model(&occ).Exec(ctx)
			return err
		}
		return nil
	})
	return entry, err
}
func (s *Service) makeOccurrence(ctx context.Context, tx bun.Tx, userID, entryID uuid.UUID, in OccurrenceInput, now time.Time) (model.WordOccurrence, error) {
	if len(in.Locator) > 16*1024 || (len(in.Locator) > 0 && !json.Valid(in.Locator)) {
		return model.WordOccurrence{}, errors.New("invalid locator")
	}
	if len(in.Sentence) > 2000 || len(in.ContextBefore) > 2000 || len(in.ContextAfter) > 2000 {
		return model.WordOccurrence{}, errors.New("occurrence context is too long")
	}
	if in.BookID != nil {
		var owns bool
		if err := tx.NewSelect().ColumnExpr("EXISTS(SELECT 1 FROM books WHERE id=? AND user_id=? AND deleted_at IS NULL)", *in.BookID, userID).Scan(ctx, &owns); err != nil {
			return model.WordOccurrence{}, err
		}
		if !owns {
			return model.WordOccurrence{}, errors.New("book not found")
		}
	}
	if in.ChapterID != nil {
		if in.BookID == nil {
			return model.WordOccurrence{}, errors.New("book_id is required with chapter_id")
		}
		var valid bool
		if err := tx.NewSelect().ColumnExpr("EXISTS(SELECT 1 FROM book_chapters WHERE id=? AND book_id=?)", *in.ChapterID, *in.BookID).Scan(ctx, &valid); err != nil {
			return model.WordOccurrence{}, err
		}
		if !valid {
			return model.WordOccurrence{}, errors.New("chapter does not belong to book")
		}
	}
	if len(in.Locator) == 0 {
		in.Locator = json.RawMessage(`{}`)
	}
	if in.EncounteredAt.IsZero() || in.EncounteredAt.After(now.Add(time.Minute)) {
		in.EncounteredAt = now
	}
	return model.WordOccurrence{ID: uuid.New(), DictionaryEntryID: entryID, BookID: in.BookID, ChapterID: in.ChapterID, Locator: in.Locator, Sentence: in.Sentence, ContextBefore: in.ContextBefore, ContextAfter: in.ContextAfter, EncounteredAt: in.EncounteredAt, CreatedAt: now}, nil
}
func (s *Service) AddOccurrence(ctx context.Context, userID, entryID uuid.UUID, in OccurrenceInput) (model.WordOccurrence, error) {
	var out model.WordOccurrence
	err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var entry model.DictionaryEntry
		if err := tx.NewSelect().Model(&entry).Where("id=? AND user_id=? AND deleted_at IS NULL", entryID, userID).For("UPDATE").Scan(ctx); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		now := s.now().UTC()
		occ, err := s.makeOccurrence(ctx, tx, userID, entryID, in, now)
		if err != nil {
			return err
		}
		if _, err = tx.NewInsert().Model(&occ).Exec(ctx); err != nil {
			return err
		}
		_, err = tx.NewUpdate().Model((*model.DictionaryEntry)(nil)).Set("encounter_count=encounter_count+1").Set("last_seen_at=?", now).Set("updated_at=?", now).Where("id=?", entryID).Exec(ctx)
		out = occ
		return err
	})
	return out, err
}
func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (model.DictionaryEntry, error) {
	var e model.DictionaryEntry
	err := s.db.NewSelect().Model(&e).Where("id=? AND user_id=? AND deleted_at IS NULL", id, userID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	return e, err
}
func (s *Service) List(ctx context.Context, userID uuid.UUID, search, status, language string, limit, offset int) ([]model.DictionaryEntry, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	q := s.db.NewSelect().Model((*model.DictionaryEntry)(nil)).Where("user_id=? AND deleted_at IS NULL", userID)
	if search != "" {
		needle := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"
		q = q.Where("(lower(original_word) LIKE ? OR lower(translation) LIKE ?)", needle, needle)
	}
	if status != "" {
		q = q.Where("status=?", status)
	}
	if language != "" {
		q = q.Where("source_language=?", strings.ToLower(language))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	var items []model.DictionaryEntry
	err = q.Model(&items).Order("updated_at DESC").Limit(limit).Offset(offset).Scan(ctx)
	return items, total, err
}
func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, in UpdateInput) (model.DictionaryEntry, error) {
	entry, err := s.Get(ctx, userID, id)
	if err != nil {
		return entry, err
	}
	q := s.db.NewUpdate().Model(&entry).Set("updated_at=?", s.now().UTC()).WherePK().Where("user_id=? AND deleted_at IS NULL", userID)
	if in.Translation != nil {
		if strings.TrimSpace(*in.Translation) == "" || len(*in.Translation) > 5000 {
			return entry, errors.New("invalid translation")
		}
		q = q.Set("translation=?", strings.TrimSpace(*in.Translation))
	}
	if in.Transcription != nil {
		q = q.Set("transcription=?", truncate(*in.Transcription, 500))
	}
	if in.PartOfSpeech != nil {
		q = q.Set("part_of_speech=?", truncate(*in.PartOfSpeech, 100))
	}
	if in.Definition != nil {
		q = q.Set("definition=?", truncate(*in.Definition, 5000))
	}
	if in.Note != nil {
		q = q.Set("note=?", truncate(*in.Note, 10000))
	}
	if in.Status != nil {
		if !validStatus(*in.Status) {
			return entry, errors.New("invalid dictionary status")
		}
		q = q.Set("status=?", *in.Status)
	}
	if in.NextReviewAt != nil {
		q = q.Set("next_review_at=?", *in.NextReviewAt)
	}
	if _, err = q.Returning("*").Exec(ctx); err != nil {
		return entry, err
	}
	return entry, nil
}
func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID) error {
	res, err := s.db.NewUpdate().Model((*model.DictionaryEntry)(nil)).Set("deleted_at=?", s.now().UTC()).Set("updated_at=?", s.now().UTC()).Where("id=? AND user_id=? AND deleted_at IS NULL", id, userID).Exec(ctx)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Service) Restore(ctx context.Context, userID, id uuid.UUID) (model.DictionaryEntry, error) {
	var e model.DictionaryEntry
	res, err := s.db.NewUpdate().Model(&e).Set("deleted_at=NULL").Set("updated_at=?", s.now().UTC()).Where("id=? AND user_id=? AND deleted_at IS NOT NULL", id, userID).Returning("*").Exec(ctx)
	if err != nil {
		return e, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return e, ErrNotFound
	}
	return e, nil
}
func (s *Service) Occurrences(ctx context.Context, userID, entryID uuid.UUID, limit, offset int) ([]model.WordOccurrence, error) {
	if _, err := s.Get(ctx, userID, entryID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	var items []model.WordOccurrence
	err := s.db.NewSelect().Model(&items).Where("dictionary_entry_id=?", entryID).Order("encountered_at DESC").Limit(limit).Offset(offset).Scan(ctx)
	return items, err
}
func validateCreate(in CreateInput, normalized string) error {
	if normalized == "" || utf8.RuneCountInString(normalized) > 200 {
		return errors.New("invalid word")
	}
	if len(in.SourceLanguage) == 0 || len(in.SourceLanguage) > 32 || len(in.TargetLanguage) == 0 || len(in.TargetLanguage) > 32 {
		return errors.New("invalid languages")
	}
	if strings.TrimSpace(in.Translation) == "" || len(in.Translation) > 5000 {
		return errors.New("invalid translation")
	}
	if in.Status != "" && !validStatus(in.Status) {
		return errors.New("invalid status")
	}
	return nil
}
func validStatus(v string) bool {
	switch v {
	case "unknown", "learning", "known", "mastered", "ignored":
		return true
	}
	return false
}
func truncate(v string, n int) string {
	v = strings.TrimSpace(v)
	if len(v) <= n {
		return v
	}
	return v[:n]
}
