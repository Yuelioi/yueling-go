CREATE TABLE IF NOT EXISTS bilibili_checkpoints (
    account_id bigint PRIMARY KEY,
    payload jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
