package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"luck-mpc/internal/domain"
)

func TestEnsureRepo_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresDocumentRepository(db)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "root_path", "description", "tags", "active", "last_indexed_at", "created_at", "updated_at"}).
		AddRow(int64(7), "luck-mpc", "/workspace", nil, pq.StringArray{"local"}, true, nil, now, now)

	mock.ExpectQuery(regexp.QuoteMeta(`
INSERT INTO repos (name, root_path, description, tags, active)
VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4, COALESCE($5, TRUE))
ON CONFLICT (name)
DO UPDATE SET
	root_path = COALESCE(NULLIF(EXCLUDED.root_path, ''), repos.root_path),
	description = COALESCE(NULLIF(EXCLUDED.description, ''), repos.description),
	tags = CASE WHEN COALESCE(array_length(EXCLUDED.tags, 1), 0) > 0 THEN EXCLUDED.tags ELSE repos.tags END,
	active = COALESCE(EXCLUDED.active, repos.active),
	updated_at = NOW()
RETURNING id, name, root_path, description, COALESCE(tags, ARRAY[]::text[]), active, last_indexed_at, created_at, updated_at`)).
		WithArgs("luck-mpc", "/workspace", nil, nil, sqlmock.AnyArg()).
		WillReturnRows(rows)

	root := "/workspace"
	got, err := repo.EnsureRepo(context.Background(), "luck-mpc", &root)
	if err != nil {
		t.Fatalf("EnsureRepo returned error: %v", err)
	}
	if got.ID != 7 || got.Name != "luck-mpc" {
		t.Fatalf("unexpected repo: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestUpsertRepo_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresDocumentRepository(db)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "root_path", "description", "tags", "active", "last_indexed_at", "created_at", "updated_at"}).
		AddRow(int64(8), "repo-a", "/repos/repo-a", "API principal", pq.StringArray{"backend", "api"}, true, nil, now, now)

	active := true
	root := "/repos/repo-a"
	desc := "API principal"
	mock.ExpectQuery(regexp.QuoteMeta(`
INSERT INTO repos (name, root_path, description, tags, active)
VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4, COALESCE($5, TRUE))
ON CONFLICT (name)
DO UPDATE SET
	root_path = COALESCE(NULLIF(EXCLUDED.root_path, ''), repos.root_path),
	description = COALESCE(NULLIF(EXCLUDED.description, ''), repos.description),
	tags = CASE WHEN COALESCE(array_length(EXCLUDED.tags, 1), 0) > 0 THEN EXCLUDED.tags ELSE repos.tags END,
	active = COALESCE(EXCLUDED.active, repos.active),
	updated_at = NOW()
RETURNING id, name, root_path, description, COALESCE(tags, ARRAY[]::text[]), active, last_indexed_at, created_at, updated_at`)).
		WithArgs("repo-a", root, desc, sqlmock.AnyArg(), &active).
		WillReturnRows(rows)

	got, err := repo.UpsertRepo(context.Background(), UpsertRepoInput{
		Name:        "repo-a",
		RootPath:    &root,
		Description: &desc,
		Tags:        []string{"backend", "api"},
		Active:      &active,
	})
	if err != nil {
		t.Fatalf("UpsertRepo returned error: %v", err)
	}
	if got.ID != 8 || got.Name != "repo-a" {
		t.Fatalf("unexpected repo: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestInsertMemoryWithEmbedding_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresDocumentRepository(db)
	path := "internal/service/context.go"
	hash := "abc123"
	input := AddMemoryInput{
		RepoID:      3,
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
INSERT INTO memory_entries (repo_id, kind, path, tags, content, importance, content_hash, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, NOW()), COALESCE($9, NOW()))
ON CONFLICT (repo_id, content_hash) WHERE content_hash IS NOT NULL
DO UPDATE SET updated_at = NOW()
RETURNING id`)).
		WithArgs(
			input.RepoID,
			string(input.Kind),
			input.Path,
			sqlmock.AnyArg(),
			input.Content,
			input.Importance,
			input.ContentHash,
			nil,
			nil,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))

	mock.ExpectExec(regexp.QuoteMeta(`
INSERT INTO memory_embeddings (entry_id, embedding)
VALUES ($1, $2::vector)
ON CONFLICT (entry_id) DO NOTHING`)).
		WithArgs(int64(42), "[0.1,0.2,0.3]").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	id, err := repo.InsertMemoryWithEmbedding(context.Background(), input)
	if err != nil {
		t.Fatalf("InsertMemoryWithEmbedding returned error: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected id 42, got %d", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestSearchMemory_WithFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresDocumentRepository(db)
	prefix := "internal/auth/"
	in := SearchMemoryInput{
		RepoID:         3,
		Kind:           domain.KindSummary,
		PathPrefix:     &prefix,
		Tags:           []string{"auth", "flow"},
		K:              8,
		QueryEmbedding: []float64{0.11, 0.22, 0.33},
	}

	q := `(?s)SELECT.*FROM memory_entries m.*JOIN memory_embeddings e ON e.entry_id = m.id.*` +
		`WHERE m.repo_id = \$1 AND m.path IS NOT NULL AND left\(m.path, char_length\(\$3\)\) = \$3 AND COALESCE\(m.tags, ARRAY\[\]::text\[\]\) @> \$4::text\[\] AND m.kind = \$5.*` +
		`ORDER BY e.embedding <=> \$2::vector ASC.*LIMIT \$6`

	rows := sqlmock.NewRows([]string{"id", "score", "kind", "path", "tags", "content"}).
		AddRow(int64(7), 0.94, "summary", "internal/auth/service.go", pq.StringArray{"auth", "flow"}, "Fluxo principal")

	mock.ExpectQuery(q).
		WithArgs(int64(3), "[0.11,0.22,0.33]", "internal/auth/", sqlmock.AnyArg(), "summary", 8).
		WillReturnRows(rows)

	got, err := repo.SearchMemory(context.Background(), in)
	if err != nil {
		t.Fatalf("SearchMemory returned error: %v", err)
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

func TestListIndexedFiles_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresDocumentRepository(db)
	rows := sqlmock.NewRows([]string{"path", "content_hash", "language", "file_type", "size_bytes", "chunk_count", "status"}).
		AddRow("internal/auth/service.go", "hash1", "go", "code", int64(1234), 3, "indexed").
		AddRow("README.md", "hash2", "markdown", "doc", int64(4567), 4, "indexed")

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT path, content_hash, language, file_type, size_bytes, chunk_count, status
FROM indexed_files
WHERE repo_id = $1
  AND status = 'indexed'`)).
		WithArgs(int64(3)).
		WillReturnRows(rows)

	got, err := repo.ListIndexedFiles(context.Background(), 3)
	if err != nil {
		t.Fatalf("ListIndexedFiles returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	if got[0].Path != "internal/auth/service.go" || got[0].ContentHash != "hash1" || got[0].Language != "go" || got[0].ChunkCount != 3 {
		t.Fatalf("unexpected first row: %+v", got[0])
	}
	if got[1].FileType != "doc" || got[1].SizeBytes != 4567 {
		t.Fatalf("unexpected second row: %+v", got[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestDeleteIndexedChunksByPath_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresDocumentRepository(db)

	mock.ExpectExec(regexp.QuoteMeta(`
DELETE FROM indexed_chunks
WHERE repo_id = $1 AND path = $2`)).
		WithArgs(int64(3), "internal/auth/service.go").
		WillReturnResult(sqlmock.NewResult(0, 3))

	deleted, err := repo.DeleteIndexedChunksByPath(context.Background(), 3, "internal/auth/service.go")
	if err != nil {
		t.Fatalf("DeleteIndexedChunksByPath returned error: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("expected 3 deleted rows, got %d", deleted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestReplaceFileSignals_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresDocumentRepository(db)
	signals := []FileSignalInput{
		{SignalType: "endpoint", Value: "/api/v1/auth", NormalizedValue: "/api/v1/auth"},
		{SignalType: "env_var", Value: "DATABASE_URL", NormalizedValue: "database_url"},
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`
DELETE FROM file_signals
WHERE repo_id = $1 AND path = $2`)).
		WithArgs(int64(3), "internal/auth/service.go").
		WillReturnResult(sqlmock.NewResult(0, 2))

	mock.ExpectExec(regexp.QuoteMeta(`
INSERT INTO file_signals (repo_id, path, signal_type, value, normalized_value)
VALUES ($1, $2, $3, $4, $5)`)).
		WithArgs(int64(3), "internal/auth/service.go", "endpoint", "/api/v1/auth", "/api/v1/auth").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`
INSERT INTO file_signals (repo_id, path, signal_type, value, normalized_value)
VALUES ($1, $2, $3, $4, $5)`)).
		WithArgs(int64(3), "internal/auth/service.go", "env_var", "DATABASE_URL", "database_url").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := repo.ReplaceFileSignals(context.Background(), 3, "internal/auth/service.go", signals); err != nil {
		t.Fatalf("ReplaceFileSignals returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestFindFiles_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPostgresDocumentRepository(db)
	in := FindFilesInput{
		RepoNames: []string{"luck-mpc"},
		Query:     "README",
		FileType:  "doc",
		K:         10,
	}

	q := `(?s)SELECT\s+r\.name,\s+f\.path,.*FROM indexed_files f.*JOIN repos r ON r\.id = f\.repo_id.*LEFT JOIN indexed_chunks c.*LEFT JOIN file_signals fs.*WHERE r\.active = TRUE AND f\.status = 'indexed' AND r\.name = ANY\(\$1::text\[\]\) AND f\.file_type = \$2.*LIMIT \$4`
	rows := sqlmock.NewRows([]string{"name", "path", "score", "file_type", "language", "size_bytes", "snippet"}).
		AddRow("luck-mpc", "README.md", 0.97, "doc", "markdown", int64(14000), "README bootstrap")

	mock.ExpectQuery(q).
		WithArgs(sqlmock.AnyArg(), "doc", "README", 10).
		WillReturnRows(rows)

	got, err := repo.FindFiles(context.Background(), in)
	if err != nil {
		t.Fatalf("FindFiles returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].Path != "README.md" || got[0].Repo != "luck-mpc" {
		t.Fatalf("unexpected result: %+v", got[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
