package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const defaultEmbeddingDim = 768

type Config struct {
	DatabaseURL       string
	OllamaURL         string
	OllamaEmbedModel  string
	ProjectDefault    string
	LogLevel          slog.Level
	ExpectedEmbedding int
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:       strings.TrimSpace(os.Getenv("DATABASE_URL")),
		OllamaURL:         envOrDefault("OLLAMA_URL", "http://ollama:11434"),
		OllamaEmbedModel:  envOrDefault("OLLAMA_EMBED_MODEL", "nomic-embed-text"),
		ProjectDefault:    strings.TrimSpace(os.Getenv("MCP_PROJECT_DEFAULT")),
		LogLevel:          parseLevel(envOrDefault("LOG_LEVEL", "info")),
		ExpectedEmbedding: defaultEmbeddingDim,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	if cfg.OllamaURL == "" {
		return Config{}, errors.New("OLLAMA_URL cannot be empty")
	}

	if cfg.OllamaEmbedModel == "" {
		return Config{}, errors.New("OLLAMA_EMBED_MODEL cannot be empty")
	}

	return cfg, nil
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func (c Config) Redacted() string {
	return fmt.Sprintf("ollama_url=%s ollama_model=%s project_default=%s log_level=%s embedding_dim=%d",
		c.OllamaURL,
		c.OllamaEmbedModel,
		c.ProjectDefault,
		c.LogLevel.String(),
		c.ExpectedEmbedding,
	)
}
