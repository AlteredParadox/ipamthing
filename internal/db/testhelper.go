package db

import (
	"database/sql"
	"testing"
)

// TestDB wraps a *sql.DB connected to an in-memory SQLite database with the
// full schema applied. Use OpenTest in tests that need a real database.
type TestDB struct {
	DB *sql.DB
}

// OpenTest creates a fresh in-memory SQLite database, applies the schema and
// migrations, and registers a cleanup that closes the connection when the
// test finishes.
func OpenTest(t *testing.T) *TestDB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal("db.OpenTest:", err)
	}
	t.Cleanup(func() { d.Close() })
	return &TestDB{DB: d}
}
