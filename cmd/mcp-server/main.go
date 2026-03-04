package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"luck-mpc/internal/config"
	"luck-mpc/internal/db"
	"luck-mpc/internal/embeddings"
	"luck-mpc/internal/repository"
	"luck-mpc/internal/service"
	"luck-mpc/internal/transport/mcp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	database, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer database.Close()

	repo := repository.NewPostgresDocumentRepository(database)
	embedClient := embeddings.NewOllamaClient(cfg.OllamaURL, cfg.OllamaEmbedModel, 15*time.Second, logger)
	ctxService := service.NewContextService(repo, embedClient, cfg.ProjectDefault, cfg.ExpectedEmbedding, logger)
	server := mcp.NewServer(ctxService, logger, "context-memory-mcp", "0.1.0", os.Stdin, os.Stdout)

	logger.Info("starting mcp server", slog.String("config", cfg.Redacted()))
	if err := server.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("mcp server stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("mcp server stopped")
}
