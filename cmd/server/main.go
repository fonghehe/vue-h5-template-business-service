// Command server starts the vue-h5-template business service.
//
// It owns process lifecycle only: load configuration, open dependencies, serve
// HTTP, and shut down cleanly on SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fonghehe/vue-h5-template-business-service/internal/config"
	"github.com/fonghehe/vue-h5-template-business-service/internal/database"
	"github.com/fonghehe/vue-h5-template-business-service/internal/httpapi"
	"github.com/fonghehe/vue-h5-template-business-service/internal/logging"
)

// version is stamped in at build time via -ldflags.
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	logger := logging.New(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(logger)

	db, err := database.Open(database.OptionsFromConfig(cfg))
	if err != nil {
		return err
	}

	server := httpapi.New(cfg, db, logger)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Router(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	// Serve in the background so the main goroutine can own signal handling.
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("starting business service",
			"version", version,
			"env", cfg.AppEnv,
			"addr", httpServer.Addr,
			"driver", cfg.DatabaseDriver,
		)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)
	case sig := <-shutdown:
		logger.Info("shutdown signal received", "signal", sig.String())

		// Drain in-flight requests, then close the database pool.
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			_ = httpServer.Close()
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		logger.Info("server stopped")
	}

	return nil
}
