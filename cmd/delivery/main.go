package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/app"
	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/app/config"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

var logLVL slog.Level = slog.LevelInfo

func init() {
	if _, debug := os.LookupEnv("DEBUG"); debug {
		logLVL = slog.LevelDebug
	}
}

func main() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLVL}))

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		select {
		case <-signalCh:
			logger.Info("captured closing signal ctrl+C")
		case <-ctx.Done():
		}

		ctxCancel()
	}()

	connStr := "postgres://postgres:secret@localhost:5432/postgres?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		logger.Error("failed to open connection", slog.Any("error", err))
		return
	}
	defer db.Close()

	if err := runMigrations(db, logger); err != nil {
		logger.Error("failed running migrations", slog.Any("error", err))
		return
	}

	config := &config.Config{
		WorkerConfig: config.WorkerConfig{
			PendingIntervalInSeconds: int64((1 * time.Hour).Seconds()),
			PendingWorkerBatchSize:   100,
			ShipmentWorkerBatchSize:  100,
		},
	}

	app := app.New(ctx, config, db, logger)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := app.StartPendingWorker(ctx, 10*time.Second); err != nil && err != context.Canceled {
			logger.Error("Pending worker failed", slog.Any("error", err))
			ctxCancel()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := app.StartShippingWorker(ctx, 10*time.Second); err != nil && err != context.Canceled {
			logger.Error("Shipping worker failed", slog.Any("error", err))
			ctxCancel()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		app.StartServer()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()
		app.ShutdownServer(context.Background())
	}()

	wg.Wait()
}

func runMigrations(db *sql.DB, logger *slog.Logger) error {
	logger = logger.With("func", "runMigrations")

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		logger.Error("could not create migration driver", slog.Any("error", err))
		return err
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres", driver)
	if err != nil {
		logger.Error("migration instance error", slog.Any("error", err))
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Error("migration failed", slog.Any("error", err))
		return err
	}

	return nil
}
