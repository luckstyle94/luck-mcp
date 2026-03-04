package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"luck-mpc/internal/domain"
	"luck-mpc/internal/repository"
	"luck-mpc/internal/service"
)

type stubRepository struct{}

func (s *stubRepository) FindByProjectAndContentHash(_ context.Context, _, _ string) (int64, bool, error) {
	return 0, false, nil
}

func (s *stubRepository) InsertDocumentWithEmbedding(_ context.Context, _ repository.AddDocumentInput) (int64, error) {
	return 1, nil
}

func (s *stubRepository) Search(_ context.Context, _ repository.SearchDocumentsInput) ([]domain.SearchResult, error) {
	return nil, nil
}

func (s *stubRepository) ListBriefItems(_ context.Context, _ string, _ int) ([]domain.BriefItem, error) {
	return []domain.BriefItem{{
		Kind:       domain.KindSummary,
		Content:    "Resumo de bootstrap",
		Importance: 5,
		UpdatedAt:  time.Now(),
	}}, nil
}

type stubEmbeddings struct{}

func (s *stubEmbeddings) Embed(_ context.Context, _ string) ([]float64, error) {
	return make([]float64, 768), nil
}

func TestHandleToolCall_AcceptsMeta(t *testing.T) {
	svc := service.NewContextService(&stubRepository{}, &stubEmbeddings{}, "", 768, nil)
	srv := NewServer(svc, nil, "", "", bytes.NewBuffer(nil), io.Discard)

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "project_brief",
			"arguments": {"project": "review-local"},
			"_meta": {"client": "cursor"}
		}`),
	}

	res := srv.handleToolCall(context.Background(), req)
	if res.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", res.Error)
	}

	result, ok := res.Result.(toolCallResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", res.Result)
	}
	if result.IsError {
		t.Fatalf("expected successful tool call, got error result")
	}
}

func TestHandleToolCall_IgnoresUnknownEnvelopeFields(t *testing.T) {
	svc := service.NewContextService(&stubRepository{}, &stubEmbeddings{}, "", 768, nil)
	srv := NewServer(svc, nil, "", "", bytes.NewBuffer(nil), io.Discard)

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("2"),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "project_brief",
			"arguments": {"project": "review-local"},
			"_meta": {"client": "claude"},
			"extra": "ignored"
		}`),
	}

	res := srv.handleToolCall(context.Background(), req)
	if res.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", res.Error)
	}
}
