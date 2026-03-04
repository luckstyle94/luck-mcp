package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
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

func TestInitialize_EchoesClientProtocolVersion(t *testing.T) {
	svc := service.NewContextService(&stubRepository{}, &stubEmbeddings{}, "", 768, nil)
	srv := NewServer(svc, nil, "", "", bytes.NewBuffer(nil), io.Discard)

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("3"),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2025-06-18"}`),
	}

	res := srv.handleRequest(context.Background(), req)
	if res.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", res.Error)
	}

	var out initializeResult
	b, err := json.Marshal(res.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal initialize result: %v", err)
	}
	if out.ProtocolVersion != "2025-06-18" {
		t.Fatalf("expected protocol version 2025-06-18, got %q", out.ProtocolVersion)
	}
}

func TestReadMessage_LineDelimitedJSON(t *testing.T) {
	svc := service.NewContextService(&stubRepository{}, &stubEmbeddings{}, "", 768, nil)
	srv := NewServer(svc, nil, "", "", strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}\n"), io.Discard)

	msg, err := srv.readMessage()
	if err != nil {
		t.Fatalf("readMessage returned error: %v", err)
	}
	if string(msg) != "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}" {
		t.Fatalf("unexpected message: %s", string(msg))
	}
}

func TestReadMessage_ContentLengthFramed(t *testing.T) {
	payload := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}"
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload)

	svc := service.NewContextService(&stubRepository{}, &stubEmbeddings{}, "", 768, nil)
	srv := NewServer(svc, nil, "", "", strings.NewReader(frame), io.Discard)

	msg, err := srv.readMessage()
	if err != nil {
		t.Fatalf("readMessage returned error: %v", err)
	}
	if string(msg) != payload {
		t.Fatalf("unexpected framed message: %s", string(msg))
	}
}

func TestWriteResponse_LineDelimitedMode(t *testing.T) {
	in := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}\n")
	var out bytes.Buffer

	svc := service.NewContextService(&stubRepository{}, &stubEmbeddings{}, "", 768, nil)
	srv := NewServer(svc, nil, "", "", in, &out)

	_, err := srv.readMessage()
	if err != nil {
		t.Fatalf("readMessage returned error: %v", err)
	}

	err = srv.writeResponse(rpcResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Result:  map[string]any{"ok": true},
	})
	if err != nil {
		t.Fatalf("writeResponse returned error: %v", err)
	}

	got := out.String()
	if strings.Contains(got, "Content-Length:") {
		t.Fatalf("unexpected framed output in line-delimited mode: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected newline-delimited response, got %q", got)
	}
}
