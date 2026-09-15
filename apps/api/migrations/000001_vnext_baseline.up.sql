-- BodySense vNext canonical PostgreSQL 18 baseline.
--
-- Phase 08 intentionally resets the pre-user migration chain. Git history
-- preserves migrations 000001-000063; runtime databases are explicitly reset
-- rather than pretending that legacy application data must be upgraded.
--
-- This schema was captured from the Phase 07 accepted PG18 schema after
-- removing migration-era columns with no current runtime authority.

--
-- PostgreSQL database dump
--


-- Dumped from database version 18.6 (Debian 18.6-1.pgdg12+2)
-- Dumped by pg_dump version 18.6 (Debian 18.6-1.pgdg12+2)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;


--
-- Name: update_updated_at_column(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_updated_at_column() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: agent_interactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_interactions (
    id uuid DEFAULT uuidv7() NOT NULL,
    run_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    tool_call_id text NOT NULL,
    tool_name text DEFAULT 'ask_user'::text NOT NULL,
    question jsonb DEFAULT '{}'::jsonb NOT NULL,
    answer jsonb,
    status character varying(30) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    answered_at timestamp with time zone,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    expires_at timestamp with time zone,
    CONSTRAINT agent_interactions_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'answered'::character varying, 'cancelled'::character varying, 'expired'::character varying])::text[])))
);


--
-- Name: agent_tool_calls; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_tool_calls (
    id uuid DEFAULT uuidv7() NOT NULL,
    run_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    message_id uuid,
    tool_call_id text NOT NULL,
    tool_name text NOT NULL,
    arguments jsonb DEFAULT '{}'::jsonb NOT NULL,
    status character varying(30) DEFAULT 'running'::character varying NOT NULL,
    result jsonb,
    error jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    finished_at timestamp with time zone,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT agent_tool_calls_status_check CHECK (((status)::text = ANY ((ARRAY['running'::character varying, 'succeeded'::character varying, 'failed'::character varying])::text[])))
);


--
-- Name: ai_output_reviews; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_output_reviews (
    id uuid DEFAULT uuidv7() NOT NULL,
    run_id uuid,
    job_id uuid,
    conversation_id uuid,
    output_type character varying(50) NOT NULL,
    status character varying(30) DEFAULT 'accepted'::character varying NOT NULL,
    issues jsonb DEFAULT '[]'::jsonb NOT NULL,
    validated_output jsonb,
    raw_output jsonb,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    user_id uuid
);


--
-- Name: assessment_reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.assessment_reports (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    status character varying(40) NOT NULL,
    observations jsonb DEFAULT '[]'::jsonb NOT NULL,
    summary text DEFAULT ''::text NOT NULL,
    information_gaps jsonb DEFAULT '[]'::jsonb NOT NULL,
    safety_notes jsonb DEFAULT '[]'::jsonb NOT NULL,
    body_state_revision bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    agent_configuration_id character varying(80) DEFAULT ''::character varying NOT NULL,
    agent_configuration jsonb DEFAULT '{}'::jsonb NOT NULL,
    execution_provenance jsonb DEFAULT '{}'::jsonb NOT NULL,
    generation_decision_trace jsonb DEFAULT '{}'::jsonb NOT NULL,
    replay_input jsonb DEFAULT '{}'::jsonb NOT NULL,
    contract_revision character varying(80) DEFAULT 'assessment-output-v1'::character varying NOT NULL,
    evidence_coverage jsonb DEFAULT '{}'::jsonb NOT NULL,
    evidence_gaps jsonb DEFAULT '[]'::jsonb NOT NULL,
    CONSTRAINT assessment_reports_status_check CHECK (((status)::text = ANY ((ARRAY['completed'::character varying, 'insufficient_information'::character varying])::text[])))
);


--
-- Name: assessment_rollout_observations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.assessment_rollout_observations (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    source_report_id uuid NOT NULL,
    stage character varying(24) NOT NULL,
    subject_bucket integer NOT NULL,
    canary_bps integer DEFAULT 0 NOT NULL,
    champion_configuration_id character varying(80) CONSTRAINT assessment_rollout_observati_champion_configuration_id_not_null NOT NULL,
    challenger_configuration_id character varying(80) CONSTRAINT assessment_rollout_observat_challenger_configuration_i_not_null NOT NULL,
    served_configuration_id character varying(80) CONSTRAINT assessment_rollout_observation_served_configuration_id_not_null NOT NULL,
    shadow_configuration_id character varying(80) DEFAULT ''::character varying CONSTRAINT assessment_rollout_observation_shadow_configuration_id_not_null NOT NULL,
    promotion_record character varying(80) DEFAULT ''::character varying NOT NULL,
    comparison jsonb DEFAULT '{}'::jsonb NOT NULL,
    forbidden_side_effect boolean DEFAULT false NOT NULL,
    configuration_mismatch boolean DEFAULT false NOT NULL,
    shadow_error text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT assessment_rollout_observations_canary_bps_check CHECK (((canary_bps >= 0) AND (canary_bps <= 10000))),
    CONSTRAINT assessment_rollout_observations_subject_bucket_check CHECK (((subject_bucket >= 0) AND (subject_bucket < 10000)))
);


--
-- Name: body_state_evidence; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_state_evidence (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    source_type character varying(40) NOT NULL,
    source_key text NOT NULL,
    source_version text DEFAULT ''::text NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    summary text DEFAULT ''::text NOT NULL,
    excerpt text DEFAULT ''::text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    retrieved_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: body_state_facts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_state_facts (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    concern_key character varying(120) DEFAULT ''::character varying NOT NULL,
    kind character varying(80) NOT NULL,
    body_region character varying(120) DEFAULT ''::character varying NOT NULL,
    value text DEFAULT ''::text NOT NULL,
    details jsonb DEFAULT '{}'::jsonb NOT NULL,
    origin character varying(40) NOT NULL,
    review_state character varying(40) DEFAULT 'unverified'::character varying NOT NULL,
    lifecycle_state character varying(30) DEFAULT 'active'::character varying NOT NULL,
    trend character varying(30) DEFAULT 'unknown'::character varying NOT NULL,
    source_key text DEFAULT ''::text NOT NULL,
    provenance jsonb DEFAULT '{}'::jsonb NOT NULL,
    observed_at timestamp with time zone,
    valid_from timestamp with time zone,
    valid_until timestamp with time zone,
    supersedes_fact_id uuid,
    excluded_from_reasoning boolean DEFAULT false NOT NULL,
    created_revision bigint NOT NULL,
    updated_revision bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    body_region_id character varying(80)
);


--
-- Name: body_state_hypotheses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_state_hypotheses (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    concern_key character varying(120) DEFAULT 'general'::character varying NOT NULL,
    statement text NOT NULL,
    lifecycle_state character varying(30) DEFAULT 'active'::character varying NOT NULL,
    confidence character varying(20),
    supporting_fact_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    supporting_observation_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    supporting_evidence_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    counterevidence_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    source_analysis_id uuid,
    provenance jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_revision bigint NOT NULL,
    updated_revision bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT body_state_hypotheses_lifecycle_state_check CHECK (((lifecycle_state)::text = ANY ((ARRAY['active'::character varying, 'strengthened'::character varying, 'weakened'::character varying, 'unsupported'::character varying, 'retired'::character varying])::text[])))
);


--
-- Name: body_state_observations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_state_observations (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    concern_key character varying(120) DEFAULT ''::character varying NOT NULL,
    kind character varying(80) NOT NULL,
    body_region character varying(120) DEFAULT ''::character varying NOT NULL,
    method character varying(80) DEFAULT ''::character varying NOT NULL,
    value jsonb DEFAULT '{}'::jsonb NOT NULL,
    condition jsonb DEFAULT '{}'::jsonb NOT NULL,
    source_key text DEFAULT ''::text NOT NULL,
    provenance jsonb DEFAULT '{}'::jsonb NOT NULL,
    observed_at timestamp with time zone,
    review_state character varying(40) DEFAULT 'unverified'::character varying NOT NULL,
    lifecycle_state character varying(30) DEFAULT 'active'::character varying NOT NULL,
    excluded_from_reasoning boolean DEFAULT true NOT NULL,
    created_revision bigint NOT NULL,
    updated_revision bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    supersedes_observation_id uuid,
    body_region_id character varying(80),
    CONSTRAINT body_state_observations_review_state_check CHECK (((review_state)::text = ANY ((ARRAY['unverified'::character varying, 'confirmed'::character varying, 'rejected'::character varying])::text[])))
);


--
-- Name: body_state_revisions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_state_revisions (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    revision bigint NOT NULL,
    change_type character varying(80) NOT NULL,
    source character varying(60) NOT NULL,
    changes jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: body_states; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_states (
    user_id uuid NOT NULL,
    current_revision bigint DEFAULT 0 NOT NULL,
    safety_state jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: consultation_rollout_observations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.consultation_rollout_observations (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    run_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    stage character varying(24) NOT NULL,
    champion_configuration_id character varying(80) CONSTRAINT consultation_rollout_observa_champion_configuration_id_not_null NOT NULL,
    challenger_configuration_id character varying(80) CONSTRAINT consultation_rollout_observ_challenger_configuration_i_not_null NOT NULL,
    canary_bps integer DEFAULT 0 NOT NULL,
    decision_identity_match boolean DEFAULT false CONSTRAINT consultation_rollout_observati_decision_identity_match_not_null NOT NULL,
    replay_input_frozen boolean DEFAULT false NOT NULL,
    shadow_error text,
    comparison jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: consultation_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.consultation_sessions (
    conversation_id uuid NOT NULL,
    phase character varying(30) DEFAULT 'collecting'::character varying NOT NULL,
    extracted_info jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    ended_at timestamp with time zone,
    CONSTRAINT consultation_sessions_phase_check CHECK (((phase)::text = ANY ((ARRAY['collecting'::character varying, 'ready_for_analysis'::character varying])::text[])))
);


--
-- Name: conversation_shares; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conversation_shares (
    id uuid DEFAULT uuidv7() NOT NULL,
    conversation_id uuid NOT NULL,
    share_token character varying(32) NOT NULL,
    snapshot_messages jsonb NOT NULL,
    snapshot_title character varying(200),
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: conversations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conversations (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    title text,
    title_status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    pinned boolean DEFAULT false NOT NULL,
    pinned_at timestamp with time zone,
    default_model text,
    system_prompt_version text,
    provider text,
    provider_conversation_id text,
    provider_last_response_id text,
    active_run_id uuid,
    active_stream_id text,
    summary text,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    last_message_at timestamp with time zone,
    deleted_at timestamp with time zone,
    title_agent_configuration_id character varying(80) DEFAULT ''::character varying NOT NULL,
    title_agent_configuration jsonb DEFAULT '{}'::jsonb NOT NULL,
    title_execution_provenance jsonb DEFAULT '{}'::jsonb NOT NULL,
    title_decision_trace jsonb DEFAULT '{}'::jsonb NOT NULL
);


--
-- Name: diagnosis_analyses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.diagnosis_analyses (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    body_state_revision bigint NOT NULL,
    status character varying(40) NOT NULL,
    scope character varying(40) DEFAULT 'full_body'::character varying NOT NULL,
    summary text DEFAULT ''::text NOT NULL,
    cross_concern_patterns jsonb DEFAULT '[]'::jsonb NOT NULL,
    information_gaps jsonb DEFAULT '[]'::jsonb NOT NULL,
    safety_summary jsonb DEFAULT '{}'::jsonb NOT NULL,
    citations jsonb DEFAULT '[]'::jsonb NOT NULL,
    governance jsonb DEFAULT '{}'::jsonb NOT NULL,
    raw_output jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    agent_configuration_id character varying(80) DEFAULT ''::character varying NOT NULL,
    agent_configuration jsonb DEFAULT '{}'::jsonb NOT NULL,
    decision_trace jsonb DEFAULT '{}'::jsonb NOT NULL,
    execution_provenance jsonb DEFAULT '{}'::jsonb NOT NULL,
    evidence_acquisition_trace jsonb DEFAULT '{}'::jsonb NOT NULL,
    replay_input jsonb DEFAULT '{}'::jsonb NOT NULL
);


--
-- Name: diagnosis_analysis_freshness; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.diagnosis_analysis_freshness (
    analysis_id uuid NOT NULL,
    user_id uuid NOT NULL,
    state character varying(30) DEFAULT 'fresh'::character varying NOT NULL,
    evaluated_against_revision bigint CONSTRAINT diagnosis_analysis_freshnes_evaluated_against_revision_not_null NOT NULL,
    reasons jsonb DEFAULT '[]'::jsonb NOT NULL,
    checked_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT diagnosis_analysis_freshness_state_check CHECK (((state)::text = ANY ((ARRAY['fresh'::character varying, 'potentially_stale'::character varying, 'stale'::character varying])::text[])))
);


--
-- Name: diagnosis_candidate_assessments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.diagnosis_candidate_assessments (
    id uuid DEFAULT uuidv7() NOT NULL,
    analysis_id uuid NOT NULL,
    candidate_id uuid NOT NULL,
    user_id uuid NOT NULL,
    state character varying(30) NOT NULL,
    assessed_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: diagnosis_candidates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.diagnosis_candidates (
    id uuid DEFAULT uuidv7() NOT NULL,
    analysis_id uuid NOT NULL,
    ordinal integer NOT NULL,
    concern_key character varying(120) DEFAULT ''::character varying NOT NULL,
    name text NOT NULL,
    confidence character varying(20) NOT NULL,
    severity character varying(20),
    evidence_strength character varying(20),
    impact text,
    basis text DEFAULT ''::text NOT NULL,
    typical_symptoms text DEFAULT ''::text NOT NULL,
    differential text,
    basis_fact_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    basis_observation_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    supporting_evidence_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    counterevidence_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    reasoning_summary text DEFAULT ''::text NOT NULL,
    missing_information jsonb DEFAULT '[]'::jsonb NOT NULL,
    safety_notes jsonb DEFAULT '[]'::jsonb NOT NULL,
    raw_payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: diagnosis_rollout_observations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.diagnosis_rollout_observations (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    source_analysis_id uuid,
    stage character varying(24) NOT NULL,
    subject_bucket integer NOT NULL,
    canary_bps integer DEFAULT 0 NOT NULL,
    champion_configuration_id character varying(80) CONSTRAINT diagnosis_rollout_observatio_champion_configuration_id_not_null NOT NULL,
    challenger_configuration_id character varying(80) CONSTRAINT diagnosis_rollout_observati_challenger_configuration_i_not_null NOT NULL,
    served_configuration_id character varying(80) NOT NULL,
    shadow_configuration_id character varying(80) DEFAULT ''::character varying NOT NULL,
    comparison jsonb DEFAULT '{}'::jsonb NOT NULL,
    unsafe_relaxation boolean DEFAULT false NOT NULL,
    forbidden_side_effect boolean DEFAULT false NOT NULL,
    configuration_mismatch boolean DEFAULT false NOT NULL,
    shadow_error text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT diagnosis_rollout_observations_canary_bps_check CHECK (((canary_bps >= 0) AND (canary_bps <= 10000))),
    CONSTRAINT diagnosis_rollout_observations_subject_bucket_check CHECK (((subject_bucket >= 0) AND (subject_bucket < 10000)))
);


--
-- Name: document_extraction_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.document_extraction_runs (
    id uuid DEFAULT uuidv7() NOT NULL,
    upload_id uuid NOT NULL,
    user_id uuid NOT NULL,
    job_id uuid,
    configuration_id character varying(80) NOT NULL,
    mechanism_revision character varying(120) NOT NULL,
    document_sha256 character(64) NOT NULL,
    result_sha256 character(64) NOT NULL,
    raw_text_sha256 character(64) NOT NULL,
    indicator_snapshot jsonb DEFAULT '[]'::jsonb NOT NULL,
    source_summary jsonb DEFAULT '{}'::jsonb NOT NULL,
    mechanism_provenance jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT document_extraction_runs_document_sha256_check CHECK ((document_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT document_extraction_runs_raw_text_sha256_check CHECK ((raw_text_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT document_extraction_runs_result_sha256_check CHECK ((result_sha256 ~ '^[0-9a-f]{64}$'::text))
);


--
-- Name: document_indicator_reviews; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.document_indicator_reviews (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    upload_id uuid NOT NULL,
    extraction_run_id uuid NOT NULL,
    indicator_index integer NOT NULL,
    action character varying(20) NOT NULL,
    idempotency_key character varying(128) NOT NULL,
    reviewed_payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    machine_candidate jsonb DEFAULT '{}'::jsonb NOT NULL,
    source_refs jsonb DEFAULT '[]'::jsonb NOT NULL,
    page_ref jsonb DEFAULT '{}'::jsonb NOT NULL,
    reviewer_user_id uuid NOT NULL,
    note text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT document_indicator_reviews_action_check CHECK (((action)::text = ANY ((ARRAY['confirm'::character varying, 'correct'::character varying, 'reject'::character varying])::text[]))),
    CONSTRAINT document_indicator_reviews_idempotency_key_check CHECK (((idempotency_key)::text <> ''::text)),
    CONSTRAINT document_indicator_reviews_indicator_index_check CHECK ((indicator_index >= 0))
);


--
-- Name: interventions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.interventions (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    treatment_id uuid NOT NULL,
    treatment_revision_id uuid NOT NULL,
    kind character varying(40) NOT NULL,
    title text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    prescription jsonb DEFAULT '{}'::jsonb NOT NULL,
    "position" integer DEFAULT 0 NOT NULL,
    status character varying(20) DEFAULT 'proposed'::character varying NOT NULL,
    started_at timestamp with time zone,
    ended_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT interventions_status_check CHECK (((status)::text = ANY ((ARRAY['proposed'::character varying, 'active'::character varying, 'paused'::character varying, 'completed'::character varying, 'cancelled'::character varying, 'superseded'::character varying])::text[])))
);


--
-- Name: job_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.job_events (
    id uuid DEFAULT uuidv7() NOT NULL,
    job_id uuid NOT NULL,
    event_type character varying(50) NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobs (
    id uuid DEFAULT uuidv7() NOT NULL,
    run_id uuid,
    conversation_id uuid,
    user_id uuid NOT NULL,
    job_type character varying(50) NOT NULL,
    status character varying(30) DEFAULT 'pending'::character varying NOT NULL,
    input jsonb DEFAULT '{}'::jsonb NOT NULL,
    progress jsonb,
    result jsonb,
    error jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    idempotency_key text,
    attempts integer DEFAULT 0 NOT NULL,
    max_attempts integer DEFAULT 1 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT jobs_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'running'::character varying, 'waiting_user'::character varying, 'completed'::character varying, 'failed'::character varying, 'cancelled'::character varying, 'timed_out'::character varying])::text[])))
);


--
-- Name: knowledge_clips; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_clips (
    id bigint NOT NULL,
    source_id bigint NOT NULL,
    source_unit_id bigint,
    clip_key character varying(200) NOT NULL,
    clip_type character varying(50) NOT NULL,
    title character varying(500) NOT NULL,
    file_path text NOT NULL,
    start_sec double precision NOT NULL,
    end_sec double precision NOT NULL,
    transcript_excerpt text DEFAULT ''::text NOT NULL,
    notes text,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: knowledge_clips_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.knowledge_clips_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: knowledge_clips_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.knowledge_clips_id_seq OWNED BY public.knowledge_clips.id;


--
-- Name: knowledge_publication_observations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_publication_observations (
    id uuid DEFAULT uuidv7() NOT NULL,
    publication_id uuid NOT NULL,
    observation_key character varying(200) NOT NULL,
    observation_kind character varying(32) DEFAULT 'predeploy_eval'::character varying NOT NULL,
    evaluator_revision character varying(100) NOT NULL,
    case_id character varying(120) NOT NULL,
    retrieval_status character varying(32) NOT NULL,
    citation_status character varying(32) NOT NULL,
    grounding_status character varying(32) NOT NULL,
    identity_status character varying(32) NOT NULL,
    provenance_status character varying(32) NOT NULL,
    execution_error text,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: knowledge_publications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_publications (
    id uuid DEFAULT uuidv7() NOT NULL,
    knowledge_unit_id bigint,
    publication_key character varying(200) NOT NULL,
    title character varying(500) DEFAULT ''::character varying NOT NULL,
    published_version integer DEFAULT 1 NOT NULL,
    published_at timestamp with time zone DEFAULT now() NOT NULL,
    published_by text,
    created_by text,
    status character varying(30) DEFAULT 'draft'::character varying NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    publication_batch_key character varying(200),
    rollback_of uuid,
    unit_count integer DEFAULT 0 NOT NULL,
    summary text DEFAULT ''::text NOT NULL
);


--
-- Name: knowledge_segments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_segments (
    id bigint NOT NULL,
    source_id bigint NOT NULL,
    segment_index integer NOT NULL,
    start_sec double precision NOT NULL,
    end_sec double precision NOT NULL,
    transcript text NOT NULL,
    normalized_transcript text NOT NULL,
    confidence double precision,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: knowledge_segments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.knowledge_segments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: knowledge_segments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.knowledge_segments_id_seq OWNED BY public.knowledge_segments.id;


--
-- Name: knowledge_sources; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_sources (
    id bigint NOT NULL,
    source_key character varying(200) NOT NULL,
    source_type character varying(50) NOT NULL,
    title character varying(500) NOT NULL,
    author character varying(255) NOT NULL,
    problem_slug character varying(100) NOT NULL,
    problem_display_name character varying(255) NOT NULL,
    original_file_path text NOT NULL,
    language character varying(20) DEFAULT 'zh'::character varying NOT NULL,
    duration_sec double precision,
    transcript_provider character varying(100),
    transcript_model character varying(100),
    transcript_file_path text,
    ingest_status character varying(50) DEFAULT 'pending'::character varying NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    license_status character varying(50) DEFAULT 'unknown'::character varying NOT NULL,
    content_hash text,
    canonical_url text,
    source_version character varying(100) DEFAULT 'v1'::character varying NOT NULL,
    provenance jsonb DEFAULT '{}'::jsonb NOT NULL,
    registered_by uuid,
    registered_at timestamp with time zone
);


--
-- Name: knowledge_sources_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.knowledge_sources_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: knowledge_sources_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.knowledge_sources_id_seq OWNED BY public.knowledge_sources.id;


--
-- Name: knowledge_units; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_units (
    id bigint NOT NULL,
    source_id bigint NOT NULL,
    unit_key character varying(200) NOT NULL,
    problem_slug character varying(100) NOT NULL,
    category character varying(100) NOT NULL,
    unit_type character varying(50) NOT NULL,
    title character varying(500) NOT NULL,
    summary text NOT NULL,
    body_markdown text NOT NULL,
    source_start_sec double precision NOT NULL,
    source_end_sec double precision NOT NULL,
    evidence_segment_indices integer[] DEFAULT '{}'::integer[] NOT NULL,
    tags text[] DEFAULT '{}'::text[] NOT NULL,
    transcript_excerpt text DEFAULT ''::text NOT NULL,
    review_status character varying(50) DEFAULT 'generated'::character varying NOT NULL,
    embedding public.vector(1536),
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    quality_score real DEFAULT 0.0,
    content_hash text,
    published_version integer,
    lifecycle_status character varying(50) DEFAULT 'generated'::character varying NOT NULL,
    publication_id uuid
);


--
-- Name: knowledge_units_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.knowledge_units_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: knowledge_units_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.knowledge_units_id_seq OWNED BY public.knowledge_units.id;


--
-- Name: messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.messages (
    id uuid DEFAULT uuidv7() NOT NULL,
    conversation_id uuid NOT NULL,
    turn_id uuid NOT NULL,
    parent_message_id uuid,
    role character varying(20) NOT NULL,
    status character varying(20) DEFAULT 'completed'::character varying NOT NULL,
    seq integer NOT NULL,
    parts jsonb DEFAULT '[]'::jsonb NOT NULL,
    content_text text,
    model text,
    provider text,
    provider_message_id text,
    provider_response_id text,
    input_tokens integer,
    output_tokens integer,
    total_tokens integer,
    error jsonb,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    run_id uuid
);


--
-- Name: outcomes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.outcomes (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    treatment_id uuid,
    treatment_revision_id uuid,
    intervention_id uuid,
    source_type character varying(40) NOT NULL,
    source_key text NOT NULL,
    kind character varying(50) NOT NULL,
    concern_key character varying(120) DEFAULT 'general'::character varying NOT NULL,
    body_region text DEFAULT ''::text NOT NULL,
    value jsonb DEFAULT '{}'::jsonb NOT NULL,
    notes text DEFAULT ''::text NOT NULL,
    association_statement text DEFAULT ''::text NOT NULL,
    causality_level character varying(30) DEFAULT 'association_only'::character varying NOT NULL,
    occurred_at timestamp with time zone NOT NULL,
    provenance jsonb DEFAULT '{}'::jsonb NOT NULL,
    body_state_revision bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT outcomes_causality_level_check CHECK (((causality_level)::text = ANY ((ARRAY['association_only'::character varying, 'user_attributed'::character varying, 'clinician_attributed'::character varying])::text[])))
);


--
-- Name: privacy_erasure_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.privacy_erasure_requests (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    subject_user_id uuid,
    subject_digest character(64) NOT NULL,
    status character varying(24) DEFAULT 'pending'::character varying NOT NULL,
    attempt_count integer DEFAULT 0 NOT NULL,
    report jsonb DEFAULT '{}'::jsonb NOT NULL,
    last_error text,
    lease_owner character varying(120),
    lease_expires_at timestamp with time zone,
    requested_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    CONSTRAINT privacy_erasure_requests_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'running'::character varying, 'retryable'::character varying, 'completed'::character varying])::text[])))
);


--
-- Name: runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.runs (
    id uuid DEFAULT uuidv7() NOT NULL,
    conversation_id uuid NOT NULL,
    turn_id uuid NOT NULL,
    request_id text NOT NULL,
    user_id uuid NOT NULL,
    status character varying(20) DEFAULT 'running'::character varying NOT NULL,
    model text NOT NULL,
    provider text,
    provider_response_id text,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    error jsonb,
    usage jsonb,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    agent_configuration_id character varying(80) DEFAULT ''::character varying NOT NULL,
    agent_configuration jsonb DEFAULT '{}'::jsonb NOT NULL,
    execution_provenance jsonb DEFAULT '{}'::jsonb NOT NULL,
    replay_input jsonb DEFAULT '{}'::jsonb NOT NULL,
    lease_expires_at timestamp with time zone,
    lease_owner character varying(120) DEFAULT ''::character varying NOT NULL,
    lease_heartbeat_at timestamp with time zone,
    CONSTRAINT runs_status_check CHECK (((status)::text = ANY ((ARRAY['running'::character varying, 'waiting_user'::character varying, 'completed'::character varying, 'failed'::character varying, 'cancelled'::character varying])::text[])))
);


--
-- Name: runtime_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.runtime_events (
    id uuid DEFAULT uuidv7() NOT NULL,
    conversation_id uuid NOT NULL,
    run_id uuid NOT NULL,
    turn_id uuid,
    seq integer NOT NULL,
    channel character varying(40) NOT NULL,
    type character varying(120) NOT NULL,
    ids jsonb DEFAULT '{}'::jsonb NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    source character varying(20) DEFAULT 'go'::character varying NOT NULL,
    replayable boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: thread_projection_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.thread_projection_messages (
    message_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    turn_id uuid NOT NULL,
    run_id uuid,
    parent_message_id uuid,
    seq integer NOT NULL,
    role character varying(20) NOT NULL,
    status character varying(20) NOT NULL,
    parts jsonb DEFAULT '[]'::jsonb NOT NULL,
    content_text text DEFAULT ''::text NOT NULL,
    model text,
    provider text,
    provider_message_id text,
    provider_response_id text,
    input_tokens integer,
    output_tokens integer,
    total_tokens integer,
    error jsonb,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL
);


--
-- Name: thread_projection_tool_calls; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.thread_projection_tool_calls (
    tool_call_id text NOT NULL,
    conversation_id uuid NOT NULL,
    run_id uuid NOT NULL,
    message_id uuid,
    tool_name text NOT NULL,
    arguments jsonb DEFAULT '{}'::jsonb NOT NULL,
    status character varying(30) NOT NULL,
    result jsonb,
    error jsonb,
    created_at timestamp with time zone NOT NULL,
    started_at timestamp with time zone NOT NULL,
    finished_at timestamp with time zone,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL
);


--
-- Name: thread_projections; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.thread_projections (
    conversation_id uuid NOT NULL,
    user_id uuid NOT NULL,
    title text,
    title_status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    pinned boolean DEFAULT false NOT NULL,
    pinned_at timestamp with time zone,
    default_model text,
    active_run_id uuid,
    last_message_at timestamp with time zone,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    phase character varying(30) DEFAULT 'collecting'::character varying NOT NULL,
    extracted_info jsonb DEFAULT '[]'::jsonb NOT NULL,
    pending_interactions jsonb DEFAULT '[]'::jsonb NOT NULL,
    conversation_created_at timestamp with time zone NOT NULL,
    conversation_updated_at timestamp with time zone NOT NULL,
    session_created_at timestamp with time zone NOT NULL,
    session_updated_at timestamp with time zone NOT NULL,
    ended_at timestamp with time zone,
    refreshed_at timestamp with time zone DEFAULT now() NOT NULL,
    interaction_history jsonb DEFAULT '[]'::jsonb NOT NULL,
    CONSTRAINT thread_projections_phase_check CHECK (((phase)::text = ANY ((ARRAY['collecting'::character varying, 'ready_for_analysis'::character varying])::text[])))
);


--
-- Name: training_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.training_logs (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    plan_id uuid NOT NULL,
    date date DEFAULT CURRENT_DATE NOT NULL,
    exercises jsonb DEFAULT '[]'::jsonb NOT NULL,
    notes text,
    is_checked_in boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    treatment_revision_id uuid,
    intervention_id uuid,
    outcome_recorded_at timestamp with time zone
);


--
-- Name: training_plans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.training_plans (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    consultation_id uuid,
    goal text NOT NULL,
    duration_weeks integer DEFAULT 4 NOT NULL,
    current_week integer DEFAULT 1 NOT NULL,
    phases jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    treatment_id uuid,
    treatment_revision_id uuid,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL
);


--
-- Name: treatment_revisions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.treatment_revisions (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    treatment_id uuid NOT NULL,
    revision integer NOT NULL,
    acceptance_state character varying(20) DEFAULT 'proposed'::character varying NOT NULL,
    lifecycle_state character varying(30) DEFAULT 'review_recommended'::character varying NOT NULL,
    source_body_state_revision bigint NOT NULL,
    source_diagnosis_analysis_id uuid NOT NULL,
    goal text NOT NULL,
    duration_weeks integer NOT NULL,
    plan jsonb NOT NULL,
    user_constraints jsonb DEFAULT '{}'::jsonb NOT NULL,
    evidence_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    governance jsonb DEFAULT '{}'::jsonb NOT NULL,
    change_reason text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    accepted_at timestamp with time zone,
    agent_configuration_id character varying(80) DEFAULT ''::character varying NOT NULL,
    agent_configuration jsonb DEFAULT '{}'::jsonb NOT NULL,
    execution_provenance jsonb DEFAULT '{}'::jsonb NOT NULL,
    evidence_acquisition_trace jsonb DEFAULT '{}'::jsonb NOT NULL,
    generation_decision_trace jsonb DEFAULT '{}'::jsonb NOT NULL,
    acceptance_decision_trace jsonb DEFAULT '{}'::jsonb NOT NULL,
    replay_input jsonb DEFAULT '{}'::jsonb NOT NULL,
    rollout_provenance jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT treatment_revisions_acceptance_state_check CHECK (((acceptance_state)::text = ANY ((ARRAY['proposed'::character varying, 'accepted'::character varying, 'rejected'::character varying])::text[]))),
    CONSTRAINT treatment_revisions_duration_weeks_check CHECK ((duration_weeks > 0)),
    CONSTRAINT treatment_revisions_lifecycle_state_check CHECK (((lifecycle_state)::text = ANY ((ARRAY['active'::character varying, 'review_recommended'::character varying, 'paused'::character varying, 'superseded'::character varying, 'completed'::character varying])::text[])))
);


--
-- Name: treatment_rollout_observations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.treatment_rollout_observations (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    source_revision_id uuid NOT NULL,
    stage character varying(24) NOT NULL,
    subject_bucket integer NOT NULL,
    canary_bps integer DEFAULT 0 NOT NULL,
    champion_configuration_id character varying(80) CONSTRAINT treatment_rollout_observatio_champion_configuration_id_not_null NOT NULL,
    challenger_configuration_id character varying(80) CONSTRAINT treatment_rollout_observati_challenger_configuration_i_not_null NOT NULL,
    served_configuration_id character varying(80) NOT NULL,
    shadow_configuration_id character varying(80) DEFAULT ''::character varying NOT NULL,
    promotion_record character varying(80) DEFAULT ''::character varying NOT NULL,
    comparison jsonb DEFAULT '{}'::jsonb NOT NULL,
    unsafe_relaxation boolean DEFAULT false NOT NULL,
    forbidden_side_effect boolean DEFAULT false NOT NULL,
    configuration_mismatch boolean DEFAULT false NOT NULL,
    shadow_error text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT treatment_rollout_observations_canary_bps_check CHECK (((canary_bps >= 0) AND (canary_bps <= 10000))),
    CONSTRAINT treatment_rollout_observations_subject_bucket_check CHECK (((subject_bucket >= 0) AND (subject_bucket < 10000)))
);


--
-- Name: treatments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.treatments (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    current_revision integer DEFAULT 0 NOT NULL,
    status character varying(30) DEFAULT 'review_recommended'::character varying NOT NULL,
    source_body_state_revision bigint,
    source_diagnosis_analysis_id uuid,
    status_reasons jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT treatments_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'review_recommended'::character varying, 'paused'::character varying, 'superseded'::character varying, 'completed'::character varying])::text[])))
);


--
-- Name: user_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_profiles (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    gender character varying(20),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    birth_date date
);


--
-- Name: user_uploads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_uploads (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    file_type character varying(50) NOT NULL,
    original_name character varying(255) NOT NULL,
    file_size bigint NOT NULL,
    mime_type character varying(100) NOT NULL,
    ocr_result jsonb,
    ocr_status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    analysis_status character varying(20) DEFAULT 'none'::character varying NOT NULL,
    analysis_result jsonb,
    agent_configuration_id character varying(80) DEFAULT ''::character varying NOT NULL,
    storage_backend character varying(20) NOT NULL,
    storage_key character varying(500) NOT NULL,
    CONSTRAINT ck_user_uploads_storage_backend CHECK (((storage_backend)::text = ANY ((ARRAY['local'::character varying, 'oss'::character varying])::text[]))),
    CONSTRAINT ck_user_uploads_storage_key CHECK ((((storage_key)::text <> ''::text) AND (length((storage_key)::text) <= 492) AND ((storage_key)::text !~ '^/'::text) AND ((storage_key)::text !~ '(^|/)\.\.(/|$)'::text) AND (POSITION((chr(92)) IN (storage_key)) = 0))),
    CONSTRAINT user_uploads_analysis_status_check CHECK (((analysis_status)::text = ANY ((ARRAY['none'::character varying, 'pending'::character varying, 'processing'::character varying, 'completed'::character varying, 'failed'::character varying])::text[]))),
    CONSTRAINT user_uploads_ocr_status_check CHECK (((ocr_status)::text = ANY ((ARRAY['pending'::character varying, 'processing'::character varying, 'completed'::character varying, 'failed'::character varying])::text[])))
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    email character varying(255) NOT NULL,
    password_hash character varying(255) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_login_at timestamp with time zone,
    role character varying(30) DEFAULT 'member'::character varying NOT NULL,
    CONSTRAINT chk_users_role CHECK (((role)::text = ANY ((ARRAY['member'::character varying, 'operator'::character varying])::text[])))
);


--
-- Name: knowledge_clips id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_clips ALTER COLUMN id SET DEFAULT nextval('public.knowledge_clips_id_seq'::regclass);


--
-- Name: knowledge_segments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_segments ALTER COLUMN id SET DEFAULT nextval('public.knowledge_segments_id_seq'::regclass);


--
-- Name: knowledge_sources id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_sources ALTER COLUMN id SET DEFAULT nextval('public.knowledge_sources_id_seq'::regclass);


--
-- Name: knowledge_units id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_units ALTER COLUMN id SET DEFAULT nextval('public.knowledge_units_id_seq'::regclass);


--
-- Name: agent_interactions agent_interactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_interactions
    ADD CONSTRAINT agent_interactions_pkey PRIMARY KEY (id);


--
-- Name: agent_interactions agent_interactions_run_id_tool_call_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_interactions
    ADD CONSTRAINT agent_interactions_run_id_tool_call_id_key UNIQUE (run_id, tool_call_id);


--
-- Name: agent_tool_calls agent_tool_calls_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_tool_calls
    ADD CONSTRAINT agent_tool_calls_pkey PRIMARY KEY (id);


--
-- Name: agent_tool_calls agent_tool_calls_run_id_tool_call_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_tool_calls
    ADD CONSTRAINT agent_tool_calls_run_id_tool_call_id_key UNIQUE (run_id, tool_call_id);


--
-- Name: ai_output_reviews ai_output_reviews_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_output_reviews
    ADD CONSTRAINT ai_output_reviews_pkey PRIMARY KEY (id);


--
-- Name: assessment_reports assessment_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.assessment_reports
    ADD CONSTRAINT assessment_reports_pkey PRIMARY KEY (id);


--
-- Name: assessment_rollout_observations assessment_rollout_observations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.assessment_rollout_observations
    ADD CONSTRAINT assessment_rollout_observations_pkey PRIMARY KEY (id);


--
-- Name: body_state_evidence body_state_evidence_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_evidence
    ADD CONSTRAINT body_state_evidence_pkey PRIMARY KEY (id);


--
-- Name: body_state_evidence body_state_evidence_user_id_source_type_source_key_source_v_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_evidence
    ADD CONSTRAINT body_state_evidence_user_id_source_type_source_key_source_v_key UNIQUE (user_id, source_type, source_key, source_version);


--
-- Name: body_state_facts body_state_facts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_facts
    ADD CONSTRAINT body_state_facts_pkey PRIMARY KEY (id);


--
-- Name: body_state_hypotheses body_state_hypotheses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_hypotheses
    ADD CONSTRAINT body_state_hypotheses_pkey PRIMARY KEY (id);


--
-- Name: body_state_observations body_state_observations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_observations
    ADD CONSTRAINT body_state_observations_pkey PRIMARY KEY (id);


--
-- Name: body_state_revisions body_state_revisions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_revisions
    ADD CONSTRAINT body_state_revisions_pkey PRIMARY KEY (id);


--
-- Name: body_state_revisions body_state_revisions_user_id_revision_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_revisions
    ADD CONSTRAINT body_state_revisions_user_id_revision_key UNIQUE (user_id, revision);


--
-- Name: body_states body_states_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_states
    ADD CONSTRAINT body_states_pkey PRIMARY KEY (user_id);


--
-- Name: consultation_rollout_observations consultation_rollout_observations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.consultation_rollout_observations
    ADD CONSTRAINT consultation_rollout_observations_pkey PRIMARY KEY (id);


--
-- Name: consultation_sessions consultation_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.consultation_sessions
    ADD CONSTRAINT consultation_sessions_pkey PRIMARY KEY (conversation_id);


--
-- Name: conversation_shares conversation_shares_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversation_shares
    ADD CONSTRAINT conversation_shares_pkey PRIMARY KEY (id);


--
-- Name: conversation_shares conversation_shares_share_token_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversation_shares
    ADD CONSTRAINT conversation_shares_share_token_key UNIQUE (share_token);


--
-- Name: conversations conversations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversations
    ADD CONSTRAINT conversations_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_analyses diagnosis_analyses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_analyses
    ADD CONSTRAINT diagnosis_analyses_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_analysis_freshness diagnosis_analysis_freshness_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_analysis_freshness
    ADD CONSTRAINT diagnosis_analysis_freshness_pkey PRIMARY KEY (analysis_id);


--
-- Name: diagnosis_candidate_assessments diagnosis_candidate_assessments_candidate_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_candidate_assessments
    ADD CONSTRAINT diagnosis_candidate_assessments_candidate_id_user_id_key UNIQUE (candidate_id, user_id);


--
-- Name: diagnosis_candidate_assessments diagnosis_candidate_assessments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_candidate_assessments
    ADD CONSTRAINT diagnosis_candidate_assessments_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_candidates diagnosis_candidates_analysis_id_ordinal_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_candidates
    ADD CONSTRAINT diagnosis_candidates_analysis_id_ordinal_key UNIQUE (analysis_id, ordinal);


--
-- Name: diagnosis_candidates diagnosis_candidates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_candidates
    ADD CONSTRAINT diagnosis_candidates_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_rollout_observations diagnosis_rollout_observations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_rollout_observations
    ADD CONSTRAINT diagnosis_rollout_observations_pkey PRIMARY KEY (id);


--
-- Name: document_extraction_runs document_extraction_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_extraction_runs
    ADD CONSTRAINT document_extraction_runs_pkey PRIMARY KEY (id);


--
-- Name: document_indicator_reviews document_indicator_reviews_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_indicator_reviews
    ADD CONSTRAINT document_indicator_reviews_pkey PRIMARY KEY (id);


--
-- Name: interventions interventions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interventions
    ADD CONSTRAINT interventions_pkey PRIMARY KEY (id);


--
-- Name: job_events job_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.job_events
    ADD CONSTRAINT job_events_pkey PRIMARY KEY (id);


--
-- Name: jobs jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_pkey PRIMARY KEY (id);


--
-- Name: knowledge_clips knowledge_clips_clip_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_clips
    ADD CONSTRAINT knowledge_clips_clip_key_key UNIQUE (clip_key);


--
-- Name: knowledge_clips knowledge_clips_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_clips
    ADD CONSTRAINT knowledge_clips_pkey PRIMARY KEY (id);


--
-- Name: knowledge_publication_observations knowledge_publication_observa_publication_id_observation_ke_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_publication_observations
    ADD CONSTRAINT knowledge_publication_observa_publication_id_observation_ke_key UNIQUE (publication_id, observation_key);


--
-- Name: knowledge_publication_observations knowledge_publication_observations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_publication_observations
    ADD CONSTRAINT knowledge_publication_observations_pkey PRIMARY KEY (id);


--
-- Name: knowledge_publications knowledge_publications_knowledge_unit_id_published_version_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_publications
    ADD CONSTRAINT knowledge_publications_knowledge_unit_id_published_version_key UNIQUE (knowledge_unit_id, published_version);


--
-- Name: knowledge_publications knowledge_publications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_publications
    ADD CONSTRAINT knowledge_publications_pkey PRIMARY KEY (id);


--
-- Name: knowledge_publications knowledge_publications_publication_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_publications
    ADD CONSTRAINT knowledge_publications_publication_key_key UNIQUE (publication_key);


--
-- Name: knowledge_segments knowledge_segments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_segments
    ADD CONSTRAINT knowledge_segments_pkey PRIMARY KEY (id);


--
-- Name: knowledge_segments knowledge_segments_source_id_segment_index_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_segments
    ADD CONSTRAINT knowledge_segments_source_id_segment_index_key UNIQUE (source_id, segment_index);


--
-- Name: knowledge_sources knowledge_sources_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_sources
    ADD CONSTRAINT knowledge_sources_pkey PRIMARY KEY (id);


--
-- Name: knowledge_sources knowledge_sources_source_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_sources
    ADD CONSTRAINT knowledge_sources_source_key_key UNIQUE (source_key);


--
-- Name: knowledge_units knowledge_units_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_units
    ADD CONSTRAINT knowledge_units_pkey PRIMARY KEY (id);


--
-- Name: knowledge_units knowledge_units_unit_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_units
    ADD CONSTRAINT knowledge_units_unit_key_key UNIQUE (unit_key);


--
-- Name: messages messages_conversation_id_seq_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.messages
    ADD CONSTRAINT messages_conversation_id_seq_key UNIQUE (conversation_id, seq);


--
-- Name: messages messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.messages
    ADD CONSTRAINT messages_pkey PRIMARY KEY (id);


--
-- Name: outcomes outcomes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outcomes
    ADD CONSTRAINT outcomes_pkey PRIMARY KEY (id);


--
-- Name: outcomes outcomes_user_id_source_type_source_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outcomes
    ADD CONSTRAINT outcomes_user_id_source_type_source_key_key UNIQUE (user_id, source_type, source_key);


--
-- Name: privacy_erasure_requests privacy_erasure_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.privacy_erasure_requests
    ADD CONSTRAINT privacy_erasure_requests_pkey PRIMARY KEY (id);


--
-- Name: privacy_erasure_requests privacy_erasure_requests_subject_digest_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.privacy_erasure_requests
    ADD CONSTRAINT privacy_erasure_requests_subject_digest_key UNIQUE (subject_digest);


--
-- Name: runs runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.runs
    ADD CONSTRAINT runs_pkey PRIMARY KEY (id);


--
-- Name: runs runs_user_id_request_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.runs
    ADD CONSTRAINT runs_user_id_request_id_key UNIQUE (user_id, request_id);


--
-- Name: runtime_events runtime_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.runtime_events
    ADD CONSTRAINT runtime_events_pkey PRIMARY KEY (id);


--
-- Name: runtime_events runtime_events_run_id_seq_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.runtime_events
    ADD CONSTRAINT runtime_events_run_id_seq_key UNIQUE (run_id, seq);


--
-- Name: thread_projection_messages thread_projection_messages_conversation_id_seq_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_projection_messages
    ADD CONSTRAINT thread_projection_messages_conversation_id_seq_key UNIQUE (conversation_id, seq);


--
-- Name: thread_projection_messages thread_projection_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_projection_messages
    ADD CONSTRAINT thread_projection_messages_pkey PRIMARY KEY (message_id);


--
-- Name: thread_projection_tool_calls thread_projection_tool_calls_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_projection_tool_calls
    ADD CONSTRAINT thread_projection_tool_calls_pkey PRIMARY KEY (tool_call_id);


--
-- Name: thread_projections thread_projections_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_projections
    ADD CONSTRAINT thread_projections_pkey PRIMARY KEY (conversation_id);


--
-- Name: training_logs training_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_logs
    ADD CONSTRAINT training_logs_pkey PRIMARY KEY (id);


--
-- Name: training_plans training_plans_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_plans
    ADD CONSTRAINT training_plans_pkey PRIMARY KEY (id);


--
-- Name: treatment_revisions treatment_revisions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.treatment_revisions
    ADD CONSTRAINT treatment_revisions_pkey PRIMARY KEY (id);


--
-- Name: treatment_revisions treatment_revisions_treatment_id_revision_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.treatment_revisions
    ADD CONSTRAINT treatment_revisions_treatment_id_revision_key UNIQUE (treatment_id, revision);


--
-- Name: treatment_rollout_observations treatment_rollout_observations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.treatment_rollout_observations
    ADD CONSTRAINT treatment_rollout_observations_pkey PRIMARY KEY (id);


--
-- Name: treatments treatments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.treatments
    ADD CONSTRAINT treatments_pkey PRIMARY KEY (id);


--
-- Name: treatments treatments_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.treatments
    ADD CONSTRAINT treatments_user_id_key UNIQUE (user_id);


--
-- Name: user_profiles user_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT user_profiles_pkey PRIMARY KEY (id);


--
-- Name: user_uploads user_uploads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_uploads
    ADD CONSTRAINT user_uploads_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: idx_agent_interactions_conversation_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_interactions_conversation_id ON public.agent_interactions USING btree (conversation_id);


--
-- Name: idx_agent_interactions_pending_expires; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_interactions_pending_expires ON public.agent_interactions USING btree (expires_at) WHERE (((status)::text = 'pending'::text) AND (expires_at IS NOT NULL));


--
-- Name: idx_agent_interactions_run_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_interactions_run_id ON public.agent_interactions USING btree (run_id);


--
-- Name: idx_agent_interactions_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_interactions_status ON public.agent_interactions USING btree (status);


--
-- Name: idx_agent_tool_calls_conversation_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_tool_calls_conversation_id ON public.agent_tool_calls USING btree (conversation_id);


--
-- Name: idx_agent_tool_calls_run_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_tool_calls_run_id ON public.agent_tool_calls USING btree (run_id);


--
-- Name: idx_ai_output_reviews_conversation_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_output_reviews_conversation_id ON public.ai_output_reviews USING btree (conversation_id);


--
-- Name: idx_ai_output_reviews_job_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_output_reviews_job_id ON public.ai_output_reviews USING btree (job_id);


--
-- Name: idx_ai_output_reviews_run_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_output_reviews_run_id ON public.ai_output_reviews USING btree (run_id);


--
-- Name: idx_ai_output_reviews_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_output_reviews_status ON public.ai_output_reviews USING btree (status);


--
-- Name: idx_ai_output_reviews_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_output_reviews_user_id ON public.ai_output_reviews USING btree (user_id);


--
-- Name: idx_assessment_reports_agent_configuration; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assessment_reports_agent_configuration ON public.assessment_reports USING btree (agent_configuration_id, created_at DESC) WHERE ((agent_configuration_id)::text <> ''::text);


--
-- Name: idx_assessment_reports_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assessment_reports_user_created ON public.assessment_reports USING btree (user_id, created_at DESC);


--
-- Name: idx_assessment_rollout_observations_pair_stage; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assessment_rollout_observations_pair_stage ON public.assessment_rollout_observations USING btree (champion_configuration_id, challenger_configuration_id, stage, canary_bps, created_at DESC);


--
-- Name: idx_body_state_evidence_user_retrieved; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_body_state_evidence_user_retrieved ON public.body_state_evidence USING btree (user_id, retrieved_at DESC);


--
-- Name: idx_body_state_facts_active_context_kind; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_body_state_facts_active_context_kind ON public.body_state_facts USING btree (user_id, kind) WHERE (((lifecycle_state)::text = 'active'::text) AND ((review_state)::text = 'confirmed'::text) AND (excluded_from_reasoning = false) AND ((kind)::text = ANY ((ARRAY['lifestyle.activity'::character varying, 'lifestyle.sleep'::character varying, 'lifestyle.exercise'::character varying, 'lifestyle.nutrition'::character varying, 'lifestyle.substances'::character varying, 'lifestyle.recovery'::character varying, 'history.injury_summary'::character varying])::text[])));


--
-- Name: idx_body_state_facts_user_concern; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_body_state_facts_user_concern ON public.body_state_facts USING btree (user_id, concern_key, updated_at DESC);


--
-- Name: idx_body_state_facts_user_current; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_body_state_facts_user_current ON public.body_state_facts USING btree (user_id, lifecycle_state, updated_at DESC);


--
-- Name: idx_body_state_facts_user_source_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_body_state_facts_user_source_key ON public.body_state_facts USING btree (user_id, source_key) WHERE (source_key <> ''::text);


--
-- Name: idx_body_state_hypotheses_concern; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_body_state_hypotheses_concern ON public.body_state_hypotheses USING btree (user_id, concern_key);


--
-- Name: idx_body_state_hypotheses_user_state; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_body_state_hypotheses_user_state ON public.body_state_hypotheses USING btree (user_id, lifecycle_state, updated_at DESC);


--
-- Name: idx_body_state_observations_active_anthropometry_kind; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_body_state_observations_active_anthropometry_kind ON public.body_state_observations USING btree (user_id, kind) WHERE (((lifecycle_state)::text = 'active'::text) AND (excluded_from_reasoning = false) AND ((kind)::text = ANY ((ARRAY['anthropometry.height'::character varying, 'anthropometry.weight'::character varying])::text[])));


--
-- Name: idx_body_state_observations_supersedes; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_body_state_observations_supersedes ON public.body_state_observations USING btree (supersedes_observation_id) WHERE (supersedes_observation_id IS NOT NULL);


--
-- Name: idx_body_state_observations_user_concern; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_body_state_observations_user_concern ON public.body_state_observations USING btree (user_id, concern_key, updated_at DESC);


--
-- Name: idx_body_state_observations_user_current; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_body_state_observations_user_current ON public.body_state_observations USING btree (user_id, lifecycle_state, review_state, updated_at DESC);


--
-- Name: idx_body_state_observations_user_source_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_body_state_observations_user_source_key ON public.body_state_observations USING btree (user_id, source_key) WHERE (source_key <> ''::text);


--
-- Name: idx_body_state_revisions_user_revision; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_body_state_revisions_user_revision ON public.body_state_revisions USING btree (user_id, revision DESC);


--
-- Name: idx_consultation_phase; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_consultation_phase ON public.consultation_sessions USING btree (phase);


--
-- Name: idx_consultation_rollout_observations_pair_stage; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_consultation_rollout_observations_pair_stage ON public.consultation_rollout_observations USING btree (champion_configuration_id, challenger_configuration_id, stage, canary_bps, created_at DESC);


--
-- Name: idx_conversation_shares_conversation; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_conversation_shares_conversation ON public.conversation_shares USING btree (conversation_id);


--
-- Name: idx_conversation_shares_token; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_conversation_shares_token ON public.conversation_shares USING btree (share_token);


--
-- Name: idx_conversations_pinned; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_conversations_pinned ON public.conversations USING btree (user_id, pinned, pinned_at DESC) WHERE ((deleted_at IS NULL) AND (pinned = true));


--
-- Name: idx_conversations_title_agent_configuration; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_conversations_title_agent_configuration ON public.conversations USING btree (title_agent_configuration_id) WHERE ((title_agent_configuration_id)::text <> ''::text);


--
-- Name: idx_conversations_user_last; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_conversations_user_last ON public.conversations USING btree (user_id, last_message_at DESC) WHERE (deleted_at IS NULL);


--
-- Name: idx_diagnosis_analyses_agent_configuration; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_diagnosis_analyses_agent_configuration ON public.diagnosis_analyses USING btree (agent_configuration_id, created_at DESC) WHERE ((agent_configuration_id)::text <> ''::text);


--
-- Name: idx_diagnosis_analyses_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_diagnosis_analyses_user_created ON public.diagnosis_analyses USING btree (user_id, created_at DESC);


--
-- Name: idx_diagnosis_analyses_user_revision; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_diagnosis_analyses_user_revision ON public.diagnosis_analyses USING btree (user_id, body_state_revision DESC);


--
-- Name: idx_diagnosis_candidate_assessments_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_diagnosis_candidate_assessments_user ON public.diagnosis_candidate_assessments USING btree (user_id, assessed_at DESC);


--
-- Name: idx_diagnosis_candidates_analysis; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_diagnosis_candidates_analysis ON public.diagnosis_candidates USING btree (analysis_id, ordinal);


--
-- Name: idx_diagnosis_freshness_user_state; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_diagnosis_freshness_user_state ON public.diagnosis_analysis_freshness USING btree (user_id, state, checked_at DESC);


--
-- Name: idx_diagnosis_rollout_observations_pair_stage; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_diagnosis_rollout_observations_pair_stage ON public.diagnosis_rollout_observations USING btree (champion_configuration_id, challenger_configuration_id, stage, canary_bps, created_at DESC);


--
-- Name: idx_document_extraction_runs_configuration; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_document_extraction_runs_configuration ON public.document_extraction_runs USING btree (configuration_id, created_at DESC);


--
-- Name: idx_document_extraction_runs_upload_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_document_extraction_runs_upload_created ON public.document_extraction_runs USING btree (upload_id, created_at DESC);


--
-- Name: idx_document_extraction_runs_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_document_extraction_runs_user_created ON public.document_extraction_runs USING btree (user_id, created_at DESC);


--
-- Name: idx_document_indicator_reviews_extraction_run; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_document_indicator_reviews_extraction_run ON public.document_indicator_reviews USING btree (extraction_run_id, indicator_index, created_at DESC);


--
-- Name: idx_document_indicator_reviews_idem; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_document_indicator_reviews_idem ON public.document_indicator_reviews USING btree (user_id, extraction_run_id, indicator_index, idempotency_key);


--
-- Name: idx_document_indicator_reviews_upload_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_document_indicator_reviews_upload_created ON public.document_indicator_reviews USING btree (upload_id, created_at DESC);


--
-- Name: idx_document_indicator_reviews_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_document_indicator_reviews_user_created ON public.document_indicator_reviews USING btree (user_id, created_at DESC);


--
-- Name: idx_interventions_revision; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_interventions_revision ON public.interventions USING btree (treatment_revision_id, "position");


--
-- Name: idx_interventions_user_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_interventions_user_status ON public.interventions USING btree (user_id, status, created_at DESC);


--
-- Name: idx_job_events_job_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_job_events_job_id ON public.job_events USING btree (job_id);


--
-- Name: idx_jobs_idempotency_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_jobs_idempotency_key ON public.jobs USING btree (idempotency_key) WHERE (idempotency_key IS NOT NULL);


--
-- Name: idx_jobs_job_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_jobs_job_type ON public.jobs USING btree (job_type);


--
-- Name: idx_jobs_run_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_jobs_run_id ON public.jobs USING btree (run_id);


--
-- Name: idx_jobs_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_jobs_status ON public.jobs USING btree (status);


--
-- Name: idx_jobs_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_jobs_user_id ON public.jobs USING btree (user_id);


--
-- Name: idx_knowledge_clips_source_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_clips_source_id ON public.knowledge_clips USING btree (source_id);


--
-- Name: idx_knowledge_clips_source_unit_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_clips_source_unit_id ON public.knowledge_clips USING btree (source_unit_id);


--
-- Name: idx_knowledge_publication_observations_publication_kind_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_publication_observations_publication_kind_created ON public.knowledge_publication_observations USING btree (publication_id, observation_kind, created_at DESC);


--
-- Name: idx_knowledge_publications_batch_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_publications_batch_key ON public.knowledge_publications USING btree (publication_batch_key);


--
-- Name: idx_knowledge_publications_rollback_of; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_publications_rollback_of ON public.knowledge_publications USING btree (rollback_of);


--
-- Name: idx_knowledge_publications_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_publications_status ON public.knowledge_publications USING btree (status);


--
-- Name: idx_knowledge_publications_unit_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_publications_unit_id ON public.knowledge_publications USING btree (knowledge_unit_id);


--
-- Name: idx_knowledge_segments_source_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_segments_source_id ON public.knowledge_segments USING btree (source_id, segment_index);


--
-- Name: idx_knowledge_sources_content_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_sources_content_hash ON public.knowledge_sources USING btree (content_hash);


--
-- Name: idx_knowledge_sources_license_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_sources_license_status ON public.knowledge_sources USING btree (license_status);


--
-- Name: idx_knowledge_sources_problem_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_sources_problem_slug ON public.knowledge_sources USING btree (problem_slug);


--
-- Name: idx_knowledge_sources_registered_by; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_sources_registered_by ON public.knowledge_sources USING btree (registered_by);


--
-- Name: idx_knowledge_units_lifecycle_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_units_lifecycle_status ON public.knowledge_units USING btree (lifecycle_status);


--
-- Name: idx_knowledge_units_problem_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_units_problem_slug ON public.knowledge_units USING btree (problem_slug);


--
-- Name: idx_knowledge_units_publication_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_units_publication_id ON public.knowledge_units USING btree (publication_id);


--
-- Name: idx_knowledge_units_quality_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_units_quality_score ON public.knowledge_units USING btree (quality_score);


--
-- Name: idx_knowledge_units_source_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_units_source_id ON public.knowledge_units USING btree (source_id);


--
-- Name: idx_knowledge_units_unit_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_units_unit_type ON public.knowledge_units USING btree (unit_type);


--
-- Name: idx_messages_content_text_search; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_messages_content_text_search ON public.messages USING gin (to_tsvector('simple'::regconfig, COALESCE(content_text, ''::text)));


--
-- Name: idx_messages_conversation_role; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_messages_conversation_role ON public.messages USING btree (conversation_id, role);


--
-- Name: idx_messages_conversation_seq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_messages_conversation_seq ON public.messages USING btree (conversation_id, seq);


--
-- Name: idx_messages_run_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_messages_run_id ON public.messages USING btree (run_id);


--
-- Name: idx_messages_turn; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_messages_turn ON public.messages USING btree (turn_id);


--
-- Name: idx_outcomes_concern; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_outcomes_concern ON public.outcomes USING btree (user_id, concern_key, occurred_at DESC);


--
-- Name: idx_outcomes_treatment; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_outcomes_treatment ON public.outcomes USING btree (treatment_id, occurred_at DESC);


--
-- Name: idx_outcomes_user_occurred; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_outcomes_user_occurred ON public.outcomes USING btree (user_id, occurred_at DESC);


--
-- Name: idx_privacy_erasure_requests_recovery; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_privacy_erasure_requests_recovery ON public.privacy_erasure_requests USING btree (status, lease_expires_at, updated_at) WHERE ((status)::text = ANY ((ARRAY['pending'::character varying, 'running'::character varying, 'retryable'::character varying])::text[]));


--
-- Name: idx_runs_agent_configuration; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_runs_agent_configuration ON public.runs USING btree (agent_configuration_id, started_at DESC) WHERE ((agent_configuration_id)::text <> ''::text);


--
-- Name: idx_runs_conversation; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_runs_conversation ON public.runs USING btree (conversation_id);


--
-- Name: idx_runs_lease_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_runs_lease_owner ON public.runs USING btree (lease_owner) WHERE ((status)::text = 'running'::text);


--
-- Name: idx_runs_one_active_per_conversation; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_runs_one_active_per_conversation ON public.runs USING btree (conversation_id) WHERE ((status)::text = ANY ((ARRAY['running'::character varying, 'waiting_user'::character varying])::text[]));


--
-- Name: idx_runs_running_lease_expires_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_runs_running_lease_expires_at ON public.runs USING btree (status, lease_expires_at) WHERE ((status)::text = 'running'::text);


--
-- Name: idx_runs_turn; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_runs_turn ON public.runs USING btree (turn_id);


--
-- Name: idx_runtime_events_conversation_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_runtime_events_conversation_created ON public.runtime_events USING btree (conversation_id, created_at DESC);


--
-- Name: idx_runtime_events_run_seq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_runtime_events_run_seq ON public.runtime_events USING btree (run_id, seq);


--
-- Name: idx_runtime_events_type_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_runtime_events_type_created ON public.runtime_events USING btree (type, created_at DESC);


--
-- Name: idx_thread_projection_messages_conversation_seq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_thread_projection_messages_conversation_seq ON public.thread_projection_messages USING btree (conversation_id, seq);


--
-- Name: idx_thread_projection_tool_calls_conversation_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_thread_projection_tool_calls_conversation_created ON public.thread_projection_tool_calls USING btree (conversation_id, created_at);


--
-- Name: idx_thread_projection_tool_calls_run_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_thread_projection_tool_calls_run_created ON public.thread_projection_tool_calls USING btree (run_id, created_at);


--
-- Name: idx_thread_projections_user_updated; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_thread_projections_user_updated ON public.thread_projections USING btree (user_id, conversation_updated_at DESC);


--
-- Name: idx_training_logs_intervention; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_training_logs_intervention ON public.training_logs USING btree (intervention_id, date DESC);


--
-- Name: idx_training_logs_plan_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_training_logs_plan_id ON public.training_logs USING btree (plan_id);


--
-- Name: idx_training_logs_user_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_training_logs_user_date ON public.training_logs USING btree (user_id, date);


--
-- Name: idx_training_plans_treatment_revision; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_training_plans_treatment_revision ON public.training_plans USING btree (treatment_revision_id) WHERE (treatment_revision_id IS NOT NULL);


--
-- Name: idx_training_plans_user_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_training_plans_user_active ON public.training_plans USING btree (user_id) WHERE ((status)::text = 'active'::text);


--
-- Name: idx_training_plans_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_training_plans_user_id ON public.training_plans USING btree (user_id);


--
-- Name: idx_treatment_revisions_agent_configuration; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_treatment_revisions_agent_configuration ON public.treatment_revisions USING btree (agent_configuration_id, created_at DESC) WHERE ((agent_configuration_id)::text <> ''::text);


--
-- Name: idx_treatment_revisions_source_analysis; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_treatment_revisions_source_analysis ON public.treatment_revisions USING btree (source_diagnosis_analysis_id);


--
-- Name: idx_treatment_revisions_treatment_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_treatment_revisions_treatment_created ON public.treatment_revisions USING btree (treatment_id, revision DESC);


--
-- Name: idx_treatment_rollout_observations_pair_stage; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_treatment_rollout_observations_pair_stage ON public.treatment_rollout_observations USING btree (champion_configuration_id, challenger_configuration_id, stage, canary_bps, created_at DESC);


--
-- Name: idx_user_profiles_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_user_profiles_user_id ON public.user_profiles USING btree (user_id);


--
-- Name: idx_user_uploads_agent_configuration; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_uploads_agent_configuration ON public.user_uploads USING btree (agent_configuration_id) WHERE ((agent_configuration_id)::text <> ''::text);


--
-- Name: idx_user_uploads_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_uploads_created_at ON public.user_uploads USING btree (created_at);


--
-- Name: idx_user_uploads_file_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_uploads_file_type ON public.user_uploads USING btree (file_type);


--
-- Name: idx_user_uploads_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_uploads_user_id ON public.user_uploads USING btree (user_id);


--
-- Name: idx_user_uploads_user_type_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_uploads_user_type_created ON public.user_uploads USING btree (user_id, file_type, created_at DESC);


--
-- Name: idx_users_role; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_role ON public.users USING btree (role);


--
-- Name: uq_assessment_rollout_observation_pair; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_assessment_rollout_observation_pair ON public.assessment_rollout_observations USING btree (source_report_id, shadow_configuration_id);


--
-- Name: uq_consultation_rollout_observation_pair; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_consultation_rollout_observation_pair ON public.consultation_rollout_observations USING btree (run_id, challenger_configuration_id);


--
-- Name: uq_privacy_erasure_requests_active_subject; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_privacy_erasure_requests_active_subject ON public.privacy_erasure_requests USING btree (subject_user_id) WHERE (subject_user_id IS NOT NULL);


--
-- Name: uq_user_uploads_storage_identity; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_user_uploads_storage_identity ON public.user_uploads USING btree (storage_backend, storage_key);


--
-- Name: jobs update_jobs_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_jobs_updated_at BEFORE UPDATE ON public.jobs FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: knowledge_clips update_knowledge_clips_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_knowledge_clips_updated_at BEFORE UPDATE ON public.knowledge_clips FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: knowledge_publications update_knowledge_publications_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_knowledge_publications_updated_at BEFORE UPDATE ON public.knowledge_publications FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: knowledge_sources update_knowledge_sources_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_knowledge_sources_updated_at BEFORE UPDATE ON public.knowledge_sources FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: knowledge_units update_knowledge_units_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_knowledge_units_updated_at BEFORE UPDATE ON public.knowledge_units FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: agent_interactions agent_interactions_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_interactions
    ADD CONSTRAINT agent_interactions_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: agent_interactions agent_interactions_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_interactions
    ADD CONSTRAINT agent_interactions_run_id_fkey FOREIGN KEY (run_id) REFERENCES public.runs(id) ON DELETE CASCADE;


--
-- Name: agent_tool_calls agent_tool_calls_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_tool_calls
    ADD CONSTRAINT agent_tool_calls_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: agent_tool_calls agent_tool_calls_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_tool_calls
    ADD CONSTRAINT agent_tool_calls_message_id_fkey FOREIGN KEY (message_id) REFERENCES public.messages(id) ON DELETE SET NULL;


--
-- Name: agent_tool_calls agent_tool_calls_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.agent_tool_calls
    ADD CONSTRAINT agent_tool_calls_run_id_fkey FOREIGN KEY (run_id) REFERENCES public.runs(id) ON DELETE CASCADE;


--
-- Name: ai_output_reviews ai_output_reviews_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_output_reviews
    ADD CONSTRAINT ai_output_reviews_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE SET NULL;


--
-- Name: ai_output_reviews ai_output_reviews_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_output_reviews
    ADD CONSTRAINT ai_output_reviews_job_id_fkey FOREIGN KEY (job_id) REFERENCES public.jobs(id) ON DELETE SET NULL;


--
-- Name: ai_output_reviews ai_output_reviews_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_output_reviews
    ADD CONSTRAINT ai_output_reviews_run_id_fkey FOREIGN KEY (run_id) REFERENCES public.runs(id) ON DELETE SET NULL;


--
-- Name: assessment_reports assessment_reports_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.assessment_reports
    ADD CONSTRAINT assessment_reports_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: assessment_rollout_observations assessment_rollout_observations_source_report_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.assessment_rollout_observations
    ADD CONSTRAINT assessment_rollout_observations_source_report_id_fkey FOREIGN KEY (source_report_id) REFERENCES public.assessment_reports(id) ON DELETE CASCADE;


--
-- Name: body_state_evidence body_state_evidence_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_evidence
    ADD CONSTRAINT body_state_evidence_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: body_state_facts body_state_facts_supersedes_fact_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_facts
    ADD CONSTRAINT body_state_facts_supersedes_fact_id_fkey FOREIGN KEY (supersedes_fact_id) REFERENCES public.body_state_facts(id) ON DELETE SET NULL;


--
-- Name: body_state_facts body_state_facts_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_facts
    ADD CONSTRAINT body_state_facts_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: body_state_hypotheses body_state_hypotheses_source_analysis_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_hypotheses
    ADD CONSTRAINT body_state_hypotheses_source_analysis_id_fkey FOREIGN KEY (source_analysis_id) REFERENCES public.diagnosis_analyses(id) ON DELETE SET NULL;


--
-- Name: body_state_hypotheses body_state_hypotheses_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_hypotheses
    ADD CONSTRAINT body_state_hypotheses_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: body_state_observations body_state_observations_supersedes_observation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_observations
    ADD CONSTRAINT body_state_observations_supersedes_observation_id_fkey FOREIGN KEY (supersedes_observation_id) REFERENCES public.body_state_observations(id) ON DELETE SET NULL;


--
-- Name: body_state_observations body_state_observations_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_observations
    ADD CONSTRAINT body_state_observations_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: body_state_revisions body_state_revisions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_revisions
    ADD CONSTRAINT body_state_revisions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: body_states body_states_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_states
    ADD CONSTRAINT body_states_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: consultation_rollout_observations consultation_rollout_observations_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.consultation_rollout_observations
    ADD CONSTRAINT consultation_rollout_observations_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: consultation_rollout_observations consultation_rollout_observations_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.consultation_rollout_observations
    ADD CONSTRAINT consultation_rollout_observations_run_id_fkey FOREIGN KEY (run_id) REFERENCES public.runs(id) ON DELETE CASCADE;


--
-- Name: consultation_sessions consultation_sessions_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.consultation_sessions
    ADD CONSTRAINT consultation_sessions_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: conversation_shares conversation_shares_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversation_shares
    ADD CONSTRAINT conversation_shares_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: diagnosis_analyses diagnosis_analyses_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_analyses
    ADD CONSTRAINT diagnosis_analyses_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: diagnosis_analysis_freshness diagnosis_analysis_freshness_analysis_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_analysis_freshness
    ADD CONSTRAINT diagnosis_analysis_freshness_analysis_id_fkey FOREIGN KEY (analysis_id) REFERENCES public.diagnosis_analyses(id) ON DELETE CASCADE;


--
-- Name: diagnosis_analysis_freshness diagnosis_analysis_freshness_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_analysis_freshness
    ADD CONSTRAINT diagnosis_analysis_freshness_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: diagnosis_candidate_assessments diagnosis_candidate_assessments_analysis_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_candidate_assessments
    ADD CONSTRAINT diagnosis_candidate_assessments_analysis_id_fkey FOREIGN KEY (analysis_id) REFERENCES public.diagnosis_analyses(id) ON DELETE CASCADE;


--
-- Name: diagnosis_candidate_assessments diagnosis_candidate_assessments_candidate_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_candidate_assessments
    ADD CONSTRAINT diagnosis_candidate_assessments_candidate_id_fkey FOREIGN KEY (candidate_id) REFERENCES public.diagnosis_candidates(id) ON DELETE CASCADE;


--
-- Name: diagnosis_candidate_assessments diagnosis_candidate_assessments_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_candidate_assessments
    ADD CONSTRAINT diagnosis_candidate_assessments_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: diagnosis_candidates diagnosis_candidates_analysis_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_candidates
    ADD CONSTRAINT diagnosis_candidates_analysis_id_fkey FOREIGN KEY (analysis_id) REFERENCES public.diagnosis_analyses(id) ON DELETE CASCADE;


--
-- Name: diagnosis_rollout_observations diagnosis_rollout_observations_source_analysis_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.diagnosis_rollout_observations
    ADD CONSTRAINT diagnosis_rollout_observations_source_analysis_id_fkey FOREIGN KEY (source_analysis_id) REFERENCES public.diagnosis_analyses(id) ON DELETE CASCADE;


--
-- Name: document_extraction_runs document_extraction_runs_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_extraction_runs
    ADD CONSTRAINT document_extraction_runs_job_id_fkey FOREIGN KEY (job_id) REFERENCES public.jobs(id) ON DELETE SET NULL;


--
-- Name: document_extraction_runs document_extraction_runs_upload_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_extraction_runs
    ADD CONSTRAINT document_extraction_runs_upload_id_fkey FOREIGN KEY (upload_id) REFERENCES public.user_uploads(id) ON DELETE CASCADE;


--
-- Name: document_extraction_runs document_extraction_runs_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_extraction_runs
    ADD CONSTRAINT document_extraction_runs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: document_indicator_reviews document_indicator_reviews_extraction_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_indicator_reviews
    ADD CONSTRAINT document_indicator_reviews_extraction_run_id_fkey FOREIGN KEY (extraction_run_id) REFERENCES public.document_extraction_runs(id) ON DELETE CASCADE;


--
-- Name: document_indicator_reviews document_indicator_reviews_reviewer_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_indicator_reviews
    ADD CONSTRAINT document_indicator_reviews_reviewer_user_id_fkey FOREIGN KEY (reviewer_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: document_indicator_reviews document_indicator_reviews_upload_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_indicator_reviews
    ADD CONSTRAINT document_indicator_reviews_upload_id_fkey FOREIGN KEY (upload_id) REFERENCES public.user_uploads(id) ON DELETE CASCADE;


--
-- Name: document_indicator_reviews document_indicator_reviews_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_indicator_reviews
    ADD CONSTRAINT document_indicator_reviews_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: ai_output_reviews fk_ai_output_reviews_user_erasure; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_output_reviews
    ADD CONSTRAINT fk_ai_output_reviews_user_erasure FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: conversations fk_conversations_user_erasure; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversations
    ADD CONSTRAINT fk_conversations_user_erasure FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: jobs fk_jobs_user_erasure; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT fk_jobs_user_erasure FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: knowledge_units fk_knowledge_units_publication_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_units
    ADD CONSTRAINT fk_knowledge_units_publication_id FOREIGN KEY (publication_id) REFERENCES public.knowledge_publications(id) ON DELETE SET NULL;


--
-- Name: runs fk_runs_user_erasure; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.runs
    ADD CONSTRAINT fk_runs_user_erasure FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: interventions interventions_treatment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interventions
    ADD CONSTRAINT interventions_treatment_id_fkey FOREIGN KEY (treatment_id) REFERENCES public.treatments(id) ON DELETE CASCADE;


--
-- Name: interventions interventions_treatment_revision_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interventions
    ADD CONSTRAINT interventions_treatment_revision_id_fkey FOREIGN KEY (treatment_revision_id) REFERENCES public.treatment_revisions(id) ON DELETE CASCADE;


--
-- Name: interventions interventions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interventions
    ADD CONSTRAINT interventions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: job_events job_events_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.job_events
    ADD CONSTRAINT job_events_job_id_fkey FOREIGN KEY (job_id) REFERENCES public.jobs(id) ON DELETE CASCADE;


--
-- Name: jobs jobs_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE SET NULL;


--
-- Name: jobs jobs_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_run_id_fkey FOREIGN KEY (run_id) REFERENCES public.runs(id) ON DELETE SET NULL;


--
-- Name: knowledge_clips knowledge_clips_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_clips
    ADD CONSTRAINT knowledge_clips_source_id_fkey FOREIGN KEY (source_id) REFERENCES public.knowledge_sources(id) ON DELETE CASCADE;


--
-- Name: knowledge_clips knowledge_clips_source_unit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_clips
    ADD CONSTRAINT knowledge_clips_source_unit_id_fkey FOREIGN KEY (source_unit_id) REFERENCES public.knowledge_units(id) ON DELETE SET NULL;


--
-- Name: knowledge_publication_observations knowledge_publication_observations_publication_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_publication_observations
    ADD CONSTRAINT knowledge_publication_observations_publication_id_fkey FOREIGN KEY (publication_id) REFERENCES public.knowledge_publications(id) ON DELETE CASCADE;


--
-- Name: knowledge_publications knowledge_publications_knowledge_unit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_publications
    ADD CONSTRAINT knowledge_publications_knowledge_unit_id_fkey FOREIGN KEY (knowledge_unit_id) REFERENCES public.knowledge_units(id) ON DELETE CASCADE;


--
-- Name: knowledge_publications knowledge_publications_rollback_of_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_publications
    ADD CONSTRAINT knowledge_publications_rollback_of_fkey FOREIGN KEY (rollback_of) REFERENCES public.knowledge_publications(id) ON DELETE SET NULL;


--
-- Name: knowledge_segments knowledge_segments_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_segments
    ADD CONSTRAINT knowledge_segments_source_id_fkey FOREIGN KEY (source_id) REFERENCES public.knowledge_sources(id) ON DELETE CASCADE;


--
-- Name: knowledge_sources knowledge_sources_registered_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_sources
    ADD CONSTRAINT knowledge_sources_registered_by_fkey FOREIGN KEY (registered_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: knowledge_units knowledge_units_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_units
    ADD CONSTRAINT knowledge_units_source_id_fkey FOREIGN KEY (source_id) REFERENCES public.knowledge_sources(id) ON DELETE CASCADE;


--
-- Name: messages messages_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.messages
    ADD CONSTRAINT messages_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: messages messages_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.messages
    ADD CONSTRAINT messages_run_id_fkey FOREIGN KEY (run_id) REFERENCES public.runs(id);


--
-- Name: outcomes outcomes_intervention_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outcomes
    ADD CONSTRAINT outcomes_intervention_id_fkey FOREIGN KEY (intervention_id) REFERENCES public.interventions(id) ON DELETE SET NULL;


--
-- Name: outcomes outcomes_treatment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outcomes
    ADD CONSTRAINT outcomes_treatment_id_fkey FOREIGN KEY (treatment_id) REFERENCES public.treatments(id) ON DELETE SET NULL;


--
-- Name: outcomes outcomes_treatment_revision_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outcomes
    ADD CONSTRAINT outcomes_treatment_revision_id_fkey FOREIGN KEY (treatment_revision_id) REFERENCES public.treatment_revisions(id) ON DELETE SET NULL;


--
-- Name: outcomes outcomes_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outcomes
    ADD CONSTRAINT outcomes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: runs runs_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.runs
    ADD CONSTRAINT runs_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: runtime_events runtime_events_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.runtime_events
    ADD CONSTRAINT runtime_events_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: runtime_events runtime_events_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.runtime_events
    ADD CONSTRAINT runtime_events_run_id_fkey FOREIGN KEY (run_id) REFERENCES public.runs(id) ON DELETE CASCADE;


--
-- Name: thread_projection_messages thread_projection_messages_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_projection_messages
    ADD CONSTRAINT thread_projection_messages_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: thread_projection_messages thread_projection_messages_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_projection_messages
    ADD CONSTRAINT thread_projection_messages_message_id_fkey FOREIGN KEY (message_id) REFERENCES public.messages(id) ON DELETE CASCADE;


--
-- Name: thread_projection_tool_calls thread_projection_tool_calls_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_projection_tool_calls
    ADD CONSTRAINT thread_projection_tool_calls_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: thread_projection_tool_calls thread_projection_tool_calls_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_projection_tool_calls
    ADD CONSTRAINT thread_projection_tool_calls_message_id_fkey FOREIGN KEY (message_id) REFERENCES public.messages(id) ON DELETE SET NULL;


--
-- Name: thread_projection_tool_calls thread_projection_tool_calls_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_projection_tool_calls
    ADD CONSTRAINT thread_projection_tool_calls_run_id_fkey FOREIGN KEY (run_id) REFERENCES public.runs(id) ON DELETE CASCADE;


--
-- Name: thread_projections thread_projections_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_projections
    ADD CONSTRAINT thread_projections_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: thread_projections thread_projections_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.thread_projections
    ADD CONSTRAINT thread_projections_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: training_logs training_logs_intervention_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_logs
    ADD CONSTRAINT training_logs_intervention_id_fkey FOREIGN KEY (intervention_id) REFERENCES public.interventions(id) ON DELETE SET NULL;


--
-- Name: training_logs training_logs_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_logs
    ADD CONSTRAINT training_logs_plan_id_fkey FOREIGN KEY (plan_id) REFERENCES public.training_plans(id) ON DELETE CASCADE;


--
-- Name: training_logs training_logs_treatment_revision_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_logs
    ADD CONSTRAINT training_logs_treatment_revision_id_fkey FOREIGN KEY (treatment_revision_id) REFERENCES public.treatment_revisions(id) ON DELETE SET NULL;


--
-- Name: training_logs training_logs_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_logs
    ADD CONSTRAINT training_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: training_plans training_plans_treatment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_plans
    ADD CONSTRAINT training_plans_treatment_id_fkey FOREIGN KEY (treatment_id) REFERENCES public.treatments(id) ON DELETE SET NULL;


--
-- Name: training_plans training_plans_treatment_revision_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_plans
    ADD CONSTRAINT training_plans_treatment_revision_id_fkey FOREIGN KEY (treatment_revision_id) REFERENCES public.treatment_revisions(id) ON DELETE SET NULL;


--
-- Name: training_plans training_plans_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_plans
    ADD CONSTRAINT training_plans_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: treatment_revisions treatment_revisions_source_diagnosis_analysis_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.treatment_revisions
    ADD CONSTRAINT treatment_revisions_source_diagnosis_analysis_id_fkey FOREIGN KEY (source_diagnosis_analysis_id) REFERENCES public.diagnosis_analyses(id) ON DELETE RESTRICT;


--
-- Name: treatment_revisions treatment_revisions_treatment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.treatment_revisions
    ADD CONSTRAINT treatment_revisions_treatment_id_fkey FOREIGN KEY (treatment_id) REFERENCES public.treatments(id) ON DELETE CASCADE;


--
-- Name: treatment_rollout_observations treatment_rollout_observations_source_revision_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.treatment_rollout_observations
    ADD CONSTRAINT treatment_rollout_observations_source_revision_id_fkey FOREIGN KEY (source_revision_id) REFERENCES public.treatment_revisions(id) ON DELETE CASCADE;


--
-- Name: treatments treatments_source_diagnosis_analysis_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.treatments
    ADD CONSTRAINT treatments_source_diagnosis_analysis_id_fkey FOREIGN KEY (source_diagnosis_analysis_id) REFERENCES public.diagnosis_analyses(id) ON DELETE SET NULL;


--
-- Name: treatments treatments_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.treatments
    ADD CONSTRAINT treatments_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_profiles user_profiles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT user_profiles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_uploads user_uploads_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_uploads
    ADD CONSTRAINT user_uploads_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--
