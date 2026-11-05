-- #510 Stage B baseline: aggregate-only, read-only, no health text or source titles.
-- Usage (PostgreSQL client already inside a trusted container/host):
--   psql -X -qAt -v ON_ERROR_STOP=1 -v environment=dev \
--        -U <db-user> -d <database> -f scripts/knowledge-baseline.sql
-- Label the environment explicitly; do not reuse a report across dev/staging/prod.
-- Never run against a DSN you have not independently identified.
BEGIN TRANSACTION READ ONLY;
SET LOCAL statement_timeout = '10s';

SELECT jsonb_pretty(
  jsonb_build_object(
    'report_schema', 'bodysense.knowledge-baseline.v1',
    'declared_environment', :'environment',
    'captured_at_utc', to_char(current_timestamp AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
    'transaction_read_only', current_setting('transaction_read_only') = 'on',
    'schema_migration', (SELECT max(version) FROM public.schema_migrations),
    'G1_source_types_and_licenses',
      (SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.source_type, x.license_status, x.ingest_status), '[]'::jsonb)
       FROM (
         SELECT source_type, license_status, ingest_status, count(*) AS sources
         FROM public.knowledge_sources GROUP BY source_type, license_status, ingest_status
       ) AS x),
    'G2_unit_lifecycle',
      (SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.lifecycle_status, x.review_status), '[]'::jsonb)
       FROM (
         SELECT lifecycle_status, review_status, count(*) AS units
         FROM public.knowledge_units GROUP BY lifecycle_status, review_status
       ) AS x),
    'G3_publications',
      jsonb_build_object(
        'total_registered_sources', (SELECT count(*) FROM public.knowledge_sources),
        'total_units', (SELECT count(*) FROM public.knowledge_units),
        'published_units', (SELECT count(*) FROM public.knowledge_units WHERE lifecycle_status = 'published'),
        'publication_batches', (SELECT count(*) FROM public.knowledge_publications),
        'latest_published_at', (SELECT max(published_at) FROM public.knowledge_publications WHERE status = 'published'),
        'states',
          (SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.status), '[]'::jsonb)
           FROM (
             SELECT status, count(*) AS batches, coalesce(sum(unit_count), 0) AS units,
                    max(published_version) AS latest_version
             FROM public.knowledge_publications GROUP BY status
           ) AS x)),
    'G4_source_type_distribution',
      (SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.source_type), '[]'::jsonb)
       FROM (SELECT source_type, count(*) AS sources
             FROM public.knowledge_sources GROUP BY source_type) AS x),
    'G5_observations',
      (SELECT COALESCE(jsonb_agg(to_jsonb(x)
              ORDER BY x.observation_kind, x.retrieval_status,
                       x.citation_status, x.grounding_status), '[]'::jsonb)
       FROM (
         SELECT observation_kind, retrieval_status, citation_status, grounding_status,
                count(*) AS samples, min(created_at) AS first_sample,
                max(created_at) AS last_sample
         FROM public.knowledge_publication_observations
         GROUP BY observation_kind, retrieval_status, citation_status, grounding_status
       ) AS x),
    'G6_live_retrieval_hit_rate', NULL,
    'G6_note', 'Unmeasured: this report contains only published observations, not all real traffic.',
    'G7_stored_embedding_identities',
      (SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY x.provider, x.model), '[]'::jsonb)
       FROM (
         SELECT coalesce(nullif(metadata -> 'embedding_identity' ->> 'provider', ''), 'not_recorded') AS provider,
                coalesce(nullif(metadata -> 'embedding_identity' ->> 'model', ''), 'not_recorded') AS model,
                count(*) AS units
         FROM public.knowledge_units
         GROUP BY 1, 2
       ) AS x),
    'G7_runtime_embedding_provider', NULL,
    'G7_note', 'Stored vector metadata is not evidence of the effective AI runtime environment. Verify separately without displaying keys.'
  )
);
COMMIT;
