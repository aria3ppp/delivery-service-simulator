package app

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/app/config"
	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/app/router"
	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/domain"
	_3pl "github.com/aria3ppp/delivery-service-simulator/internal/delivery/infras/3pl"
	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/infras/core"
	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/infras/repo"
	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/usecase"
	"github.com/lib/pq"
	"github.com/samber/lo"
)

type app struct {
	logger *slog.Logger
	config *config.WorkerConfig
	sqlDB  *sql.DB
	server *http.Server

	core usecase.Core
	_3pl usecase.ThirdPartyLogistics
}

func New(
	ctx context.Context,
	config *config.Config,
	sqlDB *sql.DB,
	logger *slog.Logger,
) *app {
	core := core.NewCore(logger)
	_3pl := _3pl.New3PL(logger)
	repo := repo.NewRepo(sqlDB, logger)

	usecase := usecase.NewUseCase(core, _3pl, repo, logger)

	router := router.NewRouter(usecase, logger)
	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	return &app{
		logger: logger,
		config: &config.WorkerConfig,
		sqlDB:  sqlDB,
		server: server,
		core:   core,
		_3pl:   _3pl,
	}
}

func (a *app) StartServer() {
	a.logger.Info("Starting server", slog.String("addr", a.server.Addr))
	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		a.logger.Error("Failed to listen and serve", slog.String("addr", a.server.Addr), slog.Any("error", err))
	}
}

func (a *app) ShutdownServer(ctx context.Context) {
	a.logger.Info("Shutting down server")
	if err := a.server.Shutdown(ctx); err != nil {
		a.logger.Error("Failed to shutdown server", slog.Any("error", err))
	}
}

func (a *app) StartPendingWorker(ctx context.Context, interval time.Duration) error {
	a.logger.Info("Starting pending worker")

	logger := a.logger.With(slog.String("worker", "pending"))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if err := ctx.Err(); err != nil {
		logger.Error("ctx have error", slog.Any("error", err))
		return err
	}

	if err := a.runPendingWorker(ctx, logger); err != nil {
		a.logger.Error("Running pending worker failed", slog.Any("error", err))
		return err
	}

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Pending worker stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := a.runPendingWorker(ctx, logger); err != nil {
				a.logger.Error("Running pending worker failed", slog.Any("error", err))
				return err
			}
		}
	}
}
func (a *app) runPendingWorker(ctx context.Context, logger *slog.Logger) error {
	for {
		tx, err := a.sqlDB.Begin()
		if err != nil {
			logger.Error("failed to begin transaction", slog.Any("error", err))
			return err
		}

		query := `
	UPDATE shipments
	SET status = 'pending'
	WHERE uid IN (
		SELECT uid FROM shipments
		WHERE status = 'queued'
		  AND scheduled_delivery_min_time <= NOW() + ($1 * INTERVAL '1 second')
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	)
	RETURNING uid;
	`

		rows, err := tx.Query(query, a.config.PendingIntervalInSeconds, a.config.PendingWorkerBatchSize)
		if err != nil {
			tx.Rollback()
			logger.Error("failed to execute batch update", slog.Any("error", err))
			return err
		}

		var updatedCount int
		for rows.Next() {
			var uid string
			if err := rows.Scan(&uid); err != nil {
				logger.Error("error scanning uid", slog.Any("error", err))
				continue
			}
			updatedCount++
		}
		rows.Close()

		if err := tx.Commit(); err != nil {
			logger.Error("transaction commit failed", slog.Any("error", err))
			return err
		}

		if updatedCount == 0 {
			logger.Debug("No more shipments to update in this cycle.")
			break
		}

		logger.Info("Successfully updated batch shipments to pending status", slog.Int("batch_length", updatedCount))
	}

	return nil
}

func (a *app) StartShippingWorker(ctx context.Context, interval time.Duration) error {
	a.logger.Info("Starting shipping worker")

	logger := a.logger.With(slog.String("worker", "shipping"))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if err := ctx.Err(); err != nil {
		logger.Error("ctx have error", slog.Any("error", err))
		return err
	}

	if err := a.runShippingWorker(ctx, logger); err != nil {
		a.logger.Error("Running shipping worker failed", slog.Any("error", err))
		return err
	}

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Shipping worker stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := a.runShippingWorker(ctx, logger); err != nil {
				a.logger.Error("Running shipping worker failed", slog.Any("error", err))
				return err
			}
		}

	}
}

func (a *app) runShippingWorker(ctx context.Context, logger *slog.Logger) error {
	for {
		tx, err := a.sqlDB.Begin()
		if err != nil {
			logger.Error("failed to begin transaction", slog.Any("error", err))
			return err
		}

		query := `
			SELECT uid, user_uid, user_addr, origin_point, destination_point, scheduled_delivery_min_time, scheduled_delivery_max_time, status
			FROM shipments
			WHERE status = 'pending'
			LIMIT $1
			FOR UPDATE SKIP LOCKED;
		`

		rows, err := tx.Query(query, a.config.PendingWorkerBatchSize)
		if err != nil {
			tx.Rollback()
			logger.Error("failed to query", slog.Any("error", err))
			return err
		}

		shipments := make([]domain.Shipment, 0, a.config.PendingWorkerBatchSize)
		for rows.Next() {
			var s domain.Shipment
			err := rows.Scan(
				&s.UID,
				&s.UserUID,
				&s.UserAddr,
				&s.OriginPoint,
				&s.DestinationPoint,
				&s.ScheduledDeliveryMinTime,
				&s.ScheduledDeliveryMaxTime,
				&s.Status,
			)
			if err != nil {
				logger.Error("error scanning shipment", slog.Any("error", err))
				continue
			}
			shipments = append(shipments, s)
		}
		rows.Close()

		if len(shipments) == 0 {
			logger.Debug("No more pending shipments in this cycle.")

			if err := tx.Commit(); err != nil {
				logger.Error("transaction commit failed", slog.Any("error", err))
				return err
			}

			break
		}

		var wg sync.WaitGroup
		wg.Add(len(shipments))

		shipmentRequestUIDsCh := make(chan string, len(shipments))

		for _, shipment := range shipments {
			go func() {
				defer wg.Done()

				if _, err := a._3pl.RequestDeliveryGuy(
					ctx,
					&domain.ThirdPartyLogisticsRequestDeliveryGuyInput{
						ShipmentUID: shipment.UID,
						RoutingInfo: domain.RoutingInfo{
							Origin:      shipment.OriginPoint,
							Destination: shipment.DestinationPoint,
						},
						ScheduledDeliveryWindow: domain.ScheduledDeliveryWindow{
							StartTime: shipment.ScheduledDeliveryMinTime,
							EndTime:   shipment.ScheduledDeliveryMaxTime,
						},
					},
				); err != nil {
					logger.Error("failed to request delivery guy", slog.Any("error", err))
				} else {
					shipmentRequestUIDsCh <- shipment.UID
				}
			}()
		}

		wg.Wait()
		close(shipmentRequestUIDsCh)
		shipmentRequestUIDs := lo.ChannelToSlice(shipmentRequestUIDsCh)

		query = `
		UPDATE shipments
		SET status = 'requested'
		WHERE uid = ANY($1)
		RETURNING uid;
		`

		rows, err = tx.Query(query, pq.Array(shipmentRequestUIDs))
		if err != nil {
			tx.Rollback()
			logger.Error("failed to execute batch update", slog.Any("error", err))
			return err
		}

		for rows.Next() {
			var uid string
			if err := rows.Scan(&uid); err != nil {
				logger.Error("error scanning uid", slog.Any("error", err))
				continue
			}
		}
		rows.Close()

		if err := tx.Commit(); err != nil {
			logger.Error("transaction commit failed", slog.Any("error", err))
			return err
		}

		wg.Add(len(shipmentRequestUIDs))

		for _, shipmentUID := range shipmentRequestUIDs {
			go func() {
				defer wg.Done()

				if _, err := a.core.Webhook(ctx, &domain.CoreWebhookInput{ShipmentUID: shipmentUID, Status: "requested"}); err != nil {
					logger.Error("failed to webhook", slog.Any("shipment_uid", shipmentUID), slog.Any("error", err))
				}
			}()
		}

		wg.Wait()
	}

	return nil
}
