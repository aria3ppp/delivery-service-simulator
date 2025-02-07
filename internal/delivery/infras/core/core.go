package core

import (
	"context"
	"log/slog"

	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/domain"
	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/usecase"
)

type core struct {
	logger *slog.Logger
}

var _ usecase.Core = (*core)(nil)

func NewCore(logger *slog.Logger) *core {
	return &core{logger: logger}
}

func (c *core) Webhook(ctx context.Context, input *domain.CoreWebhookInput) (*domain.CoreWebhookResult, error) {
	logger := c.logger.With(slog.String("infra", "core"))

	logger.Info("invoke webhook", slog.String("shipment_uid", input.ShipmentUID), slog.String("status", input.Status))

	return nil, nil
}
