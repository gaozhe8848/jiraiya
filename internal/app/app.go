package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"jiraiya/internal/handler"
	"jiraiya/internal/service"
)

// Config holds the application configuration.
type Config struct {
	DatabaseURL string
	Addr        string
}

// App orchestrates the full server lifecycle.
type App struct {
	cfg Config
	log *slog.Logger
}

// New creates a new App.
func New(cfg Config, log *slog.Logger) *App {
	return &App{cfg: cfg, log: log}
}

// Run starts the application and blocks until ctx is cancelled or the server
// fails. It returns nil on clean shutdown.
func (a *App) Run(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, a.cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}
	defer pool.Close()

	// Wait for database to be ready.
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		a.log.Info("waiting for database...")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database not ready after 30s: %w", err)
	}

	svc := service.New(pool, a.log)
	if err := svc.LoadTrees(ctx); err != nil {
		return fmt.Errorf("load trees: %w", err)
	}
	a.log.Info("trees loaded")

	h := handler.New(svc, a.log)
	srv := &http.Server{Addr: a.cfg.Addr, Handler: h.Routes()}

	errCh := make(chan error, 1)
	go func() {
		a.log.Info("server starting", "addr", a.cfg.Addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		a.log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	a.log.Info("server stopped")
	return nil
}
