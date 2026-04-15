package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ipamthing/internal/auth"
	"ipamthing/internal/db"
	"ipamthing/internal/scan"
	"ipamthing/internal/web"
)

func purgeEvents(ctx context.Context, database *sql.DB, retention time.Duration, logger *slog.Logger) {
	run := func() {
		cutoff := time.Now().Add(-retention).Unix()
		res, err := database.ExecContext(ctx, `DELETE FROM scan_events WHERE ts < ?`, cutoff)
		if err != nil {
			logger.Error("scan_events purge", "err", err)
			return
		}
		if n, _ := res.RowsAffected(); n > 0 {
			logger.Info("scan_events purged", "deleted", n, "retention", retention)
		}
	}
	run()
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "ipamthing.db", "SQLite database file path")
	setupTokenFlag := flag.String("setup-token", "", "one-time token required for first-run /setup (random + logged if empty and no users exist)")
	trustProxy := flag.Bool("trust-proxy", false, "honor X-Forwarded-Proto / X-Forwarded-For from a reverse proxy")
	retentionDays := flag.Int("event-retention-days", 90, "purge scan events older than N days (0 = keep forever)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	database, err := db.Open(*dbPath)
	if err != nil {
		logger.Error("open db", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sessions := auth.NewSessionStore(database)
	go sessions.PurgeLoop(ctx, time.Hour)

	loginLimiter := auth.NewLoginLimiter(10, 15*time.Minute, 15*time.Minute)
	go loginLimiter.PurgeLoop(ctx, 5*time.Minute)

	if *retentionDays > 0 {
		retention := time.Duration(*retentionDays) * 24 * time.Hour
		go purgeEvents(ctx, database, retention, logger)
	}

	scanner := scan.NewScanner(logger)
	scheduler := scan.NewScheduler(database, scanner, logger)
	go scheduler.Run(ctx)

	setupToken := ""
	var userCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		logger.Error("count users for setup check", "err", err)
		os.Exit(1)
	}
	if userCount == 0 {
		setupToken = *setupTokenFlag
		if setupToken == "" {
			buf := make([]byte, 16)
			if _, err := rand.Read(buf); err != nil {
				logger.Error("generate setup token", "err", err)
				os.Exit(1)
			}
			setupToken = hex.EncodeToString(buf)
			logger.Warn("first-run setup required — enter this token in /setup",
				"setup_token", setupToken,
				"note", "suppress this message by passing -setup-token=<value> next startup")
		} else {
			logger.Info("first-run setup required; using -setup-token value")
		}
	}

	handler := web.NewRouter(logger, database, sessions, loginLimiter, scanner, scheduler, setupToken, *trustProxy)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", *addr)
		errc <- srv.ListenAndServe()
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server", "err", err)
			os.Exit(1)
		}
	case <-sigc:
		logger.Info("shutting down")
		cancel() // signal background goroutines (scheduler, purge loops) to stop
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
	}
}
