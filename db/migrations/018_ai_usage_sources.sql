ALTER TABLE ai_model_usage ADD COLUMN profile_id text NOT NULL DEFAULT '';
ALTER TABLE ai_model_usage ADD COLUMN feature text NOT NULL DEFAULT '';
ALTER TABLE ai_model_usage DROP CONSTRAINT ai_model_usage_pkey;
ALTER TABLE ai_model_usage ADD PRIMARY KEY (usage_date, group_id, model, profile_id, feature);
