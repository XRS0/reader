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
	"path"
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

const (
	JobProcessBook = "process_book"
	JobCleanupBook = "cleanup_book"
)

type ProcessPayload struct {
	BookID  uuid.UUID `json:"book_id"`
	UserID  uuid.UUID `json:"user_id"`
	Version int       `json:"version"`
}
type CleanupPayload struct {
	BookID         uuid.UUID `json:"book_id"`
	UserID         uuid.UUID `json:"user_id"`
	OriginalBucket string    `json:"original_bucket"`
	OriginalKey    string    `json:"original_key"`
}
type Processor struct {
	db       *bun.DB
	store    storage.ObjectStore
	registry *bookprocessing.Registry
	cfg      config.Config
	now      func() time.Time
}

func NewProcessor(db *bun.DB, store storage.ObjectStore, registry *bookprocessing.Registry, cfg config.Config) *Processor {
	return &Processor{db: db, store: store, registry: registry, cfg: cfg, now: time.Now}
}

func (p *Processor) Process(ctx context.Context, payload ProcessPayload) error {
	var b model.Book
	err := p.db.NewSelect().Model(&b).Where("id=? AND user_id=? AND deleted_at IS NULL", payload.BookID, payload.UserID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if b.ProcessingVersion != payload.Version {
		return nil
	}
	count, err := p.db.NewSelect().Model((*model.BookChapter)(nil)).Where("book_id=? AND version=?", b.ID, payload.Version).Count(ctx)
	if err != nil {
		return err
	}
	if b.Status == "ready" && count > 0 {
		return nil
	}
	_, err = p.db.NewUpdate().Model((*model.Book)(nil)).Set("status='processing'").Set("processing_error='' ").Set("updated_at=?", p.now().UTC()).Where("id=? AND processing_version=?", b.ID, payload.Version).Exec(ctx)
	if err != nil {
		return err
	}
	if err = p.process(ctx, b, payload.Version); err != nil {
		p.markFailed(context.WithoutCancel(ctx), b.ID, payload.Version, err)
		return err
	}
	return nil
}
func (p *Processor) process(ctx context.Context, b model.Book, version int) error {
	data, err := p.store.Get(ctx, b.OriginalBucket, b.OriginalKey, p.cfg.Upload.MaxBytes)
	if err != nil {
		return fmt.Errorf("load original: %w", err)
	}
	file := bookprocessing.BookFile{Name: b.OriginalFilename, MediaType: b.OriginalMIME, Data: data}
	parser, err := p.registry.Detect(ctx, file)
	if err != nil {
		return fmt.Errorf("detect parser: %w", err)
	}
	metadata, err := parser.ParseMetadata(ctx, file)
	if err != nil {
		return fmt.Errorf("parse metadata: %w", err)
	}
	toc, err := parser.ParseTableOfContents(ctx, file)
	if err != nil {
		return fmt.Errorf("parse table of contents: %w", err)
	}
	chapters, err := parser.ExtractChapters(ctx, file)
	if err != nil {
		return fmt.Errorf("extract chapters: %w", err)
	}
	assets, err := parser.ExtractAssets(ctx, file)
	if err != nil {
		return fmt.Errorf("extract assets: %w", err)
	}
	if len(chapters) == 0 {
		return fmt.Errorf("%w: parser returned no chapters", bookprocessing.ErrCorruptBook)
	}
	assetModels := make([]model.BookAsset, 0, len(assets))
	assetURLs := map[string]string{}
	var coverBucket, coverKey string
	for _, asset := range assets {
		id := stableID(b.ID, fmt.Sprintf("asset:%d:%s", version, asset.SourceRef))
		sum := sha256.Sum256(asset.Data)
		key := fmt.Sprintf("books/%s/versions/%d/assets/%s", b.ID, version, id)
		bucket := p.cfg.S3.AssetsBucket
		if asset.IsCover {
			bucket = p.cfg.S3.CoversBucket
			key = fmt.Sprintf("books/%s/cover/%s", b.ID, id)
			coverBucket, coverKey = bucket, key
		}
		if err := p.store.Put(ctx, bucket, key, bytes.NewReader(asset.Data), int64(len(asset.Data)), asset.MediaType, map[string]string{"book-id": b.ID.String(), "sha256": hex.EncodeToString(sum[:])}); err != nil {
			return fmt.Errorf("store asset %q: %w", asset.SourceRef, err)
		}
		assetModels = append(assetModels, model.BookAsset{ID: id, BookID: b.ID, Version: version, SourceRef: asset.SourceRef, MediaType: asset.MediaType, Bucket: bucket, ObjectKey: key, Size: int64(len(asset.Data)), SHA256: hex.EncodeToString(sum[:]), CreatedAt: p.now().UTC()})
		assetURLs[asset.SourceRef] = fmt.Sprintf("/api/v1/books/%s/assets/%s", b.ID, id)
	}
	chapterModels := make([]model.BookChapter, 0, len(chapters))
	for i, ch := range chapters {
		id := stableID(b.ID, fmt.Sprintf("chapter:%d:%d:%s", version, i, ch.SourceRef))
		content := rewriteAssetReferences(ch.HTML, ch.SourceRef, assetURLs)
		content = bookprocessing.SanitizeHTML(content)
		plain := ch.PlainText
		if plain == "" {
			plain = bookprocessing.PlainTextFromHTML(content)
		}
		m := model.BookChapter{ID: id, BookID: b.ID, Version: version, Ordinal: i, Title: truncate(ch.Title, 500), SourceRef: truncate(ch.SourceRef, 2000), ContentHTML: content, ContentText: plain, CharacterCount: utf8.RuneCountInString(plain), WordCount: len(strings.Fields(plain)), CreatedAt: p.now().UTC()}
		if len(content) > 512*1024 {
			key := fmt.Sprintf("books/%s/versions/%d/chapters/%s.html", b.ID, version, id)
			if err := p.store.Put(ctx, p.cfg.S3.ContentBucket, key, strings.NewReader(content), int64(len(content)), "text/html; charset=utf-8", map[string]string{"book-id": b.ID.String()}); err != nil {
				return err
			}
			m.ContentHTML = ""
			m.ContentBucket = p.cfg.S3.ContentBucket
			m.ContentKey = key
		}
		chapterModels = append(chapterModels, m)
	}
	metadataJSON, err := json.Marshal(map[string]any{"identifier": metadata.Identifier, "authors": metadata.Authors, "toc": toc, "chapter_count": len(chapterModels), "asset_count": len(assetModels)})
	if err != nil {
		return fmt.Errorf("encode processing metadata: %w", err)
	}
	title := strings.TrimSpace(metadata.Title)
	if title == "" {
		title = b.Title
	}
	author := strings.Join(metadata.Authors, ", ")
	return p.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var current int
		if err := tx.NewSelect().Table("books").Column("processing_version").Where("id=?", b.ID).For("UPDATE").Scan(ctx, &current); err != nil {
			return err
		}
		if current != version {
			return nil
		}
		if _, err := tx.NewDelete().Model((*model.BookChapter)(nil)).Where("book_id=? AND version=?", b.ID, version).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*model.BookAsset)(nil)).Where("book_id=? AND version=?", b.ID, version).Exec(ctx); err != nil {
			return err
		}
		if len(chapterModels) > 0 {
			if _, err := tx.NewInsert().Model(&chapterModels).Exec(ctx); err != nil {
				return err
			}
		}
		if len(assetModels) > 0 {
			if _, err := tx.NewInsert().Model(&assetModels).Exec(ctx); err != nil {
				return err
			}
		}
		if err := remapChapterReferences(ctx, tx, b.ID, version, p.now().UTC()); err != nil {
			return fmt.Errorf("remap chapter references: %w", err)
		}
		_, err := tx.NewUpdate().Model((*model.Book)(nil)).Set("title=?", title).Set("author=?", truncate(author, 500)).Set("language=?", truncate(strings.ToLower(metadata.Language), 32)).Set("description=?", truncate(metadata.Description, 20000)).Set("metadata=CAST(? AS jsonb)", string(metadataJSON)).Set("cover_bucket=?", coverBucket).Set("cover_key=?", coverKey).Set("status='ready'").Set("processing_error='' ").Set("updated_at=?", p.now().UTC()).Where("id=? AND processing_version=?", b.ID, version).Exec(ctx)
		return err
	})
}

// remapChapterReferences keeps durable reader state attached to the current
// chapter IDs after a reprocess. Chapter IDs intentionally include the
// processing version, while source_ref identifies the same source chapter
// across versions. Repeated source refs are paired by their occurrence order
// so PostgreSQL never chooses an arbitrary UPDATE ... FROM match.
//
// The caller must invoke this after inserting the new-version chapters and in
// the same transaction that marks the book ready. A repeated invocation is a
// no-op because references already pointing at the current version are not in
// old_chapters.
func remapChapterReferences(ctx context.Context, tx bun.Tx, bookID uuid.UUID, version int, now time.Time) error {
	const chapterMapCTE = `
WITH params(book_id, version) AS (
  VALUES (CAST(? AS uuid), CAST(? AS integer))
),
old_chapters AS (
  SELECT bc.id,
         bc.source_ref,
         row_number() OVER (
           PARTITION BY bc.version, bc.source_ref
           ORDER BY bc.ordinal, bc.id
         ) AS occurrence
  FROM book_chapters AS bc
  CROSS JOIN params AS p
  WHERE bc.book_id = p.book_id
    AND bc.version < p.version
),
new_chapters AS (
  SELECT bc.id,
         bc.source_ref,
         row_number() OVER (
           PARTITION BY bc.source_ref
           ORDER BY bc.ordinal, bc.id
         ) AS occurrence
  FROM book_chapters AS bc
  CROSS JOIN params AS p
  WHERE bc.book_id = p.book_id
    AND bc.version = p.version
),
chapter_map AS (
  SELECT old_chapters.id AS old_id, new_chapters.id AS new_id
  FROM old_chapters
  JOIN new_chapters
    ON new_chapters.source_ref = old_chapters.source_ref
   AND new_chapters.occurrence = old_chapters.occurrence
)
`
	queries := []struct {
		query string
		args  []any
	}{
		{
			query: chapterMapCTE + `
UPDATE reading_progress AS rp
SET chapter_id = chapter_map.new_id,
    revision = rp.revision + 1,
    updated_at = ?
FROM chapter_map
WHERE rp.book_id = ?
  AND rp.chapter_id = chapter_map.old_id`,
			args: []any{bookID, version, now, bookID},
		},
		{
			query: chapterMapCTE + `
UPDATE bookmarks AS bm
SET chapter_id = chapter_map.new_id
FROM chapter_map
WHERE bm.book_id = ?
  AND bm.chapter_id = chapter_map.old_id`,
			args: []any{bookID, version, bookID},
		},
		{
			query: chapterMapCTE + `
UPDATE highlights AS h
SET chapter_id = chapter_map.new_id
FROM chapter_map
WHERE h.book_id = ?
  AND h.chapter_id = chapter_map.old_id`,
			args: []any{bookID, version, bookID},
		},
		{
			query: chapterMapCTE + `
UPDATE word_occurrences AS wo
SET chapter_id = chapter_map.new_id
FROM chapter_map
WHERE wo.book_id = ?
  AND wo.chapter_id = chapter_map.old_id`,
			args: []any{bookID, version, bookID},
		},
	}
	for _, statement := range queries {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) markFailed(ctx context.Context, id uuid.UUID, version int, cause error) {
	message := cause.Error()
	if len(message) > 2000 {
		message = message[:2000]
	}
	_, _ = p.db.NewUpdate().Model((*model.Book)(nil)).Set("status='failed'").Set("processing_error=?", message).Set("updated_at=?", p.now().UTC()).Where("id=? AND processing_version=?", id, version).Exec(ctx)
}
func (p *Processor) Cleanup(ctx context.Context, payload CleanupPayload) error {
	var book model.Book
	if err := p.db.NewSelect().Model(&book).Where("id=? AND user_id=? AND deleted_at IS NOT NULL", payload.BookID, payload.UserID).Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	var assets []model.BookAsset
	if err := p.db.NewSelect().Model(&assets).Where("book_id=?", payload.BookID).Scan(ctx); err != nil {
		return err
	}
	var chapters []model.BookChapter
	if err := p.db.NewSelect().Model(&chapters).Where("book_id=?", payload.BookID).Scan(ctx); err != nil {
		return err
	}
	locations := map[string][2]string{}
	add := func(bucket, key string) {
		if bucket != "" && key != "" {
			locations[bucket+"\x00"+key] = [2]string{bucket, key}
		}
	}
	add(book.OriginalBucket, book.OriginalKey)
	add(book.CoverBucket, book.CoverKey)
	add(book.CustomCoverBucket, book.CustomCoverKey)
	add(payload.OriginalBucket, payload.OriginalKey)
	for _, asset := range assets {
		add(asset.Bucket, asset.ObjectKey)
	}
	for _, chapter := range chapters {
		add(chapter.ContentBucket, chapter.ContentKey)
	}
	for _, location := range locations {
		if err := p.store.Delete(ctx, location[0], location[1]); err != nil {
			return err
		}
	}
	_, err := p.db.NewDelete().Model((*model.Book)(nil)).Where("id=? AND user_id=? AND deleted_at IS NOT NULL", payload.BookID, payload.UserID).Exec(ctx)
	return err
}
func stableID(namespace uuid.UUID, name string) uuid.UUID {
	return uuid.NewSHA1(namespace, []byte(name))
}
func rewriteAssetReferences(content, chapterRef string, assets map[string]string) string {
	base := path.Dir(chapterRef)
	for source, target := range assets {
		candidates := []string{source, path.Base(source)}
		if rel, err := filepath.Rel(filepath.FromSlash(base), filepath.FromSlash(source)); err == nil {
			candidates = append(candidates, filepath.ToSlash(rel))
		}
		for _, candidate := range candidates {
			for _, quote := range []string{"\"", "'"} {
				content = strings.ReplaceAll(content, "src="+quote+candidate+quote, "src="+quote+target+quote)
				content = strings.ReplaceAll(content, "href="+quote+candidate+quote, "href="+quote+target+quote)
			}
		}
	}
	return content
}
func truncate(v string, n int) string {
	v = strings.TrimSpace(v)
	if len(v) <= n {
		return v
	}
	return v[:n]
}
