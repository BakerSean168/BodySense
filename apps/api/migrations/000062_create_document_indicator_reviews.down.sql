DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM document_indicator_reviews LIMIT 1) THEN
        RAISE EXCEPTION
            'cannot downgrade document indicator review schema while review history exists';
    END IF;
END
$$;

DROP TABLE IF EXISTS document_indicator_reviews;
