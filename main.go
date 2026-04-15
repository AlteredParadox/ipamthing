package main

import (
	"context"
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

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "ipamthing.db", "SQLite database file path")
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

	scanner := scan.NewScanner(logger)
	scheduler := scan.NewScheduler(database, scanner, logger)
	go scheduler.Run(ctx)

	handler := web.NewRouter(logger, database, sessions, scanner, scheduler)

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
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
	}
}
