-- Initial schema setup
CREATE TABLE IF NOT EXISTS nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    application TEXT,
    ip TEXT,
    port INTEGER,
    version TEXT,
    country_code TEXT,
    type TEXT,
    server_name TEXT,
    map_name TEXT,
    players INTEGER,
    max_players INTEGER,
    game_version TEXT,
    game_name TEXT,
    server_os TEXT,
    count INTEGER DEFAULT 1,
    first_seen DATETIME,
    last_seen DATETIME,
    UNIQUE(application, ip, port)
);

CREATE INDEX IF NOT EXISTS idx_last_seen ON nodes(last_seen DESC);
