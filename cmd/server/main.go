package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"jiraiya/internal/app"
	"jiraiya/internal/logger"
)

func main() {
	godotenv.Load()

	log := logger.New()

	cfg := app.Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Addr:        os.Getenv("ADDR"),
	}
	if cfg.DatabaseURL == "" {
		log.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.New(cfg, log).Run(ctx); err != nil {
		log.Error("application error", "error", err)
		os.Exit(1)
	}
}
