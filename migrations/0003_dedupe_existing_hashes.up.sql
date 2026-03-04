WITH ranked AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY project, content_hash
            ORDER BY updated_at DESC, id DESC
        ) AS rn
    FROM documents
    WHERE content_hash IS NOT NULL
),
to_delete AS (
    SELECT id
    FROM ranked
    WHERE rn > 1
)
DELETE FROM documents d
USING to_delete td
WHERE d.id = td.id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_documents_project_content_hash
    ON documents(project, content_hash)
    WHERE content_hash IS NOT NULL;
