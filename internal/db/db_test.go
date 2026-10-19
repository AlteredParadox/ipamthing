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

import "testing"

func TestOpenCreatesSchema(t *testing.T) {
	d := OpenTest(t)
	tables := []string{"users", "sessions", "subnets", "hosts", "scan_events", "settings"}
	for _, tbl := range tables {
		var name string
		err := d.DB.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", tbl, err)
		}
	}
}

func TestOpenIdempotentMigrations(t *testing.T) {
	d := OpenTest(t)
	for _, col := range []struct{ table, column string }{
		{"subnets", "last_scan_at"},
		{"subnets", "last_scan_err"},
		{"hosts", "tags"},
		{"users", "ui_theme"},
		{"users", "ui_layout"},
		{"users", "ui_density"},
	} {
		var count int
		err := d.DB.QueryRow(
			`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`,
			col.table, col.column,
		).Scan(&count)
		if err != nil || count != 1 {
			t.Errorf("column %s.%s missing (count=%d, err=%v)", col.table, col.column, count, err)
		}
	}
}
