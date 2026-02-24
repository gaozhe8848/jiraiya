package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"jiraiya/sql/schema"
)

func main() {
	godotenv.Load()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Wait for database to be ready
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		log.Info("waiting for database...")
		time.Sleep(time.Second)
	}
	if err := pool.Ping(ctx); err != nil {
		log.Error("database not ready after 30s", "error", err)
		os.Exit(1)
	}

	if _, err := pool.Exec(ctx, schema.InitSQL); err != nil {
		log.Error("migration 001 failed", "error", err)
		os.Exit(1)
	}

	if _, err := pool.Exec(ctx, schema.LtreeSQL); err != nil {
		log.Error("migration 002 failed", "error", err)
		os.Exit(1)
	}

	log.Info("migrations applied")
}
