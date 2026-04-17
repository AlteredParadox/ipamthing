package scan

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

type Scheduler struct {
	db      *sql.DB
	scanner *Scanner
	logger  *slog.Logger
	trigger chan int64
}

func NewScheduler(db *sql.DB, scanner *Scanner, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		db:      db,
		scanner: scanner,
		logger:  logger,
		trigger: make(chan int64, 32),
	}
}

func (s *Scheduler) Trigger(subnetID int64) {
	select {
	case s.trigger <- subnetID:
	default:
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	s.pass(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.pass(ctx)
		case id := <-s.trigger:
			s.scanSubnet(ctx, id)
		}
	}
}

func (s *Scheduler) pass(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM subnets
		WHERE scan_enabled = 1
		  AND (CAST(strftime('%s','now') AS INTEGER) - COALESCE(last_scan_at, 0)) >= scan_interval_sec
	`)
	if err != nil {
		s.logger.Error("scheduler pass query", "err", err)
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		s.logger.Error("scheduler pass rows", "err", err)
	}
	rows.Close()
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		s.scanSubnet(ctx, id)
	}
}

func (s *Scheduler) scanSubnet(ctx context.Context, id int64) {
	var cidr string
	if err := s.db.QueryRowContext(ctx,
		`SELECT cidr FROM subnets WHERE id = ?`, id).Scan(&cidr); err != nil {
		s.logger.Error("load subnet", "id", id, "err", err)
		return
	}

	s.logger.Info("scan starting", "subnet_id", id, "cidr", cidr)
	start := time.Now()
	results, err := s.scanner.Scan(ctx, cidr)

	errText := ""
	aliveCount := 0
	if err != nil {
		errText = err.Error()
		s.logger.Error("scan failed", "subnet_id", id, "cidr", cidr, "err", err)
	} else {
		for _, r := range results {
			if r.Alive {
				aliveCount++
			}
		}
		if serr := storeResults(ctx, s.db, id, results); serr != nil {
			errText = "store: " + serr.Error()
			s.logger.Error("store results", "subnet_id", id, "err", serr)
		}
	}
	_, _ = s.db.ExecContext(ctx,
		`UPDATE subnets SET last_scan_at = ?, last_scan_err = ? WHERE id = ?`,
		time.Now().Unix(), errText, id,
	)
	s.logger.Info("scan done",
		"subnet_id", id, "cidr", cidr,
		"alive", aliveCount, "total", len(results),
		"took", time.Since(start).Round(time.Millisecond),
		"err", errText)
}
