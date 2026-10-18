-- BodySense V2 canonical PostgreSQL 18 baseline.
--
-- 2026-10-15 V2 big-bang rebuild: there is no user data to preserve, so the
-- vNext chain 000001-000012 was folded into this single baseline and runtime
-- databases are reset instead of upgraded. Git history keeps the old chain.
-- Later V2 domain work edits this baseline directly until first real release.

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
    CONSTRAINT agent_interactions_status_check CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('answered'::character varying)::text, ('cancelled'::character varying)::text, ('expired'::character varying)::text])))
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
    CONSTRAINT agent_tool_calls_status_check CHECK (((status)::text = ANY (ARRAY[('running'::character varying)::text, ('succeeded'::character varying)::text, ('failed'::character varying)::text])))
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
    CONSTRAINT assessment_reports_status_check CHECK (((status)::text = ANY (ARRAY[('completed'::character varying)::text, ('insufficient_information'::character varying)::text])))
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
-- Name: body_fact_commits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_fact_commits (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    idempotency_key character varying(128) NOT NULL,
    fingerprint character(64) NOT NULL,
    revision bigint NOT NULL,
    revision_id uuid NOT NULL,
    event_id uuid NOT NULL,
    fact_ids jsonb NOT NULL,
    committed_at timestamp with time zone NOT NULL,
    CONSTRAINT body_fact_commits_fact_ids_check CHECK (((jsonb_typeof(fact_ids) = 'array'::text) AND ((jsonb_array_length(fact_ids) >= 1) AND (jsonb_array_length(fact_ids) <= 20)))),
    CONSTRAINT body_fact_commits_idempotency_key_check CHECK ((length(TRIM(BOTH FROM idempotency_key)) > 0))
);


--
-- Name: body_facts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_facts (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    subject text NOT NULL,
    kind text NOT NULL,
    statement text NOT NULL,
    body_region_ids jsonb NOT NULL,
    temporal jsonb NOT NULL,
    details jsonb NOT NULL,
    source_evidence_id uuid NOT NULL,
    assertion_status text NOT NULL,
    created_revision bigint NOT NULL,
    recorded_at timestamp with time zone NOT NULL,
    supersedes_fact_id uuid,
    CONSTRAINT body_facts_assertion_status_check CHECK ((assertion_status = ANY (ARRAY['asserted'::text, 'superseded'::text, 'withdrawn'::text]))),
    CONSTRAINT body_facts_body_region_ids_check CHECK (((jsonb_typeof(body_region_ids) = 'array'::text) AND (jsonb_array_length(body_region_ids) <= 20))),
    CONSTRAINT body_facts_created_revision_check CHECK ((created_revision > 0)),
    CONSTRAINT body_facts_details_check CHECK ((jsonb_typeof(details) = 'object'::text)),
    CONSTRAINT body_facts_kind_check CHECK ((kind = ANY (ARRAY['symptom'::text, 'health_history'::text, 'activity'::text, 'lifestyle'::text, 'training_feedback'::text]))),
    CONSTRAINT body_facts_no_self_supersession CHECK (((supersedes_fact_id IS NULL) OR (supersedes_fact_id <> id))),
    CONSTRAINT body_facts_statement_check CHECK ((((char_length(statement) >= 1) AND (char_length(statement) <= 2000)) AND (char_length(btrim(statement)) > 0))),
    CONSTRAINT body_facts_subject_check CHECK ((subject = 'self'::text)),
    CONSTRAINT body_facts_temporal_check CHECK (((jsonb_typeof(temporal) = 'object'::text) AND (temporal ? 'precision'::text) AND (jsonb_typeof((temporal -> 'precision'::text)) = 'string'::text) AND ((temporal ->> 'precision'::text) = ANY (ARRAY['exact'::text, 'day'::text, 'range'::text, 'approximate'::text, 'unknown'::text]))))
);


--
-- Name: body_hypotheses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_hypotheses (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    statement text NOT NULL,
    source_evidence_id uuid NOT NULL,
    assertion_status text NOT NULL,
    created_revision bigint NOT NULL,
    recorded_at timestamp with time zone NOT NULL,
    withdrawn_at timestamp with time zone,
    withdrawn_revision bigint,
    withdrawal_reason text,
    supersedes_hypothesis_id uuid,
    lineage_id uuid NOT NULL,
    CONSTRAINT body_hypotheses_assertion_status_check CHECK ((assertion_status = ANY (ARRAY['asserted'::text, 'superseded'::text, 'withdrawn'::text]))),
    CONSTRAINT body_hypotheses_created_revision_check CHECK ((created_revision > 0)),
    CONSTRAINT body_hypotheses_lineage_check CHECK ((((supersedes_hypothesis_id IS NULL) AND (lineage_id = id)) OR ((supersedes_hypothesis_id IS NOT NULL) AND (supersedes_hypothesis_id <> id) AND (lineage_id <> id)))),
    CONSTRAINT body_hypotheses_statement_check CHECK ((((char_length(statement) >= 1) AND (char_length(statement) <= 2000)) AND (statement = btrim(statement)))),
    CONSTRAINT body_hypotheses_withdrawal_check CHECK ((((assertion_status = ANY (ARRAY['asserted'::text, 'superseded'::text])) AND (withdrawn_at IS NULL) AND (withdrawn_revision IS NULL) AND (withdrawal_reason IS NULL)) OR ((assertion_status = 'withdrawn'::text) AND (withdrawn_at IS NOT NULL) AND (withdrawn_revision > created_revision)))),
    CONSTRAINT body_hypotheses_withdrawal_reason_check CHECK (((withdrawal_reason IS NULL) OR (((char_length(withdrawal_reason) >= 1) AND (char_length(withdrawal_reason) <= 500)) AND (withdrawal_reason = btrim(withdrawal_reason)))))
);


--
-- Name: body_hypothesis_commits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_hypothesis_commits (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    idempotency_key character varying(128) NOT NULL,
    fingerprint character(64) NOT NULL,
    operation text NOT NULL,
    hypothesis_id uuid NOT NULL,
    source_evidence_id uuid NOT NULL,
    revision bigint NOT NULL,
    revision_id uuid NOT NULL,
    event_id uuid NOT NULL,
    committed_at timestamp with time zone NOT NULL,
    target_hypothesis_id uuid,
    CONSTRAINT body_hypothesis_commits_fingerprint_check CHECK ((fingerprint ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT body_hypothesis_commits_idempotency_key_check CHECK ((((char_length((idempotency_key)::text) >= 1) AND (char_length((idempotency_key)::text) <= 128)) AND ((idempotency_key)::text = btrim((idempotency_key)::text)))),
    CONSTRAINT body_hypothesis_commits_operation_check CHECK ((operation = ANY (ARRAY['record'::text, 'correct'::text, 'withdraw'::text]))),
    CONSTRAINT body_hypothesis_commits_operation_target_check CHECK ((((operation = 'record'::text) AND (target_hypothesis_id IS NULL)) OR ((operation = 'correct'::text) AND (target_hypothesis_id IS NOT NULL) AND (hypothesis_id <> target_hypothesis_id)) OR ((operation = 'withdraw'::text) AND (target_hypothesis_id IS NOT NULL) AND (hypothesis_id = target_hypothesis_id)))),
    CONSTRAINT body_hypothesis_commits_revision_check CHECK ((revision > 0))
);


--
-- Name: body_hypothesis_fact_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_hypothesis_fact_links (
    user_id uuid NOT NULL,
    hypothesis_id uuid NOT NULL,
    fact_id uuid NOT NULL,
    ordinal smallint NOT NULL,
    CONSTRAINT body_hypothesis_fact_links_ordinal_check CHECK (((ordinal >= 0) AND (ordinal < 20)))
);


--
-- Name: body_hypothesis_observation_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_hypothesis_observation_links (
    user_id uuid NOT NULL,
    hypothesis_id uuid NOT NULL,
    observation_id uuid NOT NULL,
    ordinal smallint NOT NULL,
    CONSTRAINT body_hypothesis_observation_links_ordinal_check CHECK (((ordinal >= 0) AND (ordinal < 20)))
);


--
-- Name: body_observation_commits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_observation_commits (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    idempotency_key character varying(128) NOT NULL,
    fingerprint character(64) NOT NULL,
    observation_id uuid NOT NULL,
    source_evidence_id uuid NOT NULL,
    revision bigint NOT NULL,
    revision_id uuid NOT NULL,
    event_id uuid NOT NULL,
    committed_at timestamp with time zone NOT NULL,
    operation text DEFAULT 'record'::text NOT NULL,
    target_observation_id uuid,
    CONSTRAINT body_observation_commits_fingerprint_check CHECK ((fingerprint ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT body_observation_commits_idempotency_key_check CHECK ((((char_length((idempotency_key)::text) >= 1) AND (char_length((idempotency_key)::text) <= 128)) AND ((idempotency_key)::text = btrim((idempotency_key)::text)))),
    CONSTRAINT body_observation_commits_revision_check CHECK ((revision > 0)),
    CONSTRAINT body_observation_receipt_operation_check CHECK ((operation = ANY (ARRAY['record'::text, 'correct'::text, 'withdraw'::text]))),
    CONSTRAINT body_observation_receipt_operation_target_check CHECK ((((operation = 'record'::text) AND (target_observation_id IS NULL)) OR ((operation = 'correct'::text) AND (target_observation_id IS NOT NULL) AND (observation_id <> target_observation_id)) OR ((operation = 'withdraw'::text) AND (target_observation_id IS NOT NULL) AND (observation_id = target_observation_id))))
);


--
-- Name: body_observations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_observations (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    kind character varying(64) NOT NULL,
    raw_value text DEFAULT ''::text NOT NULL,
    raw_unit text DEFAULT ''::text NOT NULL,
    method text DEFAULT ''::text NOT NULL,
    measurement_context text DEFAULT ''::text NOT NULL,
    observed_at jsonb NOT NULL,
    source_evidence_id uuid NOT NULL,
    assertion_status text NOT NULL,
    supersedes_observation_id uuid,
    created_revision bigint NOT NULL,
    recorded_at timestamp with time zone NOT NULL,
    CONSTRAINT body_observations_assertion_status_check CHECK ((assertion_status = ANY (ARRAY['asserted'::text, 'superseded'::text, 'withdrawn'::text]))),
    CONSTRAINT body_observations_check CHECK (((raw_value <> ''::text) OR (raw_unit = ''::text))),
    CONSTRAINT body_observations_check1 CHECK (((supersedes_observation_id IS NULL) OR (supersedes_observation_id <> id))),
    CONSTRAINT body_observations_created_revision_check CHECK ((created_revision > 0)),
    CONSTRAINT body_observations_kind_check CHECK (((kind)::text ~ '^[a-z][a-z0-9_]{1,63}$'::text)),
    CONSTRAINT body_observations_measurement_context_check CHECK ((char_length(measurement_context) <= 500)),
    CONSTRAINT body_observations_method_check CHECK ((char_length(method) <= 240)),
    CONSTRAINT body_observations_observed_at_check CHECK (((jsonb_typeof(observed_at) = 'object'::text) AND (observed_at ? 'precision'::text) AND ((observed_at ->> 'precision'::text) = ANY (ARRAY['exact'::text, 'day'::text, 'range'::text, 'approximate'::text, 'unknown'::text])))),
    CONSTRAINT body_observations_raw_unit_check CHECK ((char_length(raw_unit) <= 48)),
    CONSTRAINT body_observations_raw_value_check CHECK ((char_length(raw_value) <= 120))
);


--
-- Name: body_state_outbox; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.body_state_outbox (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    revision bigint NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL,
    CONSTRAINT body_state_outbox_payload_check CHECK ((jsonb_typeof(payload) = 'object'::text))
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
    CONSTRAINT consultation_sessions_phase_check CHECK (((phase)::text = ANY (ARRAY[('collecting'::character varying)::text, ('ready_for_analysis'::character varying)::text])))
);


--
-- Name: conversations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conversations (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
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
    processed_through_seq integer DEFAULT 0 NOT NULL,
    context_start_seq integer DEFAULT 1 NOT NULL,
    pruned_through_seq integer DEFAULT 0 NOT NULL,
    retention_revision bigint DEFAULT 0 NOT NULL,
    CONSTRAINT conversations_retention_cursor_order_check CHECK (((pruned_through_seq >= 0) AND (processed_through_seq >= pruned_through_seq) AND (context_start_seq > pruned_through_seq) AND (context_start_seq <= (processed_through_seq + 1)) AND (retention_revision >= 0)))
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
    CONSTRAINT diagnosis_analysis_freshness_state_check CHECK (((state)::text = ANY (ARRAY[('fresh'::character varying)::text, ('potentially_stale'::character varying)::text, ('stale'::character varying)::text])))
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
    user_id uuid NOT NULL,
    asset_id uuid NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    job_id uuid,
    configuration_id character varying(80) NOT NULL,
    mechanism_revision character varying(120),
    document_sha256 character(64),
    result_sha256 character(64),
    raw_text_sha256 character(64),
    result_payload jsonb,
    indicator_snapshot jsonb DEFAULT '[]'::jsonb NOT NULL,
    source_summary jsonb DEFAULT '{}'::jsonb NOT NULL,
    mechanism_provenance jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT document_extraction_runs_document_sha256_check CHECK ((document_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT document_extraction_runs_raw_text_sha256_check CHECK ((raw_text_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT document_extraction_runs_result_sha256_check CHECK ((result_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT document_extraction_runs_status_check CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('processing'::character varying)::text, ('completed'::character varying)::text, ('failed'::character varying)::text]))),
    CONSTRAINT document_extraction_runs_completed_audit_check CHECK (((status)::text <> 'completed'::text) OR ((document_sha256 IS NOT NULL) AND (result_sha256 IS NOT NULL) AND (raw_text_sha256 IS NOT NULL) AND (mechanism_revision IS NOT NULL) AND (result_payload IS NOT NULL)))
);


--
-- Name: document_indicator_reviews; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.document_indicator_reviews (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    asset_id uuid NOT NULL,
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
    CONSTRAINT document_indicator_reviews_action_check CHECK (((action)::text = ANY (ARRAY[('confirm'::character varying)::text, ('correct'::character varying)::text, ('reject'::character varying)::text]))),
    CONSTRAINT document_indicator_reviews_idempotency_key_check CHECK (((idempotency_key)::text <> ''::text)),
    CONSTRAINT document_indicator_reviews_indicator_index_check CHECK ((indicator_index >= 0))
);


--
-- Name: episode_command_receipts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.episode_command_receipts (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    idempotency_key character varying(128) NOT NULL,
    fingerprint character(64) NOT NULL,
    operation text NOT NULL,
    episode_id uuid NOT NULL,
    fact_id uuid NOT NULL,
    revision bigint NOT NULL,
    revision_id uuid NOT NULL,
    event_id uuid NOT NULL,
    committed_at timestamp with time zone NOT NULL,
    CONSTRAINT episode_command_receipts_fingerprint_check CHECK ((fingerprint ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT episode_command_receipts_idempotency_key_check CHECK ((((length((idempotency_key)::text) >= 1) AND (length((idempotency_key)::text) <= 128)) AND ((idempotency_key)::text = btrim((idempotency_key)::text)))),
    CONSTRAINT episode_command_receipts_operation_check CHECK ((operation = ANY (ARRAY['create'::text, 'link'::text, 'status'::text, 'recurrence'::text]))),
    CONSTRAINT episode_command_receipts_revision_check CHECK ((revision > 0))
);


--
-- Name: episode_fact_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.episode_fact_links (
    user_id uuid NOT NULL,
    episode_id uuid NOT NULL,
    fact_id uuid NOT NULL,
    fact_kind text DEFAULT 'symptom'::text NOT NULL,
    basis_evidence_id uuid NOT NULL,
    role text NOT NULL,
    association_method text NOT NULL,
    is_primary boolean NOT NULL,
    authorization_id uuid,
    created_revision bigint NOT NULL,
    CONSTRAINT episode_fact_links_association_method_check CHECK ((association_method = ANY (ARRAY['explicit'::text, 'inferred'::text, 'user_corrected'::text]))),
    CONSTRAINT episode_fact_links_check CHECK (((association_method <> 'user_corrected'::text) OR (authorization_id IS NOT NULL))),
    CONSTRAINT episode_fact_links_created_revision_check CHECK ((created_revision > 0)),
    CONSTRAINT episode_fact_links_fact_kind_check CHECK ((fact_kind = 'symptom'::text)),
    CONSTRAINT episode_fact_links_role_check CHECK ((role = ANY (ARRAY['onset'::text, 'observation'::text, 'change'::text, 'resolution'::text])))
);


--
-- Name: episode_relations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.episode_relations (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    from_episode_id uuid NOT NULL,
    to_episode_id uuid NOT NULL,
    kind text NOT NULL,
    basis_fact_id uuid NOT NULL,
    basis_evidence_id uuid NOT NULL,
    authorization_id uuid NOT NULL,
    created_revision bigint NOT NULL,
    created_at timestamp with time zone NOT NULL,
    CONSTRAINT episode_relations_check CHECK ((from_episode_id <> to_episode_id)),
    CONSTRAINT episode_relations_created_revision_check CHECK ((created_revision > 0)),
    CONSTRAINT episode_relations_kind_check CHECK ((kind = 'recurrence_reported'::text))
);


--
-- Name: episode_state_evidence; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.episode_state_evidence (
    user_id uuid NOT NULL,
    episode_id uuid NOT NULL,
    fact_id uuid NOT NULL,
    source_evidence_id uuid NOT NULL,
    claim text NOT NULL,
    as_of jsonb NOT NULL,
    created_revision bigint NOT NULL,
    CONSTRAINT episode_state_evidence_as_of_check CHECK (((jsonb_typeof(as_of) = 'object'::text) AND (as_of ? 'precision'::text) AND ((as_of ->> 'precision'::text) = ANY (ARRAY['exact'::text, 'day'::text, 'range'::text, 'approximate'::text, 'unknown'::text])))),
    CONSTRAINT episode_state_evidence_claim_check CHECK ((claim = ANY (ARRAY['explicit_current_report'::text, 'explicit_resolution'::text]))),
    CONSTRAINT episode_state_evidence_created_revision_check CHECK ((created_revision > 0))
);


--
-- Name: equipment_assets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.equipment_assets (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    display_name text NOT NULL,
    notes text,
    access_kind text NOT NULL,
    availability text NOT NULL,
    valid_from timestamp with time zone,
    valid_until timestamp with time zone,
    revision bigint DEFAULT 1 NOT NULL,
    create_idempotency_key text NOT NULL,
    create_request_digest text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT equipment_assets_access_kind_check CHECK ((access_kind = ANY (ARRAY['owned'::text, 'borrowed'::text, 'facility_access'::text]))),
    CONSTRAINT equipment_assets_availability_check CHECK ((availability = ANY (ARRAY['available'::text, 'unavailable'::text, 'retired'::text]))),
    CONSTRAINT equipment_assets_create_idempotency_key_check CHECK (((char_length(create_idempotency_key) >= 1) AND (char_length(create_idempotency_key) <= 128))),
    CONSTRAINT equipment_assets_create_request_digest_check CHECK ((create_request_digest ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT equipment_assets_display_name_check CHECK ((((char_length(display_name) >= 1) AND (char_length(display_name) <= 80)) AND (display_name = btrim(display_name)))),
    CONSTRAINT equipment_assets_notes_check CHECK (((notes IS NULL) OR (((char_length(notes) >= 1) AND (char_length(notes) <= 240)) AND (notes = btrim(notes))))),
    CONSTRAINT equipment_assets_revision_check CHECK ((revision > 0)),
    CONSTRAINT equipment_assets_validity_check CHECK (((valid_from IS NULL) OR (valid_until IS NULL) OR (valid_until >= valid_from)))
);


--
-- Name: interaction_resumes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.interaction_resumes (
    interaction_id uuid NOT NULL,
    user_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    run_id uuid NOT NULL,
    request_id text NOT NULL,
    answer_fingerprint text NOT NULL,
    question_fingerprint text NOT NULL,
    user_message_id uuid NOT NULL,
    assistant_message_id uuid NOT NULL,
    claimed_at timestamp with time zone NOT NULL,
    CONSTRAINT interaction_resumes_answer_fingerprint_check CHECK ((answer_fingerprint ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT interaction_resumes_check CHECK ((user_message_id <> assistant_message_id)),
    CONSTRAINT interaction_resumes_question_fingerprint_check CHECK ((question_fingerprint ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT interaction_resumes_request_id_check CHECK (((length(request_id) >= 1) AND (length(request_id) <= 200)))
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
    CONSTRAINT jobs_status_check CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('running'::character varying)::text, ('waiting_user'::character varying)::text, ('completed'::character varying)::text, ('failed'::character varying)::text, ('cancelled'::character varying)::text, ('timed_out'::character varying)::text])))
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
    CONSTRAINT outcomes_causality_level_check CHECK (((causality_level)::text = ANY (ARRAY[('association_only'::character varying)::text, ('user_attributed'::character varying)::text, ('clinician_attributed'::character varying)::text])))
);


--
-- Name: posture_analysis_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.posture_analysis_runs (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    asset_id uuid NOT NULL,
    view character varying(10) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    job_id uuid,
    agent_configuration_id character varying(80) DEFAULT ''::character varying NOT NULL,
    result_payload jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT posture_analysis_runs_status_check CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('processing'::character varying)::text, ('completed'::character varying)::text, ('failed'::character varying)::text]))),
    CONSTRAINT posture_analysis_runs_view_check CHECK (((view)::text = ANY (ARRAY[('front'::character varying)::text, ('side'::character varying)::text, ('back'::character varying)::text])))
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
    CONSTRAINT privacy_erasure_requests_status_check CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('running'::character varying)::text, ('retryable'::character varying)::text, ('completed'::character varying)::text])))
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
    CONSTRAINT runs_status_check CHECK (((status)::text = ANY (ARRAY[('running'::character varying)::text, ('waiting_user'::character varying)::text, ('completed'::character varying)::text, ('failed'::character varying)::text, ('cancelled'::character varying)::text])))
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
-- Name: safety_screens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.safety_screens (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    capture_id text NOT NULL,
    body_fact_id uuid NOT NULL,
    source_conversation_id uuid NOT NULL,
    source_run_id uuid NOT NULL,
    source_message_id uuid NOT NULL,
    source_interaction_id uuid NOT NULL,
    source_tool_call_id text NOT NULL,
    trauma boolean NOT NULL,
    radiating_pain boolean NOT NULL,
    numbness boolean NOT NULL,
    weakness boolean NOT NULL,
    dizziness boolean NOT NULL,
    body_state_revision bigint NOT NULL,
    recorded_at timestamp with time zone NOT NULL,
    CONSTRAINT safety_screens_body_state_revision_check CHECK ((body_state_revision > 0)),
    CONSTRAINT safety_screens_capture_id_check CHECK ((capture_id ~ '^[0-9a-f]{24}$'::text)),
    CONSTRAINT safety_screens_tool_call_binding_check CHECK ((source_tool_call_id = ('intake-'::text || capture_id)))
);


--
-- Name: statement_evidence; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.statement_evidence (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    source_kind text NOT NULL,
    source_version bigint NOT NULL,
    excerpt text NOT NULL,
    availability text NOT NULL,
    recorded_at timestamp with time zone NOT NULL,
    source_conversation_id uuid,
    source_run_id uuid,
    source_message_id uuid,
    source_interaction_id uuid,
    source_tool_call_id text,
    question_excerpt text,
    CONSTRAINT statement_evidence_availability_check CHECK ((availability = ANY (ARRAY['available'::text, 'blocked'::text, 'erased'::text]))),
    CONSTRAINT statement_evidence_chat_binding_check CHECK ((((source_kind = 'direct_edit'::text) AND (source_conversation_id IS NULL) AND (source_run_id IS NULL) AND (source_message_id IS NULL) AND (source_interaction_id IS NULL) AND (source_tool_call_id IS NULL) AND (question_excerpt IS NULL)) OR ((source_kind = 'user_message'::text) AND (source_conversation_id IS NOT NULL) AND (source_run_id IS NOT NULL) AND (source_message_id IS NOT NULL) AND (source_interaction_id IS NULL) AND (source_tool_call_id IS NULL) AND (question_excerpt IS NULL)) OR ((source_kind = 'structured_answer'::text) AND (source_conversation_id IS NOT NULL) AND (source_run_id IS NOT NULL) AND (source_message_id IS NOT NULL) AND (source_interaction_id IS NOT NULL) AND (source_tool_call_id IS NOT NULL) AND ((length(source_tool_call_id) >= 1) AND (length(source_tool_call_id) <= 256)) AND (question_excerpt IS NOT NULL) AND ((length(question_excerpt) >= 1) AND (length(question_excerpt) <= 2000))))),
    CONSTRAINT statement_evidence_check CHECK (((availability <> 'erased'::text) OR (excerpt = ''::text))),
    CONSTRAINT statement_evidence_excerpt_check CHECK ((char_length(excerpt) <= 2000)),
    CONSTRAINT statement_evidence_source_kind_check CHECK ((source_kind = ANY (ARRAY['direct_edit'::text, 'user_message'::text, 'structured_answer'::text]))),
    CONSTRAINT statement_evidence_source_version_check CHECK ((source_version > 0))
);


--
-- Name: symptom_episodes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.symptom_episodes (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    symptom_type text NOT NULL,
    body_region_ids jsonb NOT NULL,
    laterality text NOT NULL,
    identity_status text DEFAULT 'current'::text NOT NULL,
    created_revision bigint NOT NULL,
    created_at timestamp with time zone NOT NULL,
    CONSTRAINT symptom_episodes_body_region_ids_check CHECK (((jsonb_typeof(body_region_ids) = 'array'::text) AND (jsonb_array_length(body_region_ids) <= 20))),
    CONSTRAINT symptom_episodes_created_revision_check CHECK ((created_revision > 0)),
    CONSTRAINT symptom_episodes_identity_status_check CHECK ((identity_status = ANY (ARRAY['current'::text, 'superseded'::text, 'invalidated'::text]))),
    CONSTRAINT symptom_episodes_laterality_check CHECK ((laterality = ANY (ARRAY['left'::text, 'right'::text, 'bilateral'::text, 'not_applicable'::text, 'unknown'::text]))),
    CONSTRAINT symptom_episodes_symptom_type_check CHECK (((char_length(btrim(symptom_type)) >= 1) AND (char_length(btrim(symptom_type)) <= 120)))
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
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
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
    CONSTRAINT thread_projections_phase_check CHECK (((phase)::text = ANY (ARRAY[('collecting'::character varying)::text, ('ready_for_analysis'::character varying)::text])))
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
    outcome_recorded_at timestamp with time zone
);


--
-- Name: training_plans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.training_plans (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    consultation_id uuid,
    title text NOT NULL DEFAULT '',
    current_accepted_revision_id uuid,
    goal text NOT NULL,
    duration_weeks integer DEFAULT 4 NOT NULL,
    current_week integer DEFAULT 1 NOT NULL,
    phases jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    status_revision bigint DEFAULT 0 NOT NULL CHECK (status_revision >= 0)
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
-- Name: user_assets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_assets (
    id uuid DEFAULT uuidv7() NOT NULL,
    user_id uuid NOT NULL,
    kind character varying(20) NOT NULL,
    photo_view character varying(10),
    original_name character varying(255) NOT NULL,
    mime_type character varying(100) NOT NULL,
    byte_size bigint NOT NULL,
    content_sha256 character(64) NOT NULL,
    storage_backend character varying(20) NOT NULL,
    storage_key character varying(500) NOT NULL,
    availability character varying(20) DEFAULT 'available'::character varying NOT NULL,
    source_removed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT ck_user_assets_availability CHECK (((availability)::text = ANY (ARRAY[('available'::character varying)::text, ('source_removed'::character varying)::text, ('erased'::character varying)::text]))),
    CONSTRAINT ck_user_assets_byte_size CHECK ((byte_size >= 0)),
    CONSTRAINT ck_user_assets_content_sha256 CHECK ((content_sha256 ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT ck_user_assets_kind CHECK (((kind)::text = ANY (ARRAY[('photo'::character varying)::text, ('report'::character varying)::text, ('document'::character varying)::text]))),
    CONSTRAINT ck_user_assets_photo_view CHECK (((photo_view IS NULL) OR ((photo_view)::text = ANY (ARRAY[('front'::character varying)::text, ('side'::character varying)::text, ('back'::character varying)::text])))),
    CONSTRAINT ck_user_assets_report_no_view CHECK (((kind)::text <> 'report'::text) OR (photo_view IS NULL)),
    CONSTRAINT ck_user_assets_source_removed_at CHECK (((availability)::text <> 'source_removed'::text) OR (source_removed_at IS NOT NULL)),
    CONSTRAINT ck_user_assets_storage_backend CHECK (((storage_backend)::text = ANY (ARRAY[('local'::character varying)::text, ('oss'::character varying)::text]))),
    CONSTRAINT ck_user_assets_storage_key CHECK ((((storage_key)::text <> ''::text) AND (length((storage_key)::text) <= 492) AND ((storage_key)::text !~ '^/'::text) AND ((storage_key)::text !~ '(^|/)\.\.(/|$)'::text) AND (POSITION((chr(92)) IN (storage_key)) = 0)))
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
    CONSTRAINT chk_users_role CHECK (((role)::text = ANY (ARRAY[('member'::character varying)::text, ('operator'::character varying)::text])))
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
-- Name: body_fact_commits body_fact_commits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_fact_commits
    ADD CONSTRAINT body_fact_commits_pkey PRIMARY KEY (id);


--
-- Name: body_fact_commits body_fact_commits_user_id_idempotency_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_fact_commits
    ADD CONSTRAINT body_fact_commits_user_id_idempotency_key_key UNIQUE (user_id, idempotency_key);


--
-- Name: body_facts body_facts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_facts
    ADD CONSTRAINT body_facts_pkey PRIMARY KEY (id);


--
-- Name: body_facts body_facts_user_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_facts
    ADD CONSTRAINT body_facts_user_id_id_key UNIQUE (user_id, id);


--
-- Name: body_hypotheses body_hypotheses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypotheses
    ADD CONSTRAINT body_hypotheses_pkey PRIMARY KEY (id);


--
-- Name: body_hypotheses body_hypotheses_user_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypotheses
    ADD CONSTRAINT body_hypotheses_user_id_id_key UNIQUE (user_id, id);


--
-- Name: body_hypotheses body_hypotheses_user_id_id_source_evidence_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypotheses
    ADD CONSTRAINT body_hypotheses_user_id_id_source_evidence_id_key UNIQUE (user_id, id, source_evidence_id);


--
-- Name: body_hypothesis_commits body_hypothesis_commits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_commits
    ADD CONSTRAINT body_hypothesis_commits_pkey PRIMARY KEY (id);


--
-- Name: body_hypothesis_commits body_hypothesis_commits_user_id_idempotency_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_commits
    ADD CONSTRAINT body_hypothesis_commits_user_id_idempotency_key_key UNIQUE (user_id, idempotency_key);


--
-- Name: body_hypothesis_fact_links body_hypothesis_fact_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_fact_links
    ADD CONSTRAINT body_hypothesis_fact_links_pkey PRIMARY KEY (user_id, hypothesis_id, fact_id);


--
-- Name: body_hypothesis_fact_links body_hypothesis_fact_links_user_id_hypothesis_id_ordinal_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_fact_links
    ADD CONSTRAINT body_hypothesis_fact_links_user_id_hypothesis_id_ordinal_key UNIQUE (user_id, hypothesis_id, ordinal);


--
-- Name: body_hypothesis_observation_links body_hypothesis_observation_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_observation_links
    ADD CONSTRAINT body_hypothesis_observation_links_pkey PRIMARY KEY (user_id, hypothesis_id, observation_id);


--
-- Name: body_hypothesis_observation_links body_hypothesis_observation_links_user_id_hypothesis_id_ordinal_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_observation_links
    ADD CONSTRAINT body_hypothesis_observation_links_user_id_hypothesis_id_ordinal_key UNIQUE (user_id, hypothesis_id, ordinal);


--
-- Name: body_observation_commits body_observation_commits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observation_commits
    ADD CONSTRAINT body_observation_commits_pkey PRIMARY KEY (id);


--
-- Name: body_observation_commits body_observation_commits_user_id_idempotency_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observation_commits
    ADD CONSTRAINT body_observation_commits_user_id_idempotency_key_key UNIQUE (user_id, idempotency_key);


--
-- Name: body_observations body_observations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observations
    ADD CONSTRAINT body_observations_pkey PRIMARY KEY (id);


--
-- Name: body_observations body_observations_user_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observations
    ADD CONSTRAINT body_observations_user_id_id_key UNIQUE (user_id, id);


--
-- Name: body_observations body_observations_user_id_id_source_evidence_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observations
    ADD CONSTRAINT body_observations_user_id_id_source_evidence_id_key UNIQUE (user_id, id, source_evidence_id);


--
-- Name: body_state_outbox body_state_outbox_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_outbox
    ADD CONSTRAINT body_state_outbox_pkey PRIMARY KEY (id);


--
-- Name: body_state_outbox body_state_outbox_user_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_outbox
    ADD CONSTRAINT body_state_outbox_user_id_id_key UNIQUE (user_id, id);


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
-- Name: conversations conversations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversations
    ADD CONSTRAINT conversations_pkey PRIMARY KEY (id);


--
-- Name: conversations conversations_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
-- PC-D4: every user owns exactly one Conversation.
--

ALTER TABLE ONLY public.conversations
    ADD CONSTRAINT conversations_user_id_key UNIQUE (user_id);


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
-- Name: episode_command_receipts episode_command_receipts_owner_identity; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_owner_identity UNIQUE (user_id, id);


--
-- Name: episode_command_receipts episode_command_receipts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_pkey PRIMARY KEY (id);


--
-- Name: episode_command_receipts episode_command_receipts_user_id_idempotency_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_user_id_idempotency_key_key UNIQUE (user_id, idempotency_key);


--
-- Name: episode_fact_links episode_fact_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_fact_links
    ADD CONSTRAINT episode_fact_links_pkey PRIMARY KEY (user_id, episode_id, fact_id);


--
-- Name: episode_fact_links episode_fact_links_user_id_episode_id_fact_id_basis_evidenc_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_fact_links
    ADD CONSTRAINT episode_fact_links_user_id_episode_id_fact_id_basis_evidenc_key UNIQUE (user_id, episode_id, fact_id, basis_evidence_id);


--
-- Name: episode_relations episode_relations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_relations
    ADD CONSTRAINT episode_relations_pkey PRIMARY KEY (id);


--
-- Name: episode_relations episode_relations_user_id_authorization_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_relations
    ADD CONSTRAINT episode_relations_user_id_authorization_id_key UNIQUE (user_id, authorization_id);


--
-- Name: episode_relations episode_relations_user_id_from_episode_id_to_episode_id_kin_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_relations
    ADD CONSTRAINT episode_relations_user_id_from_episode_id_to_episode_id_kin_key UNIQUE (user_id, from_episode_id, to_episode_id, kind);


--
-- Name: episode_relations episode_relations_user_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_relations
    ADD CONSTRAINT episode_relations_user_id_id_key UNIQUE (user_id, id);


--
-- Name: episode_state_evidence episode_state_evidence_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_state_evidence
    ADD CONSTRAINT episode_state_evidence_pkey PRIMARY KEY (user_id, episode_id, fact_id);


--
-- Name: equipment_assets equipment_assets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.equipment_assets
    ADD CONSTRAINT equipment_assets_pkey PRIMARY KEY (id);


--
-- Name: equipment_assets equipment_assets_user_id_create_idempotency_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.equipment_assets
    ADD CONSTRAINT equipment_assets_user_id_create_idempotency_key_key UNIQUE (user_id, create_idempotency_key);


--
-- Name: equipment_assets equipment_assets_user_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.equipment_assets
    ADD CONSTRAINT equipment_assets_user_id_id_key UNIQUE (user_id, id);


--
-- Name: interaction_resumes interaction_resumes_assistant_message_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interaction_resumes
    ADD CONSTRAINT interaction_resumes_assistant_message_id_key UNIQUE (assistant_message_id);


--
-- Name: interaction_resumes interaction_resumes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interaction_resumes
    ADD CONSTRAINT interaction_resumes_pkey PRIMARY KEY (interaction_id);


--
-- Name: interaction_resumes interaction_resumes_user_id_request_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interaction_resumes
    ADD CONSTRAINT interaction_resumes_user_id_request_id_key UNIQUE (user_id, request_id);


--
-- Name: interaction_resumes interaction_resumes_user_message_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interaction_resumes
    ADD CONSTRAINT interaction_resumes_user_message_id_key UNIQUE (user_message_id);


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
-- Name: posture_analysis_runs posture_analysis_runs_asset_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posture_analysis_runs
    ADD CONSTRAINT posture_analysis_runs_asset_key UNIQUE (user_id, asset_id);


--
-- Name: posture_analysis_runs posture_analysis_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posture_analysis_runs
    ADD CONSTRAINT posture_analysis_runs_pkey PRIMARY KEY (id);


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
-- Name: safety_screens safety_screens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.safety_screens
    ADD CONSTRAINT safety_screens_pkey PRIMARY KEY (id);


--
-- Name: safety_screens safety_screens_user_id_capture_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.safety_screens
    ADD CONSTRAINT safety_screens_user_id_capture_id_key UNIQUE (user_id, capture_id);


--
-- Name: safety_screens safety_screens_user_id_source_interaction_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.safety_screens
    ADD CONSTRAINT safety_screens_user_id_source_interaction_id_key UNIQUE (user_id, source_interaction_id);


--
-- Name: statement_evidence statement_evidence_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.statement_evidence
    ADD CONSTRAINT statement_evidence_pkey PRIMARY KEY (id);


--
-- Name: statement_evidence statement_evidence_user_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.statement_evidence
    ADD CONSTRAINT statement_evidence_user_id_id_key UNIQUE (user_id, id);


--
-- Name: symptom_episodes symptom_episodes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.symptom_episodes
    ADD CONSTRAINT symptom_episodes_pkey PRIMARY KEY (id);


--
-- Name: symptom_episodes symptom_episodes_user_id_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.symptom_episodes
    ADD CONSTRAINT symptom_episodes_user_id_id_key UNIQUE (user_id, id);


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
    ADD CONSTRAINT thread_projection_tool_calls_pkey PRIMARY KEY (run_id, tool_call_id);


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
ALTER TABLE public.training_plans ADD CONSTRAINT training_plans_owner_identity UNIQUE (user_id, id);


--
-- Name: user_assets user_assets_owner_identity_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_assets
    ADD CONSTRAINT user_assets_owner_identity_key UNIQUE (user_id, id);


--
-- Name: user_assets user_assets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_assets
    ADD CONSTRAINT user_assets_pkey PRIMARY KEY (id);


--
-- Name: user_profiles user_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT user_profiles_pkey PRIMARY KEY (id);


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
-- Name: body_facts_episode_link_source_identity; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX body_facts_episode_link_source_identity ON public.body_facts USING btree (user_id, id, kind, source_evidence_id);


--
-- Name: body_facts_one_corrective_successor; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX body_facts_one_corrective_successor ON public.body_facts USING btree (user_id, supersedes_fact_id) WHERE (supersedes_fact_id IS NOT NULL);


--
-- Name: body_facts_source; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX body_facts_source ON public.body_facts USING btree (user_id, source_evidence_id);


--
-- Name: body_facts_user_revision; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX body_facts_user_revision ON public.body_facts USING btree (user_id, created_revision DESC, id);


--
-- Name: body_hypotheses_owner_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX body_hypotheses_owner_active ON public.body_hypotheses USING btree (user_id, created_revision DESC, id) WHERE (assertion_status = 'asserted'::text);


--
-- Name: body_hypotheses_owner_source; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX body_hypotheses_owner_source ON public.body_hypotheses USING btree (user_id, source_evidence_id);


--
-- Name: body_hypotheses_one_corrective_successor; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX body_hypotheses_one_corrective_successor ON public.body_hypotheses USING btree (user_id, supersedes_hypothesis_id) WHERE (supersedes_hypothesis_id IS NOT NULL);


--
-- Name: body_hypothesis_commits_owner_rev; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX body_hypothesis_commits_owner_rev ON public.body_hypothesis_commits USING btree (user_id, revision DESC);


--
-- Name: body_hypothesis_fact_links_owner_fact; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX body_hypothesis_fact_links_owner_fact ON public.body_hypothesis_fact_links USING btree (user_id, fact_id);


--
-- Name: body_hypothesis_observation_links_owner_observation; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX body_hypothesis_observation_links_owner_observation ON public.body_hypothesis_observation_links USING btree (user_id, observation_id);


--
-- Name: body_observation_commits_owner_rev; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX body_observation_commits_owner_rev ON public.body_observation_commits USING btree (user_id, revision DESC);


--
-- Name: body_observation_receipts_owner_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX body_observation_receipts_owner_target ON public.body_observation_commits USING btree (user_id, target_observation_id) WHERE (target_observation_id IS NOT NULL);


--
-- Name: body_observations_one_corrective_successor; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX body_observations_one_corrective_successor ON public.body_observations USING btree (user_id, supersedes_observation_id) WHERE (supersedes_observation_id IS NOT NULL);


--
-- Name: body_observations_owner_revision; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX body_observations_owner_revision ON public.body_observations USING btree (user_id, created_revision DESC, id);


--
-- Name: body_observations_owner_source; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX body_observations_owner_source ON public.body_observations USING btree (user_id, source_evidence_id);


--
-- Name: body_state_outbox_replay; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX body_state_outbox_replay ON public.body_state_outbox USING btree (user_id, revision, id);


--
-- Name: episode_command_receipts_owner_episode; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX episode_command_receipts_owner_episode ON public.episode_command_receipts USING btree (user_id, episode_id, revision DESC);


--
-- Name: episode_fact_links_by_fact; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX episode_fact_links_by_fact ON public.episode_fact_links USING btree (user_id, fact_id);


--
-- Name: episode_fact_one_primary; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX episode_fact_one_primary ON public.episode_fact_links USING btree (user_id, fact_id) WHERE is_primary;


--
-- Name: episode_relations_owner_from; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX episode_relations_owner_from ON public.episode_relations USING btree (user_id, from_episode_id, created_revision DESC);


--
-- Name: episode_relations_owner_to; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX episode_relations_owner_to ON public.episode_relations USING btree (user_id, to_episode_id, created_revision DESC);


--
-- Name: equipment_assets_owner_current; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX equipment_assets_owner_current ON public.equipment_assets USING btree (user_id, created_at DESC, id DESC) WHERE (availability <> 'retired'::text);


--
-- Name: equipment_assets_owner_current_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX equipment_assets_owner_current_name ON public.equipment_assets USING btree (user_id, lower(display_name)) WHERE (availability <> 'retired'::text);


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
-- Name: idx_document_extraction_runs_asset_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_document_extraction_runs_asset_created ON public.document_extraction_runs USING btree (user_id, asset_id, created_at DESC);


--
-- Name: idx_document_extraction_runs_configuration; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_document_extraction_runs_configuration ON public.document_extraction_runs USING btree (configuration_id, created_at DESC);


--
-- Name: idx_document_extraction_runs_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_document_extraction_runs_user_created ON public.document_extraction_runs USING btree (user_id, created_at DESC);


--
-- Name: idx_document_indicator_reviews_asset_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_document_indicator_reviews_asset_created ON public.document_indicator_reviews USING btree (user_id, asset_id, created_at DESC);


--
-- Name: idx_document_indicator_reviews_extraction_run; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_document_indicator_reviews_extraction_run ON public.document_indicator_reviews USING btree (extraction_run_id, indicator_index, created_at DESC);


--
-- Name: idx_document_indicator_reviews_idem; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_document_indicator_reviews_idem ON public.document_indicator_reviews USING btree (user_id, extraction_run_id, indicator_index, idempotency_key);


--
-- Name: idx_document_indicator_reviews_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_document_indicator_reviews_user_created ON public.document_indicator_reviews USING btree (user_id, created_at DESC);


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
-- Name: idx_outcomes_user_occurred; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_outcomes_user_occurred ON public.outcomes USING btree (user_id, occurred_at DESC);


--
-- Name: idx_posture_analysis_runs_user_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_posture_analysis_runs_user_status ON public.posture_analysis_runs USING btree (user_id, status, created_at DESC);


--
-- Name: idx_privacy_erasure_requests_recovery; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_privacy_erasure_requests_recovery ON public.privacy_erasure_requests USING btree (status, lease_expires_at, updated_at) WHERE ((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('running'::character varying)::text, ('retryable'::character varying)::text]));


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
-- Name: idx_runs_one_active_per_user; Type: INDEX; Schema: public; Owner: -
-- PC-D5: at most one running or waiting_user Run per user, across devices.
--

CREATE UNIQUE INDEX idx_runs_one_active_per_user ON public.runs USING btree (user_id) WHERE ((status)::text = ANY (ARRAY[('running'::character varying)::text, ('waiting_user'::character varying)::text]));


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
-- Name: idx_training_logs_plan_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_training_logs_plan_id ON public.training_logs USING btree (plan_id);


--
-- Name: idx_training_logs_user_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_training_logs_user_date ON public.training_logs USING btree (user_id, date);


--
-- Name: idx_training_plans_user_active; Type: INDEX; Schema: public; Owner: -
--

-- Independent plans deliberately have no owner-wide single-active index.


--
-- Name: idx_training_plans_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_training_plans_user_id ON public.training_plans USING btree (user_id);


--
-- Name: idx_user_profiles_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_user_profiles_user_id ON public.user_profiles USING btree (user_id);


--
-- Name: idx_user_assets_user_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_assets_user_created ON public.user_assets USING btree (user_id, created_at DESC);


--
-- Name: idx_user_assets_user_kind_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_assets_user_kind_created ON public.user_assets USING btree (user_id, kind, created_at DESC);


--
-- Name: idx_users_role; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_role ON public.users USING btree (role);


--
-- Name: interaction_resumes_run; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX interaction_resumes_run ON public.interaction_resumes USING btree (user_id, run_id);


--
-- Name: safety_screens_user_revision; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX safety_screens_user_revision ON public.safety_screens USING btree (user_id, body_state_revision DESC, id);


--
-- Name: statement_evidence_source_interaction; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX statement_evidence_source_interaction ON public.statement_evidence USING btree (user_id, source_interaction_id) WHERE (source_interaction_id IS NOT NULL);


--
-- Name: statement_evidence_source_message; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX statement_evidence_source_message ON public.statement_evidence USING btree (user_id, source_message_id) WHERE (source_message_id IS NOT NULL);


--
-- Name: symptom_episodes_owner_identity; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX symptom_episodes_owner_identity ON public.symptom_episodes USING btree (user_id, identity_status, created_revision DESC);


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
-- Name: uq_user_assets_storage_identity; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_user_assets_storage_identity ON public.user_assets USING btree (storage_backend, storage_key);


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
-- Name: body_fact_commits body_fact_commits_revision_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_fact_commits
    ADD CONSTRAINT body_fact_commits_revision_id_fkey FOREIGN KEY (revision_id) REFERENCES public.body_state_revisions(id) ON DELETE CASCADE;


--
-- Name: body_fact_commits body_fact_commits_user_id_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_fact_commits
    ADD CONSTRAINT body_fact_commits_user_id_event_id_fkey FOREIGN KEY (user_id, event_id) REFERENCES public.body_state_outbox(user_id, id) ON DELETE CASCADE;


--
-- Name: body_fact_commits body_fact_commits_user_id_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_fact_commits
    ADD CONSTRAINT body_fact_commits_user_id_revision_fkey FOREIGN KEY (user_id, revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


--
-- Name: body_facts body_facts_supersession_owner_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_facts
    ADD CONSTRAINT body_facts_supersession_owner_fk FOREIGN KEY (user_id, supersedes_fact_id) REFERENCES public.body_facts(user_id, id);


--
-- Name: body_facts body_facts_user_id_created_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_facts
    ADD CONSTRAINT body_facts_user_id_created_revision_fkey FOREIGN KEY (user_id, created_revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


--
-- Name: body_facts body_facts_user_id_source_evidence_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_facts
    ADD CONSTRAINT body_facts_user_id_source_evidence_id_fkey FOREIGN KEY (user_id, source_evidence_id) REFERENCES public.statement_evidence(user_id, id);


--
-- Name: body_hypotheses body_hypotheses_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypotheses
    ADD CONSTRAINT body_hypotheses_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: body_hypotheses body_hypotheses_user_id_created_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypotheses
    ADD CONSTRAINT body_hypotheses_user_id_created_revision_fkey FOREIGN KEY (user_id, created_revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


--
-- Name: body_hypotheses body_hypotheses_user_id_withdrawn_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypotheses
    ADD CONSTRAINT body_hypotheses_user_id_withdrawn_revision_fkey FOREIGN KEY (user_id, withdrawn_revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


--
-- Name: body_hypotheses body_hypotheses_user_id_source_evidence_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypotheses
    ADD CONSTRAINT body_hypotheses_user_id_source_evidence_id_fkey FOREIGN KEY (user_id, source_evidence_id) REFERENCES public.statement_evidence(user_id, id);


--
-- Name: body_hypotheses body_hypotheses_supersedes_owner_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypotheses
    ADD CONSTRAINT body_hypotheses_supersedes_owner_fk FOREIGN KEY (user_id, supersedes_hypothesis_id) REFERENCES public.body_hypotheses(user_id, id) ON DELETE CASCADE;


--
-- Name: body_hypotheses body_hypotheses_lineage_owner_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypotheses
    ADD CONSTRAINT body_hypotheses_lineage_owner_fk FOREIGN KEY (user_id, lineage_id) REFERENCES public.body_hypotheses(user_id, id) ON DELETE CASCADE;


--
-- Name: body_hypothesis_commits body_hypothesis_commits_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_commits
    ADD CONSTRAINT body_hypothesis_commits_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: body_hypothesis_commits body_hypothesis_commits_revision_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_commits
    ADD CONSTRAINT body_hypothesis_commits_revision_id_fkey FOREIGN KEY (revision_id) REFERENCES public.body_state_revisions(id) ON DELETE CASCADE;


--
-- Name: body_hypothesis_commits body_hypothesis_commits_user_id_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_commits
    ADD CONSTRAINT body_hypothesis_commits_user_id_revision_fkey FOREIGN KEY (user_id, revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


--
-- Name: body_hypothesis_commits body_hypothesis_commits_user_id_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_commits
    ADD CONSTRAINT body_hypothesis_commits_user_id_event_id_fkey FOREIGN KEY (user_id, event_id) REFERENCES public.body_state_outbox(user_id, id) ON DELETE CASCADE;


--
-- Name: body_hypothesis_commits body_hypothesis_commits_hypothesis_source_owner_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_commits
    ADD CONSTRAINT body_hypothesis_commits_hypothesis_source_owner_fk FOREIGN KEY (user_id, hypothesis_id, source_evidence_id) REFERENCES public.body_hypotheses(user_id, id, source_evidence_id) ON DELETE CASCADE;


--
-- Name: body_hypothesis_commits body_hypothesis_commits_target_owner_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_commits
    ADD CONSTRAINT body_hypothesis_commits_target_owner_fk FOREIGN KEY (user_id, target_hypothesis_id) REFERENCES public.body_hypotheses(user_id, id) ON DELETE CASCADE;


--
-- Name: body_hypothesis_fact_links body_hypothesis_fact_links_hypothesis_owner_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_fact_links
    ADD CONSTRAINT body_hypothesis_fact_links_hypothesis_owner_fk FOREIGN KEY (user_id, hypothesis_id) REFERENCES public.body_hypotheses(user_id, id) ON DELETE CASCADE;


--
-- Name: body_hypothesis_fact_links body_hypothesis_fact_links_fact_owner_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_fact_links
    ADD CONSTRAINT body_hypothesis_fact_links_fact_owner_fk FOREIGN KEY (user_id, fact_id) REFERENCES public.body_facts(user_id, id) ON DELETE CASCADE;


--
-- Name: body_hypothesis_observation_links body_hypothesis_observation_links_hypothesis_owner_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_observation_links
    ADD CONSTRAINT body_hypothesis_observation_links_hypothesis_owner_fk FOREIGN KEY (user_id, hypothesis_id) REFERENCES public.body_hypotheses(user_id, id) ON DELETE CASCADE;


--
-- Name: body_hypothesis_observation_links body_hypothesis_observation_links_observation_owner_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_hypothesis_observation_links
    ADD CONSTRAINT body_hypothesis_observation_links_observation_owner_fk FOREIGN KEY (user_id, observation_id) REFERENCES public.body_observations(user_id, id) ON DELETE CASCADE;


--
-- Name: body_observation_commits body_observation_commits_revision_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observation_commits
    ADD CONSTRAINT body_observation_commits_revision_id_fkey FOREIGN KEY (revision_id) REFERENCES public.body_state_revisions(id) ON DELETE CASCADE;


--
-- Name: body_observation_commits body_observation_commits_user_id_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observation_commits
    ADD CONSTRAINT body_observation_commits_user_id_event_id_fkey FOREIGN KEY (user_id, event_id) REFERENCES public.body_state_outbox(user_id, id) ON DELETE CASCADE;


--
-- Name: body_observation_commits body_observation_commits_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observation_commits
    ADD CONSTRAINT body_observation_commits_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: body_observation_commits body_observation_commits_user_id_observation_id_source_evi_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observation_commits
    ADD CONSTRAINT body_observation_commits_user_id_observation_id_source_evi_fkey FOREIGN KEY (user_id, observation_id, source_evidence_id) REFERENCES public.body_observations(user_id, id, source_evidence_id) ON DELETE CASCADE;


--
-- Name: body_observation_commits body_observation_commits_user_id_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observation_commits
    ADD CONSTRAINT body_observation_commits_user_id_revision_fkey FOREIGN KEY (user_id, revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


--
-- Name: body_observation_commits body_observation_receipt_target_owner_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observation_commits
    ADD CONSTRAINT body_observation_receipt_target_owner_fk FOREIGN KEY (user_id, target_observation_id) REFERENCES public.body_observations(user_id, id) ON DELETE CASCADE;


--
-- Name: body_observations body_observations_user_id_created_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observations
    ADD CONSTRAINT body_observations_user_id_created_revision_fkey FOREIGN KEY (user_id, created_revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


--
-- Name: body_observations body_observations_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observations
    ADD CONSTRAINT body_observations_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: body_observations body_observations_user_id_source_evidence_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observations
    ADD CONSTRAINT body_observations_user_id_source_evidence_id_fkey FOREIGN KEY (user_id, source_evidence_id) REFERENCES public.statement_evidence(user_id, id);


--
-- Name: body_observations body_observations_user_id_supersedes_observation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_observations
    ADD CONSTRAINT body_observations_user_id_supersedes_observation_id_fkey FOREIGN KEY (user_id, supersedes_observation_id) REFERENCES public.body_observations(user_id, id);


--
-- Name: body_state_outbox body_state_outbox_user_id_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.body_state_outbox
    ADD CONSTRAINT body_state_outbox_user_id_revision_fkey FOREIGN KEY (user_id, revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


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
-- Name: document_extraction_runs document_extraction_runs_user_asset_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_extraction_runs
    ADD CONSTRAINT document_extraction_runs_user_asset_fkey FOREIGN KEY (user_id, asset_id) REFERENCES public.user_assets(user_id, id) ON DELETE CASCADE;


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
-- Name: document_indicator_reviews document_indicator_reviews_user_asset_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_indicator_reviews
    ADD CONSTRAINT document_indicator_reviews_user_asset_fkey FOREIGN KEY (user_id, asset_id) REFERENCES public.user_assets(user_id, id) ON DELETE CASCADE;


--
-- Name: document_indicator_reviews document_indicator_reviews_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.document_indicator_reviews
    ADD CONSTRAINT document_indicator_reviews_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: episode_command_receipts episode_command_receipts_revision_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_revision_id_fkey FOREIGN KEY (revision_id) REFERENCES public.body_state_revisions(id) ON DELETE CASCADE;


--
-- Name: episode_command_receipts episode_command_receipts_user_id_episode_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_user_id_episode_id_fkey FOREIGN KEY (user_id, episode_id) REFERENCES public.symptom_episodes(user_id, id) ON DELETE CASCADE;


--
-- Name: episode_command_receipts episode_command_receipts_user_id_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_user_id_event_id_fkey FOREIGN KEY (user_id, event_id) REFERENCES public.body_state_outbox(user_id, id) ON DELETE CASCADE;


--
-- Name: episode_command_receipts episode_command_receipts_user_id_fact_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_user_id_fact_id_fkey FOREIGN KEY (user_id, fact_id) REFERENCES public.body_facts(user_id, id) ON DELETE CASCADE;


--
-- Name: episode_command_receipts episode_command_receipts_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: episode_command_receipts episode_command_receipts_user_id_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_user_id_revision_fkey FOREIGN KEY (user_id, revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


--
-- Name: episode_fact_links episode_fact_links_user_id_created_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_fact_links
    ADD CONSTRAINT episode_fact_links_user_id_created_revision_fkey FOREIGN KEY (user_id, created_revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


--
-- Name: episode_fact_links episode_fact_links_user_id_episode_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_fact_links
    ADD CONSTRAINT episode_fact_links_user_id_episode_id_fkey FOREIGN KEY (user_id, episode_id) REFERENCES public.symptom_episodes(user_id, id) ON DELETE CASCADE;


--
-- Name: episode_fact_links episode_fact_links_user_id_fact_id_fact_kind_basis_evidenc_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_fact_links
    ADD CONSTRAINT episode_fact_links_user_id_fact_id_fact_kind_basis_evidenc_fkey FOREIGN KEY (user_id, fact_id, fact_kind, basis_evidence_id) REFERENCES public.body_facts(user_id, id, kind, source_evidence_id) ON DELETE CASCADE;


--
-- Name: episode_relations episode_relations_user_id_authorization_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_relations
    ADD CONSTRAINT episode_relations_user_id_authorization_id_fkey FOREIGN KEY (user_id, authorization_id) REFERENCES public.episode_command_receipts(user_id, id) ON DELETE CASCADE;


--
-- Name: episode_relations episode_relations_user_id_created_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_relations
    ADD CONSTRAINT episode_relations_user_id_created_revision_fkey FOREIGN KEY (user_id, created_revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


--
-- Name: episode_relations episode_relations_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_relations
    ADD CONSTRAINT episode_relations_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: episode_relations episode_relations_user_id_from_episode_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_relations
    ADD CONSTRAINT episode_relations_user_id_from_episode_id_fkey FOREIGN KEY (user_id, from_episode_id) REFERENCES public.symptom_episodes(user_id, id) ON DELETE CASCADE;


--
-- Name: episode_relations episode_relations_user_id_to_episode_id_basis_fact_id_basi_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_relations
    ADD CONSTRAINT episode_relations_user_id_to_episode_id_basis_fact_id_basi_fkey FOREIGN KEY (user_id, to_episode_id, basis_fact_id, basis_evidence_id) REFERENCES public.episode_fact_links(user_id, episode_id, fact_id, basis_evidence_id) ON DELETE CASCADE;


--
-- Name: episode_relations episode_relations_user_id_to_episode_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_relations
    ADD CONSTRAINT episode_relations_user_id_to_episode_id_fkey FOREIGN KEY (user_id, to_episode_id) REFERENCES public.symptom_episodes(user_id, id) ON DELETE CASCADE;


--
-- Name: episode_state_evidence episode_state_evidence_user_id_created_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_state_evidence
    ADD CONSTRAINT episode_state_evidence_user_id_created_revision_fkey FOREIGN KEY (user_id, created_revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


--
-- Name: episode_state_evidence episode_state_evidence_user_id_episode_id_fact_id_source_e_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.episode_state_evidence
    ADD CONSTRAINT episode_state_evidence_user_id_episode_id_fact_id_source_e_fkey FOREIGN KEY (user_id, episode_id, fact_id, source_evidence_id) REFERENCES public.episode_fact_links(user_id, episode_id, fact_id, basis_evidence_id) ON DELETE CASCADE;


--
-- Name: equipment_assets equipment_assets_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.equipment_assets
    ADD CONSTRAINT equipment_assets_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


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
-- Name: interaction_resumes interaction_resumes_assistant_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interaction_resumes
    ADD CONSTRAINT interaction_resumes_assistant_message_id_fkey FOREIGN KEY (assistant_message_id) REFERENCES public.messages(id) ON DELETE CASCADE;


--
-- Name: interaction_resumes interaction_resumes_conversation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interaction_resumes
    ADD CONSTRAINT interaction_resumes_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.conversations(id) ON DELETE CASCADE;


--
-- Name: interaction_resumes interaction_resumes_interaction_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interaction_resumes
    ADD CONSTRAINT interaction_resumes_interaction_id_fkey FOREIGN KEY (interaction_id) REFERENCES public.agent_interactions(id) ON DELETE CASCADE;


--
-- Name: interaction_resumes interaction_resumes_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interaction_resumes
    ADD CONSTRAINT interaction_resumes_run_id_fkey FOREIGN KEY (run_id) REFERENCES public.runs(id) ON DELETE CASCADE;


--
-- Name: interaction_resumes interaction_resumes_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interaction_resumes
    ADD CONSTRAINT interaction_resumes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: interaction_resumes interaction_resumes_user_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interaction_resumes
    ADD CONSTRAINT interaction_resumes_user_message_id_fkey FOREIGN KEY (user_message_id) REFERENCES public.messages(id) ON DELETE CASCADE;


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
-- Name: posture_analysis_runs posture_analysis_runs_user_asset_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.posture_analysis_runs
    ADD CONSTRAINT posture_analysis_runs_user_asset_fkey FOREIGN KEY (user_id, asset_id) REFERENCES public.user_assets(user_id, id) ON DELETE CASCADE;


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
-- Name: safety_screens safety_screens_user_id_body_fact_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.safety_screens
    ADD CONSTRAINT safety_screens_user_id_body_fact_id_fkey FOREIGN KEY (user_id, body_fact_id) REFERENCES public.body_facts(user_id, id) ON DELETE CASCADE;


--
-- Name: safety_screens safety_screens_user_id_body_state_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.safety_screens
    ADD CONSTRAINT safety_screens_user_id_body_state_revision_fkey FOREIGN KEY (user_id, body_state_revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


--
-- Name: safety_screens safety_screens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.safety_screens
    ADD CONSTRAINT safety_screens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: statement_evidence statement_evidence_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.statement_evidence
    ADD CONSTRAINT statement_evidence_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: symptom_episodes symptom_episodes_user_id_created_revision_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.symptom_episodes
    ADD CONSTRAINT symptom_episodes_user_id_created_revision_fkey FOREIGN KEY (user_id, created_revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE;


--
-- Name: symptom_episodes symptom_episodes_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.symptom_episodes
    ADD CONSTRAINT symptom_episodes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


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
-- Name: training_logs training_logs_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_logs
    ADD CONSTRAINT training_logs_plan_id_fkey FOREIGN KEY (user_id, plan_id) REFERENCES public.training_plans(user_id, id) ON DELETE CASCADE;


--
-- Name: training_logs training_logs_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_logs
    ADD CONSTRAINT training_logs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: training_plans training_plans_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_plans
    ADD CONSTRAINT training_plans_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_assets user_assets_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_assets
    ADD CONSTRAINT user_assets_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_profiles user_profiles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_profiles
    ADD CONSTRAINT user_profiles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


-- Go-owned, immutable restrictions for complete structured Safety screens.
ALTER TABLE public.safety_screens ADD CONSTRAINT safety_screens_owner_revision_key UNIQUE (user_id, id, body_state_revision);
CREATE TABLE public.safety_restrictions (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    id uuid NOT NULL,
    safety_screen_id uuid NOT NULL,
    concept text NOT NULL CHECK (concept IN ('trauma','radiating_pain','numbness','weakness','dizziness')),
    policy_revision text NOT NULL CHECK (policy_revision = 'safety-screen-restriction-v1'),
    risk_level text NOT NULL CHECK (risk_level IN ('advisory','restricted','urgent')),
    scope_kind text NOT NULL CHECK (scope_kind IN ('all_training','plan','exercise')),
    plan_id uuid,
    lift_policy text NOT NULL CHECK (lift_policy IN ('on_expiry','user_reverification','professional_clearance')),
    lift_conditions jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(lift_conditions) = 'array' AND jsonb_array_length(lift_conditions) <= 20),
    CONSTRAINT safety_restrictions_reverification_conditions CHECK (lift_policy <> 'user_reverification' OR jsonb_array_length(lift_conditions) > 0),
    reason text NOT NULL CHECK (length(btrim(reason)) > 0),
    created_revision bigint NOT NULL,
    valid_from timestamptz NOT NULL,
    -- on_expiry: the real end of validity; any other policy: only a review point.
    valid_until timestamptz,
    PRIMARY KEY (user_id, id),
    UNIQUE (user_id, safety_screen_id, concept),
    UNIQUE (user_id, id, created_revision),
    CONSTRAINT safety_restrictions_scope_plan CHECK (
        (scope_kind = 'all_training' AND plan_id IS NULL)
        OR (scope_kind = 'plan' AND plan_id IS NOT NULL)
        OR scope_kind = 'exercise'),
    CONSTRAINT safety_restrictions_expiry CHECK (
        (lift_policy <> 'on_expiry' OR valid_until IS NOT NULL)
        AND (valid_until IS NULL OR valid_until > valid_from)),
    -- urgent blocks all training and is never auto-expired or self-cleared.
    CONSTRAINT safety_restrictions_urgent CHECK (
        risk_level <> 'urgent'
        OR (scope_kind = 'all_training' AND lift_policy = 'professional_clearance')),
    -- Each Go policy revision owns exactly its approved mapping. The structured
    -- screen policy never classifies urgency, scopes or self-lifts (B4b); a
    -- later reviewed policy must add its own revision and mapping here.
    CONSTRAINT safety_restrictions_screen_policy CHECK (
        policy_revision <> 'safety-screen-restriction-v1'
        OR (risk_level = 'restricted' AND scope_kind = 'all_training' AND plan_id IS NULL
            AND lift_policy = 'professional_clearance' AND valid_until IS NULL)),
    FOREIGN KEY (user_id, safety_screen_id, created_revision) REFERENCES public.safety_screens(user_id, id, body_state_revision) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (user_id, created_revision) REFERENCES public.body_state_revisions(user_id, revision) DEFERRABLE INITIALLY DEFERRED
);

-- Exercise-scoped coverage by standard exercise concept id (or a stable custom
-- exercise id), never a display name or plan-local planExerciseId. Immutable.
CREATE TABLE public.safety_restriction_exercises (
    user_id uuid NOT NULL,
    restriction_id uuid NOT NULL,
    exercise_id text NOT NULL CHECK (length(exercise_id) BETWEEN 1 AND 128 AND exercise_id = btrim(exercise_id)),
    PRIMARY KEY (user_id, restriction_id, exercise_id),
    FOREIGN KEY (user_id, restriction_id) REFERENCES public.safety_restrictions(user_id, id) ON DELETE CASCADE
);


-- Per-restriction Go-authorized expiry or exact user evidence; never a global clear.
ALTER TABLE public.statement_evidence ADD CONSTRAINT safety_resolution_evidence_identity UNIQUE (user_id, id, source_message_id, source_version);
ALTER TABLE public.interaction_resumes ADD CONSTRAINT safety_resolution_answer_identity UNIQUE (user_id, interaction_id, user_message_id);
ALTER TABLE public.body_state_revisions ADD CONSTRAINT safety_resolution_revision_identity UNIQUE (user_id, revision, id);
CREATE TABLE public.safety_resolutions (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    id uuid NOT NULL,
    restriction_id uuid NOT NULL,
    restriction_version bigint NOT NULL CHECK (restriction_version > 0),
    source_evidence_id uuid,
    source_message_id uuid,
    check_interaction_id uuid,
    source_version bigint NOT NULL,
    basis_kind text NOT NULL CHECK (basis_kind IN ('expiry','user_reverification','professional_clearance')),
    source_level text NOT NULL CHECK (source_level IN ('go_policy','user_reported')),
    scope_kind text NOT NULL CHECK (scope_kind IN ('all_training','plan','exercise')),
    policy_revision text NOT NULL CHECK (policy_revision = 'restriction-lift-v2'),
    idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
    fingerprint text NOT NULL CHECK (length(fingerprint) = 64),
    revision bigint NOT NULL,
    revision_id uuid NOT NULL,
    event_id uuid NOT NULL,
    actor text NOT NULL CHECK (actor IN ('user','system')),
    lifted_at timestamptz NOT NULL,
    CONSTRAINT safety_resolutions_basis_source CHECK (
        (basis_kind = 'expiry' AND source_evidence_id IS NULL AND source_message_id IS NULL AND check_interaction_id IS NULL AND source_version = 0 AND source_level = 'go_policy' AND actor = 'system')
        OR (basis_kind = 'professional_clearance' AND source_evidence_id IS NOT NULL AND source_message_id IS NOT NULL AND check_interaction_id IS NULL AND source_version = 1 AND source_level = 'user_reported' AND actor = 'user')
        OR (basis_kind = 'user_reverification' AND source_evidence_id IS NOT NULL AND source_message_id IS NOT NULL AND check_interaction_id IS NOT NULL AND source_version = 1 AND source_level = 'user_reported' AND actor = 'user')),
    PRIMARY KEY (user_id, id),
    UNIQUE (user_id, restriction_id),
    UNIQUE (user_id, source_message_id),
    UNIQUE (user_id, check_interaction_id),
    UNIQUE (user_id, idempotency_key),
    FOREIGN KEY (user_id, check_interaction_id, source_message_id) REFERENCES public.interaction_resumes(user_id, interaction_id, user_message_id) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (user_id, restriction_id, restriction_version) REFERENCES public.safety_restrictions(user_id, id, created_revision) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (user_id, source_evidence_id, source_message_id, source_version) REFERENCES public.statement_evidence(user_id, id, source_message_id, source_version) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (user_id, revision) REFERENCES public.body_state_revisions(user_id, revision) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (user_id, revision, revision_id) REFERENCES public.body_state_revisions(user_id, revision, id) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (user_id, event_id) REFERENCES public.body_state_outbox(user_id, id) DEFERRABLE INITIALLY DEFERRED
);

-- DX-002 slice 1: owner-bound Diagnosis storage. No decision/adoption API.
-- Formal analysis identities are independent of legacy per-inference analyses.
ALTER TABLE public.statement_evidence ADD CONSTRAINT dx_request_source_identity
    UNIQUE (user_id, id, source_message_id, source_version);
ALTER TABLE public.body_facts ADD CONSTRAINT dx_fact_version_identity UNIQUE (user_id, id, created_revision);
ALTER TABLE public.body_observations ADD CONSTRAINT dx_observation_version_identity UNIQUE (user_id, id, created_revision);
ALTER TABLE public.document_indicator_reviews ADD CONSTRAINT dx_document_admission_identity UNIQUE (user_id, id, action);

CREATE TABLE public.diagnoses (
    user_id uuid PRIMARY KEY REFERENCES public.users(id) ON DELETE CASCADE,
    current_revision bigint NOT NULL DEFAULT 0 CHECK (current_revision >= 0),
    head_revision bigint GENERATED ALWAYS AS (nullif(current_revision, 0)) STORED,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE public.condition_analysis_runs (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    id uuid NOT NULL,
    request_message_id uuid NOT NULL,
    request_evidence_id uuid NOT NULL,
    request_source_version bigint NOT NULL CHECK (request_source_version > 0),
    trigger text NOT NULL CHECK (trigger IN ('new_analysis','partial_update','full_reanalysis')),
    scope jsonb NOT NULL CHECK (jsonb_typeof(scope) = 'object'),
    based_on_body_state_revision bigint NOT NULL CHECK (based_on_body_state_revision >= 0),
    based_on_diagnosis_revision bigint NOT NULL CHECK (based_on_diagnosis_revision >= 0),
    input_evidence_refs jsonb NOT NULL CHECK (jsonb_typeof(input_evidence_refs) = 'array'),
    agent_hypotheses jsonb NOT NULL CHECK (jsonb_typeof(agent_hypotheses) = 'array'),
    replay_input jsonb NOT NULL CHECK (jsonb_typeof(replay_input) = 'object'),
    execution_provenance jsonb NOT NULL CHECK (jsonb_typeof(execution_provenance) = 'object'),
    idempotency_key text NOT NULL CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 128),
    request_fingerprint text NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    started_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, id),
    UNIQUE (user_id, idempotency_key),
    UNIQUE (user_id, id, based_on_diagnosis_revision),
    FOREIGN KEY (user_id) REFERENCES public.diagnoses(user_id),
    FOREIGN KEY (user_id, request_evidence_id, request_message_id, request_source_version)
        REFERENCES public.statement_evidence(user_id, id, source_message_id, source_version) DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE public.diagnosis_proposals (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    id uuid NOT NULL,
    run_id uuid NOT NULL,
    target_condition_id uuid,
    change_kind text NOT NULL CHECK (change_kind IN ('create','update','set_disposition','update_relation')),
    content jsonb NOT NULL CHECK (jsonb_typeof(content) = 'object'),
    content_fingerprint text NOT NULL CHECK (content_fingerprint ~ '^[0-9a-f]{64}$'),
    base_revision bigint NOT NULL CHECK (base_revision >= 0),
    evidence_refs jsonb NOT NULL CHECK (jsonb_typeof(evidence_refs) = 'array'),
    counter_evidence_refs jsonb NOT NULL CHECK (jsonb_typeof(counter_evidence_refs) = 'array'),
    supersedes_proposal_id uuid,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, id),
    UNIQUE (user_id, id, run_id),
    UNIQUE (user_id, id, run_id, content_fingerprint, base_revision),
    CHECK ((change_kind = 'create' AND target_condition_id IS NULL) OR
           (change_kind IN ('update','set_disposition') AND target_condition_id IS NOT NULL) OR
           change_kind = 'update_relation'),
    CHECK (supersedes_proposal_id IS NULL OR supersedes_proposal_id <> id),
    FOREIGN KEY (user_id, run_id, base_revision)
        REFERENCES public.condition_analysis_runs(user_id, id, based_on_diagnosis_revision),
    FOREIGN KEY (user_id, supersedes_proposal_id, run_id)
        REFERENCES public.diagnosis_proposals(user_id, id, run_id) DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE public.diagnosis_decisions (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    id uuid NOT NULL,
    proposal_id uuid NOT NULL,
    run_id uuid NOT NULL,
    content_fingerprint text NOT NULL CHECK (content_fingerprint ~ '^[0-9a-f]{64}$'),
    decided_evidence_refs jsonb NOT NULL CHECK (jsonb_typeof(decided_evidence_refs) = 'array'),
    decision text NOT NULL CHECK (decision IN ('accept','reject')),
    tool_call_id text NOT NULL CHECK (length(btrim(tool_call_id)) BETWEEN 1 AND 256),
    decided_at timestamptz NOT NULL,
    idempotency_key text NOT NULL CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 128),
    request_fingerprint text NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    expected_revision bigint NOT NULL CHECK (expected_revision >= 0),
    receipt_id uuid NOT NULL UNIQUE,
    PRIMARY KEY (user_id, id),
    UNIQUE (user_id, proposal_id),
    UNIQUE (user_id, idempotency_key),
    UNIQUE (user_id, id, decision),
    FOREIGN KEY (user_id, proposal_id, run_id, content_fingerprint, expected_revision)
        REFERENCES public.diagnosis_proposals(user_id, id, run_id, content_fingerprint, base_revision)
);

CREATE TABLE public.diagnosis_revisions (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    id uuid NOT NULL,
    previous_revision bigint NOT NULL CHECK (previous_revision >= 0),
    revision bigint NOT NULL CHECK (revision = previous_revision + 1),
    run_id uuid,
    direct_edit_id uuid,
    patch jsonb NOT NULL CHECK (jsonb_typeof(patch) = 'array' AND jsonb_array_length(patch) > 0),
    decision_ids jsonb NOT NULL CHECK (jsonb_typeof(decision_ids) = 'array'),
    body_state_revision bigint NOT NULL CHECK (body_state_revision >= 0),
    evidence_refs jsonb NOT NULL CHECK (jsonb_typeof(evidence_refs) = 'array'),
    replay_input jsonb NOT NULL CHECK (jsonb_typeof(replay_input) = 'object'),
    execution_provenance jsonb NOT NULL CHECK (jsonb_typeof(execution_provenance) = 'object'),
    idempotency_key text NOT NULL CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 128),
    request_fingerprint text NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    receipt_id uuid NOT NULL UNIQUE,
    committed_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, id),
    UNIQUE (user_id, revision),
    UNIQUE (user_id, run_id),
    UNIQUE (user_id, direct_edit_id),
    UNIQUE (user_id, idempotency_key),
    CHECK ((run_id IS NOT NULL) <> (direct_edit_id IS NOT NULL)),
    FOREIGN KEY (user_id) REFERENCES public.diagnoses(user_id),
    FOREIGN KEY (user_id, run_id, previous_revision)
        REFERENCES public.condition_analysis_runs(user_id, id, based_on_diagnosis_revision)
);
ALTER TABLE public.diagnoses ADD CONSTRAINT diagnosis_head_receipt_fk
    FOREIGN KEY (user_id, head_revision) REFERENCES public.diagnosis_revisions(user_id, revision) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE public.condition_entries (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    id uuid NOT NULL,
    medical_name text NOT NULL CHECK (length(btrim(medical_name)) BETWEEN 1 AND 240),
    concept_ref jsonb CHECK (concept_ref IS NULL OR COALESCE((jsonb_typeof(concept_ref) = 'object' AND
        length(btrim(concept_ref->>'system')) > 0 AND length(btrim(concept_ref->>'code')) > 0 AND
        concept_ref ? 'system' AND concept_ref ? 'code'), false)),
    qualifiers jsonb CHECK (qualifiers IS NULL OR jsonb_typeof(qualifiers) = 'object'),
    assessment jsonb CHECK (assessment IS NULL OR COALESCE((jsonb_typeof(assessment) = 'object' AND
        assessment ? 'level' AND assessment->>'level' IN ('suspected','likely') AND
        assessment ? 'basedOnRunId' AND assessment->>'basedOnRunId' IS NOT NULL), false)),
    assessment_run_id uuid GENERATED ALWAYS AS ((assessment->>'basedOnRunId')::uuid) STORED,
    source_claim_refs jsonb NOT NULL CHECK (jsonb_typeof(source_claim_refs) = 'array'),
    cause_analysis jsonb NOT NULL CHECK (jsonb_typeof(cause_analysis) = 'object'),
    information_gaps jsonb NOT NULL CHECK (jsonb_typeof(information_gaps) = 'array'),
    handling_plan jsonb NOT NULL CHECK (jsonb_typeof(handling_plan) = 'object'),
    disposition text NOT NULL CHECK (disposition IN ('adopted','inactive')),
    latest_decision_id uuid NOT NULL,
    latest_decision_kind text GENERATED ALWAYS AS ('accept'::text) STORED,
    last_analysis_run_id uuid,
    committed_revision bigint NOT NULL CHECK (committed_revision > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, id),
    FOREIGN KEY (user_id) REFERENCES public.diagnoses(user_id),
    FOREIGN KEY (user_id, latest_decision_id, latest_decision_kind)
        REFERENCES public.diagnosis_decisions(user_id, id, decision) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (user_id, last_analysis_run_id) REFERENCES public.condition_analysis_runs(user_id, id),
    FOREIGN KEY (user_id, assessment_run_id) REFERENCES public.condition_analysis_runs(user_id, id),
    FOREIGN KEY (user_id, committed_revision) REFERENCES public.diagnosis_revisions(user_id, revision) DEFERRABLE INITIALLY DEFERRED
);
ALTER TABLE public.diagnosis_proposals ADD CONSTRAINT diagnosis_proposal_target_fk
    FOREIGN KEY (user_id, target_condition_id) REFERENCES public.condition_entries(user_id, id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE public.condition_entry_evidence (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    condition_id uuid NOT NULL,
    evidence_type text NOT NULL CHECK (evidence_type IN ('body_fact','body_observation','symptom_episode','document_evidence')),
    evidence_id uuid NOT NULL,
    evidence_version bigint NOT NULL CHECK (evidence_version > 0),
    role text NOT NULL CHECK (role IN ('supporting','counter','context')),
    rationale text NOT NULL CHECK (length(btrim(rationale)) > 0),
    analyzed_by_run_id uuid NOT NULL,
    document_review_action varchar(20),
    body_fact_id uuid GENERATED ALWAYS AS (CASE WHEN evidence_type = 'body_fact' THEN evidence_id END) STORED,
    body_observation_id uuid GENERATED ALWAYS AS (CASE WHEN evidence_type = 'body_observation' THEN evidence_id END) STORED,
    symptom_episode_id uuid GENERATED ALWAYS AS (CASE WHEN evidence_type = 'symptom_episode' THEN evidence_id END) STORED,
    episode_body_state_revision bigint GENERATED ALWAYS AS (CASE WHEN evidence_type = 'symptom_episode' THEN evidence_version END) STORED,
    document_review_id uuid GENERATED ALWAYS AS (CASE WHEN evidence_type = 'document_evidence' THEN evidence_id END) STORED,
    PRIMARY KEY (user_id, condition_id, evidence_type, evidence_id, evidence_version, role),
    CHECK ((evidence_type = 'document_evidence' AND evidence_version = 1 AND
            document_review_action IS NOT NULL AND document_review_action IN ('confirm','correct')) OR
           (evidence_type <> 'document_evidence' AND document_review_action IS NULL)),
    FOREIGN KEY (user_id, condition_id) REFERENCES public.condition_entries(user_id, id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, analyzed_by_run_id) REFERENCES public.condition_analysis_runs(user_id, id),
    FOREIGN KEY (user_id, body_fact_id, evidence_version) REFERENCES public.body_facts(user_id, id, created_revision),
    FOREIGN KEY (user_id, body_observation_id, evidence_version) REFERENCES public.body_observations(user_id, id, created_revision),
    FOREIGN KEY (user_id, symptom_episode_id) REFERENCES public.symptom_episodes(user_id, id),
    FOREIGN KEY (user_id, episode_body_state_revision) REFERENCES public.body_state_revisions(user_id, revision),
    FOREIGN KEY (user_id, document_review_id, document_review_action) REFERENCES public.document_indicator_reviews(user_id, id, action)
);

-- Audit bases and replay material are immutable. Projection/event writes arrive
-- in slice 2. Deletion remains possible for authorized privacy erasure/cascades.
CREATE FUNCTION public.reject_diagnosis_audit_update() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'Diagnosis audit records are immutable' USING ERRCODE = '23514';
END;
$$;
CREATE TRIGGER condition_analysis_run_immutable BEFORE UPDATE ON public.condition_analysis_runs
    FOR EACH ROW EXECUTE FUNCTION public.reject_diagnosis_audit_update();
CREATE TRIGGER diagnosis_proposal_immutable BEFORE UPDATE ON public.diagnosis_proposals
    FOR EACH ROW EXECUTE FUNCTION public.reject_diagnosis_audit_update();
CREATE TRIGGER diagnosis_decision_immutable BEFORE UPDATE ON public.diagnosis_decisions
    FOR EACH ROW EXECUTE FUNCTION public.reject_diagnosis_audit_update();
CREATE TRIGGER diagnosis_revision_immutable BEFORE UPDATE ON public.diagnosis_revisions
    FOR EACH ROW EXECUTE FUNCTION public.reject_diagnosis_audit_update();

-- A symptom episode evidence version is the frozen BodyState revision at
-- assessment; it cannot predate the episode's own creation revision. Base
-- columns are read because stored generated columns are not visible to
-- BEFORE row triggers; a missing episode stays with the owner FK (23503).
CREATE FUNCTION public.reject_episode_evidence_before_creation() RETURNS trigger
    LANGUAGE plpgsql AS $$
DECLARE
    episode_created_revision bigint;
BEGIN
    IF NEW.evidence_type = 'symptom_episode' THEN
        SELECT se.created_revision INTO episode_created_revision FROM public.symptom_episodes se
            WHERE se.user_id = NEW.user_id AND se.id = NEW.evidence_id;
        IF episode_created_revision IS NOT NULL AND NEW.evidence_version < episode_created_revision THEN
            RAISE EXCEPTION 'symptom episode evidence version predates episode creation' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER episode_evidence_revision_floor BEFORE INSERT OR UPDATE ON public.condition_entry_evidence
    FOR EACH ROW EXECUTE FUNCTION public.reject_episode_evidence_before_creation();

-- DX-002 decision recording: durable selections and terminal events are not adoption.
ALTER TABLE public.diagnosis_decisions ADD CONSTRAINT dx_decision_event_identity
    UNIQUE (user_id, id, proposal_id, run_id);
CREATE TABLE public.condition_analysis_events (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    id uuid NOT NULL,
    run_id uuid NOT NULL,
    sequence bigint NOT NULL CHECK (sequence > 0),
    kind text NOT NULL CHECK (kind IN ('run_started','evidence_added','hypothesis_added',
        'proposal_created','proposal_superseded','proposal_expired','decision_recorded',
        'decision_rechecked','insufficient_evidence','safety_blocked','committed','closed_without_change')),
    proposal_id uuid,
    decision_id uuid,
    body_state_revision bigint NOT NULL CHECK (body_state_revision >= 0),
    details jsonb NOT NULL CHECK (jsonb_typeof(details) = 'object'),
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, id),
    UNIQUE (user_id, run_id, sequence),
    CHECK ((kind IN ('decision_recorded','decision_rechecked') AND proposal_id IS NOT NULL AND decision_id IS NOT NULL) OR
           (kind NOT IN ('decision_recorded','decision_rechecked') AND decision_id IS NULL)),
    FOREIGN KEY (user_id, run_id) REFERENCES public.condition_analysis_runs(user_id,id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, proposal_id, run_id) REFERENCES public.diagnosis_proposals(user_id,id,run_id),
    FOREIGN KEY (user_id, decision_id, proposal_id, run_id)
        REFERENCES public.diagnosis_decisions(user_id,id,proposal_id,run_id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX condition_analysis_one_terminal
    ON public.condition_analysis_events(user_id,run_id)
    WHERE kind IN ('committed','closed_without_change','safety_blocked');
CREATE FUNCTION public.guard_condition_analysis_event_append() RETURNS trigger
    LANGUAGE plpgsql AS $$
DECLARE expected_sequence bigint;
BEGIN
    PERFORM 1 FROM public.condition_analysis_runs WHERE user_id=NEW.user_id AND id=NEW.run_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'analysis run not found' USING ERRCODE='23503';
    END IF;
    IF EXISTS (SELECT 1 FROM public.condition_analysis_events WHERE user_id=NEW.user_id AND run_id=NEW.run_id
        AND kind IN ('committed','closed_without_change','safety_blocked')) THEN
        RAISE EXCEPTION 'analysis run already settled' USING ERRCODE='23514';
    END IF;
    SELECT COALESCE(MAX(sequence),0)+1 INTO expected_sequence FROM public.condition_analysis_events
        WHERE user_id=NEW.user_id AND run_id=NEW.run_id;
    IF NEW.sequence <> expected_sequence THEN
        RAISE EXCEPTION 'analysis event sequence must be contiguous' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER condition_analysis_event_order BEFORE INSERT ON public.condition_analysis_events
    FOR EACH ROW EXECUTE FUNCTION public.guard_condition_analysis_event_append();
CREATE TRIGGER condition_analysis_event_immutable BEFORE UPDATE ON public.condition_analysis_events
    FOR EACH ROW EXECUTE FUNCTION public.reject_diagnosis_audit_update();

-- Write-ahead inventory for source copies; not an AS-002 erasure receipt.
CREATE TABLE public.user_asset_storage_cleanups (
 asset_id uuid NOT NULL,
 user_id uuid NOT NULL,
 storage_backend varchar(20) NOT NULL,
 storage_key varchar(500) NOT NULL,
 PRIMARY KEY (user_id, asset_id, storage_backend, storage_key),
 CONSTRAINT user_asset_storage_cleanups_owner_fkey FOREIGN KEY (user_id, asset_id) REFERENCES public.user_assets(user_id, id) ON DELETE CASCADE,
 CONSTRAINT ck_user_asset_storage_cleanups_backend CHECK (storage_backend IN ('local','oss')),
 CONSTRAINT ck_user_asset_storage_cleanups_key CHECK (storage_key <> '' AND length(storage_key) <= 492 AND storage_key !~ '^/' AND storage_key !~ '(^|/)\.\.(/|$)' AND position(chr(92) in storage_key) = 0)
);

--
-- PostgreSQL database dump complete
--

-- TP-002 slice 1: accepted prescriptions and proposals are separate authorities.
-- TP-002 slice 3 removed the Treatment aggregate and its plan/log/outcome links.
CREATE TABLE public.plan_proposals (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('create','update')),
    state text NOT NULL CHECK (state IN ('pending','accepted','rejected','expired')),
    initiated_by text NOT NULL CHECK (initiated_by IN ('user','agent')),
    source_id text NOT NULL CHECK (length(btrim(source_id)) > 0),
    plan_id uuid,
    based_on_revision_id uuid,
    draft jsonb NOT NULL CHECK (jsonb_typeof(draft) = 'object'),
    idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
    request_fingerprint text NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    UNIQUE (user_id, idempotency_key),
    content_fingerprint text NOT NULL CHECK (content_fingerprint ~ '^[0-9a-f]{64}$'),
    relevant_context_fingerprint text NOT NULL CHECK (relevant_context_fingerprint ~ '^[0-9a-f]{64}$'),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    decision_id uuid,
    expired_reason text CHECK (expired_reason IN ('timeout','base_revision_changed','relevant_context_changed')),
    -- Agent generation audit (TP-002 slice 2b): the frozen Go-built input the
    -- Agent saw and its configuration/execution/governance provenance. Both are
    -- present together, only on agent-initiated proposals, and never change.
    generation_provenance jsonb CHECK (generation_provenance IS NULL OR jsonb_typeof(generation_provenance) = 'object'),
    replay_input jsonb CHECK (replay_input IS NULL OR jsonb_typeof(replay_input) = 'object'),
    CHECK ((generation_provenance IS NULL) = (replay_input IS NULL)),
    CHECK (generation_provenance IS NULL OR initiated_by = 'agent'),
    UNIQUE (user_id, id),
    CHECK ((kind = 'create' AND plan_id IS NULL AND based_on_revision_id IS NULL)
        OR (kind = 'update' AND plan_id IS NOT NULL AND based_on_revision_id IS NOT NULL)),
    CHECK (expires_at > created_at AND expires_at <= created_at + interval '168 hours'),
    CHECK ((state IN ('accepted','rejected')) = (decision_id IS NOT NULL)),
    FOREIGN KEY (user_id, plan_id) REFERENCES public.training_plans(user_id, id)
);
CREATE TABLE public.training_plan_revisions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    plan_id uuid NOT NULL,
    parent_revision_id uuid,
    snapshot jsonb NOT NULL CHECK (jsonb_typeof(snapshot) = 'object'),
    change_set jsonb NOT NULL CHECK (jsonb_typeof(change_set) = 'array'),
    source_kind text NOT NULL CHECK (source_kind = 'user_accepted_proposal'),
    source_id text NOT NULL,
    authorization_id uuid NOT NULL,
    created_at timestamptz NOT NULL,
    UNIQUE (user_id, id),
    UNIQUE (user_id, plan_id, id),
    UNIQUE (user_id, authorization_id),
    FOREIGN KEY (user_id, plan_id) REFERENCES public.training_plans(user_id, id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, plan_id, parent_revision_id) REFERENCES public.training_plan_revisions(user_id, plan_id, id)
);
ALTER TABLE public.training_plans ADD CONSTRAINT training_plans_current_revision_owner_fk
    FOREIGN KEY (user_id, id, current_accepted_revision_id) REFERENCES public.training_plan_revisions(user_id, plan_id, id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE public.training_plans ADD CONSTRAINT independent_training_plans_status
    CHECK (current_accepted_revision_id IS NULL OR status IN ('active','paused','completed'));
-- A proposal's candidate, fingerprints and generation audit are immutable;
-- only its decision outcome (state, decision_id, expired_reason) may change.
CREATE FUNCTION public.prevent_plan_proposal_content_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF (NEW.id, NEW.user_id, NEW.kind, NEW.initiated_by, NEW.source_id, NEW.plan_id,
        NEW.based_on_revision_id, NEW.draft, NEW.idempotency_key, NEW.request_fingerprint,
        NEW.content_fingerprint, NEW.relevant_context_fingerprint, NEW.created_at,
        NEW.expires_at, NEW.generation_provenance, NEW.replay_input)
       IS DISTINCT FROM
       (OLD.id, OLD.user_id, OLD.kind, OLD.initiated_by, OLD.source_id, OLD.plan_id,
        OLD.based_on_revision_id, OLD.draft, OLD.idempotency_key, OLD.request_fingerprint,
        OLD.content_fingerprint, OLD.relevant_context_fingerprint, OLD.created_at,
        OLD.expires_at, OLD.generation_provenance, OLD.replay_input) THEN
        RAISE EXCEPTION 'plan proposal content and generation provenance are immutable';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER plan_proposals_content_immutable BEFORE UPDATE ON public.plan_proposals
    FOR EACH ROW EXECUTE FUNCTION public.prevent_plan_proposal_content_update();
ALTER TABLE public.plan_proposals ADD CONSTRAINT plan_proposals_base_owner_fk
    FOREIGN KEY (user_id, plan_id, based_on_revision_id) REFERENCES public.training_plan_revisions(user_id, plan_id, id);
CREATE TABLE public.plan_decision_receipts (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    proposal_id uuid NOT NULL,
    decision_id uuid NOT NULL,
    operation text NOT NULL CHECK (operation IN ('accept','reject')),
    idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128 AND idempotency_key = btrim(idempotency_key)),
    fingerprint text NOT NULL CHECK (fingerprint ~ '^[0-9a-f]{64}$'),
    plan_id uuid,
    revision_id uuid,
    created_at timestamptz NOT NULL,
    UNIQUE (user_id, id),
    UNIQUE (user_id, idempotency_key),
    UNIQUE (user_id, decision_id),
    UNIQUE (user_id, proposal_id),
    CHECK ((operation = 'accept' AND plan_id IS NOT NULL AND revision_id IS NOT NULL)
        OR (operation = 'reject' AND plan_id IS NULL AND revision_id IS NULL)),
    FOREIGN KEY (user_id, proposal_id) REFERENCES public.plan_proposals(user_id, id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, plan_id, revision_id) REFERENCES public.training_plan_revisions(user_id, plan_id, id) DEFERRABLE INITIALLY DEFERRED
);
ALTER TABLE public.training_plan_revisions ADD CONSTRAINT training_plan_revisions_authorization_owner_fk
    FOREIGN KEY (user_id, authorization_id) REFERENCES public.plan_decision_receipts(user_id, id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE public.plan_proposals ADD CONSTRAINT plan_proposals_decision_owner_fk
    FOREIGN KEY (user_id, decision_id) REFERENCES public.plan_decision_receipts(user_id, decision_id) DEFERRABLE INITIALLY DEFERRED;

-- Native body links retain owner identity; source versions are captured in the
-- immutable snapshot and checked against the live ledger before acceptance.
CREATE TABLE public.training_plan_revision_body_refs (
    user_id uuid NOT NULL,
    revision_id uuid NOT NULL,
    body_fact_id uuid,
    symptom_episode_id uuid,
    body_observation_id uuid,
    CHECK (num_nonnulls(body_fact_id, symptom_episode_id, body_observation_id) = 1),
    FOREIGN KEY (user_id, revision_id) REFERENCES public.training_plan_revisions(user_id, id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, body_fact_id) REFERENCES public.body_facts(user_id, id),
    FOREIGN KEY (user_id, symptom_episode_id) REFERENCES public.symptom_episodes(user_id, id),
    FOREIGN KEY (user_id, body_observation_id) REFERENCES public.body_observations(user_id, id)
);
CREATE FUNCTION public.prevent_training_revision_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'accepted training plan revisions are immutable';
END;
$$;
CREATE TRIGGER training_plan_revisions_immutable BEFORE UPDATE ON public.training_plan_revisions
    FOR EACH ROW EXECUTE FUNCTION public.prevent_training_revision_update();

-- Template and entry identities remain owned by one plan across revisions.
CREATE TABLE public.training_plan_entry_identities (
    user_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('template','exercise')),
    id text NOT NULL,
    plan_id uuid NOT NULL,
    PRIMARY KEY (user_id, kind, id),
    FOREIGN KEY (user_id, plan_id) REFERENCES public.training_plans(user_id, id) ON DELETE CASCADE
);

-- A plan revision may relate only to the same owner's native ConditionEntry.
CREATE TABLE public.training_plan_revision_conditions (
    user_id uuid NOT NULL,
    revision_id uuid NOT NULL,
    condition_id uuid NOT NULL,
    PRIMARY KEY (user_id, revision_id, condition_id),
    FOREIGN KEY (user_id, revision_id) REFERENCES public.training_plan_revisions(user_id, id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, condition_id) REFERENCES public.condition_entries(user_id, id)
);

-- TP-002 slice 2: plan status is user-managed. Only an explicit user command
-- changes it, one audited event per status revision; Safety, BodyState,
-- Diagnosis or feedback never write it (they project review/gate on read).
CREATE TABLE public.training_plan_status_events (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    plan_id uuid NOT NULL,
    from_status text NOT NULL CHECK (from_status IN ('active','paused','completed')),
    to_status text NOT NULL CHECK (to_status IN ('active','paused','completed')),
    status_revision bigint NOT NULL CHECK (status_revision > 0),
    source_kind text NOT NULL CHECK (source_kind = 'user_command'),
    idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128 AND idempotency_key = btrim(idempotency_key)),
    request_fingerprint text NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    created_at timestamptz NOT NULL,
    UNIQUE (user_id, id),
    UNIQUE (user_id, idempotency_key),
    UNIQUE (user_id, plan_id, status_revision),
    CHECK (from_status <> to_status),
    CHECK ((from_status, to_status) IN (('active','paused'),('paused','active'),('active','completed'),('paused','completed'),('completed','active'))),
    FOREIGN KEY (user_id, plan_id) REFERENCES public.training_plans(user_id, id) ON DELETE CASCADE
);
CREATE FUNCTION public.prevent_training_status_event_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'training plan status events are immutable';
END;
$$;
CREATE TRIGGER training_plan_status_events_immutable BEFORE UPDATE ON public.training_plan_status_events
    FOR EACH ROW EXECUTE FUNCTION public.prevent_training_status_event_update();


-- DX-002 atomic settlement: one immutable whole-run outcome.
ALTER TABLE public.diagnosis_revisions ADD CONSTRAINT diagnosis_revision_settlement_identity
    UNIQUE (user_id, id, run_id, revision);
CREATE TABLE public.diagnosis_settlements (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    id uuid NOT NULL,
    run_id uuid NOT NULL,
    expected_revision bigint NOT NULL CHECK (expected_revision >= 0),
    revision bigint NOT NULL,
    diagnosis_revision_id uuid,
    outcome text NOT NULL CHECK (outcome IN ('committed','closed_without_change','safety_blocked')),
    condition_ids jsonb NOT NULL CHECK (jsonb_typeof(condition_ids)='array'),
    decision_ids jsonb NOT NULL CHECK (jsonb_typeof(decision_ids)='array'),
    body_state_revision bigint NOT NULL CHECK (body_state_revision >= 0),
    idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128 AND idempotency_key=btrim(idempotency_key)),
    request_fingerprint text NOT NULL CHECK (request_fingerprint ~ '^[0-9a-f]{64}$'),
    tool_call_id text NOT NULL CHECK (length(tool_call_id) BETWEEN 1 AND 256 AND tool_call_id=btrim(tool_call_id)),
    receipt_id uuid NOT NULL UNIQUE,
    settled_at timestamptz NOT NULL,
    PRIMARY KEY (user_id,id),
    UNIQUE (user_id,run_id),
    UNIQUE (user_id,idempotency_key),
    CHECK ((outcome='committed' AND diagnosis_revision_id IS NOT NULL AND revision=expected_revision+1)
        OR (outcome IN ('closed_without_change','safety_blocked') AND diagnosis_revision_id IS NULL AND revision=expected_revision)),
    FOREIGN KEY (user_id,run_id,expected_revision)
        REFERENCES public.condition_analysis_runs(user_id,id,based_on_diagnosis_revision),
    FOREIGN KEY (user_id,diagnosis_revision_id,run_id,revision)
        REFERENCES public.diagnosis_revisions(user_id,id,run_id,revision) DEFERRABLE INITIALLY DEFERRED
);
CREATE TABLE public.condition_relations (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    from_condition_id uuid NOT NULL,
    to_condition_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('differential_with','supersedes','coexisting')),
    basis_evidence_refs jsonb NOT NULL CHECK (jsonb_typeof(basis_evidence_refs)='array'),
    rationale text NOT NULL,
    latest_decision_id uuid NOT NULL,
    latest_decision_kind text GENERATED ALWAYS AS ('accept'::text) STORED,
    committed_revision bigint NOT NULL CHECK (committed_revision > 0),
    recorded_at timestamptz NOT NULL,
    PRIMARY KEY (user_id,from_condition_id,to_condition_id,kind),
    CHECK (from_condition_id <> to_condition_id),
    FOREIGN KEY (user_id,from_condition_id) REFERENCES public.condition_entries(user_id,id) ON DELETE CASCADE,
    FOREIGN KEY (user_id,to_condition_id) REFERENCES public.condition_entries(user_id,id) ON DELETE CASCADE,
    FOREIGN KEY (user_id,latest_decision_id,latest_decision_kind)
        REFERENCES public.diagnosis_decisions(user_id,id,decision) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (user_id,committed_revision)
        REFERENCES public.diagnosis_revisions(user_id,revision) DEFERRABLE INITIALLY DEFERRED
);
CREATE TABLE public.condition_disposition_changes (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    id uuid NOT NULL,
    condition_id uuid NOT NULL,
    from_disposition text NOT NULL CHECK (from_disposition IN ('adopted','inactive')),
    to_disposition text NOT NULL CHECK (to_disposition IN ('adopted','inactive')),
    reason text NOT NULL CHECK (reason IN ('user_withdrawal','superseded_by_new_conclusion','no_longer_applicable','readopted')),
    related_condition_id uuid,
    evidence_refs jsonb NOT NULL CHECK (jsonb_typeof(evidence_refs)='array'),
    details text NOT NULL,
    decision_id uuid NOT NULL,
    decision_kind text GENERATED ALWAYS AS ('accept'::text) STORED,
    committed_revision bigint NOT NULL CHECK (committed_revision > 0),
    changed_at timestamptz NOT NULL,
    PRIMARY KEY (user_id,id),
    UNIQUE (user_id,decision_id),
    CHECK (from_disposition <> to_disposition),
    CHECK ((to_disposition='adopted' AND reason='readopted') OR (to_disposition='inactive' AND reason <> 'readopted')),
    FOREIGN KEY (user_id,condition_id) REFERENCES public.condition_entries(user_id,id) ON DELETE CASCADE,
    FOREIGN KEY (user_id,related_condition_id) REFERENCES public.condition_entries(user_id,id),
    FOREIGN KEY (user_id,decision_id,decision_kind) REFERENCES public.diagnosis_decisions(user_id,id,decision) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (user_id,committed_revision) REFERENCES public.diagnosis_revisions(user_id,revision) DEFERRABLE INITIALLY DEFERRED
);
CREATE TRIGGER diagnosis_settlement_immutable BEFORE UPDATE ON public.diagnosis_settlements
    FOR EACH ROW EXECUTE FUNCTION public.reject_diagnosis_audit_update();
CREATE TRIGGER condition_disposition_change_immutable BEFORE UPDATE ON public.condition_disposition_changes
    FOR EACH ROW EXECUTE FUNCTION public.reject_diagnosis_audit_update();
