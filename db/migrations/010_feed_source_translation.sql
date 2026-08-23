ALTER TABLE feed_subscriptions
    ADD COLUMN translate_to_chinese boolean NOT NULL DEFAULT false;

UPDATE feed_subscriptions AS subscription
SET translate_to_chinese = setting.translate_to_chinese
FROM feed_group_settings AS setting
WHERE subscription.group_id = setting.group_id
  AND setting.translate_to_chinese = true;

ALTER TABLE feed_group_settings
    DROP COLUMN translate_to_chinese;
