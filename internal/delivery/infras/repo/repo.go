package repo

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/domain"
	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/usecase"
)

type repo struct {
	sqlDB  *sql.DB
	logger *slog.Logger
}

var _ usecase.Repo = (*repo)(nil)

func NewRepo(
	sqlDB *sql.DB,
	logger *slog.Logger,
) *repo {
	return &repo{
		sqlDB:  sqlDB,
		logger: logger,
	}
}

func (r *repo) GetShipment(ctx context.Context, shipmentUID string) (*domain.Shipment, error) {
	logger := r.logger.With(slog.Any("infra", "repo"), slog.String("method", "get_shipment"))

	queryStmt := `
	SELECT uid, user_uid, user_addr, origin_point, destination_point, scheduled_delivery_min_time, scheduled_delivery_max_time, status
	FROM shipments
	WHERE uid = $1
	`

	row := r.sqlDB.QueryRowContext(ctx, queryStmt, shipmentUID)

	var shipment domain.Shipment
	err := row.Scan(
		&shipment.UID,
		&shipment.UserUID,
		&shipment.UserAddr,
		&shipment.OriginPoint,
		&shipment.DestinationPoint,
		&shipment.ScheduledDeliveryMinTime,
		&shipment.ScheduledDeliveryMaxTime,
		&shipment.Status,
	)
	if err != nil {
		logger.Error("error scanning shipment", slog.Any("error", err))
		return nil, err
	}

	return &shipment, nil
}

func (r *repo) InsertShipment(ctx context.Context, shipment *domain.Shipment) error {
	logger := r.logger.With(slog.Any("infra", "repo"), slog.String("method", "insert_shipment"))

	insertStmt := `
		INSERT INTO shipments(
			uid, user_uid, user_addr, 
			origin_point, destination_point, 
			scheduled_delivery_min_time, scheduled_delivery_max_time,
			status
		) VALUES($1, $2, $3, point($4, $5), point($6, $7), $8, $9, $10)`

	if _, err := r.sqlDB.ExecContext(
		ctx,
		insertStmt,
		shipment.UID,
		shipment.UserUID,
		shipment.UserAddr,
		shipment.OriginPoint.Lat,
		shipment.OriginPoint.Long,
		shipment.DestinationPoint.Lat,
		shipment.DestinationPoint.Long,
		shipment.ScheduledDeliveryMinTime,
		shipment.ScheduledDeliveryMaxTime,
		shipment.Status,
	); err != nil {
		logger.Error("failed to insert record", slog.Any("error", err))
		return err
	}

	return nil
}

func (r *repo) SetShipmentStatus(ctx context.Context, shipmentUID string, status string) error {
	logger := r.logger.With(slog.Any("infra", "repo"), slog.String("method", "set_shipment_status"))

	updateStmt := `
	UPDATE shipments
	SET status = $1
	WHERE uid = $2
	RETURNING uid;
	`

	row := r.sqlDB.QueryRowContext(ctx, updateStmt, status, shipmentUID)

	var uid string
	if err := row.Scan(&uid); err != nil {
		logger.Error("failed to scan row from update statement result", slog.String("shipment_uid", shipmentUID), slog.String("status", status), slog.Any("error", err))
		return err
	}

	if uid != shipmentUID {
		logger.Error("scanned uid is not equal to shipment_uid", slog.String("shipment_uid", shipmentUID), slog.String("scanned uid", uid))
		return fmt.Errorf("scanned uid (%s) is not equal to shipment_uid (%s)", uid, shipmentUID)
	}

	return nil
}
