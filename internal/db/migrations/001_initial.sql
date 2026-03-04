-- +goose Up
CREATE TABLE allowed_pubkeys (
    hex_pubkey TEXT PRIMARY KEY,
    npub       TEXT NOT NULL UNIQUE,
    added_by   TEXT NOT NULL,
    added_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    note       TEXT DEFAULT ''
);

CREATE TABLE admin_pubkeys (
    hex_pubkey TEXT PRIMARY KEY,
    npub       TEXT NOT NULL UNIQUE,
    added_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    pubkey_hex TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE sessions;
DROP TABLE admin_pubkeys;
DROP TABLE allowed_pubkeys;
