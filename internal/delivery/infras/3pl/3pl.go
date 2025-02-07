package _3pl

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/domain"
	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/usecase"
)

type _3pl struct {
	logger *slog.Logger
}

var _ usecase.ThirdPartyLogistics = (*_3pl)(nil)

func New3PL(logger *slog.Logger) *_3pl {
	return &_3pl{logger: logger}
}

func (t *_3pl) RequestDeliveryGuy(ctx context.Context, input *domain.ThirdPartyLogisticsRequestDeliveryGuyInput) (*domain.ThirdPartyLogisticsRequestDeliveryGuyResult, error) {
	logger := t.logger.With(slog.String("infra", "3pl"))

	logger.Info("request delivery guy", slog.String("shipment_uid", input.ShipmentUID))

	body, err := json.Marshal(map[string]any{
		"shipment_uid": input.ShipmentUID,
	})
	if err != nil {
		logger.Error("failed to marshal body", slog.Any("error", err))
		return nil, err
	}
	resp, err := http.Post("http://localhost:9090/request", "applicaion/json", bytes.NewReader(body))
	if err != nil {
		logger.Error("failed to http post", slog.Any("error", err))
		return nil, err
	}
	if resp.StatusCode != 200 {
		logger.Error("failed to http post", slog.Int("status_code", resp.StatusCode), slog.Any("error", err))
		return nil, err
	}

	return nil, nil
}
