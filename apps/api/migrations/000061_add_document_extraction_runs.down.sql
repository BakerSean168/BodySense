DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM document_extraction_runs LIMIT 1) THEN
        RAISE EXCEPTION
            'cannot downgrade document extraction run schema while extraction history exists';
    END IF;
END
$$;

DROP TABLE IF EXISTS document_extraction_runs;
