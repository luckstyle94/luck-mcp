package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"luck-mcp/internal/domain"
	"luck-mcp/internal/repository"
)

type contextStoreStub struct {
	repos          map[string]domain.Repo
	memoryItems    map[int64][]domain.BriefItem
	filesByRepoID  map[int64][]repository.IndexedFile
	findFiles      []domain.FileMatch
	ensureCalls    int
	getRepoCalls   int
	findFilesCalls int
}

func newContextStoreStub() *contextStoreStub {
	now := time.Now()
	root := "/repos/luck-mcp"
	return &contextStoreStub{
		repos: map[string]domain.Repo{
			"luck-mcp": {
				ID:            7,
				Name:          "luck-mcp",
				RootPath:      &root,
				Active:        true,
				LastIndexedAt: &now,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
		memoryItems:   make(map[int64][]domain.BriefItem),
		filesByRepoID: make(map[int64][]repository.IndexedFile),
	}
}

func (s *contextStoreStub) EnsureRepo(_ context.Context, name string, rootPath *string) (domain.Repo, error) {
	s.ensureCalls++
	if repo, ok := s.repos[name]; ok {
		return repo, nil
	}
	now := time.Now()
	repo := domain.Repo{ID: int64(len(s.repos) + 1), Name: name, RootPath: rootPath, Active: true, CreatedAt: now, UpdatedAt: now}
	s.repos[name] = repo
	return repo, nil
}

func (s *contextStoreStub) UpsertRepo(_ context.Context, input repository.UpsertRepoInput) (domain.Repo, error) {
	now := time.Now()
	repo := domain.Repo{ID: int64(len(s.repos) + 1), Name: input.Name, RootPath: input.RootPath, Description: input.Description, Tags: input.Tags, Active: true, CreatedAt: now, UpdatedAt: now}
	s.repos[input.Name] = repo
	return repo, nil
}

func (s *contextStoreStub) GetRepoByName(_ context.Context, name string) (domain.Repo, bool, error) {
	s.getRepoCalls++
	repo, ok := s.repos[name]
	return repo, ok, nil
}

func (s *contextStoreStub) ListRepos(_ context.Context) ([]domain.Repo, error) { return nil, nil }

func (s *contextStoreStub) FindMemoryByRepoAndContentHash(_ context.Context, _ int64, _ string) (int64, bool, error) {
	return 0, false, nil
}

func (s *contextStoreStub) InsertMemoryWithEmbedding(_ context.Context, _ repository.AddMemoryInput) (int64, error) {
	return 101, nil
}

func (s *contextStoreStub) SearchMemory(_ context.Context, _ repository.SearchMemoryInput) ([]domain.SearchResult, error) {
	return []domain.SearchResult{{ID: 1, Score: 0.9, Kind: string(domain.KindSummary), Content: "saved memory"}}, nil
}

func (s *contextStoreStub) ListMemoryBriefItems(_ context.Context, repoID int64, _ int) ([]domain.BriefItem, error) {
	return s.memoryItems[repoID], nil
}

func (s *contextStoreStub) ListIndexedFiles(_ context.Context, repoID int64) ([]repository.IndexedFile, error) {
	return s.filesByRepoID[repoID], nil
}

func (s *contextStoreStub) UpsertIndexedFile(_ context.Context, _ repository.UpsertIndexedFileInput) error {
	return nil
}

func (s *contextStoreStub) DeleteIndexedFile(_ context.Context, _ int64, _ string) error { return nil }

func (s *contextStoreStub) DeleteIndexedChunksByPath(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}

func (s *contextStoreStub) InsertIndexedChunkWithEmbedding(_ context.Context, _ repository.AddIndexedChunkInput) (int64, error) {
	return 0, nil
}

func (s *contextStoreStub) SearchIndexedChunks(_ context.Context, _ repository.SearchIndexedChunksInput) ([]domain.RepoSearchResult, error) {
	return nil, nil
}

func (s *contextStoreStub) ReplaceFileSignals(_ context.Context, _ int64, _ string, _ []repository.FileSignalInput) error {
	return nil
}

func (s *contextStoreStub) DeleteFileSignalsByPath(_ context.Context, _ int64, _ string) error {
	return nil
}

func (s *contextStoreStub) FindFiles(_ context.Context, _ repository.FindFilesInput) ([]domain.FileMatch, error) {
	s.findFilesCalls++
	return s.findFiles, nil
}

func TestProjectBrief_UsesMemoryFirst(t *testing.T) {
	store := newContextStoreStub()
	store.memoryItems[7] = []domain.BriefItem{{
		Kind:       domain.KindSummary,
		Content:    "Resumo principal do projeto",
		Importance: 5,
		UpdatedAt:  time.Now(),
	}}

	svc := NewContextService(store, &fakeEmbeddings{}, "luck-mcp", 768, nil)
	brief, err := svc.ProjectBrief(context.Background(), ProjectBriefInput{Project: "luck-mcp"})
	if err != nil {
		t.Fatalf("ProjectBrief returned error: %v", err)
	}
	if !strings.Contains(brief, "Resumo principal do projeto") {
		t.Fatalf("expected memory-backed brief, got: %s", brief)
	}
	if store.findFilesCalls != 0 {
		t.Fatalf("expected no fallback lookup when memory exists")
	}
}

func TestProjectBrief_FallsBackToIndexedContent(t *testing.T) {
	store := newContextStoreStub()
	store.findFiles = []domain.FileMatch{
		{Repo: "luck-mcp", Path: "README.md", Score: 0.98, FileType: "doc", Language: "markdown", Snippet: "Multi repo memory MCP for AI agents."},
		{Repo: "luck-mcp", Path: "QUICKSTART.md", Score: 0.88, FileType: "doc", Language: "markdown", Snippet: "Quickstart diario em poucos comandos."},
	}
	store.filesByRepoID[7] = []repository.IndexedFile{
		{Path: "cmd/mcp-server/main.go", FileType: "code", Language: "go", ChunkCount: 5, Status: "indexed"},
		{Path: "internal/service/context_service.go", FileType: "code", Language: "go", ChunkCount: 4, Status: "indexed"},
		{Path: "migrations/0005_codebase_memory_refactor.up.sql", FileType: "infra", Language: "sql", ChunkCount: 3, Status: "indexed"},
	}

	svc := NewContextService(store, &fakeEmbeddings{}, "luck-mcp", 768, nil)
	brief, err := svc.ProjectBrief(context.Background(), ProjectBriefInput{Project: "luck-mcp"})
	if err != nil {
		t.Fatalf("ProjectBrief returned error: %v", err)
	}
	if !strings.Contains(brief, "fallback do indice") {
		t.Fatalf("expected fallback marker, got: %s", brief)
	}
	if !strings.Contains(brief, "README.md") || !strings.Contains(brief, "cmd/mcp-server/main.go") {
		t.Fatalf("expected docs and highlighted files in fallback brief, got: %s", brief)
	}
}

func TestProjectBrief_MissingProjectDoesNotCreateRepo(t *testing.T) {
	store := newContextStoreStub()
	svc := NewContextService(store, &fakeEmbeddings{}, "", 768, nil)

	_, err := svc.ProjectBrief(context.Background(), ProjectBriefInput{Project: "missing-repo"})
	if err == nil {
		t.Fatalf("expected error for missing project")
	}
	if !strings.Contains(err.Error(), "run repo_register or mcp-index first") {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.ensureCalls != 0 {
		t.Fatalf("expected read path to avoid EnsureRepo, got %d calls", store.ensureCalls)
	}
}

func TestSearchContext_MissingProjectDoesNotCreateRepo(t *testing.T) {
	store := newContextStoreStub()
	svc := NewContextService(store, &fakeEmbeddings{}, "", 768, nil)

	_, err := svc.SearchContext(context.Background(), SearchContextInput{Project: "missing-repo", Query: "auth"})
	if err == nil {
		t.Fatalf("expected error for missing project")
	}
	if store.ensureCalls != 0 {
		t.Fatalf("expected read path to avoid EnsureRepo, got %d calls", store.ensureCalls)
	}
}

func TestAddContext_CreatesRepoOnWrite(t *testing.T) {
	store := newContextStoreStub()
	delete(store.repos, "new-repo")
	svc := NewContextService(store, &fakeEmbeddings{}, "", 768, nil)

	_, err := svc.AddContext(context.Background(), AddContextInput{
		Project: "new-repo",
		Kind:    "summary",
		Content: "Decision: keep write path creating repos.",
	})
	if err != nil {
		t.Fatalf("AddContext returned error: %v", err)
	}
	if store.ensureCalls == 0 {
		t.Fatalf("expected write path to call EnsureRepo")
	}
}
