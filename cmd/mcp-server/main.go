package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"luck-mpc/internal/config"
	"luck-mpc/internal/db"
	"luck-mpc/internal/embeddings"
	"luck-mpc/internal/indexer"
	"luck-mpc/internal/repository"
	"luck-mpc/internal/service"
	"luck-mpc/internal/transport/mcp"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "index" {
		runIndexer(os.Args[2:])
		return
	}
	runServer()
}

func runServer() {
	cfg, logger := mustLoadConfigAndLogger()

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

func runIndexer(args []string) {
	cfg, logger := mustLoadConfigAndLogger()

	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	project := fs.String("project", cfg.ProjectDefault, "project namespace (required if MCP_PROJECT_DEFAULT is empty)")
	root := fs.String("root", ".", "absolute or relative path to project root")
	mode := fs.String("mode", "changed", "index mode: changed|full")
	chunkSize := fs.Int("chunk-size", 1600, "chunk size in characters")
	chunkOverlap := fs.Int("chunk-overlap", 200, "chunk overlap in characters")
	maxFileKB := fs.Int64("max-file-kb", 1024, "max file size in KB")

	if err := fs.Parse(args); err != nil {
		logger.Error("failed to parse index flags", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	database, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer database.Close()

	repo := repository.NewPostgresDocumentRepository(database)
	embedClient := embeddings.NewOllamaClient(cfg.OllamaURL, cfg.OllamaEmbedModel, 30*time.Second, logger)
	indexService := indexer.NewService(repo, embedClient, cfg.ExpectedEmbedding, logger)

	result, err := indexService.IndexProject(ctx, indexer.Options{
		Project:      *project,
		RootPath:     *root,
		Mode:         *mode,
		ChunkSize:    *chunkSize,
		ChunkOverlap: *chunkOverlap,
		MaxFileBytes: (*maxFileKB) * 1024,
	})
	if err != nil {
		logger.Error("index command finished with error",
			slog.String("error", err.Error()),
			slog.String("project", *project),
			slog.String("root", *root),
			slog.String("mode", *mode),
			slog.Int("scanned_files", result.ScannedFiles),
			slog.Int("indexed_files", result.IndexedFiles),
			slog.Int("unchanged_files", result.UnchangedFiles),
			slog.Int("deleted_files", result.DeletedFiles),
			slog.Int("failed_files", result.FailedFiles),
		)
		os.Exit(1)
	}

	logger.Info("index command finished",
		slog.String("project", *project),
		slog.String("root", *root),
		slog.String("mode", *mode),
		slog.Int("scanned_files", result.ScannedFiles),
		slog.Int("skipped_files", result.SkippedFiles),
		slog.Int("indexed_files", result.IndexedFiles),
		slog.Int("unchanged_files", result.UnchangedFiles),
		slog.Int("deleted_files", result.DeletedFiles),
		slog.Int("chunks_added", result.ChunksAdded),
	)
}

func mustLoadConfigAndLogger() (config.Config, *slog.Logger) {
	cfg, err := config.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)
	logger.Debug("config loaded", slog.String("config", cfg.Redacted()))
	return cfg, logger
}

func usage() string {
	return `Usage:
  mcp-server              # run MCP server in stdio mode
  mcp-server index [flags]

Index flags:
  --project string        project namespace (required if MCP_PROJECT_DEFAULT empty)
  --root string           project root path (default ".")
  --mode string           changed|full (default "changed")
  --chunk-size int        chunk size in chars (default 1600)
  --chunk-overlap int     chunk overlap in chars (default 200)
  --max-file-kb int       max file size in KB (default 1024)
`
}

func init() {
	flag.Usage = func() {
		_, _ = fmt.Fprint(os.Stderr, usage())
	}
}
