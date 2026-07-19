-- Migration 002: LLM Provider Models
-- 
-- Replaces the single JSON-blob table llm_provider_model_caches with
-- normalized relational tables:
--   llm_provider_models   — one row per model per provider
--   llm_model_capabilities — one row per capability per model
--
-- Also adds unique constraint on (llm_provider_id, model_id) to prevent
-- duplicate model entries when re-fetching.

-- ============================================================
-- 1. Create new tables
-- ============================================================

CREATE TABLE IF NOT EXISTS llm_provider_models (
    id              BIGSERIAL    PRIMARY KEY,
    llm_provider_id BIGINT       NOT NULL REFERENCES llm_providers(id) ON DELETE CASCADE,
    model_id        TEXT         NOT NULL,  -- API model identifier (e.g. "deepseek-v4-flash")
    model_name      TEXT         NOT NULL,  -- Display name (usually same as model_id)
    size            BIGINT       DEFAULT 0,  -- Model file size in bytes (0 = unknown)
    context_length  INT          DEFAULT 0,  -- Context window in tokens (0 = unknown)
    created_at      TIMESTAMPTZ  DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  DEFAULT NOW()
);

-- Unique constraint: one model_id per provider
CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_provider_models_provider_model
    ON llm_provider_models (llm_provider_id, model_id);

CREATE TABLE IF NOT EXISTS llm_model_capabilities (
    id          BIGSERIAL    PRIMARY KEY,
    model_id    BIGINT       NOT NULL REFERENCES llm_provider_models(id) ON DELETE CASCADE,
    capability  VARCHAR(50)  NOT NULL,  -- e.g. "chat", "tool_calling", "vision", "embedding"
    UNIQUE (model_id, capability)
);

-- Index for fast capability filtering
CREATE INDEX IF NOT EXISTS idx_llm_model_capabilities_model
    ON llm_model_capabilities (model_id);

-- ============================================================
-- 2. Migrate data from llm_provider_model_caches
-- ============================================================

-- Migrate each model entry from the JSON blob into llm_provider_models.
-- The JSON structure varies by provider:
--   Ollama:  {"id":"...","name":"...","size":...,"contextLength":...,"capabilities":[...]}
--   DeepSeek: {"id":"...","name":"...","contextLength":...}
--   Alibaba:  {"id":"...","name":"..."}
--   OpenAI:   {"id":"...","name":"..."}

WITH model_entries AS (
    SELECT
        lp.id AS llm_provider_id,
        cache.provider_alias,
        entry.value AS model_json
    FROM llm_provider_model_caches cache
    JOIN llm_providers lp ON lp.alias = cache.provider_alias
    CROSS JOIN LATERAL json_array_elements(
        CASE 
            WHEN cache.models_json::text LIKE '[%' 
            THEN cache.models_json::json 
            ELSE '[]'::json 
        END
    ) AS entry
)
INSERT INTO llm_provider_models (llm_provider_id, model_id, model_name, size, context_length)
SELECT
    me.llm_provider_id,
    me.model_json->>'id' AS model_id,
    COALESCE(me.model_json->>'name', me.model_json->>'id') AS model_name,
    COALESCE((me.model_json->>'size')::BIGINT, 0) AS size,
    COALESCE((me.model_json->>'contextLength')::INT, 0) AS context_length
FROM model_entries me
WHERE me.model_json->>'id' IS NOT NULL
ON CONFLICT (llm_provider_id, model_id) DO NOTHING;

-- Migrate capabilities from the JSON blob into llm_model_capabilities.
-- Only models that have a "capabilities" array in their JSON will contribute.

WITH capability_entries AS (
    SELECT
        pm.id AS model_table_id,
        cap_json
    FROM llm_provider_model_caches cache
    JOIN llm_providers lp ON lp.alias = cache.provider_alias
    JOIN llm_provider_models pm ON pm.llm_provider_id = lp.id
    CROSS JOIN LATERAL json_array_elements(
        CASE 
            WHEN cache.models_json::text LIKE '[%' 
            THEN cache.models_json::json 
            ELSE '[]'::json 
        END
    ) AS model_entry
    CROSS JOIN LATERAL json_array_elements_text(
        CASE 
            WHEN model_entry.value->'capabilities' IS NOT NULL 
                 AND json_typeof(model_entry.value->'capabilities') = 'array'
            THEN model_entry.value->'capabilities'
            ELSE '[]'::json 
        END
    ) AS cap_json
    WHERE pm.model_id = model_entry.value->>'id'
)
INSERT INTO llm_model_capabilities (model_id, capability)
SELECT DISTINCT
    ce.model_table_id,
    ce.cap_json
FROM capability_entries ce
WHERE ce.cap_json IS NOT NULL AND ce.cap_json != ''
ON CONFLICT (model_id, capability) DO NOTHING;

-- ============================================================
-- 3. Verify migration
-- ============================================================

-- Uncomment to check results:
-- SELECT 'llm_provider_models count' AS info, COUNT(*) FROM llm_provider_models
-- UNION ALL
-- SELECT 'llm_model_capabilities count' AS info, COUNT(*) FROM llm_model_capabilities;

-- ============================================================
-- 4. Drop old table (comment out until verified)
-- ============================================================

-- DROP TABLE IF EXISTS llm_provider_model_caches;