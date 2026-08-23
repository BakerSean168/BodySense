CREATE TABLE knowledge_publication_observations (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    publication_id UUID NOT NULL REFERENCES knowledge_publications(id) ON DELETE CASCADE,
    observation_key VARCHAR(200) NOT NULL,
    observation_kind VARCHAR(32) NOT NULL DEFAULT 'predeploy_eval',
    evaluator_revision VARCHAR(100) NOT NULL,
    case_id VARCHAR(120) NOT NULL,
    retrieval_status VARCHAR(32) NOT NULL,
    citation_status VARCHAR(32) NOT NULL,
    grounding_status VARCHAR(32) NOT NULL,
    identity_status VARCHAR(32) NOT NULL,
    provenance_status VARCHAR(32) NOT NULL,
    execution_error TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (publication_id, observation_key)
);

CREATE INDEX idx_knowledge_publication_observations_publication_kind_created
    ON knowledge_publication_observations (publication_id, observation_kind, created_at DESC);
