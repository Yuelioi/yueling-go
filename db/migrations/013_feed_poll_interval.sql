ALTER TABLE feed_group_settings ADD COLUMN poll_interval_minutes integer NOT NULL DEFAULT 5 CHECK (poll_interval_minutes BETWEEN 5 AND 1440);
UPDATE feed_subscriptions SET next_check_at = last_checked_at + 300 WHERE consecutive_failures = 0;
