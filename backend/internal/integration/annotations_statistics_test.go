//go:build integration

package integration_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/XRS0/reader/backend/internal/annotations"
	"github.com/XRS0/reader/backend/internal/dictionary"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/XRS0/reader/backend/internal/statistics"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAnnotationsEnforceOwnershipAndSanitizeContent(t *testing.T) {
	resetDatabase(t)
	owner := createUser(t, "UTC")
	other := createUser(t, "UTC")
	book := createBook(t, owner.ID, "Owned annotations")
	chapter := createChapter(t, book.ID, 0)
	foreignBook := createBook(t, other.ID, "Foreign annotations")
	foreignChapter := createChapter(t, foreignBook.ID, 0)
	service := annotations.NewService(integrationDB)

	_, err := service.ListBookmarks(testContext(t), owner.ID, foreignBook.ID)
	require.ErrorIs(t, err, annotations.ErrNotFound)

	_, err = service.CreateBookmark(testContext(t), owner.ID, book.ID, annotations.BookmarkInput{
		ChapterID:       &foreignChapter.ID,
		Locator:         json.RawMessage(`{"offset":5}`),
		ProgressPercent: 5,
		Title:           "Foreign chapter",
	})
	require.Error(t, err, "a bookmark chapter must belong to the selected owned book")

	bookmark, err := service.CreateBookmark(testContext(t), owner.ID, book.ID, annotations.BookmarkInput{
		ChapterID:       &chapter.ID,
		Locator:         json.RawMessage(`{"offset":50}`),
		ProgressPercent: 15,
		Title:           "  <script>alert(1)</script>Chapter marker  ",
		Note:            "<b>plain note</b>",
	})
	require.NoError(t, err)
	require.NotContains(t, bookmark.Title, "script")
	require.Equal(t, "plain note", bookmark.Note)

	foreignPatch := "stolen"
	_, err = service.PatchBookmark(testContext(t), other.ID, bookmark.ID, annotations.BookmarkPatch{Title: &foreignPatch})
	require.ErrorIs(t, err, annotations.ErrNotFound)
	require.ErrorIs(t, service.DeleteBookmark(testContext(t), other.ID, bookmark.ID), annotations.ErrNotFound)
	bookmarks, err := service.ListBookmarks(testContext(t), owner.ID, book.ID)
	require.NoError(t, err)
	require.Len(t, bookmarks, 1)
	require.Equal(t, bookmark.ID, bookmarks[0].ID)

	highlight, err := service.CreateHighlight(testContext(t), owner.ID, book.ID, annotations.HighlightInput{
		ChapterID:    &chapter.ID,
		Locator:      json.RawMessage(`{"start":10,"end":20}`),
		TextAnchor:   "anchor",
		SelectedText: "<em>Selected sentence</em>",
		Context:      "Safe context",
		Color:        "blue",
	})
	require.NoError(t, err)
	require.Equal(t, "Selected sentence", highlight.SelectedText)
	_, err = service.PatchHighlight(testContext(t), other.ID, highlight.ID, annotations.HighlightPatch{Note: &foreignPatch})
	require.ErrorIs(t, err, annotations.ErrNotFound)

	foreignHighlight, err := service.CreateHighlight(testContext(t), other.ID, foreignBook.ID, annotations.HighlightInput{
		ChapterID:    &foreignChapter.ID,
		Locator:      json.RawMessage(`{}`),
		SelectedText: "Foreign highlight",
		Color:        "blue",
	})
	require.NoError(t, err)
	_, err = service.CreateNote(testContext(t), owner.ID, annotations.NoteInput{
		BookID:        &book.ID,
		HighlightID:   &foreignHighlight.ID,
		Title:         "Must be rejected",
		SchemaVersion: 1,
		Blocks:        json.RawMessage(`[{"type":"text","text":"foreign child"}]`),
	})
	require.Error(t, err, "a note must not reference another user's highlight")

	note, err := service.CreateNote(testContext(t), owner.ID, annotations.NoteInput{
		BookID:        &book.ID,
		HighlightID:   &highlight.ID,
		Title:         " <strong>Reader note</strong> ",
		SchemaVersion: 1,
		Blocks: json.RawMessage(`[
          {"type":"heading2","text":"Important"},
          {"type":"text","text":"Read <script>alert(1)</script>carefully"},
			{"type":"link","text":"BookFlow","url":"https://example.test/bookflow"}
        ]`),
	})
	require.NoError(t, err)
	require.NotContains(t, string(note.Blocks), "<script")
	require.Contains(t, string(note.Blocks), "https://example.test/bookflow")

	patchedNote, err := service.PatchNote(testContext(t), owner.ID, note.ID, annotations.NotePatch{
		Blocks: json.RawMessage(`[{"type":"paragraph","text":"Updated PostgreSQL JSON block"}]`),
	})
	require.NoError(t, err)
	require.Contains(t, string(patchedNote.Blocks), "Updated PostgreSQL JSON block")

	_, err = service.CreateNote(testContext(t), owner.ID, annotations.NoteInput{
		BookID:        &book.ID,
		Title:         "Unsafe URL",
		SchemaVersion: 1,
		Blocks:        json.RawMessage(`[{"type":"link","text":"Unsafe link","url":"javascript:alert(document.cookie)"}]`),
	})
	require.Error(t, err, "dangerous URL schemes must be rejected")

	_, err = service.GetNote(testContext(t), other.ID, note.ID)
	require.ErrorIs(t, err, annotations.ErrNotFound)
}

func TestStatisticsAggregatesAreUserScopedAndRecomputable(t *testing.T) {
	resetDatabase(t)
	user := createUser(t, "Asia/Yekaterinburg")
	other := createUser(t, "UTC")
	firstBook := createBook(t, user.ID, "First statistics book")
	secondBook := createBook(t, user.ID, "Second statistics book")
	foreignBook := createBook(t, other.ID, "Foreign statistics book")

	insertProgress(t, user.ID, firstBook.ID, 80)
	insertProgress(t, user.ID, secondBook.ID, 100)
	insertFinishedSession(t, user.ID, firstBook.ID, time.Date(2025, 2, 1, 20, 30, 0, 0, time.UTC), "finished", 600, 120, 1000, 4)
	insertFinishedSession(t, user.ID, secondBook.ID, time.Date(2025, 2, 2, 20, 30, 0, 0, time.UTC), "stale", 300, 60, 500, 2)
	insertFinishedSession(t, other.ID, foreignBook.ID, time.Date(2025, 2, 2, 20, 30, 0, 0, time.UTC), "finished", 9999, 9999, 9999, 99)

	dictionaryService := dictionary.NewService(integrationDB)
	_, err := dictionaryService.Create(testContext(t), user.ID, dictionary.CreateInput{
		SourceLanguage: "en",
		TargetLanguage: "ru",
		OriginalWord:   "one",
		Translation:    "один",
		Status:         "mastered",
	})
	require.NoError(t, err)
	entry, err := dictionaryService.Create(testContext(t), user.ID, dictionary.CreateInput{
		SourceLanguage: "en",
		TargetLanguage: "ru",
		OriginalWord:   "two",
		Translation:    "два",
		Status:         "learning",
	})
	require.NoError(t, err)
	_, err = dictionaryService.AddOccurrence(testContext(t), user.ID, entry.ID, dictionary.OccurrenceInput{Sentence: "two again", Locator: json.RawMessage(`{}`)})
	require.NoError(t, err)

	service := statistics.NewService(integrationDB)
	overview, err := service.Overview(testContext(t), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(900), overview.ActiveSeconds)
	require.Equal(t, int64(180), overview.IdleSeconds)
	require.Equal(t, int64(2), overview.SessionCount)
	require.InDelta(t, 450, overview.AverageSessionSeconds, 0.001)
	require.Equal(t, int64(1500), overview.WordsRead)
	require.InDelta(t, 6, overview.PagesRead, 0.001)
	require.Equal(t, int64(2), overview.BooksTotal)
	require.Equal(t, int64(2), overview.BooksStarted)
	require.Equal(t, int64(1), overview.BooksCompleted)
	require.Equal(t, int64(2), overview.DictionaryWords)
	require.Equal(t, int64(1), overview.DictionaryMastered)
	rangeFrom := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	rangeTo := time.Date(2025, 2, 2, 12, 0, 0, 0, time.UTC)
	rangeOverview, err := service.OverviewRange(testContext(t), user.ID, rangeFrom, rangeTo)
	require.NoError(t, err)
	require.Equal(t, int64(600), rangeOverview.ActiveSeconds)
	require.Equal(t, int64(1), rangeOverview.SessionCount)
	require.Equal(t, int64(1000), rangeOverview.WordsRead)

	bookStats, err := service.Books(testContext(t), user.ID, 50, 0)
	require.NoError(t, err)
	require.Len(t, bookStats, 2)
	byBook := make(map[uuid.UUID]statistics.BookStat, len(bookStats))
	for _, item := range bookStats {
		byBook[item.BookID] = item
	}
	require.Equal(t, int64(600), byBook[firstBook.ID].ActiveSeconds)
	require.Equal(t, int64(1000), byBook[firstBook.ID].WordsRead)
	require.Equal(t, float64(80), byBook[firstBook.ID].ProgressPercent)
	require.Equal(t, int64(300), byBook[secondBook.ID].ActiveSeconds)
	require.Equal(t, float64(100), byBook[secondBook.ID].ProgressPercent)

	rangeBookStats, err := service.BooksRange(testContext(t), user.ID, rangeFrom, rangeTo, 50, 0)
	require.NoError(t, err)
	rangeByBook := make(map[uuid.UUID]statistics.BookStat, len(rangeBookStats))
	for _, item := range rangeBookStats {
		rangeByBook[item.BookID] = item
	}
	require.Equal(t, int64(600), rangeByBook[firstBook.ID].ActiveSeconds)
	require.Zero(t, rangeByBook[secondBook.ID].ActiveSeconds)

	buckets, err := service.Buckets(
		testContext(t),
		user.ID,
		time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 2, 4, 0, 0, 0, 0, time.UTC),
		"Asia/Yekaterinburg",
		"day",
	)
	require.NoError(t, err)
	require.Len(t, buckets, 2)
	require.Equal(t, int64(600), buckets[0].ActiveSeconds)
	require.Equal(t, int64(300), buckets[1].ActiveSeconds)

	streak, err := service.Streak(testContext(t), user.ID, "Asia/Yekaterinburg")
	require.NoError(t, err)
	require.Equal(t, 2, streak.Longest)
	require.Zero(t, streak.Current)
	require.Equal(t, []string{"2025-02-02", "2025-02-03"}, streak.Dates)

	dictionaryStats, err := service.Dictionary(testContext(t), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), dictionaryStats.Total)
	require.Equal(t, int64(1), dictionaryStats.Mastered)
	require.Equal(t, int64(1), dictionaryStats.Learning)
	require.Equal(t, int64(3), dictionaryStats.Encounters)

	for range 2 {
		require.NoError(t, service.Recompute(testContext(t)), "recomputation must be safe to repeat")
	}
	var dailyCount, bookCount int
	require.NoError(t, integrationDB.NewSelect().Table("daily_reading_stats").ColumnExpr("count(*)").Scan(testContext(t), &dailyCount))
	require.NoError(t, integrationDB.NewSelect().Table("book_reading_stats").ColumnExpr("count(*)").Scan(testContext(t), &bookCount))
	require.Equal(t, 3, dailyCount, "materialized aggregates include each user's local day")
	require.Equal(t, 3, bookCount, "materialized aggregates include each user/book pair")

	var active int64
	require.NoError(t, integrationDB.NewSelect().Table("book_reading_stats").
		Column("active_seconds").
		Where("user_id=? AND book_id=?", user.ID, firstBook.ID).
		Scan(testContext(t), &active))
	require.Equal(t, int64(600), active)
}

func insertProgress(t *testing.T, userID, bookID uuid.UUID, percent float64) {
	t.Helper()
	progress := model.ReadingProgress{
		ID:              uuid.New(),
		UserID:          userID,
		BookID:          bookID,
		LocatorType:     "character_offset",
		Locator:         json.RawMessage(`{"offset":100}`),
		ProgressPercent: percent,
		Revision:        1,
		ClientID:        "statistics-fixture",
		UpdatedAt:       time.Date(2025, 2, 3, 0, 0, 0, 0, time.UTC),
	}
	_, err := integrationDB.NewInsert().Model(&progress).Exec(testContext(t))
	require.NoError(t, err)
}

func insertFinishedSession(t *testing.T, userID, bookID uuid.UUID, startedAt time.Time, status string, active, idle, words int64, pages float64) {
	t.Helper()
	endedAt := startedAt.Add(time.Duration(active+idle) * time.Second)
	session := model.ReadingSession{
		ID:                   uuid.New(),
		UserID:               userID,
		BookID:               bookID,
		StartedAt:            startedAt,
		LastActivityAt:       endedAt,
		LastHeartbeatAt:      endedAt,
		EndedAt:              &endedAt,
		ActiveSeconds:        active,
		IdleSeconds:          idle,
		StartLocator:         json.RawMessage(`{"offset":0}`),
		EndLocator:           json.RawMessage(`{"offset":1000}`),
		StartProgressPercent: 0,
		EndProgressPercent:   50,
		CharactersRead:       words * 6,
		WordsReadEstimate:    words,
		PagesReadEstimate:    pages,
		CloseReason:          "user_closed_reader",
		Status:               status,
		LastSequence:         1,
		LastWasActive:        false,
		CreatedAt:            startedAt,
		UpdatedAt:            endedAt,
	}
	_, err := integrationDB.NewInsert().Model(&session).Exec(testContext(t))
	require.NoError(t, err)
}
