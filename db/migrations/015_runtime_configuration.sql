CREATE TABLE IF NOT EXISTS runtime_configuration (
    id integer PRIMARY KEY CHECK (id = 1),
    payload jsonb NOT NULL,
    revision bigint NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now()
);
