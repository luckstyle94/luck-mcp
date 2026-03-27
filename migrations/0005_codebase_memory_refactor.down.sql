DROP INDEX IF EXISTS idx_indexed_files_path_trgm;
DROP INDEX IF EXISTS idx_indexed_files_repo_file_type_language;
DROP INDEX IF EXISTS idx_indexed_files_repo_status;
DROP INDEX IF EXISTS uq_indexed_files_repo_path;
ALTER TABLE indexed_files DROP COLUMN IF EXISTS size_bytes;
ALTER TABLE indexed_files DROP COLUMN IF EXISTS file_type;
ALTER TABLE indexed_files DROP COLUMN IF EXISTS language;
ALTER TABLE indexed_files DROP COLUMN IF EXISTS repo_id;

DROP INDEX IF EXISTS idx_chunk_embeddings_ivfflat;
DROP TABLE IF EXISTS chunk_embeddings;

DROP INDEX IF EXISTS idx_indexed_chunks_path_trgm;
DROP INDEX IF EXISTS idx_indexed_chunks_content_trgm;
DROP INDEX IF EXISTS idx_indexed_chunks_tags_gin;
DROP INDEX IF EXISTS idx_indexed_chunks_repo_path;
DROP INDEX IF EXISTS uq_indexed_chunks_repo_path_chunk;
DROP TABLE IF EXISTS indexed_chunks;

DROP INDEX IF EXISTS idx_memory_embeddings_ivfflat;
DROP TABLE IF EXISTS memory_embeddings;

DROP INDEX IF EXISTS idx_memory_entries_tags_gin;
DROP INDEX IF EXISTS idx_memory_entries_path;
DROP INDEX IF EXISTS idx_memory_entries_kind;
DROP INDEX IF EXISTS idx_memory_entries_repo_id;
DROP INDEX IF EXISTS uq_memory_entries_repo_hash;
DROP TRIGGER IF EXISTS trg_memory_entries_updated_at ON memory_entries;
DROP TABLE IF EXISTS memory_entries;

DROP TRIGGER IF EXISTS trg_repos_updated_at ON repos;
DROP TABLE IF EXISTS repos;
