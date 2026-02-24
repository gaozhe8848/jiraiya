package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"jiraiya/internal/app"
	"jiraiya/internal/logger"
)

func main() {

	log := logger.New()

	cfg := app.Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Addr:        ":8080",
	}
	if cfg.DatabaseURL == "" {
		log.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.New(cfg, log).Run(ctx); err != nil {
		log.Error("application error", "error", err)
		os.Exit(1)
	}
}
