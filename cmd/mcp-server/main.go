package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"luck-mcp/internal/config"
	"luck-mcp/internal/db"
	"luck-mcp/internal/embeddings"
	"luck-mcp/internal/indexer"
	"luck-mcp/internal/repository"
	"luck-mcp/internal/service"
	"luck-mcp/internal/transport/mcp"
	"luck-mcp/migrations"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "index" {
		runIndexer(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrations()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "health" {
		runHealth()
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

	if _, err := ensureSchema(ctx, database, logger); err != nil {
		logger.Error("failed to ensure schema", slog.String("error", err.Error()))
		os.Exit(1)
	}

	repo := repository.NewPostgresDocumentRepository(database)
	embedClient := embeddings.NewOllamaClient(cfg.OllamaURL, cfg.OllamaEmbedModel, 15*time.Second, logger)
	ctxService := service.NewContextService(repo, embedClient, cfg.ProjectDefault, cfg.ExpectedEmbedding, logger)
	codebaseService := service.NewCodebaseService(repo, embedClient, cfg.ExpectedEmbedding, logger)
	server := mcp.NewServer(ctxService, codebaseService, logger, "context-memory-mcp", "0.2.0", os.Stdin, os.Stdout)

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

	if _, err := ensureSchema(ctx, database, logger); err != nil {
		logger.Error("failed to ensure schema", slog.String("error", err.Error()))
		os.Exit(1)
	}

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

func runMigrations() {
	cfg, logger := mustLoadConfigAndLogger()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	database, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer database.Close()

	result, err := ensureSchema(ctx, database, logger)
	if err != nil {
		logger.Error("migration command finished with error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("migration command finished",
		slog.Int("applied", len(result.Applied)),
		slog.Int("skipped", result.Skipped),
	)
}

func runHealth() {
	cfg, logger := mustLoadConfigAndLogger()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintln(os.Stdout, "luck-mcp health check")
	allGood := true

	database, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		printHealthFailure("Postgres", fmt.Sprintf("nao foi possivel conectar no banco: %v", err), "Rode: docker compose up -d postgres")
		allGood = false
	} else {
		defer database.Close()
		printHealthOK("Postgres", "conexao com o banco OK")

		quietLogger := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
		result, schemaErr := ensureSchema(ctx, database, quietLogger)
		if schemaErr != nil {
			printHealthFailure("Schema", fmt.Sprintf("falha ao alinhar migrations: %v", schemaErr), "Rode: make migrate")
			allGood = false
		} else if len(result.Applied) > 0 {
			printHealthOK("Schema", fmt.Sprintf("schema alinhado; migrations aplicadas agora: %d", len(result.Applied)))
		} else {
			printHealthOK("Schema", "schema alinhado; nenhuma migration pendente")
		}
	}

	embedClient := embeddings.NewOllamaClient(cfg.OllamaURL, cfg.OllamaEmbedModel, 20*time.Second, logger)
	embedding, err := embedClient.Embed(ctx, "healthcheck")
	if err != nil {
		printHealthFailure("Ollama", fmt.Sprintf("modelo %q indisponivel ou sem resposta: %v", cfg.OllamaEmbedModel, err), fmt.Sprintf("Rode: docker compose up -d ollama && docker compose exec ollama ollama pull %s", cfg.OllamaEmbedModel))
		allGood = false
	} else if len(embedding) != cfg.ExpectedEmbedding {
		printHealthFailure("Ollama", fmt.Sprintf("embedding com dimensao %d, esperado %d", len(embedding), cfg.ExpectedEmbedding), "Verifique modelo/configuracao do Ollama")
		allGood = false
	} else {
		printHealthOK("Ollama", fmt.Sprintf("modelo %q respondeu com %d dimensoes", cfg.OllamaEmbedModel, len(embedding)))
	}

	if allGood {
		printHealthOK("MCP", "ambiente pronto para uso diario")
		fmt.Fprintln(os.Stdout, "Proximo passo: make index PROJECT=meu-projeto ROOT=/caminho/absoluto/do/repo")
		return
	}

	printHealthFailure("Resumo", "existem pendencias antes do uso diario", "Corrija os itens acima e rode: make health")
	os.Exit(1)
}

func printHealthOK(component, detail string) {
	fmt.Fprintf(os.Stdout, "[ok] %s: %s\n", component, detail)
}

func printHealthFailure(component, detail, next string) {
	fmt.Fprintf(os.Stdout, "[fail] %s: %s\n", component, detail)
	if next != "" {
		fmt.Fprintf(os.Stdout, "       Proximo passo: %s\n", next)
	}
}

func ensureSchema(ctx context.Context, database *sql.DB, logger *slog.Logger) (migrations.Result, error) {
	runner := migrations.NewRunner(database, logger)
	return runner.Run(ctx)
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
  mcp-server health       # run readiness checks for DB/schema/Ollama
  mcp-server migrate      # apply pending database migrations
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
