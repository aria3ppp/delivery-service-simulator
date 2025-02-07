package domain

type ThirdPartyLogisticsRequestDeliveryGuyInput struct {
	ShipmentUID             string
	RoutingInfo             RoutingInfo
	ScheduledDeliveryWindow ScheduledDeliveryWindow
}

type ThirdPartyLogisticsRequestDeliveryGuyResult struct{}
