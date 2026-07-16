//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/XRS0/reader/backend/internal/bookprocessing"
	"github.com/XRS0/reader/backend/internal/books"
	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/XRS0/reader/backend/internal/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBookProcessorPersistsJSONMetadataAndChapters(t *testing.T) {
	resetDatabase(t)
	user := createUser(t, "UTC")
	book := createBook(t, user.ID, "Queued TXT")
	original := []byte("Integration Processing Title\n\nChapter 1\nFirst chapter body.\n\nChapter 2\nSecond chapter body.")

	_, err := integrationDB.NewUpdate().Model((*model.Book)(nil)).
		Set("status='queued'").
		Set("original_size=?", len(original)).
		Where("id=?", book.ID).
		Exec(testContext(t))
	require.NoError(t, err)

	store := storage.NewMemoryStore()
	require.NoError(t, store.Put(
		testContext(t),
		book.OriginalBucket,
		book.OriginalKey,
		bytes.NewReader(original),
		int64(len(original)),
		"text/plain",
		nil,
	))

	limits := bookprocessing.ArchiveLimits{
		MaxFiles:            100,
		MaxUnpackedBytes:    10 * 1024 * 1024,
		MaxCompressionRatio: 100,
		MaxEntryBytes:       10 * 1024 * 1024,
	}
	cfg := config.Config{
		Upload: config.Upload{MaxBytes: 10 * 1024 * 1024},
		S3: config.S3{
			ContentBucket: "books-content",
			AssetsBucket:  "books-assets",
			CoversBucket:  "books-covers",
		},
	}
	processor := books.NewProcessor(integrationDB, store, bookprocessing.NewRegistry(limits), cfg)
	payload := books.ProcessPayload{BookID: book.ID, UserID: user.ID, Version: 1}
	require.NoError(t, processor.Process(testContext(t), payload))

	var processed model.Book
	require.NoError(t, integrationDB.NewSelect().Model(&processed).Where("id=?", book.ID).Scan(testContext(t)))
	require.Equal(t, "ready", processed.Status)
	require.Empty(t, processed.ProcessingError)
	require.Equal(t, "Integration Processing Title", processed.Title)

	var metadata struct {
		ChapterCount int                      `json:"chapter_count"`
		AssetCount   int                      `json:"asset_count"`
		TOC          []bookprocessing.TOCItem `json:"toc"`
	}
	require.NoError(t, json.Unmarshal(processed.Metadata, &metadata))
	require.Equal(t, 3, metadata.ChapterCount)
	require.Zero(t, metadata.AssetCount)
	require.Len(t, metadata.TOC, 3)

	var chapters []model.BookChapter
	require.NoError(t, integrationDB.NewSelect().Model(&chapters).
		Where("book_id=? AND version=?", book.ID, 1).
		Order("ordinal ASC").
		Scan(testContext(t)))
	require.Len(t, chapters, 3)
	require.Contains(t, chapters[1].ContentText, "First chapter body")
	require.NotEmpty(t, chapters[1].ContentHTML)

	// A repeated delivery of the same job must be a no-op and must not create
	// duplicate chapters.
	require.NoError(t, processor.Process(testContext(t), payload))
	count, err := integrationDB.NewSelect().Model((*model.BookChapter)(nil)).
		Where("book_id=? AND version=?", book.ID, 1).
		Count(testContext(t))
	require.NoError(t, err)
	require.Equal(t, 3, count)
}

func TestCustomBookCoverReplacesAndRestoresEmbeddedCover(t *testing.T) {
	resetDatabase(t)
	user := createUser(t, "UTC")
	book := createBook(t, user.ID, "Cover lifecycle")
	store := storage.NewMemoryStore()
	embedded := []byte("embedded cover")
	embeddedBucket, embeddedKey := "books-covers", "embedded/cover.jpg"
	require.NoError(t, store.Put(testContext(t), embeddedBucket, embeddedKey, bytes.NewReader(embedded), int64(len(embedded)), "image/jpeg", nil))
	_, err := integrationDB.NewUpdate().Model((*model.Book)(nil)).
		Set("cover_bucket=?", embeddedBucket).
		Set("cover_key=?", embeddedKey).
		Where("id=?", book.ID).
		Exec(testContext(t))
	require.NoError(t, err)

	service := books.NewService(integrationDB, store, nil, config.Config{S3: config.S3{
		CoversBucket: "books-covers",
		PresignTTL:   time.Minute,
	}})
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 520)...)
	updated, err := service.UpdateCover(testContext(t), user.ID, book.ID, books.CoverInput{
		ClientMIME: "image/png",
		Data:       png,
	})
	require.NoError(t, err)
	require.NotEmpty(t, updated.CustomCoverKey)
	firstCustomKey := updated.CustomCoverKey
	coverData, mediaType, err := service.Cover(testContext(t), user.ID, book.ID)
	require.NoError(t, err)
	require.Equal(t, "image/png", mediaType)
	require.Equal(t, png, coverData)

	jpeg := append([]byte("\xff\xd8\xff\xe0"), bytes.Repeat([]byte{0}, 520)...)
	updated, err = service.UpdateCover(testContext(t), user.ID, book.ID, books.CoverInput{
		ClientMIME: "image/jpeg",
		Data:       jpeg,
	})
	require.NoError(t, err)
	require.NotEqual(t, firstCustomKey, updated.CustomCoverKey)
	exists, err := store.Exists(testContext(t), "books-covers", firstCustomKey)
	require.NoError(t, err)
	require.False(t, exists)

	restored, err := service.DeleteCover(testContext(t), user.ID, book.ID)
	require.NoError(t, err)
	require.Empty(t, restored.CustomCoverKey)
	coverData, mediaType, err = service.Cover(testContext(t), user.ID, book.ID)
	require.NoError(t, err)
	require.Equal(t, "application/octet-stream", mediaType)
	require.Equal(t, embedded, coverData)
}

func TestBookProcessorReprocessRemapsDurableChapterReferences(t *testing.T) {
	resetDatabase(t)
	user := createUser(t, "UTC")
	book := createBook(t, user.ID, "Reprocessed TXT")
	original := []byte("Reprocess title\n\nChapter 1\nFirst body.\n\nChapter 2\nSecond body.")

	_, err := integrationDB.NewUpdate().Model((*model.Book)(nil)).
		Set("status='queued'").
		Set("original_size=?", len(original)).
		Where("id=?", book.ID).
		Exec(testContext(t))
	require.NoError(t, err)

	store := storage.NewMemoryStore()
	require.NoError(t, store.Put(
		testContext(t),
		book.OriginalBucket,
		book.OriginalKey,
		bytes.NewReader(original),
		int64(len(original)),
		"text/plain",
		nil,
	))

	limits := bookprocessing.ArchiveLimits{
		MaxFiles:            100,
		MaxUnpackedBytes:    10 * 1024 * 1024,
		MaxCompressionRatio: 100,
		MaxEntryBytes:       10 * 1024 * 1024,
	}
	cfg := config.Config{
		Upload: config.Upload{MaxBytes: 10 * 1024 * 1024},
		S3: config.S3{
			ContentBucket: "books-content",
			AssetsBucket:  "books-assets",
			CoversBucket:  "books-covers",
		},
	}
	registry := bookprocessing.NewRegistry(limits)
	processor := books.NewProcessor(integrationDB, store, registry, cfg)
	require.NoError(t, processor.Process(testContext(t), books.ProcessPayload{
		BookID:  book.ID,
		UserID:  user.ID,
		Version: 1,
	}))

	var oldChapter model.BookChapter
	require.NoError(t, integrationDB.NewSelect().Model(&oldChapter).
		Where("book_id=? AND version=1", book.ID).
		Order("ordinal ASC").
		Limit(1).
		Scan(testContext(t)))

	now := time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC)
	oldChapterID := oldChapter.ID
	progress := model.ReadingProgress{
		ID:              uuid.New(),
		UserID:          user.ID,
		BookID:          book.ID,
		ChapterID:       &oldChapterID,
		LocatorType:     "chapter_offset",
		Locator:         json.RawMessage(`{"offset":42}`),
		CharacterOffset: 42,
		TextAnchor:      "durable anchor",
		ChapterProgress: 25,
		ProgressPercent: 12.5,
		ScrollPercent:   25,
		Revision:        7,
		ClientID:        "before-reprocess",
		UpdatedAt:       now,
	}
	bookmark := model.Bookmark{
		ID:              uuid.New(),
		UserID:          user.ID,
		BookID:          book.ID,
		ChapterID:       &oldChapterID,
		Locator:         json.RawMessage(`{"offset":42}`),
		ProgressPercent: 12.5,
		Title:           "Durable bookmark",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	highlight := model.Highlight{
		ID:           uuid.New(),
		UserID:       user.ID,
		BookID:       book.ID,
		ChapterID:    &oldChapterID,
		Locator:      json.RawMessage(`{"start":40,"end":50}`),
		TextAnchor:   "durable anchor",
		SelectedText: "selected text",
		Color:        "blue",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	for _, item := range []any{&progress, &bookmark, &highlight} {
		_, err = integrationDB.NewInsert().Model(item).Exec(testContext(t))
		require.NoError(t, err)
	}

	dictionaryEntryID := uuid.New()
	_, err = integrationDB.ExecContext(testContext(t), `
INSERT INTO dictionary_entries(
  id, user_id, source_language, target_language,
  original_word, normalized_word, translation
) VALUES (?, ?, 'en', 'ru', 'durable', 'durable', 'надёжный')`, dictionaryEntryID, user.ID)
	require.NoError(t, err)
	occurrence := model.WordOccurrence{
		ID:                uuid.New(),
		DictionaryEntryID: dictionaryEntryID,
		BookID:            &book.ID,
		ChapterID:         &oldChapterID,
		Locator:           json.RawMessage(`{"offset":42}`),
		Sentence:          "A durable occurrence.",
		EncounteredAt:     now,
		CreatedAt:         now,
	}
	_, err = integrationDB.NewInsert().Model(&occurrence).Exec(testContext(t))
	require.NoError(t, err)

	_, err = integrationDB.NewUpdate().Model((*model.Book)(nil)).
		Set("processing_version=2").
		Set("status='queued'").
		Where("id=? AND processing_version=1", book.ID).
		Exec(testContext(t))
	require.NoError(t, err)
	require.NoError(t, processor.Process(testContext(t), books.ProcessPayload{
		BookID:  book.ID,
		UserID:  user.ID,
		Version: 2,
	}))

	var newChapter model.BookChapter
	require.NoError(t, integrationDB.NewSelect().Model(&newChapter).
		Where("book_id=? AND version=2 AND source_ref=?", book.ID, oldChapter.SourceRef).
		Scan(testContext(t)))
	require.NotEqual(t, oldChapter.ID, newChapter.ID)

	var remappedProgress model.ReadingProgress
	require.NoError(t, integrationDB.NewSelect().Model(&remappedProgress).
		Where("id=?", progress.ID).
		Scan(testContext(t)))
	require.NotNil(t, remappedProgress.ChapterID)
	require.Equal(t, newChapter.ID, *remappedProgress.ChapterID)
	require.Equal(t, int64(8), remappedProgress.Revision)
	require.JSONEq(t, string(progress.Locator), string(remappedProgress.Locator))
	require.Equal(t, progress.CharacterOffset, remappedProgress.CharacterOffset)
	require.Equal(t, progress.ProgressPercent, remappedProgress.ProgressPercent)
	require.True(t, remappedProgress.UpdatedAt.After(progress.UpdatedAt))

	assertRemappedChapter := func(table string, id uuid.UUID) {
		t.Helper()
		var chapterID uuid.UUID
		err := integrationDB.NewSelect().Table(table).
			Column("chapter_id").
			Where("id=?", id).
			Scan(testContext(t), &chapterID)
		require.NoError(t, err)
		require.Equal(t, newChapter.ID, chapterID)
	}
	assertRemappedChapter("bookmarks", bookmark.ID)
	assertRemappedChapter("highlights", highlight.ID)
	assertRemappedChapter("word_occurrences", occurrence.ID)

	current, err := books.NewService(integrationDB, store, registry, cfg).
		Chapter(testContext(t), user.ID, book.ID, newChapter.ID)
	require.NoError(t, err)
	require.Equal(t, newChapter.ID, current.ID)

	// A repeated delivery is a no-op and must not advance the optimistic-lock
	// revision a second time.
	require.NoError(t, processor.Process(testContext(t), books.ProcessPayload{
		BookID:  book.ID,
		UserID:  user.ID,
		Version: 2,
	}))
	var revision int64
	require.NoError(t, integrationDB.NewSelect().Table("reading_progress").
		Column("revision").
		Where("id=?", progress.ID).
		Scan(testContext(t), &revision))
	require.Equal(t, int64(8), revision)
}
