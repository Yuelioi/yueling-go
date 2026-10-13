CREATE TABLE ai_model_usage (
 usage_date date NOT NULL,
 group_id bigint NOT NULL,
 model text NOT NULL,
 requests bigint NOT NULL DEFAULT 0,
 prompt_tokens bigint NOT NULL DEFAULT 0,
 completion_tokens bigint NOT NULL DEFAULT 0,
 cache_reported_requests bigint NOT NULL DEFAULT 0,
 cache_reported_prompt_tokens bigint NOT NULL DEFAULT 0,
 cached_tokens bigint NOT NULL DEFAULT 0,
 duration_ms bigint NOT NULL DEFAULT 0,
 PRIMARY KEY (usage_date, group_id, model)
);
