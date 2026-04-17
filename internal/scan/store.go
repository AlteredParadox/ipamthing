package scan

import (
	"context"
	"database/sql"
	"time"
)

type existingHost struct {
	id          int64
	mac         string
	hostnameDNS string
	status      string
}

func storeResults(ctx context.Context, db *sql.DB, subnetID int64, results []Result) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ex, err := loadExistingHosts(ctx, tx, subnetID)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	seen := make(map[string]bool, len(results))

	for _, r := range results {
		if !r.Alive {
			continue
		}
		seen[r.IP] = true
		e, exists := ex[r.IP]
		if !exists {
			if err := insertNewHost(ctx, tx, subnetID, r, now); err != nil {
				return err
			}
			continue
		}
		if err := updateExistingHost(ctx, tx, e, r, now); err != nil {
			return err
		}
	}

	if err := markAbsentDown(ctx, tx, ex, seen, now); err != nil {
		return err
	}
	return tx.Commit()
}

func loadExistingHosts(ctx context.Context, tx *sql.Tx, subnetID int64) (map[string]existingHost, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, ip, mac, hostname_dns, status FROM hosts WHERE subnet_id = ?`, subnetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ex := make(map[string]existingHost)
	for rows.Next() {
		var e existingHost
		var ip string
		if err := rows.Scan(&e.id, &ip, &e.mac, &e.hostnameDNS, &e.status); err != nil {
			return nil, err
		}
		ex[ip] = e
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ex, nil
}

func insertNewHost(ctx context.Context, tx *sql.Tx, subnetID int64, r Result, now int64) error {
	var hostID int64
	err := tx.QueryRowContext(ctx,
		`INSERT INTO hosts (subnet_id, ip, mac, hostname_dns, status, first_seen, last_seen)
		 VALUES (?, ?, ?, ?, 'up', ?, ?) RETURNING id`,
		subnetID, r.IP, r.MAC, r.Hostname, now, now,
	).Scan(&hostID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO scan_events (host_id, ts, event, details) VALUES (?, ?, 'new', ?)`,
		hostID, now, r.Method)
	return err
}

func updateExistingHost(ctx context.Context, tx *sql.Tx, e existingHost, r Result, now int64) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE hosts SET
			status = 'up',
			last_seen = ?,
			mac = CASE WHEN ? <> '' THEN ? ELSE mac END,
			hostname_dns = CASE WHEN ? <> '' THEN ? ELSE hostname_dns END
		 WHERE id = ?`,
		now, r.MAC, r.MAC, r.Hostname, r.Hostname, e.id,
	); err != nil {
		return err
	}

	type evt struct{ event, details string }
	var events []evt
	if e.status != "up" {
		events = append(events, evt{"up", r.Method})
	}
	if r.MAC != "" && e.mac != "" && e.mac != r.MAC {
		events = append(events, evt{"mac_changed", e.mac + " -> " + r.MAC})
	}
	if r.Hostname != "" && e.hostnameDNS != "" && e.hostnameDNS != r.Hostname {
		events = append(events, evt{"hostname_changed", e.hostnameDNS + " -> " + r.Hostname})
	}
	for _, ev := range events {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO scan_events (host_id, ts, event, details) VALUES (?, ?, ?, ?)`,
			e.id, now, ev.event, ev.details); err != nil {
			return err
		}
	}
	return nil
}

func markAbsentDown(ctx context.Context, tx *sql.Tx, ex map[string]existingHost, seen map[string]bool, now int64) error {
	for ip, e := range ex {
		if seen[ip] || e.status == "down" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE hosts SET status = 'down' WHERE id = ?`, e.id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO scan_events (host_id, ts, event, details) VALUES (?, ?, 'down', '')`,
			e.id, now); err != nil {
			return err
		}
	}
	return nil
}
