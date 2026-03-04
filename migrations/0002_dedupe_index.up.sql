DO $$
BEGIN
    BEGIN
        EXECUTE 'CREATE UNIQUE INDEX IF NOT EXISTS uq_documents_project_content_hash
            ON documents(project, content_hash)
            WHERE content_hash IS NOT NULL';
    EXCEPTION
        WHEN unique_violation THEN
            RAISE NOTICE 'uq_documents_project_content_hash not created yet due to duplicate data; waiting for 0003 cleanup';
    END;
END;
$$;
