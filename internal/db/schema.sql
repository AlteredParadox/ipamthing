CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    ui_theme      TEXT    NOT NULL DEFAULT 'light',
    ui_layout     TEXT    NOT NULL DEFAULT 'normal',
    ui_density    TEXT    NOT NULL DEFAULT 'standard',
    created_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS subnets (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    cidr              TEXT    NOT NULL UNIQUE,
    name              TEXT    NOT NULL DEFAULT '',
    scan_enabled      INTEGER NOT NULL DEFAULT 1,
    scan_interval_sec INTEGER NOT NULL DEFAULT 300,
    last_scan_at      INTEGER,
    last_scan_err     TEXT    NOT NULL DEFAULT '',
    created_at        INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS hosts (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    subnet_id         INTEGER NOT NULL,
    ip                TEXT    NOT NULL,
    mac               TEXT    NOT NULL DEFAULT '',
    hostname_dns      TEXT    NOT NULL DEFAULT '',
    hostname_override TEXT    NOT NULL DEFAULT '',
    status            TEXT    NOT NULL DEFAULT 'unknown',
    first_seen        INTEGER,
    last_seen         INTEGER,
    comments          TEXT    NOT NULL DEFAULT '',
    tags              TEXT    NOT NULL DEFAULT '',
    UNIQUE(subnet_id, ip),
    FOREIGN KEY (subnet_id) REFERENCES subnets(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_hosts_subnet ON hosts(subnet_id);

CREATE TABLE IF NOT EXISTS scan_events (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    host_id INTEGER NOT NULL,
    ts      INTEGER NOT NULL,
    event   TEXT    NOT NULL,
    details TEXT    NOT NULL DEFAULT '',
    FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_scan_events_host ON scan_events(host_id);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
