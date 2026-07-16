package books

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/XRS0/reader/backend/internal/bookprocessing"
	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/XRS0/reader/backend/internal/storage"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

var (
	ErrNotFound      = errors.New("book not found")
	ErrForbidden     = errors.New("book access denied")
	ErrInvalidFile   = errors.New("invalid book file")
	ErrTooLarge      = errors.New("book file is too large")
	ErrInvalidCover  = errors.New("invalid book cover")
	ErrCoverTooLarge = errors.New("book cover is too large")
)

const (
	MaxCoverBytes       int64 = 5 * 1024 * 1024
	maxStoredCoverBytes int64 = 20 * 1024 * 1024
)

type Service struct {
	db       *bun.DB
	store    storage.ObjectStore
	registry *bookprocessing.Registry
	cfg      config.Config
	now      func() time.Time
}
type UploadInput struct {
	Filename   string
	ClientMIME string
	Data       []byte
}
type UploadResult struct {
	Book      model.Book `json:"book"`
	Duplicate bool       `json:"duplicate"`
}

type CoverInput struct {
	ClientMIME string
	Data       []byte
}
type ListFilter struct {
	Search   string
	Status   string
	Format   string
	Limit    int
	Offset   int
	Sort     string
	Favorite *bool
}
type ListResult struct {
	Items      []model.Book `json:"items"`
	Total      int          `json:"total"`
	HasMore    bool         `json:"has_more"`
	NextOffset int          `json:"next_offset,omitempty"`
}

func NewService(db *bun.DB, store storage.ObjectStore, registry *bookprocessing.Registry, cfg config.Config) *Service {
	return &Service{db: db, store: store, registry: registry, cfg: cfg, now: time.Now}
}

func (s *Service) Upload(ctx context.Context, userID uuid.UUID, in UploadInput) (UploadResult, error) {
	if len(in.Data) == 0 {
		return UploadResult{}, fmt.Errorf("%w: empty file", ErrInvalidFile)
	}
	if int64(len(in.Data)) > s.cfg.Upload.MaxBytes {
		return UploadResult{}, ErrTooLarge
	}
	filename := safeFilename(in.Filename)
	format, mediaType, err := s.validate(ctx, filename, in.ClientMIME, in.Data)
	if err != nil {
		return UploadResult{}, err
	}
	sum := sha256.Sum256(in.Data)
	digest := hex.EncodeToString(sum[:])
	var existing model.Book
	err = s.db.NewSelect().Model(&existing).Where("user_id=?", userID).Where("sha256=?", digest).Where("deleted_at IS NULL").Scan(ctx)
	if err == nil {
		return UploadResult{Book: existing, Duplicate: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return UploadResult{}, err
	}
	now := s.now().UTC()
	id := uuid.New()
	key := fmt.Sprintf("users/%s/books/%s/original/%s", userID, id, digest)
	if err := s.store.Put(ctx, s.cfg.S3.OriginalBucket, key, bytes.NewReader(in.Data), int64(len(in.Data)), mediaType, map[string]string{"sha256": digest, "book-id": id.String()}); err != nil {
		return UploadResult{}, fmt.Errorf("store original: %w", err)
	}
	title := strings.TrimSuffix(filename, filepath.Ext(filename))
	if title == "" {
		title = "Untitled"
	}
	book := model.Book{ID: id, UserID: userID, Title: title, Format: format, Status: "queued", SHA256: digest, OriginalFilename: filename, OriginalMIME: mediaType, OriginalSize: int64(len(in.Data)), OriginalBucket: s.cfg.S3.OriginalBucket, OriginalKey: key, ProcessingVersion: 1, Metadata: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
	payload, _ := json.Marshal(ProcessPayload{BookID: id, UserID: userID, Version: 1})
	job := model.Job{ID: uuid.New(), Type: JobProcessBook, Payload: payload, Status: "queued", MaxAttempts: s.cfg.Worker.MaxAttempts, RunAfter: now, CreatedAt: now, UpdatedAt: now}
	err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(&book).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewInsert().Model(&job).Exec(ctx)
		return err
	})
	if err != nil {
		_ = s.store.Delete(context.WithoutCancel(ctx), s.cfg.S3.OriginalBucket, key)
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			if findErr := s.db.NewSelect().Model(&existing).Where("user_id=? AND sha256=? AND deleted_at IS NULL", userID, digest).Scan(ctx); findErr == nil {
				return UploadResult{Book: existing, Duplicate: true}, nil
			}
		}
		return UploadResult{}, err
	}
	return UploadResult{Book: book}, nil
}

func (s *Service) validate(ctx context.Context, name, clientMIME string, data []byte) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".txt" && ext != ".fb2" && ext != ".epub" {
		return "", "", fmt.Errorf("%w: unsupported extension", ErrInvalidFile)
	}
	detected := http.DetectContentType(data)
	allowed := map[string][]string{".txt": {"text/plain", "application/octet-stream"}, ".fb2": {"text/plain", "text/xml", "application/xml", "application/octet-stream"}, ".epub": {"application/zip", "application/epub+zip", "application/octet-stream"}}
	normalized := strings.ToLower(strings.TrimSpace(strings.Split(clientMIME, ";")[0]))
	if normalized != "" && !contains(allowed[ext], normalized) {
		return "", "", fmt.Errorf("%w: client MIME %s does not match %s", ErrInvalidFile, normalized, ext)
	}
	detectedBase := strings.ToLower(strings.TrimSpace(strings.Split(detected, ";")[0]))
	if !contains(allowed[ext], detectedBase) {
		return "", "", fmt.Errorf("%w: detected MIME %s does not match %s", ErrInvalidFile, detectedBase, ext)
	}
	file := bookprocessing.BookFile{Name: name, MediaType: detected, Data: data}
	parser, err := s.registry.Detect(ctx, file)
	if err != nil {
		if ext == ".epub" && !errors.Is(err, bookprocessing.ErrArchiveLimit) && len(data) >= 2 && data[0] == 'P' && data[1] == 'K' {
			return "epub", "application/epub+zip", nil
		}
		return "", "", fmt.Errorf("%w: %w", ErrInvalidFile, err)
	}
	if parser == nil {
		return "", "", ErrInvalidFile
	}
	mediaType := map[string]string{".txt": "text/plain; charset=utf-8", ".fb2": "application/x-fictionbook+xml", ".epub": "application/epub+zip"}[ext]
	return strings.TrimPrefix(ext, "."), mediaType, nil
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func safeFilename(raw string) string {
	name := filepath.Base(strings.ReplaceAll(raw, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r == 0 || r < ' ' || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || name == "" {
		return "book"
	}
	if len(name) > 240 {
		name = name[len(name)-240:]
	}
	return name
}

func validateCover(in CoverInput) (mediaType, extension string, err error) {
	if len(in.Data) == 0 {
		return "", "", fmt.Errorf("%w: empty image", ErrInvalidCover)
	}
	if int64(len(in.Data)) > MaxCoverBytes {
		return "", "", ErrCoverTooLarge
	}
	detected := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(in.Data), ";")[0]))
	extensions := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
	}
	extension, ok := extensions[detected]
	if !ok {
		return "", "", fmt.Errorf("%w: only JPEG, PNG and WebP are supported", ErrInvalidCover)
	}
	clientMIME := strings.ToLower(strings.TrimSpace(strings.Split(in.ClientMIME, ";")[0]))
	if clientMIME != "" && clientMIME != "application/octet-stream" && clientMIME != detected {
		return "", "", fmt.Errorf("%w: declared MIME does not match image content", ErrInvalidCover)
	}
	return detected, extension, nil
}

func normalizeTags(tags []string) ([]string, error) {
	if len(tags) > 20 {
		return nil, errors.New("a book can have at most 20 tags")
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" || utf8.RuneCountInString(tag) > 50 {
			return nil, errors.New("invalid book tag")
		}
		key := strings.ToLower(tag)
		if !seen[key] {
			seen[key] = true
			result = append(result, tag)
		}
	}
	return result, nil
}

func (s *Service) enrich(ctx context.Context, userID uuid.UUID, items []model.Book) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(items))
	index := make(map[uuid.UUID]int, len(items))
	for i := range items {
		ids[i] = items[i].ID
		index[items[i].ID] = i
		items[i].Tags = []string{}
	}
	var progress []struct {
		BookID    uuid.UUID  `bun:"book_id"`
		ChapterID *uuid.UUID `bun:"chapter_id"`
		Progress  float64    `bun:"progress_percent"`
		UpdatedAt time.Time  `bun:"updated_at"`
	}
	if err := s.db.NewSelect().Table("reading_progress").Column("book_id", "chapter_id", "progress_percent", "updated_at").Where("user_id=?", userID).Where("book_id IN (?)", bun.In(ids)).Scan(ctx, &progress); err != nil {
		return err
	}
	for _, row := range progress {
		if i, ok := index[row.BookID]; ok {
			items[i].ProgressPercent = row.Progress
			items[i].CurrentChapterID = row.ChapterID
			updated := row.UpdatedAt
			items[i].LastReadAt = &updated
		}
	}
	var sessions []struct {
		BookID     uuid.UUID `bun:"book_id"`
		LastReadAt time.Time `bun:"last_read_at"`
	}
	if err := s.db.NewSelect().Table("reading_sessions").Column("book_id").ColumnExpr("max(started_at) AS last_read_at").Where("user_id=?", userID).Where("book_id IN (?)", bun.In(ids)).Group("book_id").Scan(ctx, &sessions); err != nil {
		return err
	}
	for _, row := range sessions {
		if i, ok := index[row.BookID]; ok && (items[i].LastReadAt == nil || row.LastReadAt.After(*items[i].LastReadAt)) {
			value := row.LastReadAt
			items[i].LastReadAt = &value
		}
	}
	var tags []struct {
		BookID uuid.UUID `bun:"book_id"`
		Tag    string    `bun:"tag"`
	}
	if err := s.db.NewSelect().Table("book_tags").Column("book_id", "tag").Where("book_id IN (?)", bun.In(ids)).Order("tag ASC").Scan(ctx, &tags); err != nil {
		return err
	}
	for _, row := range tags {
		if i, ok := index[row.BookID]; ok {
			items[i].Tags = append(items[i].Tags, row.Tag)
		}
	}
	return nil
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, f ListFilter) (ListResult, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	q := s.db.NewSelect().Model((*model.Book)(nil)).Where("b.user_id=?", userID).Where("b.deleted_at IS NULL")
	if f.Search != "" {
		needle := "%" + strings.ToLower(strings.TrimSpace(f.Search)) + "%"
		q = q.Where("(lower(b.title) LIKE ? OR lower(b.author) LIKE ?)", needle, needle)
	}
	if f.Status != "" && f.Status != "all" {
		q = q.Where("b.status=?", f.Status)
	}
	if f.Format != "" && f.Format != "all" {
		q = q.Where("b.format=?", f.Format)
	}
	if f.Favorite != nil {
		q = q.Where("b.is_favorite=?", *f.Favorite)
	}
	var items []model.Book
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return ListResult{}, err
	}
	order := "b.created_at DESC"
	switch f.Sort {
	case "title":
		order = "b.title ASC"
	case "updated", "last_read":
		q = q.Join("LEFT JOIN reading_progress AS sort_rp ON sort_rp.book_id=b.id AND sort_rp.user_id=b.user_id")
		order = "sort_rp.updated_at DESC NULLS LAST, b.created_at DESC"
	case "added":
		order = "b.created_at DESC"
	case "progress":
		q = q.Join("LEFT JOIN reading_progress AS sort_rp ON sort_rp.book_id=b.id AND sort_rp.user_id=b.user_id")
		order = "sort_rp.progress_percent DESC NULLS LAST, b.created_at DESC"
	}
	err = q.Model(&items).OrderExpr(order).Limit(f.Limit).Offset(f.Offset).Scan(ctx)
	if err != nil {
		return ListResult{}, err
	}
	if err := s.enrich(ctx, userID, items); err != nil {
		return ListResult{}, err
	}
	result := ListResult{Items: items, Total: total, HasMore: f.Offset+len(items) < total}
	if result.HasMore {
		result.NextOffset = f.Offset + len(items)
	}
	return result, nil
}
func (s *Service) Get(ctx context.Context, userID, bookID uuid.UUID) (model.Book, error) {
	var b model.Book
	err := s.db.NewSelect().Model(&b).Where("id=?", bookID).Where("user_id=?", userID).Where("deleted_at IS NULL").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Book{}, ErrNotFound
	}
	if err == nil {
		items := []model.Book{b}
		if enrichErr := s.enrich(ctx, userID, items); enrichErr != nil {
			return model.Book{}, enrichErr
		}
		b = items[0]
	}
	return b, err
}
func (s *Service) Update(ctx context.Context, userID, bookID uuid.UUID, title, author, language, description *string, favorite *bool, tags *[]string) (model.Book, error) {
	b, err := s.Get(ctx, userID, bookID)
	if err != nil {
		return b, err
	}
	q := s.db.NewUpdate().Model(&b).Set("updated_at=?", s.now().UTC()).WherePK().Where("user_id=?", userID)
	if title != nil {
		v := strings.TrimSpace(*title)
		if v == "" || len(v) > 500 {
			return b, errors.New("invalid title")
		}
		q = q.Set("title=?", v)
	}
	if author != nil {
		if len(*author) > 500 {
			return b, errors.New("invalid author")
		}
		q = q.Set("author=?", strings.TrimSpace(*author))
	}
	if language != nil {
		if len(*language) > 32 {
			return b, errors.New("invalid language")
		}
		q = q.Set("language=?", strings.ToLower(strings.TrimSpace(*language)))
	}
	if description != nil {
		if len(*description) > 20000 {
			return b, errors.New("description too long")
		}
		q = q.Set("description=?", strings.TrimSpace(*description))
	}
	if favorite != nil {
		q = q.Set("is_favorite=?", *favorite)
	}
	err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := q.Conn(tx).Returning("*").Exec(ctx); err != nil {
			return err
		}
		if tags != nil {
			clean, err := normalizeTags(*tags)
			if err != nil {
				return err
			}
			if _, err = tx.NewDelete().Table("book_tags").Where("book_id=?", bookID).Exec(ctx); err != nil {
				return err
			}
			for _, tag := range clean {
				if _, err = tx.ExecContext(ctx, "INSERT INTO book_tags(book_id,tag) VALUES (?,?)", bookID, tag); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return b, err
	}
	// Read the row again after committing instead of relying on Exec to scan the
	// RETURNING clause into b. Besides making the PATCH response deterministic,
	// Get also enriches the response with the freshly committed tags and reading
	// state.
	return s.Get(ctx, userID, bookID)
}

func (s *Service) UpdateCover(ctx context.Context, userID, bookID uuid.UUID, in CoverInput) (model.Book, error) {
	b, err := s.Get(ctx, userID, bookID)
	if err != nil {
		return b, err
	}
	mediaType, extension, err := validateCover(in)
	if err != nil {
		return b, err
	}
	sum := sha256.Sum256(in.Data)
	key := fmt.Sprintf("users/%s/books/%s/custom-cover/%s%s", userID, bookID, uuid.New(), extension)
	if err := s.store.Put(ctx, s.cfg.S3.CoversBucket, key, bytes.NewReader(in.Data), int64(len(in.Data)), mediaType, map[string]string{
		"book-id": bookID.String(),
		"sha256":  hex.EncodeToString(sum[:]),
		"source":  "user",
	}); err != nil {
		return b, fmt.Errorf("store custom cover: %w", err)
	}
	oldBucket, oldKey := b.CustomCoverBucket, b.CustomCoverKey
	now := s.now().UTC()
	result, err := s.db.NewUpdate().Model((*model.Book)(nil)).
		Set("custom_cover_bucket=?", s.cfg.S3.CoversBucket).
		Set("custom_cover_key=?", key).
		Set("updated_at=?", now).
		Where("id=? AND user_id=? AND deleted_at IS NULL", bookID, userID).
		Exec(ctx)
	if err != nil {
		_ = s.store.Delete(context.WithoutCancel(ctx), s.cfg.S3.CoversBucket, key)
		return b, err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		_ = s.store.Delete(context.WithoutCancel(ctx), s.cfg.S3.CoversBucket, key)
		return b, ErrNotFound
	}
	if oldBucket != "" && oldKey != "" {
		_ = s.store.Delete(context.WithoutCancel(ctx), oldBucket, oldKey)
	}
	return s.Get(ctx, userID, bookID)
}

func (s *Service) DeleteCover(ctx context.Context, userID, bookID uuid.UUID) (model.Book, error) {
	b, err := s.Get(ctx, userID, bookID)
	if err != nil {
		return b, err
	}
	if b.CustomCoverBucket == "" || b.CustomCoverKey == "" {
		return b, nil
	}
	result, err := s.db.NewUpdate().Model((*model.Book)(nil)).
		Set("custom_cover_bucket=''").
		Set("custom_cover_key=''").
		Set("updated_at=?", s.now().UTC()).
		Where("id=? AND user_id=? AND deleted_at IS NULL", bookID, userID).
		Exec(ctx)
	if err != nil {
		return b, err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return b, ErrNotFound
	}
	_ = s.store.Delete(context.WithoutCancel(ctx), b.CustomCoverBucket, b.CustomCoverKey)
	return s.Get(ctx, userID, bookID)
}
func (s *Service) Delete(ctx context.Context, userID, bookID uuid.UUID) error {
	b, err := s.Get(ctx, userID, bookID)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	payload, _ := json.Marshal(CleanupPayload{BookID: b.ID, UserID: userID, OriginalBucket: b.OriginalBucket, OriginalKey: b.OriginalKey})
	job := model.Job{ID: uuid.New(), Type: JobCleanupBook, Payload: payload, Status: "queued", MaxAttempts: s.cfg.Worker.MaxAttempts, RunAfter: now, CreatedAt: now, UpdatedAt: now}
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewUpdate().Model((*model.Book)(nil)).Set("deleted_at=?", now).Set("updated_at=?", now).Where("id=? AND user_id=? AND deleted_at IS NULL", bookID, userID).Exec(ctx)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
		_, err = tx.NewInsert().Model(&job).Exec(ctx)
		return err
	})
}
func (s *Service) Reprocess(ctx context.Context, userID, bookID uuid.UUID) (model.Book, error) {
	b, err := s.Get(ctx, userID, bookID)
	if err != nil {
		return b, err
	}
	now := s.now().UTC()
	next := b.ProcessingVersion + 1
	payload, _ := json.Marshal(ProcessPayload{BookID: b.ID, UserID: userID, Version: next})
	job := model.Job{ID: uuid.New(), Type: JobProcessBook, Payload: payload, Status: "queued", MaxAttempts: s.cfg.Worker.MaxAttempts, RunAfter: now, CreatedAt: now, UpdatedAt: now}
	err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewUpdate().Model((*model.Book)(nil)).Set("processing_version=?", next).Set("status='queued'").Set("processing_error='' ").Set("updated_at=?", now).Where("id=? AND user_id=? AND processing_version=?", bookID, userID, b.ProcessingVersion).Exec(ctx)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return errors.New("book was concurrently reprocessed")
		}
		_, err = tx.NewInsert().Model(&job).Exec(ctx)
		return err
	})
	if err != nil {
		return b, err
	}
	b.ProcessingVersion = next
	b.Status = "queued"
	b.ProcessingError = ""
	b.UpdatedAt = now
	return b, nil
}
func (s *Service) Chapters(ctx context.Context, userID, bookID uuid.UUID) ([]model.BookChapter, error) {
	b, err := s.Get(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}
	var chapters []model.BookChapter
	err = s.db.NewSelect().Model(&chapters).Where("book_id=? AND version=?", bookID, b.ProcessingVersion).Order("ordinal ASC").Scan(ctx)
	return chapters, err
}
func (s *Service) Chapter(ctx context.Context, userID, bookID, chapterID uuid.UUID) (model.BookChapter, error) {
	b, err := s.Get(ctx, userID, bookID)
	if err != nil {
		return model.BookChapter{}, err
	}
	var c model.BookChapter
	err = s.db.NewSelect().Model(&c).Where("id=? AND book_id=? AND version=?", chapterID, bookID, b.ProcessingVersion).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	if err == nil && c.ContentHTML == "" && c.ContentKey != "" {
		data, getErr := s.store.Get(ctx, c.ContentBucket, c.ContentKey, 10*1024*1024)
		if getErr != nil {
			return c, getErr
		}
		c.ContentHTML = string(data)
	}
	return c, err
}
func (s *Service) DownloadURL(ctx context.Context, userID, bookID uuid.UUID) (string, error) {
	b, err := s.Get(ctx, userID, bookID)
	if err != nil {
		return "", err
	}
	return s.store.PresignGet(ctx, b.OriginalBucket, b.OriginalKey, s.cfg.S3.PresignTTL)
}

func (s *Service) Cover(ctx context.Context, userID, bookID uuid.UUID) ([]byte, string, error) {
	b, err := s.Get(ctx, userID, bookID)
	if err != nil {
		return nil, "", err
	}
	bucket, key := b.CustomCoverBucket, b.CustomCoverKey
	if bucket == "" || key == "" {
		bucket, key = b.CoverBucket, b.CoverKey
	}
	if bucket == "" || key == "" {
		return nil, "", ErrNotFound
	}
	data, err := s.store.Get(ctx, bucket, key, maxStoredCoverBytes)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0]))
	switch mediaType {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
	default:
		mediaType = "application/octet-stream"
	}
	return data, mediaType, nil
}
func (s *Service) AssetURL(ctx context.Context, userID, bookID, assetID uuid.UUID) (string, error) {
	b, err := s.Get(ctx, userID, bookID)
	if err != nil {
		return "", err
	}
	var a model.BookAsset
	err = s.db.NewSelect().Model(&a).Where("id=? AND book_id=? AND version=?", assetID, bookID, b.ProcessingVersion).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return s.store.PresignGet(ctx, a.Bucket, a.ObjectKey, s.cfg.S3.PresignTTL)
}
func ContentTypeForName(name string) string {
	if value := mime.TypeByExtension(filepath.Ext(name)); value != "" {
		return value
	}
	return "application/octet-stream"
}
