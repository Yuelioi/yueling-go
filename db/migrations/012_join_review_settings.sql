CREATE TABLE group_join_settings (
    group_id bigint PRIMARY KEY,
    mode text NOT NULL CHECK (mode IN ('inherit', 'override', 'disabled'))
);
INSERT INTO group_join_settings (group_id, mode)
SELECT DISTINCT group_id, 'override' FROM group_join_rules;
