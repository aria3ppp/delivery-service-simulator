package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/domain"
	internal_error "github.com/aria3ppp/delivery-service-simulator/internal/delivery/error"
)

type usecase struct {
	core   Core
	_3pl   ThirdPartyLogistics
	repo   Repo
	logger *slog.Logger
}

var _ UseCase = (*usecase)(nil)

func NewUseCase(
	core Core,
	_3pl ThirdPartyLogistics,
	repo Repo,
	logger *slog.Logger,
) *usecase {
	return &usecase{
		core:   core,
		_3pl:   _3pl,
		repo:   repo,
		logger: logger,
	}
}

func (u *usecase) Request(ctx context.Context, input *domain.RequestInput) (*domain.RequestResult, error) {
	logger := u.logger.With(slog.Any("usecase", "request"), slog.String("shipment_uid", input.ShipmentUID))

	if err := input.Validate(); err != nil {
		logger.Error("input validation failed", slog.Any("error", err))
		return nil, internal_error.ValidationError(err.Error())
	}

	status := "queued"
	if input.ScheduledDeliveryWindow.StartTime.Before(time.Now()) {
		status = "requested"
	}

	shipment := &domain.Shipment{
		UID:                      input.ShipmentUID,
		UserUID:                  input.UserInfo.UserUID,
		UserAddr:                 input.UserInfo.Address,
		OriginPoint:              input.RoutingInfo.Origin,
		DestinationPoint:         input.RoutingInfo.Destination,
		ScheduledDeliveryMinTime: input.ScheduledDeliveryWindow.StartTime,
		ScheduledDeliveryMaxTime: input.ScheduledDeliveryWindow.EndTime,
		Status:                   status,
	}

	if err := u.repo.InsertShipment(ctx, shipment); err != nil {
		logger.Error("failed to insert shipment", slog.Any("error", err))
		return nil, err
	}

	if status != "queued" {
		logger.Info("shipment is within the scheduled window: request delivery guy now")

		if _, err := u._3pl.RequestDeliveryGuy(ctx, &domain.ThirdPartyLogisticsRequestDeliveryGuyInput{
			ShipmentUID:             input.ShipmentUID,
			RoutingInfo:             input.RoutingInfo,
			ScheduledDeliveryWindow: input.ScheduledDeliveryWindow,
		}); err != nil {
			logger.Error("failed to request delivery guy", slog.Any("error", err))
			return nil, err
		}
	}

	return nil, nil
}

func (u *usecase) Webhook(ctx context.Context, input *domain.WebhookInput) (*domain.WebhookResult, error) {
	logger := u.logger.With(slog.Any("usecase", "webhook"), slog.String("shipment_uid", input.ShipmentUID))

	if err := input.Validate(); err != nil {
		logger.Error("input validation failed", slog.Any("error", err))
		return nil, internal_error.ValidationError(err.Error())
	}

	if err := u.repo.SetShipmentStatus(ctx, input.ShipmentUID, input.Status); err != nil {
		logger.Error("failed to set shipment status", slog.Any("error", err))
		return nil, err
	}

	if input.Status == "not_found" {
		logger.Info("could not find a delivery guy")

		if time.Now().Hour() < 23 {
			logger.Info("clock is not 23:00 yet: request another delivery guy")

			shipment, err := u.repo.GetShipment(ctx, input.ShipmentUID)
			if err != nil {
				logger.Error("failed to fetch shipment", slog.Any("error", err))
				return nil, err
			}

			if _, err := u._3pl.RequestDeliveryGuy(ctx, &domain.ThirdPartyLogisticsRequestDeliveryGuyInput{
				ShipmentUID: input.ShipmentUID,
				RoutingInfo: domain.RoutingInfo{
					Origin:      shipment.OriginPoint,
					Destination: shipment.DestinationPoint,
				},
				ScheduledDeliveryWindow: domain.ScheduledDeliveryWindow{
					StartTime: shipment.ScheduledDeliveryMinTime,
					EndTime:   shipment.ScheduledDeliveryMaxTime,
				},
			}); err != nil {
				logger.Error("failed to request delivery guy", slog.Any("error", err))
				return nil, err
			}
		}
	}

	if _, err := u.core.Webhook(ctx, &domain.CoreWebhookInput{
		ShipmentUID: input.ShipmentUID,
		Status:      input.Status,
	}); err != nil {
		logger.Error("failed to invoke core webhook", slog.Any("error", err))
		return nil, err
	}

	return nil, nil
}
