package usecase

import (
	"context"

	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/domain"
)

type (
	Core interface {
		Webhook(ctx context.Context, input *domain.CoreWebhookInput) (*domain.CoreWebhookResult, error)
	}

	ThirdPartyLogistics interface {
		RequestDeliveryGuy(ctx context.Context, input *domain.ThirdPartyLogisticsRequestDeliveryGuyInput) (*domain.ThirdPartyLogisticsRequestDeliveryGuyResult, error)
	}

	Repo interface {
		GetShipment(ctx context.Context, shipmentUID string) (*domain.Shipment, error)
		InsertShipment(ctx context.Context, shipment *domain.Shipment) error
		SetShipmentStatus(ctx context.Context, shipmentUID string, status string) error
	}

	UseCase interface {
		Request(ctx context.Context, input *domain.RequestInput) (*domain.RequestResult, error)
		Webhook(ctx context.Context, input *domain.WebhookInput) (*domain.WebhookResult, error)
	}
)
