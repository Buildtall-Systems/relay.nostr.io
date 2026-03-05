package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr/nip19"
)

type AllowedPubkey struct {
	HexPubkey string
	Npub      string
	AddedBy   string
	AddedAt   time.Time
	Note      string
}

func (db *DB) IsAllowedPubkey(ctx context.Context, hexPubkey string) (bool, error) {
	var exists int
	err := db.QueryRowContext(ctx,
		`SELECT 1 FROM allowed_pubkeys WHERE hex_pubkey = ?`, hexPubkey,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking allowed pubkey: %w", err)
	}
	return true, nil
}

func (db *DB) ListAllowedPubkeys(ctx context.Context) ([]AllowedPubkey, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT hex_pubkey, npub, added_by, added_at, note FROM allowed_pubkeys ORDER BY added_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing allowed pubkeys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var pubkeys []AllowedPubkey
	for rows.Next() {
		var pk AllowedPubkey
		if err := rows.Scan(&pk.HexPubkey, &pk.Npub, &pk.AddedBy, &pk.AddedAt, &pk.Note); err != nil {
			return nil, fmt.Errorf("scanning allowed pubkey: %w", err)
		}
		pubkeys = append(pubkeys, pk)
	}
	return pubkeys, rows.Err()
}

func (db *DB) AddAllowedPubkey(ctx context.Context, npub, addedByHex, note string) error {
	_, hexPubkey, err := nip19.Decode(npub)
	if err != nil {
		return fmt.Errorf("decoding npub: %w", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT OR IGNORE INTO allowed_pubkeys (hex_pubkey, npub, added_by, note) VALUES (?, ?, ?, ?)`,
		hexPubkey.(string), npub, addedByHex, note,
	)
	if err != nil {
		return fmt.Errorf("adding allowed pubkey: %w", err)
	}
	return nil
}

func (db *DB) RemoveAllowedPubkey(ctx context.Context, hexPubkey string) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM allowed_pubkeys WHERE hex_pubkey = ?`, hexPubkey,
	)
	if err != nil {
		return fmt.Errorf("removing allowed pubkey: %w", err)
	}
	return nil
}

func (db *DB) IsAdmin(ctx context.Context, hexPubkey string) (bool, error) {
	var exists int
	err := db.QueryRowContext(ctx,
		`SELECT 1 FROM admin_pubkeys WHERE hex_pubkey = ?`, hexPubkey,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking admin pubkey: %w", err)
	}
	return true, nil
}

func (db *DB) SeedAdmins(ctx context.Context, npubs []string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR IGNORE INTO admin_pubkeys (hex_pubkey, npub) VALUES (?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, npub := range npubs {
		_, hexPubkey, err := nip19.Decode(npub)
		if err != nil {
			return fmt.Errorf("decoding npub %s: %w", npub, err)
		}
		if _, err := stmt.ExecContext(ctx, hexPubkey.(string), npub); err != nil {
			return fmt.Errorf("inserting admin %s: %w", npub, err)
		}
	}

	return tx.Commit()
}
