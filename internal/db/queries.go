package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr/nip19"
)

type AllowedPubkey struct {
	Npub      string
	CreatedBy string
	CreatedAt time.Time
	Note      string
}

func (db *DB) IsAllowedPubkey(ctx context.Context, npub string) (bool, error) {
	var exists int
	err := db.QueryRowContext(ctx,
		`SELECT 1 FROM allowed_pubkeys WHERE npub = ?
		UNION
		SELECT 1 FROM admin_pubkeys WHERE npub = ?`, npub, npub,
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
		`SELECT npub, created_by, created_at, note FROM allowed_pubkeys ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing allowed pubkeys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var pubkeys []AllowedPubkey
	for rows.Next() {
		var pk AllowedPubkey
		if err := rows.Scan(&pk.Npub, &pk.CreatedBy, &pk.CreatedAt, &pk.Note); err != nil {
			return nil, fmt.Errorf("scanning allowed pubkey: %w", err)
		}
		pubkeys = append(pubkeys, pk)
	}
	return pubkeys, rows.Err()
}

func (db *DB) AddAllowedPubkey(ctx context.Context, npub, createdByNpub, note string) error {
	if _, _, err := nip19.Decode(npub); err != nil {
		return fmt.Errorf("invalid npub: %w", err)
	}

	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO allowed_pubkeys (npub, created_by, note) VALUES (?, ?, ?)`,
		npub, createdByNpub, note,
	)
	if err != nil {
		return fmt.Errorf("adding allowed pubkey: %w", err)
	}
	return nil
}

func (db *DB) RemoveAllowedPubkey(ctx context.Context, npub string) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM allowed_pubkeys WHERE npub = ?`, npub,
	)
	if err != nil {
		return fmt.Errorf("removing allowed pubkey: %w", err)
	}
	return nil
}

func (db *DB) IsAdmin(ctx context.Context, npub string) (bool, error) {
	var exists int
	err := db.QueryRowContext(ctx,
		`SELECT 1 FROM admin_pubkeys WHERE npub = ?`, npub,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking admin: %w", err)
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
		`INSERT OR IGNORE INTO admin_pubkeys (npub) VALUES (?)`,
	)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, npub := range npubs {
		if _, _, err := nip19.Decode(npub); err != nil {
			return fmt.Errorf("invalid npub %s: %w", npub, err)
		}
		if _, err := stmt.ExecContext(ctx, npub); err != nil {
			return fmt.Errorf("inserting admin %s: %w", npub, err)
		}
	}

	return tx.Commit()
}
