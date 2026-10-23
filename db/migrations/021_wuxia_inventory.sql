ALTER TABLE wuxia_characters
    ADD COLUMN inventory jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN equipment jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN learned_skills jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN active_skill text NOT NULL DEFAULT '',
    ADD COLUMN draws jsonb NOT NULL DEFAULT '{}'::jsonb;
UPDATE wuxia_characters AS c SET inventory = (
    SELECT COALESCE(jsonb_object_agg(item_id, amount), '{}'::jsonb)
    FROM (SELECT value AS item_id, count(*) AS amount FROM jsonb_each_text(c.weapons) GROUP BY value) AS owned
);
