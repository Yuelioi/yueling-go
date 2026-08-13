ALTER TABLE auto_replies
    ADD COLUMN group_id bigint NOT NULL DEFAULT 0;
CREATE INDEX idx_auto_replies_group_id ON auto_replies (group_id);

DROP TABLE user_tags;
DROP TABLE episodic_memories;

ALTER TABLE semantic_memories
    ADD COLUMN source varchar(16) NOT NULL DEFAULT 'auto',
    ADD COLUMN confidence double precision NOT NULL DEFAULT 0.8,
    ADD COLUMN importance double precision NOT NULL DEFAULT 1.0,
    ADD COLUMN updated_at double precision NOT NULL DEFAULT 0,
    ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
        to_tsvector('public.chinese_zhparser'::regconfig, coalesce(content, ''))
    ) STORED;
CREATE INDEX idx_semantic_memories_search_vector
    ON semantic_memories USING GIN (search_vector);
