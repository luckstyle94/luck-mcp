CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS documents (
    id BIGSERIAL PRIMARY KEY,
    project TEXT NOT NULL,
    kind TEXT NOT NULL,
    path TEXT NULL,
    tags TEXT[] NULL,
    content TEXT NOT NULL,
    importance SMALLINT NOT NULL DEFAULT 0,
    content_hash TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT documents_kind_check CHECK (kind IN ('note', 'chunk', 'summary', 'memory')),
    CONSTRAINT documents_importance_check CHECK (importance BETWEEN 0 AND 5)
);

CREATE TABLE IF NOT EXISTS doc_embeddings (
    doc_id BIGINT PRIMARY KEY REFERENCES documents(id) ON DELETE CASCADE,
    embedding VECTOR(768) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_doc_embeddings_embedding_ivfflat
    ON doc_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

CREATE INDEX IF NOT EXISTS idx_documents_project ON documents(project);
CREATE INDEX IF NOT EXISTS idx_documents_kind ON documents(kind);
CREATE INDEX IF NOT EXISTS idx_documents_path ON documents(path);
CREATE INDEX IF NOT EXISTS idx_documents_tags_gin ON documents USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_documents_project_hash ON documents(project, content_hash);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_documents_updated_at ON documents;
CREATE TRIGGER trg_documents_updated_at
BEFORE UPDATE ON documents
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
