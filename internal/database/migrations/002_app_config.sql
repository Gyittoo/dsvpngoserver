-- App configuration table (single-row, upsert pattern)
CREATE TABLE IF NOT EXISTS app_config (
    id              INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    latest_version  VARCHAR(50) NOT NULL DEFAULT '1.0.0',
    version_code    INTEGER NOT NULL DEFAULT 1,
    min_version_code INTEGER NOT NULL DEFAULT 1,
    force_update    BOOLEAN NOT NULL DEFAULT FALSE,
    update_title    VARCHAR(255) NOT NULL DEFAULT 'Update Available',
    update_message  TEXT NOT NULL DEFAULT '',
    download_url    TEXT NOT NULL DEFAULT '',
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default row
INSERT INTO app_config (id) VALUES (1) ON CONFLICT DO NOTHING;
