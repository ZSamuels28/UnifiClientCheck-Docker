package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/zsamuels28/unificlientalerts/internal/config"
	"github.com/zsamuels28/unificlientalerts/internal/unifi"
	_ "modernc.org/sqlite"
)

// Database manages the SQLite store of known MAC addresses.
type Database struct {
	db *sql.DB
}

func New(path string) (*Database, error) {
	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Production SQLite settings: WAL mode for concurrent reads (if supported),
	// busy timeout to avoid "database is locked" errors, and foreign keys for data integrity.
	// Try WAL first; on network storage (Unraid, NAS), fall back to DELETE mode.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Printf("WAL mode not supported (network storage?); falling back to DELETE mode: %v", err)
	}

	pragmas := []string{
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set %s: %w", p, err)
		}
	}

	// Limit to 2 connections: WAL mode allows 1 writer + 1 concurrent reader.
	db.SetMaxOpenConns(2)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS known_macs (
		id          INTEGER PRIMARY KEY,
		mac_address TEXT UNIQUE NOT NULL,
		last_seen   INTEGER
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

// LoadKnownMacs returns the union of env-provided MACs and DB-stored MACs.
func (d *Database) LoadKnownMacs(envMacs []string) ([]string, error) {
	seen := make(map[string]struct{})
	for _, mac := range envMacs {
		if mac != "" {
			seen[mac] = struct{}{}
		}
	}

	rows, err := d.db.Query("SELECT mac_address FROM known_macs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var mac string
		if err := rows.Scan(&mac); err != nil {
			return nil, err
		}
		seen[mac] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]string, 0, len(seen))
	for mac := range seen {
		result = append(result, mac)
	}
	return result, nil
}

// UpdateKnownMacs inserts a new MAC/ID into the database (no-op if already present).
func (d *Database) UpdateKnownMacs(mac string) error {
	if mac == "" {
		return fmt.Errorf("cannot store empty MAC/identifier")
	}
	_, err := d.db.Exec("INSERT OR IGNORE INTO known_macs (mac_address) VALUES (?)", mac)
	return err
}

// RemoveOldMacs removes devices that have been absent longer than delay seconds.
// Devices that reappear have their last_seen timestamp cleared.
// Uses a transaction to batch all updates/deletes for consistency and performance.
func (d *Database) RemoveOldMacs(clients []unifi.NetworkClient, delay int64) error {
	currentMacs := make(map[string]struct{}, len(clients))
	for _, c := range clients {
		identifier := c.Identifier(true)
		if identifier != "" {
			currentMacs[identifier] = struct{}{}
		}
	}

	rows, err := d.db.Query("SELECT mac_address, last_seen FROM known_macs")
	if err != nil {
		return err
	}

	type entry struct {
		mac      string
		lastSeen sql.NullInt64
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.mac, &e.lastSeen); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if len(entries) == 0 {
		return nil
	}

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback is a no-op after commit

	now := time.Now().Unix()
	for _, e := range entries {
		if _, online := currentMacs[e.mac]; !online {
			if !e.lastSeen.Valid {
				if _, err := tx.Exec("UPDATE known_macs SET last_seen = ? WHERE mac_address = ?", now, e.mac); err != nil {
					return fmt.Errorf("failed to update last_seen for %s: %w", e.mac, err)
				}
			} else if e.lastSeen.Int64+delay < now {
				absentDuration := config.HumanDuration(now - e.lastSeen.Int64)
				log.Printf("Forgetting device %s (absent for %s)", e.mac, absentDuration)
				if _, err := tx.Exec("DELETE FROM known_macs WHERE mac_address = ?", e.mac); err != nil {
					return fmt.Errorf("failed to delete %s: %w", e.mac, err)
				}
			}
		} else {
			if e.lastSeen.Valid {
				if _, err := tx.Exec("UPDATE known_macs SET last_seen = NULL WHERE mac_address = ?", e.mac); err != nil {
					return fmt.Errorf("failed to clear last_seen for %s: %w", e.mac, err)
				}
			}
		}
	}

	return tx.Commit()
}
