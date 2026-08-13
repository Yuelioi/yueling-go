CREATE TABLE group_knowledge_shortcuts (
    id bigserial PRIMARY KEY,
    knowledge_id bigint NOT NULL REFERENCES group_knowledges(id) ON DELETE CASCADE,
    group_id bigint NOT NULL,
    trigger varchar(128) NOT NULL,
    created_at bigint NOT NULL,
    CONSTRAINT idx_group_knowledge_shortcut UNIQUE (group_id, trigger)
);
CREATE INDEX idx_group_knowledge_shortcuts_knowledge_id
    ON group_knowledge_shortcuts (knowledge_id);

-- Legacy replies are intentionally discarded. There was only one global row,
-- and it can be recreated explicitly as group knowledge after the upgrade.
DROP TABLE auto_replies;

DELETE FROM group_plugin_disableds WHERE plugin_id = 6;
