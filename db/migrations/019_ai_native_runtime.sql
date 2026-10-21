-- Provider transcripts are deliberately discarded; keep generations as tombstones.
UPDATE ai_conversations SET generation = generation + 1, payload = '{}', expires_at = now();

-- Resolve the old routing rules exactly once. Runtime accepts explicit bindings only.
DO $$
DECLARE
    document jsonb;
    ai jsonb;
    profiles jsonb;
    connections jsonb;
    updated_profiles jsonb := '[]';
    bindings jsonb;
    profile jsonb;
    capabilities jsonb;
    feature text;
    target text;
    connection_id text;
BEGIN
    SELECT payload INTO document FROM runtime_configuration WHERE id=1;
    IF document IS NULL THEN RETURN; END IF;
    ai := COALESCE(document->'ai', '{}');
    profiles := ai->'profiles';
    connections := COALESCE(NULLIF(ai->'connections', 'null'), '[]');
    IF profiles IS NULL OR profiles = 'null' THEN
        profiles := '[]';
        IF COALESCE(ai->>'model','') <> '' THEN
            profiles := profiles || jsonb_build_array(jsonb_build_object('id','legacy-chat','name','对话模型','model',ai->>'model','api_key',ai->>'deepseek_key','base_url',ai->>'base_url'));
            ai := ai || jsonb_build_object('conversation_profile','legacy-chat');
        END IF;
        IF COALESCE(ai->'vl'->>'model','') <> '' THEN
            profiles := profiles || jsonb_build_array(jsonb_build_object('id','legacy-vision','name','视觉模型','model',ai->'vl'->>'model','api_key',ai->'vl'->>'key','base_url',ai->'vl'->>'base_url','vision',true));
            ai := ai || jsonb_build_object('vision_profile','legacy-vision');
        END IF;
    END IF;
    FOR profile IN SELECT value FROM jsonb_array_elements(profiles) LOOP
        connection_id := COALESCE(profile->>'connection_id','');
        IF connection_id = '' THEN
            connection_id := 'migrated-' || md5(profile->>'id');
            IF EXISTS (SELECT 1 FROM jsonb_array_elements(connections) c WHERE c->>'id'=connection_id) THEN
                RAISE EXCEPTION 'AI migration: generated connection ID collision';
            END IF;
            connections := connections || jsonb_build_array(jsonb_build_object('id',connection_id,'name',profile->>'name','kind','openai','base_url',COALESCE(profile->>'base_url',''),'api_key',COALESCE(profile->>'api_key','')));
        END IF;
        IF NOT EXISTS (SELECT 1 FROM jsonb_array_elements(connections) c WHERE c->>'id'=connection_id) THEN
            RAISE EXCEPTION 'AI migration: profile references a missing connection';
        END IF;
        capabilities := profile->'capabilities';
        IF capabilities IS NULL OR capabilities='null' THEN
            capabilities := '["chat","tools"]';
            IF profile->>'vision'='true' THEN capabilities:=capabilities || '["vision"]'; END IF;
        END IF;
        IF jsonb_typeof(capabilities) <> 'array' OR jsonb_array_length(capabilities)=0 THEN
            RAISE EXCEPTION 'AI migration: capabilities must be explicit and nonempty';
        END IF;
        IF EXISTS (SELECT 1 FROM jsonb_array_elements_text(capabilities) c WHERE c NOT IN ('chat','tools','vision','structured_output','image_generation','image_edit','web_search')) THEN
            RAISE EXCEPTION 'AI migration: unknown capability';
        END IF;
        profile := (profile - 'api_key' - 'base_url' - 'vision') || jsonb_build_object('connection_id',connection_id,'capabilities',capabilities);
        updated_profiles := updated_profiles || jsonb_build_array(profile);
    END LOOP;
    bindings := COALESCE(NULLIF(ai->'bindings','null'),'{}') - 'zssm';
    IF EXISTS (SELECT 1 FROM jsonb_object_keys(bindings) k WHERE k NOT IN ('conversation','vision','image_generation','image_edit','search','summary','digest','memory','compression','knowledge','proactive','translation','bilibili')) THEN
        RAISE EXCEPTION 'AI migration: unknown feature binding';
    END IF;
    FOREACH feature IN ARRAY ARRAY['conversation','vision','image_generation','image_edit','search','summary','digest','memory','compression','knowledge','proactive','translation','bilibili'] LOOP
        target := bindings->>feature;
        IF target IS NULL OR target='' THEN
            target := CASE WHEN feature='vision' THEN ai->>'vision_profile'
                           WHEN feature IN ('image_generation','image_edit','search') THEN 'off'
                           ELSE ai->>'conversation_profile' END;
        END IF;
        target := COALESCE(NULLIF(target,''),'off');
        IF target <> 'off' AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(updated_profiles) p WHERE p->>'id'=target) THEN
            RAISE EXCEPTION 'AI migration: binding references a missing profile';
        END IF;
        bindings := bindings || jsonb_build_object(feature,target);
    END LOOP;
    ai := (ai - 'conversation_profile' - 'vision_profile' - 'deepseek_key' - 'base_url' - 'model' - 'vl') || jsonb_build_object('profiles',updated_profiles,'connections',connections,'bindings',bindings);
    UPDATE runtime_configuration SET payload=jsonb_set(document,'{ai}',ai),revision=revision+1,updated_at=now() WHERE id=1;
END $$;
