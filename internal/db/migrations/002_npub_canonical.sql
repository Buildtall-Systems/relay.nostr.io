-- +goose Up

-- allowed_pubkeys first (needs admin_pubkeys.hex_pubkey for the JOIN)
CREATE TABLE allowed_pubkeys_new (
    npub       TEXT PRIMARY KEY,
    created_by TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    note       TEXT DEFAULT ''
);
INSERT INTO allowed_pubkeys_new (npub, created_by, created_at, note)
    SELECT ap.npub, COALESCE(adm.npub, ap.added_by), ap.added_at, ap.note
    FROM allowed_pubkeys ap
    LEFT JOIN admin_pubkeys adm ON ap.added_by = adm.hex_pubkey;
DROP TABLE allowed_pubkeys;
ALTER TABLE allowed_pubkeys_new RENAME TO allowed_pubkeys;

-- admin_pubkeys: npub becomes PRIMARY KEY, drop hex_pubkey
CREATE TABLE admin_pubkeys_new (
    npub       TEXT PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO admin_pubkeys_new (npub, created_at)
    SELECT npub, added_at FROM admin_pubkeys;
DROP TABLE admin_pubkeys;
ALTER TABLE admin_pubkeys_new RENAME TO admin_pubkeys;

-- sessions: pubkey_hex → npub (existing sessions invalidated)
DROP TABLE sessions;
CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    npub       TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);

-- +goose Down

-- Reverse: restore hex_pubkey-based schema
-- Note: hex data is lost after forward migration; down migration creates empty tables
DROP TABLE sessions;
CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    pubkey_hex TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);

DROP TABLE allowed_pubkeys;
CREATE TABLE allowed_pubkeys (
    hex_pubkey TEXT PRIMARY KEY,
    npub       TEXT NOT NULL UNIQUE,
    added_by   TEXT NOT NULL,
    added_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    note       TEXT DEFAULT ''
);

DROP TABLE admin_pubkeys;
CREATE TABLE admin_pubkeys (
    hex_pubkey TEXT PRIMARY KEY,
    npub       TEXT NOT NULL UNIQUE,
    added_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
