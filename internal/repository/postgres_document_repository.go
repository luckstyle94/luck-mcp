package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"

	"luck-mpc/internal/domain"
)

type PostgresDocumentRepository struct {
	db *sql.DB
}

func NewPostgresDocumentRepository(db *sql.DB) *PostgresDocumentRepository {
	return &PostgresDocumentRepository{db: db}
}

func (r *PostgresDocumentRepository) FindByProjectAndContentHash(ctx context.Context, project, contentHash string) (int64, bool, error) {
	const q = `
SELECT id
FROM documents
WHERE project = $1 AND content_hash = $2
ORDER BY updated_at DESC
LIMIT 1`

	var id int64
	err := r.db.QueryRowContext(ctx, q, project, contentHash).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("find document by hash: %w", err)
	}
	return id, true, nil
}

func (r *PostgresDocumentRepository) InsertDocumentWithEmbedding(ctx context.Context, input AddDocumentInput) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const insertDoc = `
INSERT INTO documents (project, kind, path, tags, content, importance, content_hash)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (project, content_hash) WHERE content_hash IS NOT NULL
DO UPDATE SET updated_at = NOW()
RETURNING id`

	var id int64
	if err := tx.QueryRowContext(
		ctx,
		insertDoc,
		input.Project,
		string(input.Kind),
		input.Path,
		pq.Array(input.Tags),
		input.Content,
		input.Importance,
		input.ContentHash,
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert document: %w", err)
	}

	const insertEmbedding = `
INSERT INTO doc_embeddings (doc_id, embedding)
VALUES ($1, $2::vector)
ON CONFLICT (doc_id) DO NOTHING`

	if _, err := tx.ExecContext(ctx, insertEmbedding, id, toVectorLiteral(input.Embedding)); err != nil {
		return 0, fmt.Errorf("insert document embedding: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return id, nil
}

func (r *PostgresDocumentRepository) Search(ctx context.Context, input SearchDocumentsInput) ([]domain.SearchResult, error) {
	whereClauses := []string{"d.project = $1"}
	args := []any{input.Project, toVectorLiteral(input.QueryEmbedding)}
	argPos := 3

	if input.PathPrefix != nil && strings.TrimSpace(*input.PathPrefix) != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("d.path IS NOT NULL AND left(d.path, char_length($%d)) = $%d", argPos, argPos))
		args = append(args, strings.TrimSpace(*input.PathPrefix))
		argPos++
	}

	if len(input.Tags) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("COALESCE(d.tags, ARRAY[]::text[]) @> $%d::text[]", argPos))
		args = append(args, pq.Array(input.Tags))
		argPos++
	}

	if input.Kind != "" && input.Kind != domain.KindAny {
		whereClauses = append(whereClauses, fmt.Sprintf("d.kind = $%d", argPos))
		args = append(args, string(input.Kind))
		argPos++
	}

	limitPos := argPos
	args = append(args, input.K)

	query := fmt.Sprintf(`
SELECT
	d.id,
	(1 - (e.embedding <=> $2::vector)) AS score,
	d.kind,
	d.path,
	COALESCE(d.tags, ARRAY[]::text[]),
	d.content
FROM documents d
JOIN doc_embeddings e ON e.doc_id = d.id
WHERE %s
ORDER BY e.embedding <=> $2::vector ASC
LIMIT $%d`, strings.Join(whereClauses, " AND "), limitPos)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search documents: %w", err)
	}
	defer rows.Close()

	results := make([]domain.SearchResult, 0, input.K)
	for rows.Next() {
		var item domain.SearchResult
		var tags pq.StringArray
		if err := rows.Scan(&item.ID, &item.Score, &item.Kind, &item.Path, &tags, &item.Content); err != nil {
			return nil, fmt.Errorf("scan search row: %w", err)
		}
		item.Tags = []string(tags)
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search rows: %w", err)
	}

	return results, nil
}

func (r *PostgresDocumentRepository) ListBriefItems(ctx context.Context, project string, maxItems int) ([]domain.BriefItem, error) {
	const q = `
SELECT
	kind,
	path,
	COALESCE(tags, ARRAY[]::text[]),
	content,
	importance,
	updated_at
FROM documents
WHERE project = $1
ORDER BY
	CASE WHEN kind = 'summary' THEN 1 ELSE 0 END DESC,
	importance DESC,
	updated_at DESC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, q, project, maxItems)
	if err != nil {
		return nil, fmt.Errorf("list brief items: %w", err)
	}
	defer rows.Close()

	items := make([]domain.BriefItem, 0, maxItems)
	for rows.Next() {
		var item domain.BriefItem
		var tags pq.StringArray
		if err := rows.Scan(&item.Kind, &item.Path, &tags, &item.Content, &item.Importance, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan brief item: %w", err)
		}
		item.Tags = []string(tags)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate brief rows: %w", err)
	}

	return items, nil
}

func (r *PostgresDocumentRepository) ListIndexedFiles(ctx context.Context, project string) ([]IndexedFile, error) {
	const q = `
SELECT path, content_hash
FROM indexed_files
WHERE project = $1
  AND status = 'indexed'`

	rows, err := r.db.QueryContext(ctx, q, project)
	if err != nil {
		return nil, fmt.Errorf("list indexed files: %w", err)
	}
	defer rows.Close()

	files := make([]IndexedFile, 0)
	for rows.Next() {
		var f IndexedFile
		if err := rows.Scan(&f.Path, &f.ContentHash); err != nil {
			return nil, fmt.Errorf("scan indexed file: %w", err)
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate indexed files rows: %w", err)
	}
	return files, nil
}

func (r *PostgresDocumentRepository) UpsertIndexedFile(ctx context.Context, input UpsertIndexedFileInput) error {
	const q = `
INSERT INTO indexed_files (project, path, content_hash, chunk_count, status, error, indexed_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW())
ON CONFLICT (project, path)
DO UPDATE SET
	content_hash = EXCLUDED.content_hash,
	chunk_count = EXCLUDED.chunk_count,
	status = EXCLUDED.status,
	error = EXCLUDED.error,
	indexed_at = NOW()`

	if _, err := r.db.ExecContext(
		ctx,
		q,
		input.Project,
		input.Path,
		input.ContentHash,
		input.ChunkCount,
		input.Status,
		input.Error,
	); err != nil {
		return fmt.Errorf("upsert indexed file: %w", err)
	}
	return nil
}

func (r *PostgresDocumentRepository) DeleteIndexedFile(ctx context.Context, project, path string) error {
	const q = `
DELETE FROM indexed_files
WHERE project = $1 AND path = $2`
	if _, err := r.db.ExecContext(ctx, q, project, path); err != nil {
		return fmt.Errorf("delete indexed file: %w", err)
	}
	return nil
}

func (r *PostgresDocumentRepository) DeleteAutoChunksByPath(ctx context.Context, project, path string) (int64, error) {
	const q = `
DELETE FROM documents d
WHERE
	d.project = $1
	AND d.path = $2
	AND d.kind = 'chunk'
	AND COALESCE(d.tags, ARRAY[]::text[]) @> $3::text[]`

	res, err := r.db.ExecContext(ctx, q, project, path, pq.Array([]string{AutoIndexTag}))
	if err != nil {
		return 0, fmt.Errorf("delete auto chunks by path: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected auto chunks delete: %w", err)
	}
	return affected, nil
}

func toVectorLiteral(values []float64) string {
	if len(values) == 0 {
		return "[]"
	}

	var b strings.Builder
	b.Grow(len(values) * 8)
	b.WriteByte('[')
	for i, v := range values {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.8f", v), "0"), "."))
	}
	b.WriteByte(']')
	return b.String()
}
