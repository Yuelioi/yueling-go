ALTER TABLE feed_group_settings
    ADD COLUMN item_max_chars integer NOT NULL DEFAULT 0,
    ADD CONSTRAINT chk_feed_group_item_max_chars
        CHECK (item_max_chars = 0 OR item_max_chars BETWEEN 80 AND 4000);

ALTER TABLE feed_pending_items
    ALTER COLUMN title TYPE text;
