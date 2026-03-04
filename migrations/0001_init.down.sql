DROP TRIGGER IF EXISTS trg_documents_updated_at ON documents;
DROP FUNCTION IF EXISTS set_updated_at;

DROP TABLE IF EXISTS doc_embeddings;
DROP TABLE IF EXISTS documents;
