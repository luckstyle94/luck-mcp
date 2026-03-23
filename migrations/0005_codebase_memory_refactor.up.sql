CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS repos (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    root_path TEXT NULL,
    description TEXT NULL,
    tags TEXT[] NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    last_indexed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS trg_repos_updated_at ON repos;
CREATE TRIGGER trg_repos_updated_at
BEFORE UPDATE ON repos
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

INSERT INTO repos (name)
SELECT DISTINCT project
FROM (
    SELECT project FROM documents
    UNION
    SELECT project FROM indexed_files
) p
WHERE project IS NOT NULL AND BTRIM(project) <> ''
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS memory_entries (
    id BIGSERIAL PRIMARY KEY,
    repo_id BIGINT NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    path TEXT NULL,
    tags TEXT[] NULL,
    content TEXT NOT NULL,
    importance SMALLINT NOT NULL DEFAULT 0,
    content_hash TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT memory_entries_kind_check CHECK (kind IN ('note', 'chunk', 'summary', 'memory')),
    CONSTRAINT memory_entries_importance_check CHECK (importance BETWEEN 0 AND 5)
);

DROP TRIGGER IF EXISTS trg_memory_entries_updated_at ON memory_entries;
CREATE TRIGGER trg_memory_entries_updated_at
BEFORE UPDATE ON memory_entries
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE UNIQUE INDEX IF NOT EXISTS uq_memory_entries_repo_hash
    ON memory_entries(repo_id, content_hash)
    WHERE content_hash IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_memory_entries_repo_id ON memory_entries(repo_id);
CREATE INDEX IF NOT EXISTS idx_memory_entries_kind ON memory_entries(kind);
CREATE INDEX IF NOT EXISTS idx_memory_entries_path ON memory_entries(path);
CREATE INDEX IF NOT EXISTS idx_memory_entries_tags_gin ON memory_entries USING GIN(tags);

CREATE TABLE IF NOT EXISTS memory_embeddings (
    entry_id BIGINT PRIMARY KEY REFERENCES memory_entries(id) ON DELETE CASCADE,
    embedding VECTOR(768) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memory_embeddings_ivfflat
    ON memory_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

CREATE TABLE IF NOT EXISTS indexed_chunks (
    id BIGSERIAL PRIMARY KEY,
    repo_id BIGINT NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    tags TEXT[] NULL,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_indexed_chunks_repo_path_chunk
    ON indexed_chunks(repo_id, path, chunk_index);
CREATE INDEX IF NOT EXISTS idx_indexed_chunks_repo_path ON indexed_chunks(repo_id, path);
CREATE INDEX IF NOT EXISTS idx_indexed_chunks_tags_gin ON indexed_chunks USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_indexed_chunks_content_trgm ON indexed_chunks USING GIN(content gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_indexed_chunks_path_trgm ON indexed_chunks USING GIN(path gin_trgm_ops);

CREATE TABLE IF NOT EXISTS chunk_embeddings (
    chunk_id BIGINT PRIMARY KEY REFERENCES indexed_chunks(id) ON DELETE CASCADE,
    embedding VECTOR(768) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_chunk_embeddings_ivfflat
    ON chunk_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

ALTER TABLE indexed_files ADD COLUMN IF NOT EXISTS repo_id BIGINT NULL REFERENCES repos(id) ON DELETE CASCADE;
ALTER TABLE indexed_files ADD COLUMN IF NOT EXISTS language TEXT NOT NULL DEFAULT 'text';
ALTER TABLE indexed_files ADD COLUMN IF NOT EXISTS file_type TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE indexed_files ADD COLUMN IF NOT EXISTS size_bytes BIGINT NOT NULL DEFAULT 0;

UPDATE indexed_files f
SET repo_id = r.id
FROM repos r
WHERE f.repo_id IS NULL AND r.name = f.project;

CREATE UNIQUE INDEX IF NOT EXISTS uq_indexed_files_repo_path ON indexed_files(repo_id, path);
CREATE INDEX IF NOT EXISTS idx_indexed_files_repo_status ON indexed_files(repo_id, status);
CREATE INDEX IF NOT EXISTS idx_indexed_files_repo_file_type_language ON indexed_files(repo_id, file_type, language);
CREATE INDEX IF NOT EXISTS idx_indexed_files_path_trgm ON indexed_files USING GIN(path gin_trgm_ops);

INSERT INTO memory_entries (repo_id, kind, path, tags, content, importance, content_hash, created_at, updated_at)
SELECT r.id, d.kind, d.path, d.tags, d.content, d.importance, d.content_hash, d.created_at, d.updated_at
FROM documents d
JOIN repos r ON r.name = d.project
WHERE NOT (
    d.kind = 'chunk'
    AND COALESCE(d.tags, ARRAY[]::text[]) @> ARRAY['_auto_index']::text[]
)
ON CONFLICT (repo_id, content_hash) WHERE content_hash IS NOT NULL DO NOTHING;

INSERT INTO memory_embeddings (entry_id, embedding)
SELECT me.id, de.embedding
FROM memory_entries me
JOIN repos r ON r.id = me.repo_id
JOIN documents d ON d.project = r.name AND d.content_hash IS NOT NULL AND d.content_hash = me.content_hash
JOIN doc_embeddings de ON de.doc_id = d.id
ON CONFLICT (entry_id) DO NOTHING;
