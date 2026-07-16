package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/XRS0/reader/backend/internal/app"
	"github.com/XRS0/reader/backend/internal/books"
	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/jobs"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/XRS0/reader/backend/internal/observability"
)

func main() {
	logger := observability.NewLogger(os.Getenv("APP_ENV"))
	if err := run(logger); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}
func run(logger *slog.Logger) (runErr error) {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	shutdownTracing, err := observability.SetupTracing(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		traceCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(traceCtx)
	}()
	components, err := app.Build(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		runErr = errors.Join(runErr, components.DB.Close())
	}()
	queue := jobs.NewQueue(components.DB)
	worker := jobs.NewWorker(queue, cfg.Worker.ID, cfg.Worker.Concurrency, cfg.Worker.PollInterval, cfg.Worker.JobTimeout, cfg.Worker.StaleLockAfter, logger)
	worker.Handle(books.JobProcessBook, func(ctx context.Context, job model.Job) error {
		payload, err := jobs.DecodePayload[books.ProcessPayload](job)
		if err != nil {
			components.Metrics.WorkerJobs.WithLabelValues(job.Type, "error").Inc()
			return err
		}
		err = components.BooksProcessor.Process(ctx, payload)
		result := "ok"
		if err != nil {
			result = "error"
		}
		components.Metrics.WorkerJobs.WithLabelValues(job.Type, result).Inc()
		return err
	})
	worker.Handle(books.JobCleanupBook, func(ctx context.Context, job model.Job) error {
		payload, err := jobs.DecodePayload[books.CleanupPayload](job)
		if err != nil {
			components.Metrics.WorkerJobs.WithLabelValues(job.Type, "error").Inc()
			return err
		}
		err = components.BooksProcessor.Cleanup(ctx, payload)
		result := "ok"
		if err != nil {
			result = "error"
		}
		components.Metrics.WorkerJobs.WithLabelValues(job.Type, result).Inc()
		return err
	})
	go maintenance(ctx, queue, components, cfg, logger)
	logger.Info("worker started", "worker_id", cfg.Worker.ID, "concurrency", cfg.Worker.Concurrency)
	return worker.Run(ctx)
}
func maintenance(ctx context.Context, queue *jobs.Queue, c *app.Components, cfg config.Config, logger *slog.Logger) {
	leaseTick := time.NewTicker(maxDuration(time.Minute, cfg.Worker.StaleLockAfter/2))
	sessionTick := time.NewTicker(time.Minute)
	statsTick := time.NewTicker(5 * time.Minute)
	cacheTick := time.NewTicker(time.Hour)
	defer leaseTick.Stop()
	defer sessionTick.Stop()
	defer statsTick.Stop()
	defer cacheTick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-leaseTick.C:
			if n, err := queue.RecoverStale(ctx, cfg.Worker.StaleLockAfter); err != nil {
				logger.Error("recover stale jobs", "error", err)
			} else if n > 0 {
				logger.Warn("recovered stale jobs", "count", n)
			}
			if depth, err := queue.Depth(ctx); err == nil {
				c.Metrics.QueueDepth.Set(float64(depth))
			}
		case <-sessionTick.C:
			if n, err := c.Sessions.FinalizeStale(ctx); err != nil {
				logger.Error("finalize stale reading sessions", "error", err)
			} else if n > 0 {
				logger.Info("finalized stale reading sessions", "count", n)
			}
			var active int64
			if err := c.DB.NewSelect().Table("reading_sessions").ColumnExpr("count(*)").Where("status IN ('active','idle')").Scan(ctx, &active); err == nil {
				c.Metrics.ActiveSessions.Set(float64(active))
			}
		case <-statsTick.C:
			if err := c.Statistics.Recompute(ctx); err != nil {
				logger.Error("recompute reading statistics", "error", err)
			}
		case <-cacheTick.C:
			if n, err := c.Statistics.CleanupExpiredTranslations(ctx); err != nil {
				logger.Error("clean translation cache", "error", err)
			} else {
				logger.Info("cleaned translation cache", "count", n)
			}
		}
	}
}
func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
