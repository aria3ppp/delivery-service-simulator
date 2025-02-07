package domain

type CoreWebhookInput struct {
	ShipmentUID string `json:"shipment_uid"`
	Status      string `json:"status"`
}

type CoreWebhookResult struct{}
