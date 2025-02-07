package domain

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type UserInfo struct {
	UserUID string `json:"user_uid"`
	Address string `json:"address"`
}

func (o *UserInfo) Validate() error {
	if o.UserUID == "" {
		return errors.New(".user_uid is required")
	}

	if o.Address == "" {
		return errors.New(".address is required")
	}

	return nil
}

type Location struct {
	Lat  float64 `json:"lat"`
	Long float64 `json:"long"`
}

func (o *Location) Validate() error {
	if o.Lat < -90 || o.Lat > 90 {
		return errors.New(".lat must be between -90 and 90")
	}

	if o.Long < -180 || o.Long > 180 {
		return errors.New(".long must be between -180 and 180")
	}

	return nil
}

func (l *Location) Scan(value interface{}) error {
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}

	str := strings.Trim(string(data), "()")
	parts := strings.Split(str, ",")
	if len(parts) != 2 {
		return fmt.Errorf("invalid point format: %s", str)
	}

	x, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return err
	}

	y, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return err
	}

	l.Long = x
	l.Lat = y
	return nil
}

func (l Location) Value() (driver.Value, error) {
	return fmt.Sprintf("(%f,%f)", l.Long, l.Lat), nil
}

type RoutingInfo struct {
	Origin      Location `json:"origin"`
	Destination Location `json:"destination"`
}

func (o *RoutingInfo) Validate() error {
	if err := o.Origin.Validate(); err != nil {
		return fmt.Errorf(".origin%s", err)
	}

	if err := o.Destination.Validate(); err != nil {
		return fmt.Errorf(".destination%s", err)
	}

	return nil
}

type ScheduledDeliveryWindow struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

func (o *ScheduledDeliveryWindow) Validate() error {
	if o.EndTime.Before(time.Now()) {
		return errors.New(".end_time expired")
	}

	if o.EndTime.Sub(o.StartTime) != 2*time.Hour {
		return errors.New(" must be 2 hours")
	}

	return nil
}

type RequestInput struct {
	ShipmentUID             string                  `json:"shipment_uid"`
	UserInfo                UserInfo                `json:"user_info"`
	RoutingInfo             RoutingInfo             `json:"routing_info"`
	ScheduledDeliveryWindow ScheduledDeliveryWindow `json:"scheduled_delivery_window"`
}

func (o *RequestInput) Validate() error {
	if o.ShipmentUID == "" {
		return errors.New("shipment_uid is required")
	}

	if err := o.UserInfo.Validate(); err != nil {
		return fmt.Errorf("user_info%s", err)
	}

	if err := o.RoutingInfo.Validate(); err != nil {
		return fmt.Errorf("routing_info%s", err)
	}

	if err := o.ScheduledDeliveryWindow.Validate(); err != nil {
		return fmt.Errorf("scheduled_delivery_window%s", err)
	}

	return nil
}

type RequestResult struct{}

type WebhookInput struct {
	ShipmentUID string `json:"shipment_uid"`
	Status      string `json:"status"`
}

func (o *WebhookInput) Validate() error {
	if o.ShipmentUID == "" {
		return errors.New("shipment_uid is required")
	}

	if o.Status == "" {
		return errors.New("status is required")
	}

	return nil
}

type WebhookResult struct{}
