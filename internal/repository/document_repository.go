package repository

import (
	"context"
	"time"

	"luck-mcp/internal/domain"
)

const AutoIndexTag = "_auto_index"

type AddMemoryInput struct {
	RepoID      int64
	Kind        domain.Kind
	Path        *string
	Tags        []string
	Content     string
	Importance  int
	ContentHash *string
	Embedding   []float64
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
}

type UpsertRepoInput struct {
	Name        string
	RootPath    *string
	Description *string
	Tags        []string
	Active      *bool
}

type SearchMemoryInput struct {
	RepoID         int64
	Kind           domain.Kind
	PathPrefix     *string
	Tags           []string
	K              int
	QueryEmbedding []float64
}

type IndexedFile struct {
	Path        string
	ContentHash string
	Language    string
	FileType    string
	SizeBytes   int64
	ChunkCount  int
	Status      string
}

type UpsertIndexedFileInput struct {
	Project       string
	RepoID        int64
	Path          string
	ContentHash   string
	Language      string
	FileType      string
	SizeBytes     int64
	ChunkCount    int
	Status        string
	Error         *string
	LastIndexedAt *time.Time
}

type AddIndexedChunkInput struct {
	RepoID      int64
	Path        string
	ChunkIndex  int
	Tags        []string
	Content     string
	ContentHash string
	Embedding   []float64
}

type FileSignalInput struct {
	SignalType      string
	Value           string
	NormalizedValue string
}

type SearchIndexedChunksInput struct {
	RepoNames      []string
	Query          string
	Mode           domain.SearchMode
	PathPrefix     *string
	FileType       string
	Language       string
	K              int
	QueryEmbedding []float64
}

type FindFilesInput struct {
	RepoNames  []string
	Query      string
	PathPrefix *string
	FileType   string
	Language   string
	K          int
}

type Store interface {
	EnsureRepo(ctx context.Context, name string, rootPath *string) (domain.Repo, error)
	UpsertRepo(ctx context.Context, input UpsertRepoInput) (domain.Repo, error)
	GetRepoByName(ctx context.Context, name string) (domain.Repo, bool, error)
	ListRepos(ctx context.Context) ([]domain.Repo, error)

	FindMemoryByRepoAndContentHash(ctx context.Context, repoID int64, contentHash string) (int64, bool, error)
	InsertMemoryWithEmbedding(ctx context.Context, input AddMemoryInput) (int64, error)
	SearchMemory(ctx context.Context, input SearchMemoryInput) ([]domain.SearchResult, error)
	ListMemoryBriefItems(ctx context.Context, repoID int64, maxItems int) ([]domain.BriefItem, error)

	ListIndexedFiles(ctx context.Context, repoID int64) ([]IndexedFile, error)
	UpsertIndexedFile(ctx context.Context, input UpsertIndexedFileInput) error
	DeleteIndexedFile(ctx context.Context, repoID int64, path string) error
	DeleteIndexedChunksByPath(ctx context.Context, repoID int64, path string) (int64, error)
	InsertIndexedChunkWithEmbedding(ctx context.Context, input AddIndexedChunkInput) (int64, error)
	SearchIndexedChunks(ctx context.Context, input SearchIndexedChunksInput) ([]domain.RepoSearchResult, error)
	ReplaceFileSignals(ctx context.Context, repoID int64, path string, signals []FileSignalInput) error
	DeleteFileSignalsByPath(ctx context.Context, repoID int64, path string) error
	FindFiles(ctx context.Context, input FindFilesInput) ([]domain.FileMatch, error)
}

type DocumentRepository = Store
