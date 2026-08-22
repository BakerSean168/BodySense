--
-- PostgreSQL database dump
--

\restrict bqg8AAoaJFt45rHMleBQ37nfvCnWN6b4kNIs0EipYhwKJPkDE7UWwPu5AuSZQCW

-- Dumped from database version 16.14 (Debian 16.14-1.pgdg12+1)
-- Dumped by pg_dump version 16.14 (Debian 16.14-1.pgdg12+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
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
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;


--
-- Name: EXTENSION vector; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION vector IS 'vector data type and ivfflat and hnsw access methods';


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


--
-- Name: uuidv7(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.uuidv7() RETURNS uuid
    LANGUAGE plpgsql
    AS $$ DECLARE ts bigint; rand bytea; result bytea; BEGIN ts := (extract(epoch from clock_timestamp()) * 1000)::bigint; rand := gen_random_bytes(10); result := '\x00000000000000000000000000000000'::bytea; result := set_byte(result, 0, (ts >> 40)::int & 255); result := set_byte(result, 1, (ts >> 32)::int & 255); result := set_byte(result, 2, (ts >> 24)::int & 255); result := set_byte(result, 3, (ts >> 16)::int & 255); result := set_byte(result, 4, (ts >> 8)::int & 255); result := set_byte(result, 5, ts::int & 255); result := set_byte(result, 6, (get_byte(rand, 0) & 15) | 112); result := set_byte(result, 7, get_byte(rand, 1)); result := set_byte(result, 8, (get_byte(rand, 2) & 63) | 128); FOR i IN 9..15 LOOP result := set_byte(result, i, get_byte(rand, i - 7)); END LOOP; RETURN encode(result, 'hex')::uuid; END; $$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: agent_interactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_interactions (
    id uuid DEFAULT public.uuidv7() NOT NULL,
    run_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    tool_call_id text NOT NULL,
    tool_name text DEFAULT 'ask_user'::text NOT NULL,
    question jsonb DEFAULT '{}'::jsonb NOT NULL,
    answer jsonb,
    status character varying(30) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    answered_at timestamp with time zone,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL
);


--
-- Name: agent_tool_calls; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.agent_tool_calls (
    id uuid DEFAULT public.uuidv7() NOT NULL,
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
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL
);


--
-- Name: ai_output_reviews; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_output_reviews (
    id uuid DEFAULT public.uuidv7() NOT NULL,
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
    health_grade character varying(5) NOT NULL,
    dimension_scores jsonb DEFAULT '{}'::jsonb NOT NULL,
    identified_issues jsonb DEFAULT '[]'::jsonb NOT NULL,
    improvement_summary jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: checkpoint_blobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.checkpoint_blobs (
    thread_id text NOT NULL,
    checkpoint_ns text DEFAULT ''::text NOT NULL,
    channel text NOT NULL,
    version text NOT NULL,
    type text NOT NULL,
    blob bytea
);


--
-- Name: checkpoint_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.checkpoint_migrations (
    v integer NOT NULL
);


--
-- Name: checkpoint_writes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.checkpoint_writes (
    thread_id text NOT NULL,
    checkpoint_ns text DEFAULT ''::text NOT NULL,
    checkpoint_id text NOT NULL,
    task_id text NOT NULL,
    idx integer NOT NULL,
    channel text NOT NULL,
    type text,
    blob bytea NOT NULL,
    task_path text DEFAULT ''::text NOT NULL
);


--
-- Name: checkpoints; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.checkpoints (
    thread_id text NOT NULL,
    checkpoint_ns text DEFAULT ''::text NOT NULL,
    checkpoint_id text NOT NULL,
    parent_checkpoint_id text,
    type text,
    checkpoint jsonb NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL
);


--
-- Name: consultation_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.consultation_sessions (
    conversation_id uuid NOT NULL,
    phase character varying(30) DEFAULT 'collecting'::character varying NOT NULL,
    extracted_info jsonb DEFAULT '[]'::jsonb NOT NULL,
    diagnosis jsonb,
    treatment_plan jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    ended_at timestamp with time zone,
    health_features jsonb DEFAULT '{}'::jsonb NOT NULL
);


--
-- Name: conversation_shares; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conversation_shares (
    id uuid DEFAULT public.uuidv7() NOT NULL,
    conversation_id uuid NOT NULL,
    share_token character varying(32) NOT NULL,
    snapshot_messages jsonb NOT NULL,
    snapshot_metadata jsonb,
    snapshot_title character varying(200),
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: conversations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conversations (
    id uuid DEFAULT public.uuidv7() NOT NULL,
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
    deleted_at timestamp with time zone
);


--
-- Name: job_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.job_events (
    id uuid DEFAULT public.uuidv7() NOT NULL,
    job_id uuid NOT NULL,
    event_type character varying(50) NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.jobs (
    id uuid DEFAULT public.uuidv7() NOT NULL,
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
    updated_at timestamp with time zone DEFAULT now() NOT NULL
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
-- Name: knowledge_entries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_entries (
    id bigint NOT NULL,
    category character varying(100) NOT NULL,
    title character varying(500) NOT NULL,
    content text NOT NULL,
    embedding public.vector(1536),
    source_video character varying(500),
    source_timestamp character varying(50),
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: knowledge_entries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.knowledge_entries_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: knowledge_entries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.knowledge_entries_id_seq OWNED BY public.knowledge_entries.id;


--
-- Name: knowledge_publications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_publications (
    id uuid DEFAULT public.uuidv7() NOT NULL,
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
    publication_batch_key character varying(200)
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
    license_status character varying(50) DEFAULT 'unknown'::character varying NOT NULL
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
    lifecycle_metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
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
    id uuid DEFAULT public.uuidv7() NOT NULL,
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
-- Name: runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.runs (
    id uuid DEFAULT public.uuidv7() NOT NULL,
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
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL
);


--
-- Name: runtime_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.runtime_events (
    id uuid DEFAULT public.uuidv7() NOT NULL,
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
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
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
    diagnosis jsonb,
    treatment_plan jsonb,
    pending_interactions jsonb DEFAULT '[]'::jsonb NOT NULL,
    conversation_created_at timestamp with time zone NOT NULL,
    conversation_updated_at timestamp with time zone NOT NULL,
    session_created_at timestamp with time zone NOT NULL,
    session_updated_at timestamp with time zone NOT NULL,
    ended_at timestamp with time zone,
    refreshed_at timestamp with time zone DEFAULT now() NOT NULL,
    interaction_history jsonb DEFAULT '[]'::jsonb NOT NULL,
    health_features jsonb DEFAULT '{}'::jsonb NOT NULL
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
    created_at timestamp with time zone DEFAULT now() NOT NULL
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
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_profiles (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    gender character varying(20),
    age integer,
    height_cm numeric(5,1),
    weight_kg numeric(5,1),
    bmi numeric(4,1),
    occupation character varying(100),
    sleep_time time without time zone,
    wake_time time without time zone,
    exercise_type character varying(100),
    exercise_frequency character varying(50),
    injury_history text,
    self_description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    schedule_mode character varying(20) DEFAULT 'fixed_calendar'::character varying
);


--
-- Name: user_uploads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_uploads (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    file_type character varying(50) NOT NULL,
    original_name character varying(255) NOT NULL,
    file_path character varying(500) NOT NULL,
    file_size bigint NOT NULL,
    mime_type character varying(100) NOT NULL,
    ocr_result jsonb,
    ocr_status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    email character varying(255) NOT NULL,
    password_hash character varying(255) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_login_at timestamp with time zone
);


--
-- Name: knowledge_clips id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_clips ALTER COLUMN id SET DEFAULT nextval('public.knowledge_clips_id_seq'::regclass);


--
-- Name: knowledge_entries id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_entries ALTER COLUMN id SET DEFAULT nextval('public.knowledge_entries_id_seq'::regclass);


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
-- Name: checkpoint_blobs checkpoint_blobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.checkpoint_blobs
    ADD CONSTRAINT checkpoint_blobs_pkey PRIMARY KEY (thread_id, checkpoint_ns, channel, version);


--
-- Name: checkpoint_migrations checkpoint_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.checkpoint_migrations
    ADD CONSTRAINT checkpoint_migrations_pkey PRIMARY KEY (v);


--
-- Name: checkpoint_writes checkpoint_writes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.checkpoint_writes
    ADD CONSTRAINT checkpoint_writes_pkey PRIMARY KEY (thread_id, checkpoint_ns, checkpoint_id, task_id, idx);


--
-- Name: checkpoints checkpoints_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.checkpoints
    ADD CONSTRAINT checkpoints_pkey PRIMARY KEY (thread_id, checkpoint_ns, checkpoint_id);


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
-- Name: knowledge_entries knowledge_entries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_entries
    ADD CONSTRAINT knowledge_entries_pkey PRIMARY KEY (id);


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
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


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
-- Name: checkpoint_blobs_thread_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX checkpoint_blobs_thread_id_idx ON public.checkpoint_blobs USING btree (thread_id);


--
-- Name: checkpoint_writes_thread_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX checkpoint_writes_thread_id_idx ON public.checkpoint_writes USING btree (thread_id);


--
-- Name: checkpoints_thread_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX checkpoints_thread_id_idx ON public.checkpoints USING btree (thread_id);


--
-- Name: idx_agent_interactions_conversation_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_agent_interactions_conversation_id ON public.agent_interactions USING btree (conversation_id);


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
-- Name: idx_assessment_reports_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_assessment_reports_user_id ON public.assessment_reports USING btree (user_id);


--
-- Name: idx_consultation_phase; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_consultation_phase ON public.consultation_sessions USING btree (phase) WHERE ((phase)::text <> 'completed'::text);


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
-- Name: idx_conversations_user_last; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_conversations_user_last ON public.conversations USING btree (user_id, last_message_at DESC) WHERE (deleted_at IS NULL);


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
-- Name: idx_knowledge_entries_category; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_entries_category ON public.knowledge_entries USING btree (category);


--
-- Name: idx_knowledge_entries_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_entries_created_at ON public.knowledge_entries USING btree (created_at);


--
-- Name: idx_knowledge_publications_batch_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_publications_batch_key ON public.knowledge_publications USING btree (publication_batch_key);


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
-- Name: idx_knowledge_sources_license_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_sources_license_status ON public.knowledge_sources USING btree (license_status);


--
-- Name: idx_knowledge_sources_problem_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_knowledge_sources_problem_slug ON public.knowledge_sources USING btree (problem_slug);


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
-- Name: idx_runs_conversation; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_runs_conversation ON public.runs USING btree (conversation_id);


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
-- Name: idx_training_plans_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_training_plans_user_id ON public.training_plans USING btree (user_id);


--
-- Name: idx_user_profiles_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_user_profiles_user_id ON public.user_profiles USING btree (user_id);


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
-- Name: idx_users_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_email ON public.users USING btree (email);


--
-- Name: jobs update_jobs_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_jobs_updated_at BEFORE UPDATE ON public.jobs FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: knowledge_clips update_knowledge_clips_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_knowledge_clips_updated_at BEFORE UPDATE ON public.knowledge_clips FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: knowledge_entries update_knowledge_entries_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER update_knowledge_entries_updated_at BEFORE UPDATE ON public.knowledge_entries FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


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
-- Name: knowledge_units fk_knowledge_units_publication_id; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_units
    ADD CONSTRAINT fk_knowledge_units_publication_id FOREIGN KEY (publication_id) REFERENCES public.knowledge_publications(id) ON DELETE SET NULL;


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
-- Name: knowledge_publications knowledge_publications_knowledge_unit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_publications
    ADD CONSTRAINT knowledge_publications_knowledge_unit_id_fkey FOREIGN KEY (knowledge_unit_id) REFERENCES public.knowledge_units(id) ON DELETE CASCADE;


--
-- Name: knowledge_segments knowledge_segments_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_segments
    ADD CONSTRAINT knowledge_segments_source_id_fkey FOREIGN KEY (source_id) REFERENCES public.knowledge_sources(id) ON DELETE CASCADE;


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
-- Name: training_logs training_logs_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_logs
    ADD CONSTRAINT training_logs_plan_id_fkey FOREIGN KEY (plan_id) REFERENCES public.training_plans(id) ON DELETE CASCADE;


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

\unrestrict bqg8AAoaJFt45rHMleBQ37nfvCnWN6b4kNIs0EipYhwKJPkDE7UWwPu5AuSZQCW

-- BodySense production PostgreSQL 16 baseline. Schema only; no user data.
INSERT INTO public.schema_migrations (version, dirty) VALUES (29, false);
