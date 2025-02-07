package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func main() {
	for index := 0; ; index++ {
		timeNow := time.Now().Add(-65 * time.Minute)
		body, err := json.Marshal(map[string]any{
			"shipment_uid": fmt.Sprintf("shipment_%d", index),
			"user_info": map[string]any{
				"user_uid": fmt.Sprintf("user_%d", index),
				"address":  fmt.Sprintf("address_%d", index),
			},
			"routing_info": map[string]any{
				"lat":  0,
				"long": 0,
			},
			"scheduled_delivery_window": map[string]any{
				"start_time": timeNow,
				"end_time":   timeNow.Add(2 * time.Hour),
			},
		})
		if err != nil {
			panic(err)
		}

		resp, err := http.Post("http://localhost:8080/request", "application/json", bytes.NewReader(body))
		if err != nil {
			panic(err)
		}

		if resp.StatusCode != http.StatusOK {
			panic(fmt.Errorf("resp.StatusCode (%d) != 200", resp.StatusCode))
		}

		time.Sleep(200 * time.Millisecond) // more that 10K/hour
	}
}
