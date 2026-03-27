package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"luck-mpc/internal/domain"
)

type PostgresDocumentRepository struct {
	db *sql.DB
}

func NewPostgresDocumentRepository(db *sql.DB) *PostgresDocumentRepository {
	return &PostgresDocumentRepository{db: db}
}

func (r *PostgresDocumentRepository) EnsureRepo(ctx context.Context, name string, rootPath *string) (domain.Repo, error) {
	active := true
	return r.UpsertRepo(ctx, UpsertRepoInput{
		Name:     name,
		RootPath: rootPath,
		Active:   &active,
	})
}

func (r *PostgresDocumentRepository) UpsertRepo(ctx context.Context, input UpsertRepoInput) (domain.Repo, error) {
	const q = `
INSERT INTO repos (name, root_path, description, tags, active)
VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4, COALESCE($5, TRUE))
ON CONFLICT (name)
DO UPDATE SET
	root_path = COALESCE(NULLIF(EXCLUDED.root_path, ''), repos.root_path),
	description = COALESCE(NULLIF(EXCLUDED.description, ''), repos.description),
	tags = CASE WHEN COALESCE(array_length(EXCLUDED.tags, 1), 0) > 0 THEN EXCLUDED.tags ELSE repos.tags END,
	active = COALESCE(EXCLUDED.active, repos.active),
	updated_at = NOW()
RETURNING id, name, root_path, description, COALESCE(tags, ARRAY[]::text[]), active, last_indexed_at, created_at, updated_at`

	var repo domain.Repo
	var tags pq.StringArray
	if err := r.db.QueryRowContext(
		ctx,
		q,
		strings.TrimSpace(input.Name),
		nullableStringValue(input.RootPath),
		nullableStringValue(input.Description),
		nullableStringArray(input.Tags),
		input.Active,
	).Scan(
		&repo.ID,
		&repo.Name,
		&repo.RootPath,
		&repo.Description,
		&tags,
		&repo.Active,
		&repo.LastIndexedAt,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	); err != nil {
		return domain.Repo{}, fmt.Errorf("upsert repo: %w", err)
	}
	repo.Tags = []string(tags)
	return repo, nil
}

func (r *PostgresDocumentRepository) GetRepoByName(ctx context.Context, name string) (domain.Repo, bool, error) {
	const q = `
SELECT id, name, root_path, description, COALESCE(tags, ARRAY[]::text[]), active, last_indexed_at, created_at, updated_at
FROM repos
WHERE name = $1`

	var repo domain.Repo
	var tags pq.StringArray
	if err := r.db.QueryRowContext(ctx, q, strings.TrimSpace(name)).Scan(
		&repo.ID,
		&repo.Name,
		&repo.RootPath,
		&repo.Description,
		&tags,
		&repo.Active,
		&repo.LastIndexedAt,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return domain.Repo{}, false, nil
		}
		return domain.Repo{}, false, fmt.Errorf("get repo by name: %w", err)
	}
	repo.Tags = []string(tags)
	return repo, true, nil
}

func (r *PostgresDocumentRepository) ListRepos(ctx context.Context) ([]domain.Repo, error) {
	const q = `
SELECT id, name, root_path, description, COALESCE(tags, ARRAY[]::text[]), active, last_indexed_at, created_at, updated_at
FROM repos
WHERE active = TRUE
ORDER BY name ASC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list repos: %w", err)
	}
	defer rows.Close()

	repos := make([]domain.Repo, 0)
	for rows.Next() {
		var repo domain.Repo
		var tags pq.StringArray
		if err := rows.Scan(
			&repo.ID,
			&repo.Name,
			&repo.RootPath,
			&repo.Description,
			&tags,
			&repo.Active,
			&repo.LastIndexedAt,
			&repo.CreatedAt,
			&repo.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan repo: %w", err)
		}
		repo.Tags = []string(tags)
		repos = append(repos, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repos: %w", err)
	}
	return repos, nil
}

func (r *PostgresDocumentRepository) FindMemoryByRepoAndContentHash(ctx context.Context, repoID int64, contentHash string) (int64, bool, error) {
	const q = `
SELECT id
FROM memory_entries
WHERE repo_id = $1 AND content_hash = $2
ORDER BY updated_at DESC
LIMIT 1`

	var id int64
	err := r.db.QueryRowContext(ctx, q, repoID, contentHash).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("find memory by hash: %w", err)
	}
	return id, true, nil
}

func (r *PostgresDocumentRepository) InsertMemoryWithEmbedding(ctx context.Context, input AddMemoryInput) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const insertEntry = `
INSERT INTO memory_entries (repo_id, kind, path, tags, content, importance, content_hash, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, NOW()), COALESCE($9, NOW()))
ON CONFLICT (repo_id, content_hash) WHERE content_hash IS NOT NULL
DO UPDATE SET updated_at = NOW()
RETURNING id`

	var id int64
	if err := tx.QueryRowContext(
		ctx,
		insertEntry,
		input.RepoID,
		string(input.Kind),
		input.Path,
		pq.Array(input.Tags),
		input.Content,
		input.Importance,
		input.ContentHash,
		input.CreatedAt,
		input.UpdatedAt,
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert memory entry: %w", err)
	}

	const insertEmbedding = `
INSERT INTO memory_embeddings (entry_id, embedding)
VALUES ($1, $2::vector)
ON CONFLICT (entry_id) DO NOTHING`

	if _, err := tx.ExecContext(ctx, insertEmbedding, id, toVectorLiteral(input.Embedding)); err != nil {
		return 0, fmt.Errorf("insert memory embedding: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return id, nil
}

func (r *PostgresDocumentRepository) SearchMemory(ctx context.Context, input SearchMemoryInput) ([]domain.SearchResult, error) {
	whereClauses := []string{"m.repo_id = $1"}
	args := []any{input.RepoID, toVectorLiteral(input.QueryEmbedding)}
	argPos := 3

	if input.PathPrefix != nil && strings.TrimSpace(*input.PathPrefix) != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("m.path IS NOT NULL AND left(m.path, char_length($%d)) = $%d", argPos, argPos))
		args = append(args, strings.TrimSpace(*input.PathPrefix))
		argPos++
	}

	if len(input.Tags) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("COALESCE(m.tags, ARRAY[]::text[]) @> $%d::text[]", argPos))
		args = append(args, pq.Array(input.Tags))
		argPos++
	}

	if input.Kind != "" && input.Kind != domain.KindAny {
		whereClauses = append(whereClauses, fmt.Sprintf("m.kind = $%d", argPos))
		args = append(args, string(input.Kind))
		argPos++
	}

	limitPos := argPos
	args = append(args, input.K)

	query := fmt.Sprintf(`
SELECT
	m.id,
	(1 - (e.embedding <=> $2::vector)) AS score,
	m.kind,
	m.path,
	COALESCE(m.tags, ARRAY[]::text[]),
	m.content
FROM memory_entries m
JOIN memory_embeddings e ON e.entry_id = m.id
WHERE %s
ORDER BY e.embedding <=> $2::vector ASC
LIMIT $%d`, strings.Join(whereClauses, " AND "), limitPos)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search memory entries: %w", err)
	}
	defer rows.Close()

	results := make([]domain.SearchResult, 0, input.K)
	for rows.Next() {
		var item domain.SearchResult
		var tags pq.StringArray
		if err := rows.Scan(&item.ID, &item.Score, &item.Kind, &item.Path, &tags, &item.Content); err != nil {
			return nil, fmt.Errorf("scan memory search row: %w", err)
		}
		item.Tags = []string(tags)
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory search rows: %w", err)
	}
	return results, nil
}

func (r *PostgresDocumentRepository) ListMemoryBriefItems(ctx context.Context, repoID int64, maxItems int) ([]domain.BriefItem, error) {
	const q = `
SELECT
	kind,
	path,
	COALESCE(tags, ARRAY[]::text[]),
	content,
	importance,
	updated_at
FROM memory_entries
WHERE repo_id = $1
ORDER BY
	CASE WHEN kind = 'summary' THEN 1 ELSE 0 END DESC,
	importance DESC,
	updated_at DESC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, q, repoID, maxItems)
	if err != nil {
		return nil, fmt.Errorf("list memory brief items: %w", err)
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
		return nil, fmt.Errorf("iterate brief items: %w", err)
	}
	return items, nil
}

func (r *PostgresDocumentRepository) ListIndexedFiles(ctx context.Context, repoID int64) ([]IndexedFile, error) {
	const q = `
SELECT path, content_hash, language, file_type, size_bytes, chunk_count, status
FROM indexed_files
WHERE repo_id = $1
  AND status = 'indexed'`

	rows, err := r.db.QueryContext(ctx, q, repoID)
	if err != nil {
		return nil, fmt.Errorf("list indexed files: %w", err)
	}
	defer rows.Close()

	files := make([]IndexedFile, 0)
	for rows.Next() {
		var f IndexedFile
		if err := rows.Scan(&f.Path, &f.ContentHash, &f.Language, &f.FileType, &f.SizeBytes, &f.ChunkCount, &f.Status); err != nil {
			return nil, fmt.Errorf("scan indexed file: %w", err)
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate indexed files: %w", err)
	}
	return files, nil
}

func (r *PostgresDocumentRepository) UpsertIndexedFile(ctx context.Context, input UpsertIndexedFileInput) error {
	indexedAt := time.Now()
	if input.LastIndexedAt != nil {
		indexedAt = *input.LastIndexedAt
	}

	const q = `
INSERT INTO indexed_files (project, repo_id, path, content_hash, language, file_type, size_bytes, chunk_count, status, error, indexed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (repo_id, path)
DO UPDATE SET
	project = EXCLUDED.project,
	content_hash = EXCLUDED.content_hash,
	language = EXCLUDED.language,
	file_type = EXCLUDED.file_type,
	size_bytes = EXCLUDED.size_bytes,
	chunk_count = EXCLUDED.chunk_count,
	status = EXCLUDED.status,
	error = EXCLUDED.error,
	indexed_at = EXCLUDED.indexed_at`

	if _, err := r.db.ExecContext(ctx, q,
		input.Project,
		input.RepoID,
		input.Path,
		input.ContentHash,
		input.Language,
		input.FileType,
		input.SizeBytes,
		input.ChunkCount,
		input.Status,
		input.Error,
		indexedAt,
	); err != nil {
		return fmt.Errorf("upsert indexed file: %w", err)
	}

	if input.Status == "indexed" {
		const updateRepo = `
UPDATE repos
SET last_indexed_at = GREATEST(COALESCE(last_indexed_at, $2), $2), updated_at = NOW()
WHERE id = $1`
		if _, err := r.db.ExecContext(ctx, updateRepo, input.RepoID, indexedAt); err != nil {
			return fmt.Errorf("update repo last_indexed_at: %w", err)
		}
	}
	return nil
}

func (r *PostgresDocumentRepository) DeleteIndexedFile(ctx context.Context, repoID int64, path string) error {
	const q = `
DELETE FROM indexed_files
WHERE repo_id = $1 AND path = $2`
	if _, err := r.db.ExecContext(ctx, q, repoID, path); err != nil {
		return fmt.Errorf("delete indexed file: %w", err)
	}
	return nil
}

func (r *PostgresDocumentRepository) DeleteIndexedChunksByPath(ctx context.Context, repoID int64, path string) (int64, error) {
	const q = `
DELETE FROM indexed_chunks
WHERE repo_id = $1 AND path = $2`
	res, err := r.db.ExecContext(ctx, q, repoID, path)
	if err != nil {
		return 0, fmt.Errorf("delete indexed chunks by path: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected indexed chunk delete: %w", err)
	}
	return affected, nil
}

func (r *PostgresDocumentRepository) ReplaceFileSignals(ctx context.Context, repoID int64, path string, signals []FileSignalInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const deleteSignals = `
DELETE FROM file_signals
WHERE repo_id = $1 AND path = $2`
	if _, err := tx.ExecContext(ctx, deleteSignals, repoID, path); err != nil {
		return fmt.Errorf("delete file signals: %w", err)
	}

	if len(signals) > 0 {
		const insertSignal = `
INSERT INTO file_signals (repo_id, path, signal_type, value, normalized_value)
VALUES ($1, $2, $3, $4, $5)`
		for _, signal := range signals {
			if _, err := tx.ExecContext(ctx, insertSignal, repoID, path, signal.SignalType, signal.Value, signal.NormalizedValue); err != nil {
				return fmt.Errorf("insert file signal: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *PostgresDocumentRepository) DeleteFileSignalsByPath(ctx context.Context, repoID int64, path string) error {
	const q = `
DELETE FROM file_signals
WHERE repo_id = $1 AND path = $2`
	if _, err := r.db.ExecContext(ctx, q, repoID, path); err != nil {
		return fmt.Errorf("delete file signals by path: %w", err)
	}
	return nil
}

func (r *PostgresDocumentRepository) InsertIndexedChunkWithEmbedding(ctx context.Context, input AddIndexedChunkInput) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const insertChunk = `
INSERT INTO indexed_chunks (repo_id, path, chunk_index, tags, content, content_hash)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (repo_id, path, chunk_index)
DO UPDATE SET
	tags = EXCLUDED.tags,
	content = EXCLUDED.content,
	content_hash = EXCLUDED.content_hash
RETURNING id`

	var id int64
	if err := tx.QueryRowContext(ctx, insertChunk,
		input.RepoID,
		input.Path,
		input.ChunkIndex,
		pq.Array(input.Tags),
		input.Content,
		input.ContentHash,
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert indexed chunk: %w", err)
	}

	const insertEmbedding = `
INSERT INTO chunk_embeddings (chunk_id, embedding)
VALUES ($1, $2::vector)
ON CONFLICT (chunk_id)
DO UPDATE SET embedding = EXCLUDED.embedding`

	if _, err := tx.ExecContext(ctx, insertEmbedding, id, toVectorLiteral(input.Embedding)); err != nil {
		return 0, fmt.Errorf("insert chunk embedding: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return id, nil
}

func (r *PostgresDocumentRepository) SearchIndexedChunks(ctx context.Context, input SearchIndexedChunksInput) ([]domain.RepoSearchResult, error) {
	mode := input.Mode
	if !mode.IsValid() {
		mode = domain.SearchModeHybrid
	}

	filters := []string{"r.active = TRUE"}
	args := make([]any, 0, 8)
	argPos := 1

	if len(input.RepoNames) > 0 {
		filters = append(filters, fmt.Sprintf("r.name = ANY($%d::text[])", argPos))
		args = append(args, pq.Array(input.RepoNames))
		argPos++
	}
	if input.PathPrefix != nil && strings.TrimSpace(*input.PathPrefix) != "" {
		filters = append(filters, fmt.Sprintf("left(c.path, char_length($%d)) = $%d", argPos, argPos))
		args = append(args, strings.TrimSpace(*input.PathPrefix))
		argPos++
	}
	if ft := strings.TrimSpace(strings.ToLower(input.FileType)); ft != "" && ft != "any" {
		filters = append(filters, fmt.Sprintf("f.file_type = $%d", argPos))
		args = append(args, ft)
		argPos++
	}
	if lang := strings.TrimSpace(strings.ToLower(input.Language)); lang != "" && lang != "any" {
		filters = append(filters, fmt.Sprintf("f.language = $%d", argPos))
		args = append(args, lang)
		argPos++
	}

	queryPos := argPos
	args = append(args, input.Query)
	argPos++

	limitPos := argPos
	var query string

	switch mode {
	case domain.SearchModeText:
		args = append(args, input.K)
		query = fmt.Sprintf(`
SELECT
	r.name,
	c.path,
	(
		GREATEST(
		similarity(c.content, $%d),
		similarity(c.path, $%d),
		COALESCE((
			SELECT MAX(similarity(fs.normalized_value, lower($%d)))
			FROM file_signals fs
			WHERE fs.repo_id = c.repo_id AND fs.path = c.path
		), 0)
		) +
		CASE WHEN c.path ILIKE '%%' || $%d || '%%' THEN 0.18 ELSE 0 END +
		CASE WHEN EXISTS (
			SELECT 1
			FROM file_signals fs
			WHERE fs.repo_id = c.repo_id AND fs.path = c.path AND fs.normalized_value = lower($%d)
		) THEN 0.28 ELSE 0 END +
		CASE WHEN EXISTS (
			SELECT 1
			FROM file_signals fs
			WHERE fs.repo_id = c.repo_id AND fs.path = c.path AND fs.normalized_value LIKE '%%' || lower($%d) || '%%'
		) THEN 0.14 ELSE 0 END +
		CASE
			WHEN lower(c.path) = '.terraform.lock.hcl' OR lower(c.path) LIKE '%%/.terraform.lock.hcl' THEN -0.22
			WHEN lower(c.path) = '.terraformignore' OR lower(c.path) LIKE '%%/.terraformignore' THEN -0.18
			WHEN lower(c.path) = '.gitignore' OR lower(c.path) LIKE '%%/.gitignore' THEN -0.18
			WHEN lower(c.path) = 'go.sum' OR lower(c.path) LIKE '%%/go.sum' THEN -0.12
			WHEN lower(c.path) = 'package-lock.json' OR lower(c.path) LIKE '%%/package-lock.json' THEN -0.12
			WHEN lower(c.path) = 'pnpm-lock.yaml' OR lower(c.path) LIKE '%%/pnpm-lock.yaml' THEN -0.12
			WHEN lower(c.path) = 'yarn.lock' OR lower(c.path) LIKE '%%/yarn.lock' THEN -0.12
			WHEN lower(c.path) = 'poetry.lock' OR lower(c.path) LIKE '%%/poetry.lock' THEN -0.12
			ELSE 0
		END
	) AS score,
	f.file_type,
	f.language,
	COALESCE(c.tags, ARRAY[]::text[]),
	c.content
FROM indexed_chunks c
JOIN repos r ON r.id = c.repo_id
JOIN indexed_files f ON f.repo_id = c.repo_id AND f.path = c.path
WHERE %s
  AND (
	c.content ILIKE '%%' || $%d || '%%'
	OR c.path ILIKE '%%' || $%d || '%%'
	OR similarity(c.content, $%d) > 0.1
	OR similarity(c.path, $%d) > 0.1
	)
	ORDER BY score DESC, r.name ASC, c.path ASC
	LIMIT $%d`, queryPos, queryPos, queryPos, queryPos, queryPos, queryPos, strings.Join(filters, " AND "), queryPos, queryPos, queryPos, queryPos, limitPos)
	case domain.SearchModeSemantic:
		embPos := argPos
		args = append(args, toVectorLiteral(input.QueryEmbedding))
		argPos++
		limitPos = argPos
		args = append(args, input.K)
		query = fmt.Sprintf(`
SELECT
	r.name,
	c.path,
	(
		((1 - (e.embedding <=> $%d::vector)) * 0.85) +
		(COALESCE((
			SELECT MAX(similarity(fs.normalized_value, lower($%d)))
			FROM file_signals fs
			WHERE fs.repo_id = c.repo_id AND fs.path = c.path
		), 0) * 0.15) +
		CASE WHEN c.path ILIKE '%%' || $%d || '%%' THEN 0.08 ELSE 0 END +
		CASE WHEN EXISTS (
			SELECT 1
			FROM file_signals fs
			WHERE fs.repo_id = c.repo_id AND fs.path = c.path AND fs.normalized_value = lower($%d)
		) THEN 0.12 ELSE 0 END +
		CASE
			WHEN lower(c.path) = '.terraform.lock.hcl' OR lower(c.path) LIKE '%%/.terraform.lock.hcl' THEN -0.18
			WHEN lower(c.path) = '.terraformignore' OR lower(c.path) LIKE '%%/.terraformignore' THEN -0.15
			WHEN lower(c.path) = '.gitignore' OR lower(c.path) LIKE '%%/.gitignore' THEN -0.15
			ELSE 0
		END
	) AS score,
	f.file_type,
	f.language,
	COALESCE(c.tags, ARRAY[]::text[]),
	c.content
FROM indexed_chunks c
JOIN chunk_embeddings e ON e.chunk_id = c.id
JOIN repos r ON r.id = c.repo_id
JOIN indexed_files f ON f.repo_id = c.repo_id AND f.path = c.path
WHERE %s
ORDER BY e.embedding <=> $%d::vector ASC, r.name ASC, c.path ASC
	LIMIT $%d`, embPos, queryPos, queryPos, queryPos, strings.Join(filters, " AND "), embPos, limitPos)
	default:
		embPos := argPos
		args = append(args, toVectorLiteral(input.QueryEmbedding))
		argPos++
		limitPos = argPos
		args = append(args, input.K)
		query = fmt.Sprintf(`
SELECT
	r.name,
	c.path,
	(
		((1 - (e.embedding <=> $%d::vector)) * 0.55) +
		(GREATEST(similarity(c.content, $%d), similarity(c.path, $%d)) * 0.25) +
		(COALESCE((
			SELECT MAX(similarity(fs.normalized_value, lower($%d)))
			FROM file_signals fs
			WHERE fs.repo_id = c.repo_id AND fs.path = c.path
		), 0) * 0.20) +
		CASE WHEN c.path ILIKE '%%' || $%d || '%%' THEN 0.12 ELSE 0 END +
		CASE WHEN EXISTS (
			SELECT 1
			FROM file_signals fs
			WHERE fs.repo_id = c.repo_id AND fs.path = c.path AND fs.normalized_value = lower($%d)
		) THEN 0.20 ELSE 0 END +
		CASE WHEN EXISTS (
			SELECT 1
			FROM file_signals fs
			WHERE fs.repo_id = c.repo_id AND fs.path = c.path AND fs.normalized_value LIKE '%%' || lower($%d) || '%%'
		) THEN 0.10 ELSE 0 END +
		CASE
			WHEN lower(c.path) = '.terraform.lock.hcl' OR lower(c.path) LIKE '%%/.terraform.lock.hcl' THEN -0.20
			WHEN lower(c.path) = '.terraformignore' OR lower(c.path) LIKE '%%/.terraformignore' THEN -0.16
			WHEN lower(c.path) = '.gitignore' OR lower(c.path) LIKE '%%/.gitignore' THEN -0.16
			WHEN lower(c.path) = 'go.sum' OR lower(c.path) LIKE '%%/go.sum' THEN -0.10
			ELSE 0
		END
	) AS score,
	f.file_type,
	f.language,
	COALESCE(c.tags, ARRAY[]::text[]),
	c.content
FROM indexed_chunks c
JOIN chunk_embeddings e ON e.chunk_id = c.id
JOIN repos r ON r.id = c.repo_id
JOIN indexed_files f ON f.repo_id = c.repo_id AND f.path = c.path
WHERE %s
ORDER BY score DESC, r.name ASC, c.path ASC
	LIMIT $%d`, embPos, queryPos, queryPos, queryPos, queryPos, queryPos, queryPos, strings.Join(filters, " AND "), limitPos)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search indexed chunks: %w", err)
	}
	defer rows.Close()

	results := make([]domain.RepoSearchResult, 0, input.K)
	for rows.Next() {
		var item domain.RepoSearchResult
		var tags pq.StringArray
		if err := rows.Scan(&item.Repo, &item.Path, &item.Score, &item.FileType, &item.Language, &tags, &item.Content); err != nil {
			return nil, fmt.Errorf("scan indexed chunk search row: %w", err)
		}
		item.Tags = []string(tags)
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate indexed chunk rows: %w", err)
	}
	return results, nil
}

func (r *PostgresDocumentRepository) FindFiles(ctx context.Context, input FindFilesInput) ([]domain.FileMatch, error) {
	filters := []string{"r.active = TRUE", "f.status = 'indexed'"}
	args := make([]any, 0, 8)
	argPos := 1

	if len(input.RepoNames) > 0 {
		filters = append(filters, fmt.Sprintf("r.name = ANY($%d::text[])", argPos))
		args = append(args, pq.Array(input.RepoNames))
		argPos++
	}
	if input.PathPrefix != nil && strings.TrimSpace(*input.PathPrefix) != "" {
		filters = append(filters, fmt.Sprintf("left(f.path, char_length($%d)) = $%d", argPos, argPos))
		args = append(args, strings.TrimSpace(*input.PathPrefix))
		argPos++
	}
	if ft := strings.TrimSpace(strings.ToLower(input.FileType)); ft != "" && ft != "any" {
		filters = append(filters, fmt.Sprintf("f.file_type = $%d", argPos))
		args = append(args, ft)
		argPos++
	}
	if lang := strings.TrimSpace(strings.ToLower(input.Language)); lang != "" && lang != "any" {
		filters = append(filters, fmt.Sprintf("f.language = $%d", argPos))
		args = append(args, lang)
		argPos++
	}

	queryPos := argPos
	args = append(args, input.Query)
	argPos++

	limitPos := argPos
	args = append(args, input.K)

	const snippetCap = 240
	query := fmt.Sprintf(`
SELECT
	r.name,
	f.path,
	(
		GREATEST(
			similarity(f.path, $%d),
			COALESCE(MAX(similarity(c.content, $%d)), 0),
			COALESCE(MAX(similarity(fs.normalized_value, lower($%d))), 0)
		) +
		CASE WHEN lower(f.path) LIKE '%%' || lower($%d) || '%%' THEN 0.18 ELSE 0 END +
		CASE WHEN COALESCE(MAX(CASE WHEN fs.normalized_value = lower($%d) THEN 1 ELSE 0 END), 0) = 1 THEN 0.30 ELSE 0 END +
		CASE WHEN COALESCE(MAX(CASE WHEN fs.normalized_value LIKE '%%' || lower($%d) || '%%' THEN 1 ELSE 0 END), 0) = 1 THEN 0.12 ELSE 0 END +
		CASE WHEN COALESCE(MAX(CASE WHEN fs.signal_type IN ('endpoint', 'http_route', 'http_client_call', 'terraform_source', 'import_ref', 'route_path', 'react_component', 'ansible_module', 'ansible_role', 'openapi_path') THEN similarity(fs.normalized_value, lower($%d)) END), 0) > 0.2 THEN 0.08 ELSE 0 END +
		CASE WHEN f.file_type = 'doc' AND lower(f.path) LIKE '%%readme%%' THEN 0.18 ELSE 0 END +
		CASE WHEN f.file_type = 'doc' AND lower(f.path) LIKE '%%adr%%' THEN 0.14 ELSE 0 END +
		CASE WHEN f.file_type = 'doc' AND lower(f.path) LIKE 'docs/%%' THEN 0.10 ELSE 0 END +
		CASE WHEN f.file_type = 'doc' AND COALESCE(MAX(CASE WHEN fs.signal_type = 'doc_heading' THEN similarity(fs.normalized_value, lower($%d)) END), 0) > 0.2 THEN 0.08 ELSE 0 END +
		CASE
			WHEN lower(f.path) = '.terraform.lock.hcl' OR lower(f.path) LIKE '%%/.terraform.lock.hcl' THEN -0.22
			WHEN lower(f.path) = '.terraformignore' OR lower(f.path) LIKE '%%/.terraformignore' THEN -0.18
			WHEN lower(f.path) = '.gitignore' OR lower(f.path) LIKE '%%/.gitignore' THEN -0.18
			WHEN lower(f.path) = 'go.sum' OR lower(f.path) LIKE '%%/go.sum' THEN -0.12
			WHEN lower(f.path) = 'package-lock.json' OR lower(f.path) LIKE '%%/package-lock.json' THEN -0.12
			WHEN lower(f.path) = 'pnpm-lock.yaml' OR lower(f.path) LIKE '%%/pnpm-lock.yaml' THEN -0.12
			WHEN lower(f.path) = 'yarn.lock' OR lower(f.path) LIKE '%%/yarn.lock' THEN -0.12
			WHEN lower(f.path) = 'poetry.lock' OR lower(f.path) LIKE '%%/poetry.lock' THEN -0.12
			ELSE 0
		END
	) AS score,
	f.file_type,
	f.language,
	f.size_bytes,
	LEFT(COALESCE((ARRAY_AGG(c.content ORDER BY similarity(c.content, $%d) DESC NULLS LAST))[1], ''), %d) AS snippet
FROM indexed_files f
JOIN repos r ON r.id = f.repo_id
LEFT JOIN indexed_chunks c ON c.repo_id = f.repo_id AND c.path = f.path
LEFT JOIN file_signals fs ON fs.repo_id = f.repo_id AND fs.path = f.path
WHERE %s
  AND (
		f.path ILIKE '%%' || $%d || '%%'
		OR similarity(f.path, $%d) > 0.1
		OR COALESCE(c.content, '') ILIKE '%%' || $%d || '%%'
		OR similarity(COALESCE(c.content, ''), $%d) > 0.1
		OR COALESCE(fs.normalized_value, '') LIKE '%%' || lower($%d) || '%%'
	)
GROUP BY r.name, f.path, f.file_type, f.language, f.size_bytes
ORDER BY score DESC, r.name ASC, f.path ASC
LIMIT $%d`, queryPos, queryPos, queryPos, queryPos, queryPos, queryPos, queryPos, queryPos, queryPos, snippetCap, strings.Join(filters, " AND "), queryPos, queryPos, queryPos, queryPos, queryPos, limitPos)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find files: %w", err)
	}
	defer rows.Close()

	results := make([]domain.FileMatch, 0, input.K)
	for rows.Next() {
		var item domain.FileMatch
		if err := rows.Scan(&item.Repo, &item.Path, &item.Score, &item.FileType, &item.Language, &item.SizeBytes, &item.Snippet); err != nil {
			return nil, fmt.Errorf("scan file match row: %w", err)
		}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate file match rows: %w", err)
	}
	return results, nil
}

func nullableStringValue(v *string) any {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func nullableStringArray(values []string) any {
	if len(values) == 0 {
		return nil
	}
	return pq.Array(values)
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
