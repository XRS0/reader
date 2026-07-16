package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/XRS0/reader/backend/internal/annotations"
	"github.com/XRS0/reader/backend/internal/auth"
	"github.com/XRS0/reader/backend/internal/bookprocessing"
	"github.com/XRS0/reader/backend/internal/books"
	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/database"
	"github.com/XRS0/reader/backend/internal/dictionary"
	"github.com/XRS0/reader/backend/internal/httpapi"
	"github.com/XRS0/reader/backend/internal/observability"
	"github.com/XRS0/reader/backend/internal/preferences"
	"github.com/XRS0/reader/backend/internal/reading"
	"github.com/XRS0/reader/backend/internal/statistics"
	"github.com/XRS0/reader/backend/internal/storage"
	"github.com/XRS0/reader/backend/internal/translation"
	"github.com/uptrace/bun"
)

type Components struct {
	DB             *bun.DB
	Store          storage.ObjectStore
	Registry       *bookprocessing.Registry
	Services       httpapi.Services
	Metrics        *observability.Metrics
	BooksProcessor *books.Processor
	Statistics     *statistics.Service
	Sessions       *reading.SessionService
}

func Build(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Components, error) {
	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}
	metrics := observability.NewMetrics()
	db.AddQueryHook(observability.BunHook{Metrics: metrics})
	store, err := storage.NewS3(ctx, cfg.S3)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize RustFS adapter: %w", err)
	}
	if err = store.EnsureBuckets(ctx, cfg.S3.OriginalBucket, cfg.S3.ContentBucket, cfg.S3.AssetsBucket, cfg.S3.CoversBucket, cfg.S3.ExportsBucket); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ensure RustFS buckets: %w", err)
	}
	observedStore := storage.NewObserved(store, func(operation, result string, elapsed time.Duration) {
		metrics.StorageDuration.WithLabelValues(operation, result).Observe(elapsed.Seconds())
	})
	registry := bookprocessing.NewRegistry(bookprocessing.ArchiveLimits{MaxFiles: cfg.Upload.MaxEPUBFiles, MaxUnpackedBytes: cfg.Upload.MaxEPUBUnpackedBytes, MaxCompressionRatio: cfg.Upload.MaxCompressionRatio, MaxEntryBytes: cfg.Upload.MaxBytes})
	hasher := auth.NewPasswordHasher(cfg.JWT.ArgonMemory, cfg.JWT.ArgonTime, cfg.JWT.ArgonThreads)
	tokens := auth.NewTokenManagerWithAudience(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.Audience, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	authService := auth.NewService(db, hasher, tokens)
	bookService := books.NewService(db, observedStore, registry, cfg)
	progressService := reading.NewProgressService(db)
	sessionService := reading.NewSessionService(db, cfg.Reading)
	preferencesService := preferences.NewService(db)
	var provider translation.Provider
	switch cfg.Translation.Provider {
	case "mock":
		provider = translation.NewMockProvider(cfg.Translation.ProviderModel)
	case "openai":
		provider, err = translation.NewOpenAIProvider(cfg.Translation.Endpoint, cfg.Translation.APIKey, cfg.Translation.ProviderModel, cfg.Translation.RequestTimeout, cfg.Translation.MaxRetries)
		if err != nil {
			_ = db.Close()
			return nil, err
		}
	default:
		_ = db.Close()
		return nil, fmt.Errorf("unsupported TRANSLATION_PROVIDER %q", cfg.Translation.Provider)
	}
	translationService := translation.NewService(provider, translation.NewPostgresCache(db, cfg.Translation.PromptVersion), cfg.Translation)
	statisticsService := statistics.NewService(db)
	services := httpapi.Services{Auth: authService, Tokens: tokens, Books: bookService, Progress: progressService, Sessions: sessionService, Preferences: preferencesService, Translations: translationService, Dictionary: dictionary.NewService(db), Annotations: annotations.NewService(db), Statistics: statisticsService, Storage: observedStore}
	return &Components{DB: db, Store: observedStore, Registry: registry, Services: services, Metrics: metrics, BooksProcessor: books.NewProcessor(db, observedStore, registry, cfg), Statistics: statisticsService, Sessions: sessionService}, nil
}
