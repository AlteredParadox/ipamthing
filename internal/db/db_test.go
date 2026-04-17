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
