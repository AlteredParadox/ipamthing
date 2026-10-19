// ipamthing — a lightweight single-binary IPAM.
// Copyright (C) 2026 AlteredParadox
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License
// for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
