CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) UNIQUE NOT NULL,
    password_hash   VARCHAR(255),
    display_name    VARCHAR(255),
    photo_url       TEXT,
    google_id       VARCHAR(255) UNIQUE,
    plan            VARCHAR(20) NOT NULL DEFAULT 'free',
    expiry_date     TIMESTAMPTZ,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    data_limit_mb   INTEGER NOT NULL DEFAULT 0,
    data_used_mb    INTEGER NOT NULL DEFAULT 0,
    bound_device_id VARCHAR(255) NOT NULL DEFAULT '',
    last_login      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id);

CREATE TABLE IF NOT EXISTS servers (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(255) NOT NULL,
    region       VARCHAR(255) NOT NULL DEFAULT '',
    flag         VARCHAR(10) NOT NULL DEFAULT 'GLOBE',
    country_code VARCHAR(10) NOT NULL DEFAULT '',
    protocol     VARCHAR(50) NOT NULL DEFAULT 'vless',
    plan         VARCHAR(20) NOT NULL DEFAULT 'free',
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    host         VARCHAR(255) NOT NULL DEFAULT '',
    port         INTEGER NOT NULL DEFAULT 0,
    raw_config   TEXT NOT NULL DEFAULT '',
    note         TEXT NOT NULL DEFAULT '',
    sort_order   INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_servers_is_active ON servers(is_active);
CREATE INDEX IF NOT EXISTS idx_servers_sort_order ON servers(sort_order);

CREATE TABLE IF NOT EXISTS vouchers (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code             VARCHAR(50) UNIQUE NOT NULL,
    duration_in_days INTEGER NOT NULL DEFAULT 30,
    status           VARCHAR(20) NOT NULL DEFAULT 'active',
    used_by          UUID REFERENCES users(id),
    used_by_email    VARCHAR(255),
    used_at          TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vouchers_code ON vouchers(code);
CREATE INDEX IF NOT EXISTS idx_vouchers_status ON vouchers(status);

CREATE TABLE IF NOT EXISTS announcements (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title      VARCHAR(255) NOT NULL,
    message    TEXT NOT NULL,
    is_active  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_announcements_is_active ON announcements(is_active);

CREATE TABLE IF NOT EXISTS connection_logs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    server_id  UUID REFERENCES servers(id) ON DELETE SET NULL,
    event      VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_connection_logs_user_id ON connection_logs(user_id);

CREATE TABLE IF NOT EXISTS admins (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
