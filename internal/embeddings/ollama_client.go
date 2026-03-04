package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Client interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

type OllamaClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
	logger     *slog.Logger
	retries    int
}

func NewOllamaClient(baseURL, model string, timeout time.Duration, logger *slog.Logger) *OllamaClient {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		trimmed = "http://ollama:11434"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &OllamaClient{
		baseURL: trimmed,
		model:   strings.TrimSpace(model),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger:  logger,
		retries: 2,
	}
}

func (c *OllamaClient) Embed(ctx context.Context, text string) ([]float64, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("text cannot be empty")
	}

	payload, err := json.Marshal(OllamaEmbeddingRequest{
		Model:  c.model,
		Prompt: text,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	endpoint := c.baseURL + "/api/embeddings"
	var lastErr error

	for attempt := 1; attempt <= c.retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("create embedding request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("ollama request failed: %w", err)
			if attempt < c.retries {
				continue
			}
			break
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read ollama response: %w", readErr)
			if attempt < c.retries {
				continue
			}
			break
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
			if attempt < c.retries {
				continue
			}
			break
		}

		var out OllamaEmbeddingResponse
		if err := json.Unmarshal(body, &out); err != nil {
			lastErr = fmt.Errorf("decode ollama response: %w", err)
			if attempt < c.retries {
				continue
			}
			break
		}

		embedding := out.Embedding
		if len(embedding) == 0 && len(out.Embeddings) > 0 {
			embedding = out.Embeddings[0]
		}
		if len(embedding) == 0 {
			lastErr = errors.New("ollama response has empty embedding")
			if attempt < c.retries {
				continue
			}
			break
		}

		c.logger.Debug("embedding generated",
			slog.String("model", c.model),
			slog.Int("text_size", len(text)),
			slog.Int("dimensions", len(embedding)),
			slog.Int("attempt", attempt),
		)
		return embedding, nil
	}

	return nil, lastErr
}
