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

func (s *stubRepository) EnsureRepo(_ context.Context, name string, rootPath *string) (domain.Repo, error) {
	return domain.Repo{ID: 1, Name: name, RootPath: rootPath, Active: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (s *stubRepository) UpsertRepo(_ context.Context, input repository.UpsertRepoInput) (domain.Repo, error) {
	return domain.Repo{ID: 1, Name: input.Name, RootPath: input.RootPath, Description: input.Description, Tags: input.Tags, Active: input.Active == nil || *input.Active, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (s *stubRepository) GetRepoByName(_ context.Context, name string) (domain.Repo, bool, error) {
	repo, _ := s.EnsureRepo(context.Background(), name, nil)
	return repo, true, nil
}

func (s *stubRepository) ListRepos(_ context.Context) ([]domain.Repo, error) {
	return []domain.Repo{{ID: 1, Name: "review-local", Active: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}}, nil
}

func (s *stubRepository) FindMemoryByRepoAndContentHash(_ context.Context, _ int64, _ string) (int64, bool, error) {
	return 0, false, nil
}

func (s *stubRepository) InsertMemoryWithEmbedding(_ context.Context, _ repository.AddMemoryInput) (int64, error) {
	return 1, nil
}

func (s *stubRepository) SearchMemory(_ context.Context, _ repository.SearchMemoryInput) ([]domain.SearchResult, error) {
	return nil, nil
}

func (s *stubRepository) ListMemoryBriefItems(_ context.Context, _ int64, _ int) ([]domain.BriefItem, error) {
	return []domain.BriefItem{{
		Kind:       domain.KindSummary,
		Content:    "Resumo de bootstrap",
		Importance: 5,
		UpdatedAt:  time.Now(),
	}}, nil
}

func (s *stubRepository) ListIndexedFiles(_ context.Context, _ int64) ([]repository.IndexedFile, error) {
	return nil, nil
}

func (s *stubRepository) UpsertIndexedFile(_ context.Context, _ repository.UpsertIndexedFileInput) error {
	return nil
}

func (s *stubRepository) DeleteIndexedFile(_ context.Context, _ int64, _ string) error {
	return nil
}

func (s *stubRepository) DeleteIndexedChunksByPath(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}

func (s *stubRepository) InsertIndexedChunkWithEmbedding(_ context.Context, _ repository.AddIndexedChunkInput) (int64, error) {
	return 1, nil
}

func (s *stubRepository) SearchIndexedChunks(_ context.Context, _ repository.SearchIndexedChunksInput) ([]domain.RepoSearchResult, error) {
	return []domain.RepoSearchResult{{Repo: "review-local", Path: "README.md", Score: 0.9, FileType: "doc", Language: "markdown", Content: "README bootstrap"}}, nil
}

func (s *stubRepository) ReplaceFileSignals(_ context.Context, _ int64, _ string, _ []repository.FileSignalInput) error {
	return nil
}

func (s *stubRepository) DeleteFileSignalsByPath(_ context.Context, _ int64, _ string) error {
	return nil
}

func (s *stubRepository) FindFiles(_ context.Context, _ repository.FindFilesInput) ([]domain.FileMatch, error) {
	return []domain.FileMatch{{Repo: "review-local", Path: "README.md", Score: 0.95, FileType: "doc", Language: "markdown", SizeBytes: 1024, Snippet: "README bootstrap"}}, nil
}

type stubEmbeddings struct{}

func (s *stubEmbeddings) Embed(_ context.Context, _ string) ([]float64, error) {
	return make([]float64, 768), nil
}

func newTestServer() *Server {
	repo := &stubRepository{}
	ctxSvc := service.NewContextService(repo, &stubEmbeddings{}, "", 768, nil)
	codebaseSvc := service.NewCodebaseService(repo, &stubEmbeddings{}, 768, nil)
	return NewServer(ctxSvc, codebaseSvc, nil, "", "", bytes.NewBuffer(nil), io.Discard)
}

func TestHandleToolCall_AcceptsMeta(t *testing.T) {
	srv := newTestServer()

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

func TestHandleToolCall_RepoSearch(t *testing.T) {
	srv := newTestServer()

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("11"),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "repo_search",
			"arguments": {"query": "bootstrap", "mode": "text"}
		}`),
	}

	res := srv.handleToolCall(context.Background(), req)
	if res.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", res.Error)
	}
}

func TestHandleToolCall_SearchAcrossRepos(t *testing.T) {
	srv := newTestServer()

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("14"),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "search_across_repos",
			"arguments": {"query": "bootstrap", "mode": "hybrid"}
		}`),
	}

	res := srv.handleToolCall(context.Background(), req)
	if res.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", res.Error)
	}
}

func TestHandleToolCall_RepoRegister(t *testing.T) {
	srv := newTestServer()

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("10"),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "repo_register",
			"arguments": {"name": "review-local", "root_path": "/workspace/review-local", "tags": ["backend"]}
		}`),
	}

	res := srv.handleToolCall(context.Background(), req)
	if res.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", res.Error)
	}
}

func TestHandleToolCall_RepoFindFiles(t *testing.T) {
	srv := newTestServer()

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("12"),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "repo_find_files",
			"arguments": {"query": "README"}
		}`),
	}

	res := srv.handleToolCall(context.Background(), req)
	if res.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", res.Error)
	}
}

func TestHandleToolCall_RepoFindDocs(t *testing.T) {
	srv := newTestServer()

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("13"),
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "repo_find_docs",
			"arguments": {"query": "bootstrap"}
		}`),
	}

	res := srv.handleToolCall(context.Background(), req)
	if res.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", res.Error)
	}
}

func TestHandleToolCall_IgnoresUnknownEnvelopeFields(t *testing.T) {
	srv := newTestServer()

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
	srv := newTestServer()

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
	srv := NewServer(nil, nil, nil, "", "", strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}\n"), io.Discard)

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

	srv := NewServer(nil, nil, nil, "", "", strings.NewReader(frame), io.Discard)

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

	srv := NewServer(nil, nil, nil, "", "", in, &out)

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
