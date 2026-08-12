ALTER TABLE feed_subscriptions
    ADD COLUMN consecutive_failures integer NOT NULL DEFAULT 0,
    ADD COLUMN last_error varchar(512) NOT NULL DEFAULT '',
    ADD COLUMN last_checked_at bigint NOT NULL DEFAULT 0,
    ADD COLUMN last_success_at bigint NOT NULL DEFAULT 0,
    ADD COLUMN next_check_at bigint NOT NULL DEFAULT 0;

CREATE INDEX idx_feed_due ON feed_subscriptions (next_check_at) WHERE enabled = true;

CREATE TABLE feed_group_settings (
    group_id bigint PRIMARY KEY,
    quiet_enabled boolean NOT NULL DEFAULT false,
    quiet_start varchar(5) NOT NULL DEFAULT '23:00',
    quiet_end varchar(5) NOT NULL DEFAULT '08:00',
    updated_at bigint NOT NULL DEFAULT 0
);

CREATE TABLE feed_pending_items (
    id bigserial PRIMARY KEY,
    subscription_id bigint NOT NULL REFERENCES feed_subscriptions(id) ON DELETE CASCADE,
    group_id bigint NOT NULL,
    feed_name varchar(64) NOT NULL,
    item_key varchar(64) NOT NULL,
    title varchar(160) NOT NULL,
    link varchar(512),
    published_at bigint NOT NULL DEFAULT 0,
    queued_at bigint NOT NULL,
    CONSTRAINT idx_feed_pending_item UNIQUE (subscription_id, item_key)
);

CREATE INDEX idx_feed_pending_group ON feed_pending_items (group_id, queued_at, id);
