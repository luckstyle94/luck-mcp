package repository

import (
	"context"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"luck-mpc/internal/domain"
)

func TestInsertDocumentWithEmbedding_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresDocumentRepository(db)
	path := "internal/service/context.go"
	hash := "abc123"
	input := AddDocumentInput{
		Project:     "luck-mpc",
		Kind:        domain.KindSummary,
		Path:        &path,
		Tags:        []string{"auth", "design"},
		Content:     "Resumo de arquitetura",
		Importance:  5,
		ContentHash: &hash,
		Embedding:   []float64{0.1, 0.2, 0.3},
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`
INSERT INTO documents (project, kind, path, tags, content, importance, content_hash)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (project, content_hash) WHERE content_hash IS NOT NULL
DO UPDATE SET updated_at = NOW()
RETURNING id`)).
		WithArgs(
			input.Project,
			string(input.Kind),
			input.Path,
			sqlmock.AnyArg(),
			input.Content,
			input.Importance,
			input.ContentHash,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))

	mock.ExpectExec(regexp.QuoteMeta(`
INSERT INTO doc_embeddings (doc_id, embedding)
VALUES ($1, $2::vector)
ON CONFLICT (doc_id) DO NOTHING`)).
		WithArgs(int64(42), "[0.1,0.2,0.3]").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	id, err := repo.InsertDocumentWithEmbedding(context.Background(), input)
	if err != nil {
		t.Fatalf("InsertDocumentWithEmbedding returned error: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected id 42, got %d", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestSearch_WithFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresDocumentRepository(db)
	prefix := "internal/_auth%/"
	in := SearchDocumentsInput{
		Project:        "luck-mpc",
		Kind:           domain.KindSummary,
		PathPrefix:     &prefix,
		Tags:           []string{"auth", "flow"},
		K:              8,
		QueryEmbedding: []float64{0.11, 0.22, 0.33},
	}

	q := `(?s)SELECT.*FROM documents d.*JOIN doc_embeddings e ON e.doc_id = d.id.*` +
		`WHERE d.project = \$1 AND d.path IS NOT NULL AND left\(d.path, char_length\(\$3\)\) = \$3 AND COALESCE\(d.tags, ARRAY\[\]::text\[\]\) @> \$4::text\[\] AND d.kind = \$5.*` +
		`ORDER BY e.embedding <=> \$2::vector ASC.*LIMIT \$6`

	rows := sqlmock.NewRows([]string{"id", "score", "kind", "path", "tags", "content"}).
		AddRow(int64(7), 0.94, "summary", "internal/auth/service.go", pq.StringArray{"auth", "flow"}, "Fluxo principal")

	mock.ExpectQuery(q).
		WithArgs("luck-mpc", "[0.11,0.22,0.33]", "internal/_auth%/", sqlmock.AnyArg(), "summary", 8).
		WillReturnRows(rows)

	got, err := repo.Search(context.Background(), in)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].ID != 7 || got[0].Kind != "summary" {
		t.Fatalf("unexpected result: %+v", got[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
