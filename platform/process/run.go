package process

import (
	"context"
	"errors"
	"llamarig/config"
	"llamarig/platform/audit"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func Run(
	ctx context.Context,
	logger *zap.Logger,
	name string,
	cfg config.Config,
	run func() error,
	shutdown func(context.Context) error,
) {
	errs := make(chan error, 1)
	go func() { errs <- run() }()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	cleanupArchives(logger, cfg.LogArchiveRetention)
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cleanupArchives(logger, cfg.LogArchiveRetention)
		case <-ctx.Done():
			logger.Info("stopping "+name, zap.Error(ctx.Err()))
			ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout(cfg))
			defer cancel()
			if err := shutdown(ctx); err != nil {
				logger.Fatal("shutdown "+name, zap.Error(err))
			}
			return
		case err := <-errs:
			if !errors.Is(err, http.ErrServerClosed) {
				ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout(cfg))
				defer cancel()
				_ = shutdown(ctx)
				logger.Fatal("serve "+name, zap.Error(err))
			}
			return
		}
	}
}

func cleanupArchives(logger *zap.Logger, retention time.Duration) {
	if _, err := audit.CleanupArchives(retention, time.Now()); err != nil {
		logger.Warn("clean log archives", zap.Error(err))
	}
}

func shutdownTimeout(cfg config.Config) time.Duration {
	timeout := 5 * time.Second
	if cfg.Router.StopTimeout >= timeout {
		return cfg.Router.StopTimeout + time.Second
	}
	return timeout
}
