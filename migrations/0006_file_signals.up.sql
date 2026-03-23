CREATE TABLE IF NOT EXISTS file_signals (
    id BIGSERIAL PRIMARY KEY,
    repo_id BIGINT NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    signal_type TEXT NOT NULL,
    value TEXT NOT NULL,
    normalized_value TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_file_signals_repo_path
    ON file_signals(repo_id, path);

CREATE INDEX IF NOT EXISTS idx_file_signals_type_value
    ON file_signals(repo_id, signal_type, normalized_value);

CREATE INDEX IF NOT EXISTS idx_file_signals_value_trgm
    ON file_signals USING GIN(normalized_value gin_trgm_ops);
