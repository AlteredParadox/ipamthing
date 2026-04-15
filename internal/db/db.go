package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// idempotent column additions for DBs created with older schemas.
var alters = []string{
	`ALTER TABLE subnets ADD COLUMN last_scan_at INTEGER`,
	`ALTER TABLE subnets ADD COLUMN last_scan_err TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE hosts ADD COLUMN tags TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE users ADD COLUMN ui_theme TEXT NOT NULL DEFAULT 'light'`,
	`ALTER TABLE users ADD COLUMN ui_layout TEXT NOT NULL DEFAULT 'normal'`,
	`ALTER TABLE users ADD COLUMN ui_density TEXT NOT NULL DEFAULT 'standard'`,
}

func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	for _, stmt := range alters {
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			db.Close()
			return nil, fmt.Errorf("migrate (%q): %w", stmt, err)
		}
	}
	return db, nil
}
