// Package db handles SQLite persistence for Argus.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Store wraps the SQLite database with domain-specific methods.
type Store struct {
	db *sql.DB
}

// Open opens (and initialises) the Argus SQLite database.
func Open(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	rawDB, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_synchronous=NORMAL&_cache_size=-2000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := initSchema(rawDB); err != nil {
		rawDB.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	// Restrict file permissions to owner only
	_ = os.Chmod(dbPath, 0600)

	return &Store{db: rawDB}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}

func initSchema(db *sql.DB) error {
	schema := `
    CREATE TABLE IF NOT EXISTS process_snapshots (
        id         INTEGER PRIMARY KEY AUTOINCREMENT,
        pid        INTEGER NOT NULL,
        name       TEXT    NOT NULL,
        app_name   TEXT,
        cmdline    TEXT,
        user       TEXT,
        cpu_pct    REAL    DEFAULT 0,
        mem_mb     REAL    DEFAULT 0,
        confidence TEXT,
        score      INTEGER DEFAULT 0,
        timestamp  TEXT    NOT NULL
    );

    CREATE TABLE IF NOT EXISTS file_access_events (
        id           INTEGER PRIMARY KEY AUTOINCREMENT,
        pid          INTEGER NOT NULL DEFAULT 0,
        process_name TEXT    NOT NULL,
        file_path    TEXT    NOT NULL,
        sensitivity  TEXT,
        timestamp    TEXT    NOT NULL
    );

    CREATE TABLE IF NOT EXISTS network_events (
        id           INTEGER PRIMARY KEY AUTOINCREMENT,
        pid          INTEGER NOT NULL DEFAULT 0,
        process_name TEXT    NOT NULL,
        protocol     TEXT,
        local_port   INTEGER,
        remote_addr  TEXT,
        remote_port  INTEGER,
        endpoint     TEXT,
        label        TEXT,
        state        TEXT,
        timestamp    TEXT    NOT NULL
    );

    CREATE TABLE IF NOT EXISTS injection_alerts (
        id        INTEGER PRIMARY KEY AUTOINCREMENT,
        source    TEXT NOT NULL,
        pattern   TEXT NOT NULL,
        snippet   TEXT,
        severity  TEXT NOT NULL,
        timestamp TEXT NOT NULL
    );

    CREATE TABLE IF NOT EXISTS baselines (
        id           INTEGER PRIMARY KEY AUTOINCREMENT,
        app_name     TEXT NOT NULL,
        metric_type  TEXT NOT NULL,
        metric_value TEXT NOT NULL,
        sample_count INTEGER DEFAULT 0,
        first_seen   TEXT NOT NULL,
        last_seen    TEXT NOT NULL,
        UNIQUE(app_name, metric_type, metric_value)
    );

    CREATE INDEX IF NOT EXISTS idx_ps_ts  ON process_snapshots(timestamp);
    CREATE INDEX IF NOT EXISTS idx_fae_ts ON file_access_events(timestamp);
    CREATE INDEX IF NOT EXISTS idx_ne_ts  ON network_events(timestamp);
    CREATE INDEX IF NOT EXISTS idx_ia_ts  ON injection_alerts(timestamp);
    `
	_, err := db.Exec(schema)
	return err
}
