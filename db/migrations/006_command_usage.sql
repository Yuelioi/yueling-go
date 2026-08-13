CREATE TABLE group_command_usages (
    id bigserial PRIMARY KEY,
    usage_date varchar(10) NOT NULL,
    group_id bigint NOT NULL,
    user_id bigint NOT NULL,
    plugin_id bigint NOT NULL,
    command varchar(128) NOT NULL,
    count bigint NOT NULL DEFAULT 1,
    last_used_at bigint NOT NULL,
    CONSTRAINT idx_group_command_usage_daily UNIQUE (usage_date, group_id, user_id, plugin_id, command)
);

CREATE INDEX idx_group_command_usage_group_date
    ON group_command_usages (group_id, usage_date);

CREATE INDEX idx_group_command_usage_last_used
    ON group_command_usages (last_used_at);
