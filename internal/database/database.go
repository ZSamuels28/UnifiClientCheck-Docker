package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS known_macs (
		id          INTEGER PRIMARY KEY,
		mac_address TEXT UNIQUE,
		last_seen   INTEGER
	)`)
	if err != nil {
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
	_, err := d.db.Exec("INSERT OR IGNORE INTO known_macs (mac_address) VALUES (?)", mac)
	return err
}

// RemoveOldMacs removes devices that have been absent longer than delay seconds.
// Devices that reappear have their last_seen timestamp cleared.
func (d *Database) RemoveOldMacs(clients []unifi.NetworkClient, delay int64) error {
	currentMacs := make(map[string]struct{}, len(clients))
	for _, c := range clients {
		if c.Mac != "" {
			currentMacs[c.Mac] = struct{}{}
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

	now := time.Now().Unix()
	for _, e := range entries {
		if _, online := currentMacs[e.mac]; !online {
			if !e.lastSeen.Valid {
				if _, err := d.db.Exec("UPDATE known_macs SET last_seen = ? WHERE mac_address = ?", now, e.mac); err != nil {
					return fmt.Errorf("failed to update last_seen for %s: %w", e.mac, err)
				}
			} else if e.lastSeen.Int64+delay < now {
				fmt.Printf("Removing device from database: %s\n", e.mac)
				if _, err := d.db.Exec("DELETE FROM known_macs WHERE mac_address = ?", e.mac); err != nil {
					return fmt.Errorf("failed to delete %s: %w", e.mac, err)
				}
			}
		} else {
			if _, err := d.db.Exec("UPDATE known_macs SET last_seen = NULL WHERE mac_address = ?", e.mac); err != nil {
				return fmt.Errorf("failed to clear last_seen for %s: %w", e.mac, err)
			}
		}
	}

	return nil
}
