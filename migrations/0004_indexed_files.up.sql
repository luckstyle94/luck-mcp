CREATE TABLE IF NOT EXISTS indexed_files (
    project TEXT NOT NULL,
    path TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    chunk_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'indexed',
    error TEXT NULL,
    indexed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project, path)
);

CREATE INDEX IF NOT EXISTS idx_indexed_files_project_indexed_at
    ON indexed_files(project, indexed_at DESC);
