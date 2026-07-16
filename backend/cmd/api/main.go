package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/XRS0/reader/backend/internal/app"
	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/httpapi"
	"github.com/XRS0/reader/backend/internal/observability"
)

func main() {
	logger := observability.NewLogger(os.Getenv("APP_ENV"))
	if err := run(logger); err != nil {
		logger.Error("api stopped", "error", err)
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
	handler := httpapi.New(cfg, components.DB, components.Services, components.Metrics, logger)
	server := &http.Server{Addr: cfg.HTTP.Addr, Handler: handler, ReadTimeout: cfg.HTTP.ReadTimeout, ReadHeaderTimeout: cfg.HTTP.ReadTimeout, WriteTimeout: cfg.HTTP.WriteTimeout, IdleTimeout: cfg.HTTP.IdleTimeout, MaxHeaderBytes: 1 << 20}
	errorsCh := make(chan error, 1)
	go func() { logger.Info("api listening", "addr", cfg.HTTP.Addr); errorsCh <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err = <-errorsCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-errorsCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
