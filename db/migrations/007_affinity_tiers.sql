CREATE TABLE ai_affinity_tiers (
    id bigserial PRIMARY KEY,
    min_score integer NOT NULL UNIQUE,
    name varchar(32) NOT NULL,
    prompt text NOT NULL,
    updated_at bigint NOT NULL
);
