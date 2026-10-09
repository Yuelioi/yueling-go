CREATE TABLE ai_conversations (
    group_id bigint NOT NULL,
    user_id bigint NOT NULL,
    generation bigint NOT NULL DEFAULT 1,
    payload jsonb NOT NULL DEFAULT '{}',
    expires_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);
