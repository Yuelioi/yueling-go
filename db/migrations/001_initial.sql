CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS zhparser;

DO $migration$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_ts_config c
        JOIN pg_namespace n ON n.oid = c.cfgnamespace
        WHERE n.nspname = 'public' AND c.cfgname = 'chinese_zhparser'
    ) THEN
        CREATE TEXT SEARCH CONFIGURATION public.chinese_zhparser (PARSER = zhparser);
        ALTER TEXT SEARCH CONFIGURATION public.chinese_zhparser
            ADD MAPPING FOR n, v, a, i, e, l, j WITH simple;
    END IF;
END
$migration$;

CREATE TABLE auto_replies (
    id bigserial PRIMARY KEY,
    qq bigint NOT NULL,
    keyword varchar(128),
    reply varchar(1024),
    "group" varchar(128)
);

CREATE TABLE user_game_records (
    id bigserial PRIMARY KEY,
    user_id bigint,
    group_id bigint,
    nickname varchar(64),
    score bigint DEFAULT 0,
    win_count bigint DEFAULT 0,
    lose_count bigint DEFAULT 0,
    check_in_date varchar(10),
    streak bigint DEFAULT 0,
    check_in_month varchar(7) DEFAULT '',
    monthly_check_in bigint DEFAULT 0,
    CONSTRAINT idx_ug UNIQUE (user_id, group_id)
);

CREATE TABLE ai_affinities (
    id bigserial PRIMARY KEY,
    user_id bigint,
    group_id bigint,
    nickname varchar(64),
    score bigint DEFAULT 50,
    last_reason varchar(64),
    updated_at bigint,
    CONSTRAINT idx_ai_affinity UNIQUE (user_id, group_id)
);

CREATE TABLE reminders (
    id bigserial PRIMARY KEY,
    user_id bigint,
    group_id bigint,
    cron_expr varchar(32),
    message varchar(256),
    run_at bigint DEFAULT 0,
    recurring boolean DEFAULT false,
    active boolean DEFAULT true
);
CREATE INDEX idx_reminders_user_id ON reminders (user_id);

CREATE TABLE user_tags (
    id bigserial PRIMARY KEY,
    user_id bigint,
    tag varchar(64),
    CONSTRAINT idx_user_tag UNIQUE (user_id, tag)
);

CREATE TABLE user_profiles (
    id bigserial PRIMARY KEY,
    user_id bigint,
    key varchar(32),
    value varchar(256),
    CONSTRAINT idx_user_profile UNIQUE (user_id, key)
);

CREATE TABLE todo_items (
    id bigserial PRIMARY KEY,
    user_id bigint,
    group_id bigint,
    content varchar(256),
    done boolean DEFAULT false,
    created_at double precision
);
CREATE INDEX idx_todo_items_user_id ON todo_items (user_id);

CREATE TABLE semantic_memories (
    id bigserial PRIMARY KEY,
    user_id bigint,
    content text,
    category varchar(32),
    score double precision DEFAULT 1.0,
    created_at double precision,
    last_accessed double precision
);
CREATE INDEX idx_semantic_memories_user_id ON semantic_memories (user_id);

CREATE TABLE episodic_memories (
    id bigserial PRIMARY KEY,
    user_id bigint,
    group_id bigint,
    input_text text,
    tool_name varchar(64),
    result_summary text,
    steps bigint,
    created_at double precision
);
CREATE INDEX idx_episodic_memories_user_id ON episodic_memories (user_id);

CREATE TABLE procedural_memories (
    id bigserial PRIMARY KEY,
    group_id bigint,
    rule text,
    priority bigint DEFAULT 0,
    created_by bigint,
    created_at double precision
);
CREATE INDEX idx_procedural_memories_group_id ON procedural_memories (group_id);

CREATE TABLE group_join_rules (
    id bigserial PRIMARY KEY,
    group_id bigint,
    action varchar(8),
    keyword varchar(128)
);
CREATE INDEX idx_group_join_rules_group_id ON group_join_rules (group_id);

CREATE TABLE group_plugin_disableds (
    id bigserial PRIMARY KEY,
    group_id bigint,
    plugin_id bigint,
    CONSTRAINT idx_group_plugin_disabled UNIQUE (group_id, plugin_id)
);

CREATE TABLE daily_digests (
    id bigserial PRIMARY KEY,
    group_id bigint UNIQUE,
    created_by bigint,
    send_time varchar(5),
    cron_expr varchar(32),
    message_count bigint,
    enabled boolean DEFAULT true
);

CREATE TABLE feed_subscriptions (
    id bigserial PRIMARY KEY,
    group_id bigint,
    url varchar(1024),
    name varchar(64),
    last_item_id varchar(64),
    created_by bigint,
    enabled boolean DEFAULT true,
    created_at bigint,
    updated_at bigint,
    CONSTRAINT idx_feed_group_url UNIQUE (group_id, url)
);

CREATE TABLE group_knowledges (
    id bigserial PRIMARY KEY,
    group_id bigint,
    title varchar(80),
    content text,
    source_url varchar(1024),
    created_by bigint,
    created_at bigint,
    updated_at bigint,
    search_vector tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('public.chinese_zhparser'::regconfig, coalesce(title, '')), 'A') ||
        setweight(to_tsvector('public.chinese_zhparser'::regconfig, coalesce(content, '')), 'D')
    ) STORED
);
CREATE INDEX idx_group_knowledges_group_id ON group_knowledges (group_id);
CREATE INDEX idx_group_knowledges_search_vector ON group_knowledges USING GIN (search_vector);

CREATE TABLE group_chat_messages (
    id bigserial PRIMARY KEY,
    group_id bigint NOT NULL,
    message_id integer NOT NULL,
    user_id bigint NOT NULL,
    nickname varchar(64),
    content text,
    stat_excluded boolean NOT NULL DEFAULT false,
    created_at bigint NOT NULL,
    search_vector tsvector GENERATED ALWAYS AS (
        to_tsvector(
            'public.chinese_zhparser'::regconfig,
            regexp_replace(coalesce(content, ''), '(https?://\S+|www\.\S+)', ' ', 'gi')
        )
    ) STORED,
    CONSTRAINT idx_group_chat_message UNIQUE (group_id, message_id)
);
CREATE INDEX idx_group_chat_time ON group_chat_messages (group_id, created_at);
CREATE INDEX idx_group_chat_user_time ON group_chat_messages (group_id, user_id, created_at);
CREATE INDEX idx_group_chat_search_vector ON group_chat_messages USING GIN (search_vector);
CREATE INDEX idx_group_chat_content_trgm ON group_chat_messages USING GIN (content gin_trgm_ops);
