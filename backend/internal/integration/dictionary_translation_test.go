//go:build integration

package integration_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/dictionary"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/XRS0/reader/backend/internal/translation"
	"github.com/stretchr/testify/require"
)

func TestDictionaryDeduplicationOccurrencesAndOwnership(t *testing.T) {
	resetDatabase(t)
	owner := createUser(t, "UTC")
	other := createUser(t, "UTC")
	book := createBook(t, owner.ID, "Dictionary contexts")
	chapter := createChapter(t, book.ID, 0)
	foreignBook := createBook(t, other.ID, "Foreign dictionary context")
	foreignChapter := createChapter(t, foreignBook.ID, 0)
	service := dictionary.NewService(integrationDB)

	first, err := service.Create(testContext(t), owner.ID, dictionary.CreateInput{
		SourceLanguage: "EN",
		TargetLanguage: "RU",
		OriginalWord:   "  Hello  World ",
		Lemma:          "hello world",
		Translation:    "привет, мир",
		Status:         "learning",
		Occurrence: &dictionary.OccurrenceInput{
			BookID:        &book.ID,
			ChapterID:     &chapter.ID,
			Locator:       json.RawMessage(`{"offset":20}`),
			Sentence:      "Hello world, said the reader.",
			ContextBefore: "Before",
			ContextAfter:  "After",
			EncounteredAt: time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC),
		},
	})
	require.NoError(t, err)
	require.Equal(t, "hello world", first.NormalizedWord)
	require.Equal(t, 1, first.EncounterCount)

	second, err := service.Create(testContext(t), owner.ID, dictionary.CreateInput{
		SourceLanguage: "en",
		TargetLanguage: "ru",
		OriginalWord:   "HELLO\tWORLD",
		Translation:    "дубликат не создаётся",
		Occurrence: &dictionary.OccurrenceInput{
			BookID:        &book.ID,
			ChapterID:     &chapter.ID,
			Locator:       json.RawMessage(`{"offset":120}`),
			Sentence:      "The second hello world occurrence.",
			EncounteredAt: time.Date(2025, 1, 3, 10, 0, 0, 0, time.UTC),
		},
	})
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, 2, second.EncounterCount)
	require.Equal(t, "привет, мир", second.Translation, "deduplication must preserve the existing user's translation")

	items, total, err := service.List(testContext(t), owner.ID, "hello", "", "", 50, 0)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, first.ID, items[0].ID)

	occurrences, err := service.Occurrences(testContext(t), owner.ID, first.ID, 50, 0)
	require.NoError(t, err)
	require.Len(t, occurrences, 2)
	require.Equal(t, "The second hello world occurrence.", occurrences[0].Sentence)
	require.Equal(t, "Hello world, said the reader.", occurrences[1].Sentence)

	otherEntry, err := service.Create(testContext(t), other.ID, dictionary.CreateInput{
		SourceLanguage: "en",
		TargetLanguage: "ru",
		OriginalWord:   "hello world",
		Translation:    "другой пользователь",
	})
	require.NoError(t, err)
	require.NotEqual(t, first.ID, otherEntry.ID)

	_, err = service.AddOccurrence(testContext(t), other.ID, first.ID, dictionary.OccurrenceInput{
		Locator:  json.RawMessage(`{}`),
		Sentence: "Attempt to mutate a foreign entry.",
	})
	require.ErrorIs(t, err, dictionary.ErrNotFound)

	_, err = service.AddOccurrence(testContext(t), owner.ID, first.ID, dictionary.OccurrenceInput{
		BookID:    &book.ID,
		ChapterID: &foreignChapter.ID,
		Locator:   json.RawMessage(`{"offset":999}`),
		Sentence:  "A foreign chapter must not be accepted.",
	})
	require.Error(t, err, "an occurrence chapter must belong to the selected owned book")

	count, err := integrationDB.NewSelect().Model((*model.DictionaryEntry)(nil)).
		Where("user_id=? AND normalized_word='hello world'", owner.ID).
		Count(testContext(t))
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestTranslationPostgresCacheAndSingleFlight(t *testing.T) {
	resetDatabase(t)
	provider := translation.NewMockProvider("mock-cache-v1")
	cfg := config.Translation{
		MaxWordRunes:   128,
		MaxTextRunes:   2000,
		CacheTTL:       time.Hour,
		ProviderModel:  "mock-cache-v1",
		PromptVersion:  "prompt-v1",
		RequestTimeout: time.Second,
	}
	cache := translation.NewPostgresCache(integrationDB, cfg.PromptVersion)
	service := translation.NewService(provider, cache, cfg)
	request := translation.Request{
		SourceLanguage: "EN",
		TargetLanguage: "RU",
		Text:           " Hello ",
	}

	first, err := service.TranslateWord(testContext(t), request)
	require.NoError(t, err)
	require.Equal(t, "привет", first.Translation)
	require.False(t, first.Cached)

	request.Text = "hello"
	second, err := service.TranslateWord(testContext(t), request)
	require.NoError(t, err)
	require.Equal(t, first.Translation, second.Translation)
	require.True(t, second.Cached)
	require.Equal(t, 1, provider.Calls)

	var cached model.TranslationCache
	err = integrationDB.NewSelect().Model(&cached).Scan(testContext(t))
	require.NoError(t, err)
	require.Equal(t, "word", cached.RequestType)
	require.Equal(t, "mock", cached.Provider)
	require.Equal(t, "mock-cache-v1", cached.ProviderModel)
	require.Equal(t, "prompt-v1", cached.PromptVersion)
	require.Equal(t, int64(2), cached.UseCount)
	require.True(t, cached.ExpiresAt.After(cached.CreatedAt))

	concurrentRequest := translation.Request{SourceLanguage: "en", TargetLanguage: "ru", Text: "world"}
	const callers = 8
	start := make(chan struct{})
	errorsByCaller := make(chan error, callers)
	results := make(chan translation.WordResult, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, translateErr := service.TranslateWord(testContext(t), concurrentRequest)
			results <- result
			errorsByCaller <- translateErr
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errorsByCaller)
	for translateErr := range errorsByCaller {
		require.NoError(t, translateErr)
	}
	for result := range results {
		require.Equal(t, "мир", result.Translation)
	}
	require.Equal(t, 2, provider.Calls, "all concurrent cache misses for one key must collapse into one provider request")

	newPromptConfig := config.Translation{
		MaxWordRunes:   cfg.MaxWordRunes,
		MaxTextRunes:   cfg.MaxTextRunes,
		CacheTTL:       cfg.CacheTTL,
		ProviderModel:  cfg.ProviderModel,
		PromptVersion:  "prompt-v2",
		RequestTimeout: cfg.RequestTimeout,
	}
	serviceWithNewPrompt := translation.NewService(
		provider,
		translation.NewPostgresCache(integrationDB, newPromptConfig.PromptVersion),
		newPromptConfig,
	)
	newPromptResult, err := serviceWithNewPrompt.TranslateWord(testContext(t), translation.Request{
		SourceLanguage: "en",
		TargetLanguage: "ru",
		Text:           "hello",
	})
	require.NoError(t, err)
	require.False(t, newPromptResult.Cached)
	require.Equal(t, 3, provider.Calls, "prompt version is part of the cache key")

	count, err := integrationDB.NewSelect().Model((*model.TranslationCache)(nil)).Count(testContext(t))
	require.NoError(t, err)
	require.Equal(t, 3, count)
	v2Count, err := integrationDB.NewSelect().Model((*model.TranslationCache)(nil)).
		Where("prompt_version=?", newPromptConfig.PromptVersion).
		Count(testContext(t))
	require.NoError(t, err)
	require.Equal(t, 1, v2Count)
}
