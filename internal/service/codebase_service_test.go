package service

import (
	"context"
	"testing"
	"time"

	"luck-mpc/internal/domain"
	"luck-mpc/internal/repository"
)

type fakeStore struct{}

func (f *fakeStore) EnsureRepo(_ context.Context, name string, rootPath *string) (domain.Repo, error) {
	return domain.Repo{ID: 1, Name: name, RootPath: rootPath, Active: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (f *fakeStore) UpsertRepo(_ context.Context, input repository.UpsertRepoInput) (domain.Repo, error) {
	return domain.Repo{ID: 1, Name: input.Name, RootPath: input.RootPath, Description: input.Description, Tags: input.Tags, Active: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (f *fakeStore) GetRepoByName(_ context.Context, name string) (domain.Repo, bool, error) {
	repo, _ := f.EnsureRepo(context.Background(), name, nil)
	return repo, true, nil
}

func (f *fakeStore) ListRepos(_ context.Context) ([]domain.Repo, error) { return nil, nil }

func (f *fakeStore) FindMemoryByRepoAndContentHash(_ context.Context, _ int64, _ string) (int64, bool, error) {
	return 0, false, nil
}

func (f *fakeStore) InsertMemoryWithEmbedding(_ context.Context, _ repository.AddMemoryInput) (int64, error) {
	return 0, nil
}

func (f *fakeStore) SearchMemory(_ context.Context, _ repository.SearchMemoryInput) ([]domain.SearchResult, error) {
	return nil, nil
}

func (f *fakeStore) ListMemoryBriefItems(_ context.Context, _ int64, _ int) ([]domain.BriefItem, error) {
	return nil, nil
}

func (f *fakeStore) ListIndexedFiles(_ context.Context, _ int64) ([]repository.IndexedFile, error) {
	return nil, nil
}

func (f *fakeStore) UpsertIndexedFile(_ context.Context, _ repository.UpsertIndexedFileInput) error {
	return nil
}

func (f *fakeStore) DeleteIndexedFile(_ context.Context, _ int64, _ string) error { return nil }

func (f *fakeStore) DeleteIndexedChunksByPath(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}

func (f *fakeStore) InsertIndexedChunkWithEmbedding(_ context.Context, _ repository.AddIndexedChunkInput) (int64, error) {
	return 0, nil
}

func (f *fakeStore) SearchIndexedChunks(_ context.Context, _ repository.SearchIndexedChunksInput) ([]domain.RepoSearchResult, error) {
	return []domain.RepoSearchResult{
		{Repo: "repo-a", Path: "internal/auth/service.go", Score: 0.92, FileType: "code", Language: "go"},
		{Repo: "repo-b", Path: "docs/auth.md", Score: 0.73, FileType: "doc", Language: "markdown"},
		{Repo: "repo-a", Path: "docs/adr-auth.md", Score: 0.70, FileType: "doc", Language: "markdown"},
	}, nil
}

func (f *fakeStore) ReplaceFileSignals(_ context.Context, _ int64, _ string, _ []repository.FileSignalInput) error {
	return nil
}

func (f *fakeStore) DeleteFileSignalsByPath(_ context.Context, _ int64, _ string) error { return nil }

func (f *fakeStore) FindFiles(_ context.Context, _ repository.FindFilesInput) ([]domain.FileMatch, error) {
	return []domain.FileMatch{
		{Repo: "repo-a", Path: "internal/auth/service.go", Score: 0.95, FileType: "code", Language: "go"},
		{Repo: "repo-a", Path: "docs/adr-auth.md", Score: 0.81, FileType: "doc", Language: "markdown"},
		{Repo: "repo-b", Path: "docs/auth.md", Score: 0.80, FileType: "doc", Language: "markdown"},
	}, nil
}

type fakeEmbeddings struct{}

func (f *fakeEmbeddings) Embed(_ context.Context, _ string) ([]float64, error) {
	return make([]float64, 768), nil
}

func TestSearchAcrossRepos_AggregatesByRepo(t *testing.T) {
	svc := NewCodebaseService(&fakeStore{}, &fakeEmbeddings{}, 768, nil)

	results, err := svc.SearchAcrossRepos(context.Background(), SearchAcrossReposInput{
		Query: "auth flow",
		Mode:  "hybrid",
	})
	if err != nil {
		t.Fatalf("SearchAcrossRepos returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(results))
	}
	if results[0].Repo != "repo-a" {
		t.Fatalf("expected repo-a first, got %+v", results)
	}
	if results[0].MatchCount < 2 {
		t.Fatalf("expected aggregated matches for repo-a, got %+v", results[0])
	}
	if len(results[0].TopPaths) == 0 {
		t.Fatalf("expected top paths for repo-a, got %+v", results[0])
	}
}
