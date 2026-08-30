CREATE TABLE group_chat_history_watermarks (
    group_id bigint PRIMARY KEY,
    deleted_before_at bigint NOT NULL,
    updated_at bigint NOT NULL
);
