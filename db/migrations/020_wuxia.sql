CREATE TABLE wuxia_characters (
    group_id bigint NOT NULL,
    user_id bigint NOT NULL,
    name text NOT NULL,
    profession_id text NOT NULL,
    experience bigint NOT NULL DEFAULT 0 CHECK (experience >= 0),
    silver bigint NOT NULL DEFAULT 0 CHECK (silver >= 0),
    mastery integer NOT NULL DEFAULT 0 CHECK (mastery >= 0 AND mastery <= 1000),
    weapon_kind text NOT NULL,
    weapons jsonb NOT NULL,
    armor integer NOT NULL DEFAULT 0 CHECK (armor BETWEEN 0 AND 30),
    charm integer NOT NULL DEFAULT 0 CHECK (charm BETWEEN 0 AND 30),
    portrait text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (group_id, user_id)
);
CREATE TABLE wuxia_daily_claims (
    group_id bigint NOT NULL,
    user_id bigint NOT NULL,
    day varchar(10) NOT NULL,
    signed boolean NOT NULL DEFAULT false,
    runs integer NOT NULL DEFAULT 0 CHECK (runs BETWEEN 0 AND 12),
    PRIMARY KEY (group_id, user_id, day),
    FOREIGN KEY (group_id, user_id) REFERENCES wuxia_characters (group_id, user_id)
);
CREATE TABLE wuxia_operations (
    id uuid PRIMARY KEY,
    sequence bigserial NOT NULL UNIQUE,
    bot_id bigint NOT NULL,
    group_id bigint NOT NULL,
    message_id integer NOT NULL,
    user_id bigint NOT NULL,
    action text NOT NULL,
    argument text NOT NULL DEFAULT '',
    target_id bigint NOT NULL DEFAULT 0,
    schema_version integer NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    UNIQUE (bot_id, group_id, message_id)
);
CREATE INDEX wuxia_duel_cooldown ON wuxia_operations (group_id, user_id, target_id, created_at DESC) WHERE action='duel';
CREATE INDEX wuxia_latest_result ON wuxia_operations (bot_id, group_id, user_id, sequence DESC);
CREATE TABLE wuxia_ledger (
    id bigserial PRIMARY KEY,
    operation_id uuid NOT NULL UNIQUE REFERENCES wuxia_operations (id),
    group_id bigint NOT NULL,
    user_id bigint NOT NULL,
    delta bigint NOT NULL,
    balance bigint NOT NULL CHECK (balance >= 0),
    reason text NOT NULL,
    created_at timestamptz NOT NULL
);
CREATE INDEX wuxia_ledger_player ON wuxia_ledger (group_id, user_id, id DESC);
CREATE TABLE wuxia_settings (
    group_id bigint PRIMARY KEY,
    value jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
