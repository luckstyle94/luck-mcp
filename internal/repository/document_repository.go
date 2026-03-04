package repository

import (
	"context"

	"luck-mpc/internal/domain"
)

type AddDocumentInput struct {
	Project     string
	Kind        domain.Kind
	Path        *string
	Tags        []string
	Content     string
	Importance  int
	ContentHash *string
	Embedding   []float64
}

type SearchDocumentsInput struct {
	Project        string
	Kind           domain.Kind
	PathPrefix     *string
	Tags           []string
	K              int
	QueryEmbedding []float64
}

const AutoIndexTag = "_auto_index"

type IndexedFile struct {
	Path        string
	ContentHash string
}

type UpsertIndexedFileInput struct {
	Project     string
	Path        string
	ContentHash string
	ChunkCount  int
	Status      string
	Error       *string
}

type DocumentRepository interface {
	FindByProjectAndContentHash(ctx context.Context, project, contentHash string) (int64, bool, error)
	InsertDocumentWithEmbedding(ctx context.Context, input AddDocumentInput) (int64, error)
	Search(ctx context.Context, input SearchDocumentsInput) ([]domain.SearchResult, error)
	ListBriefItems(ctx context.Context, project string, maxItems int) ([]domain.BriefItem, error)
	ListIndexedFiles(ctx context.Context, project string) ([]IndexedFile, error)
	UpsertIndexedFile(ctx context.Context, input UpsertIndexedFileInput) error
	DeleteIndexedFile(ctx context.Context, project, path string) error
	DeleteAutoChunksByPath(ctx context.Context, project, path string) (int64, error)
}
