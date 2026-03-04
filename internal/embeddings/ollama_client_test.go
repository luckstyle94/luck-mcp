package embeddings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOllamaClientEmbed_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/embeddings" {
			t.Fatalf("expected /api/embeddings, got %s", r.URL.Path)
		}

		var req OllamaEmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "nomic-embed-text" {
			t.Fatalf("unexpected model: %s", req.Model)
		}
		if req.Prompt != "hello world" {
			t.Fatalf("unexpected prompt: %s", req.Prompt)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embedding":[0.1,0.2,0.3]}`))
	}))
	defer ts.Close()

	client := NewOllamaClient(ts.URL, "nomic-embed-text", 2*time.Second, nil)
	emb, err := client.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed returned error: %v", err)
	}
	if len(emb) != 3 {
		t.Fatalf("expected 3 dimensions, got %d", len(emb))
	}
}

func TestOllamaClientEmbed_Retry(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporary error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embedding":[0.4,0.5,0.6]}`))
	}))
	defer ts.Close()

	client := NewOllamaClient(ts.URL, "nomic-embed-text", 2*time.Second, nil)
	emb, err := client.Embed(context.Background(), "retry test")
	if err != nil {
		t.Fatalf("Embed returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if len(emb) != 3 {
		t.Fatalf("expected 3 dimensions, got %d", len(emb))
	}
}
