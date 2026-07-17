package annotations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/XRS0/reader/backend/internal/model"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
	"github.com/uptrace/bun"
)

var ErrNotFound = errors.New("annotation not found")
var numericEntityPattern = regexp.MustCompile(`(?i)&#(x[0-9a-f]+|[0-9]+);?`)

type Service struct {
	db  *bun.DB
	now func() time.Time
}

func NewService(db *bun.DB) *Service { return &Service{db: db, now: time.Now} }

type BookmarkInput struct {
	ChapterID       *uuid.UUID      `json:"chapter_id"`
	Locator         json.RawMessage `json:"locator"`
	ProgressPercent float64         `json:"progress_percent"`
	Title           string          `json:"title"`
	Note            string          `json:"note"`
}
type BookmarkPatch struct {
	Title *string `json:"title"`
	Note  *string `json:"note"`
}
type HighlightInput struct {
	ChapterID    *uuid.UUID      `json:"chapter_id"`
	Locator      json.RawMessage `json:"locator"`
	TextAnchor   string          `json:"text_anchor"`
	SelectedText string          `json:"selected_text"`
	Context      string          `json:"context"`
	Color        string          `json:"color"`
	Note         string          `json:"note"`
}
type HighlightPatch struct {
	SelectedText *string `json:"selected_text"`
	Color        *string `json:"color"`
	Note         *string `json:"note"`
}
type NoteInput struct {
	BookID        *uuid.UUID      `json:"book_id"`
	HighlightID   *uuid.UUID      `json:"highlight_id"`
	Title         string          `json:"title"`
	SchemaVersion int             `json:"schema_version"`
	Blocks        json.RawMessage `json:"blocks"`
}
type NotePatch struct {
	Title         *string         `json:"title"`
	Blocks        json.RawMessage `json:"blocks"`
	SchemaVersion *int            `json:"schema_version"`
}

func (s *Service) ListBookmarks(ctx context.Context, userID, bookID uuid.UUID) ([]model.Bookmark, error) {
	if err := s.ownsBook(ctx, userID, bookID); err != nil {
		return nil, err
	}
	var items []model.Bookmark
	err := s.db.NewSelect().Model(&items).Where("user_id=? AND book_id=?", userID, bookID).Order("created_at DESC").Limit(1000).Scan(ctx)
	return items, err
}
func (s *Service) CreateBookmark(ctx context.Context, userID, bookID uuid.UUID, in BookmarkInput) (model.Bookmark, error) {
	if err := s.ownsBook(ctx, userID, bookID); err != nil {
		return model.Bookmark{}, err
	}
	if !validLocator(in.Locator) || in.ProgressPercent < 0 || in.ProgressPercent > 100 || len(in.Title) > 500 || len(in.Note) > 5000 {
		return model.Bookmark{}, errors.New("invalid bookmark")
	}
	if in.ChapterID != nil {
		if err := s.ownsChapter(ctx, bookID, *in.ChapterID); err != nil {
			return model.Bookmark{}, err
		}
	}
	if len(in.Locator) == 0 {
		in.Locator = json.RawMessage(`{}`)
	}
	now := s.now().UTC()
	item := model.Bookmark{ID: uuid.New(), UserID: userID, BookID: bookID, ChapterID: in.ChapterID, Locator: in.Locator, ProgressPercent: in.ProgressPercent, Title: plain(in.Title), Note: plain(in.Note), CreatedAt: now, UpdatedAt: now}
	_, err := s.db.NewInsert().Model(&item).Exec(ctx)
	return item, err
}
func (s *Service) PatchBookmark(ctx context.Context, userID, id uuid.UUID, in BookmarkPatch) (model.Bookmark, error) {
	var item model.Bookmark
	err := s.db.NewSelect().Model(&item).Where("id=? AND user_id=?", id, userID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	}
	if err != nil {
		return item, err
	}
	q := s.db.NewUpdate().Model(&item).WherePK().Where("user_id=?", userID).Set("updated_at=?", s.now().UTC())
	if in.Title != nil {
		if len(*in.Title) > 500 {
			return item, errors.New("title too long")
		}
		q = q.Set("title=?", plain(*in.Title))
	}
	if in.Note != nil {
		if len(*in.Note) > 5000 {
			return item, errors.New("note too long")
		}
		q = q.Set("note=?", plain(*in.Note))
	}
	_, err = q.Returning("*").Exec(ctx)
	return item, err
}
func (s *Service) DeleteBookmark(ctx context.Context, userID, id uuid.UUID) error {
	return deleteOwned[model.Bookmark](ctx, s.db, userID, id)
}

func (s *Service) ListHighlights(ctx context.Context, userID, bookID uuid.UUID) ([]model.Highlight, error) {
	if err := s.ownsBook(ctx, userID, bookID); err != nil {
		return nil, err
	}
	var items []model.Highlight
	err := s.db.NewSelect().Model(&items).Where("user_id=? AND book_id=?", userID, bookID).Order("created_at DESC").Limit(1000).Scan(ctx)
	return items, err
}
func (s *Service) CreateHighlight(ctx context.Context, userID, bookID uuid.UUID, in HighlightInput) (model.Highlight, error) {
	if err := s.ownsBook(ctx, userID, bookID); err != nil {
		return model.Highlight{}, err
	}
	if !validLocator(in.Locator) || len(in.SelectedText) == 0 || len(in.SelectedText) > 20000 || len(in.TextAnchor) > 512 || len(in.Context) > 5000 || len(in.Note) > 5000 || !validColor(in.Color) {
		return model.Highlight{}, errors.New("invalid highlight")
	}
	if in.ChapterID != nil {
		if err := s.ownsChapter(ctx, bookID, *in.ChapterID); err != nil {
			return model.Highlight{}, err
		}
	}
	if len(in.Locator) == 0 {
		in.Locator = json.RawMessage(`{}`)
	}
	if in.Color == "" {
		in.Color = "sand"
	}
	now := s.now().UTC()
	item := model.Highlight{ID: uuid.New(), UserID: userID, BookID: bookID, ChapterID: in.ChapterID, Locator: in.Locator, TextAnchor: plain(in.TextAnchor), SelectedText: plain(in.SelectedText), Context: plain(in.Context), Color: in.Color, Note: plain(in.Note), CreatedAt: now, UpdatedAt: now}
	_, err := s.db.NewInsert().Model(&item).Exec(ctx)
	return item, err
}
func (s *Service) PatchHighlight(ctx context.Context, userID, id uuid.UUID, in HighlightPatch) (model.Highlight, error) {
	var item model.Highlight
	err := s.db.NewSelect().Model(&item).Where("id=? AND user_id=?", id, userID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	}
	if err != nil {
		return item, err
	}
	q := s.db.NewUpdate().Model(&item).WherePK().Where("user_id=?", userID).Set("updated_at=?", s.now().UTC())
	if in.SelectedText != nil {
		selectedText := plain(*in.SelectedText)
		if selectedText == "" || len(selectedText) > 20000 {
			return item, errors.New("invalid selected text")
		}
		q = q.Set("selected_text=?", selectedText)
	}
	if in.Color != nil {
		if !validColor(*in.Color) || *in.Color == "" {
			return item, errors.New("invalid color")
		}
		q = q.Set("color=?", *in.Color)
	}
	if in.Note != nil {
		if len(*in.Note) > 5000 {
			return item, errors.New("note too long")
		}
		q = q.Set("note=?", plain(*in.Note))
	}
	_, err = q.Returning("*").Exec(ctx)
	return item, err
}
func (s *Service) DeleteHighlight(ctx context.Context, userID, id uuid.UUID) error {
	return deleteOwned[model.Highlight](ctx, s.db, userID, id)
}

func (s *Service) ListNotes(ctx context.Context, userID uuid.UUID, bookID *uuid.UUID, search string, limit, offset int) ([]model.Note, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	var items []model.Note
	q := s.db.NewSelect().Model(&items).Where("user_id=? AND deleted_at IS NULL", userID)
	if bookID != nil {
		q = q.Where("book_id=?", *bookID)
	}
	if search != "" {
		needle := "%" + strings.ToLower(search) + "%"
		q = q.Where("lower(title) LIKE ? OR lower(search_text) LIKE ?", needle, needle)
	}
	err := q.Order("updated_at DESC").Limit(limit).Offset(offset).Scan(ctx)
	return items, err
}
func (s *Service) CreateNote(ctx context.Context, userID uuid.UUID, in NoteInput) (model.Note, error) {
	if in.BookID != nil {
		if err := s.ownsBook(ctx, userID, *in.BookID); err != nil {
			return model.Note{}, err
		}
	}
	if in.HighlightID != nil {
		var linkedBook *uuid.UUID
		err := s.db.NewSelect().Table("highlights").Column("book_id").Where("id=? AND user_id=?", *in.HighlightID, userID).Scan(ctx, &linkedBook)
		if errors.Is(err, sql.ErrNoRows) {
			return model.Note{}, ErrNotFound
		}
		if err != nil {
			return model.Note{}, err
		}
		if in.BookID != nil && (linkedBook == nil || *linkedBook != *in.BookID) {
			return model.Note{}, errors.New("highlight does not belong to note book")
		}
	}
	blocks, search, err := sanitizeBlocks(in.Blocks)
	if err != nil {
		return model.Note{}, err
	}
	if in.SchemaVersion == 0 {
		in.SchemaVersion = 1
	}
	if in.SchemaVersion != 1 || len(in.Title) > 500 {
		return model.Note{}, errors.New("unsupported note schema")
	}
	now := s.now().UTC()
	item := model.Note{ID: uuid.New(), UserID: userID, BookID: in.BookID, HighlightID: in.HighlightID, Title: plain(in.Title), SchemaVersion: in.SchemaVersion, Blocks: blocks, SearchText: search, CreatedAt: now, UpdatedAt: now}
	_, err = s.db.NewInsert().Model(&item).Exec(ctx)
	return item, err
}
func (s *Service) GetNote(ctx context.Context, userID, id uuid.UUID) (model.Note, error) {
	var item model.Note
	err := s.db.NewSelect().Model(&item).Where("id=? AND user_id=? AND deleted_at IS NULL", id, userID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}
func (s *Service) PatchNote(ctx context.Context, userID, id uuid.UUID, in NotePatch) (model.Note, error) {
	item, err := s.GetNote(ctx, userID, id)
	if err != nil {
		return item, err
	}
	q := s.db.NewUpdate().Model(&item).WherePK().Where("user_id=? AND deleted_at IS NULL", userID).Set("updated_at=?", s.now().UTC())
	if in.Title != nil {
		if len(*in.Title) > 500 {
			return item, errors.New("title too long")
		}
		q = q.Set("title=?", plain(*in.Title))
	}
	if len(in.Blocks) > 0 {
		blocks, search, err := sanitizeBlocks(in.Blocks)
		if err != nil {
			return item, err
		}
		q = q.Set("blocks=CAST(? AS jsonb)", string(blocks)).Set("search_text=?", search)
	}
	if in.SchemaVersion != nil {
		if *in.SchemaVersion != 1 {
			return item, errors.New("unsupported note schema")
		}
		q = q.Set("schema_version=?", *in.SchemaVersion)
	}
	_, err = q.Returning("*").Exec(ctx)
	return item, err
}
func (s *Service) DeleteNote(ctx context.Context, userID, id uuid.UUID) error {
	res, err := s.db.NewUpdate().Model((*model.Note)(nil)).Set("deleted_at=?", s.now().UTC()).Set("updated_at=?", s.now().UTC()).Where("id=? AND user_id=? AND deleted_at IS NULL", id, userID).Exec(ctx)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Service) ownsBook(ctx context.Context, userID, bookID uuid.UUID) error {
	var exists bool
	if err := s.db.NewSelect().ColumnExpr("EXISTS(SELECT 1 FROM books WHERE id=? AND user_id=? AND deleted_at IS NULL)", bookID, userID).Scan(ctx, &exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}
func (s *Service) ownsChapter(ctx context.Context, bookID, chapterID uuid.UUID) error {
	var exists bool
	if err := s.db.NewSelect().ColumnExpr("EXISTS(SELECT 1 FROM book_chapters WHERE id=? AND book_id=?)", chapterID, bookID).Scan(ctx, &exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}
func deleteOwned[T any](ctx context.Context, db *bun.DB, userID, id uuid.UUID) error {
	var item T
	res, err := db.NewDelete().Model(&item).Where("id=? AND user_id=?", id, userID).Exec(ctx)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func validLocator(v json.RawMessage) bool { return len(v) == 0 || (len(v) <= 16*1024 && json.Valid(v)) }
func validColor(v string) bool {
	switch v {
	case "", "sand", "sage", "blue", "rose", "yellow", "green", "pink", "purple", "gray":
		return true
	}
	return false
}
func plain(v string) string {
	v = decodeEntities(v)
	v = bluemonday.StrictPolicy().Sanitize(v)
	return strings.TrimSpace(decodeEntities(v))
}

func decodeEntities(v string) string {
	for range 3 {
		decoded := html.UnescapeString(v)
		decoded = numericEntityPattern.ReplaceAllStringFunc(decoded, func(entity string) string {
			match := numericEntityPattern.FindStringSubmatch(entity)
			if len(match) != 2 {
				return entity
			}
			base := 10
			value := match[1]
			if strings.HasPrefix(strings.ToLower(value), "x") {
				base = 16
				value = value[1:]
			}
			codePoint, err := strconv.ParseInt(value, base, 32)
			if err != nil || codePoint <= 0 || codePoint > 0x10ffff || codePoint >= 0xd800 && codePoint <= 0xdfff {
				return entity
			}
			return string(rune(codePoint))
		})
		if decoded == v {
			break
		}
		v = decoded
	}
	return v
}
func sanitizeBlocks(raw json.RawMessage) (json.RawMessage, string, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`[]`)
	}
	if len(raw) > 256*1024 {
		return nil, "", errors.New("note blocks are too large")
	}
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, "", errors.New("invalid note blocks")
	}
	if len(blocks) > 1000 {
		return nil, "", errors.New("too many note blocks")
	}
	allowed := map[string]bool{"paragraph": true, "text": true, "heading1": true, "heading2": true, "heading3": true, "bulleted_list": true, "numbered_list": true, "task": true, "task_list": true, "quote": true, "callout": true, "divider": true, "link": true, "book_link": true, "saved_quote": true}
	var search strings.Builder
	for _, block := range blocks {
		kind, _ := block["type"].(string)
		if !allowed[kind] {
			return nil, "", errors.New("unsupported note block type")
		}
		if err := validateURLs(block); err != nil {
			return nil, "", err
		}
		sanitizeMap(block, &search)
	}
	clean, err := json.Marshal(blocks)
	text := search.String()
	if len(text) > 100000 {
		text = text[:100000]
	}
	return clean, text, err
}
func validateURLs(value map[string]any) error {
	for key, item := range value {
		switch v := item.(type) {
		case string:
			if key == "url" || key == "href" {
				if strings.HasPrefix(v, "/api/v1/books/") {
					continue
				}
				parsed, err := url.ParseRequestURI(v)
				if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
					return errors.New("note contains an unsafe URL")
				}
			}
		case map[string]any:
			if err := validateURLs(v); err != nil {
				return err
			}
		case []any:
			for _, element := range v {
				if nested, ok := element.(map[string]any); ok {
					if err := validateURLs(nested); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
func sanitizeMap(value map[string]any, search *strings.Builder) {
	for key, item := range value {
		switch v := item.(type) {
		case string:
			clean := plain(v)
			value[key] = clean
			if key != "url" && clean != "" {
				search.WriteString(clean)
				search.WriteByte(' ')
			}
		case map[string]any:
			sanitizeMap(v, search)
		case []any:
			for i, element := range v {
				if text, ok := element.(string); ok {
					v[i] = plain(text)
					search.WriteString(v[i].(string))
					search.WriteByte(' ')
				} else if nested, ok := element.(map[string]any); ok {
					sanitizeMap(nested, search)
				}
			}
		}
	}
}
