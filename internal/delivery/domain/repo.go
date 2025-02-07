package domain

import "time"

type Shipment struct {
	UID                      string
	UserUID                  string
	UserAddr                 string
	OriginPoint              Location
	DestinationPoint         Location
	ScheduledDeliveryMinTime time.Time
	ScheduledDeliveryMaxTime time.Time
	Status                   string
}
