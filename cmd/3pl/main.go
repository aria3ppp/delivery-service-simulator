package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	goccy_json "github.com/goccy/go-json"
	_ "github.com/lib/pq"
)

type Shipment struct {
	ShipmentUID string    `json:"shipment_uid"`
	StartTime   time.Time `json:"-"`
	Retries     int       `json:"-"`
	Status      string    `json:"-"`
}

var logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
var db *sql.DB
var connStr = "postgres://postgres:secret@localhost:5432/postgres?sslmode=disable"

func main() {
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		logger.Error("failed to open connection", slog.Any("error", err))
		return
	}
	defer db.Close()

	http.HandleFunc("/request", func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()

		var shipment Shipment
		if err := goccy_json.NewDecoder(req.Body).Decode(&shipment); err != nil {
			http.Error(w, "failed to decode body as json: "+err.Error(), http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			tx.Rollback()
			http.Error(w, "failed to begin transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}

		row := tx.QueryRow(`select retries, status from shipments_3pl where shipment_uid = $1`, shipment.ShipmentUID)
		var retries int
		var status string
		err = row.Scan(&retries, &status)
		if err == nil {
			if retries < 3 {
				retries++
			} else {
				status = "found"
			}
			if _, err := tx.Exec(
				`update shipment_3pl set retries = $1, status = $2, start_time = NOW() + INTERVAL '5 minutes' where shipment_uid = $3`,
				retries,
				status,
				shipment.ShipmentUID,
			); err != nil {
				tx.Rollback()
				http.Error(w, "failed to increment retries: "+err.Error(), http.StatusInternalServerError)
				return
			}

			if err := tx.Commit(); err != nil {
				http.Error(w, "failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
				return
			}

			return

		} else {
			if err != sql.ErrNoRows {
				tx.Rollback()
				http.Error(w, "failed to scan row: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if _, err := tx.Exec(
			`insert into shipments_3pl (shipment_uid,start_time,retries,status) values ($1,NOW() + INTERVAL '5 minutes',$2,$3)`,
			shipment.ShipmentUID,
			1,
			"requested",
		); err != nil {
			tx.Rollback()
			http.Error(w, "failed to insert shipment into database: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
	})

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Info("listening", slog.String("addr", ":9090"))
		http.ListenAndServe(":9090", nil)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Info("starting searching worker")
		startSearchingWorker(logger)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Info("starting finding worker")
		startFindingWorker(logger)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Info("starting shipping worker")
		startShippingWorker(logger)
	}()

	wg.Wait()
}

func startSearchingWorker(logger *slog.Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	logger = logger.With(slog.String("worker", "searching"))

	runSearchingWorker(logger)
	for range ticker.C {
		runSearchingWorker(logger)
	}
}

func runSearchingWorker(logger *slog.Logger) {
	for {
		tx, err := db.Begin()
		if err != nil {
			logger.Error("failed to begin transaction", slog.Any("error", err))
			os.Exit(1)
		}

		query := `
		UPDATE shipments_3pl
		SET status = 'searching', start_time = NOW() + INTERVAL '5 minutes'
		WHERE shipment_uid IN (
			SELECT shipment_uid FROM shipments_3pl
			WHERE status = 'requested'
			  AND start_time <= NOW()
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING shipment_uid;
		`

		rows, err := tx.Query(query, 100)
		if err != nil {
			tx.Rollback()
			logger.Error("failed to execute batch update", slog.Any("error", err))
			os.Exit(1)
		}

		uids := make([]string, 0, 100)
		for rows.Next() {
			var uid string
			if err := rows.Scan(&uid); err != nil {
				logger.Error("error scanning uid", slog.Any("error", err))
				continue
			}
			uids = append(uids, uid)
		}
		rows.Close()

		if err := tx.Commit(); err != nil {
			logger.Error("transaction commit failed", slog.Any("error", err))
			os.Exit(1)
		}

		if len(uids) == 0 {
			logger.Info("No more shipments to update in this cycle.")
			break
		}

		logger.Info("Successfully updated batch shipments to searching status", slog.Int("batch_length", len(uids)))

		var wg sync.WaitGroup
		for _, uid := range uids {
			wg.Add(1)
			go func() {
				defer wg.Done()

				body, err := json.Marshal(map[string]any{
					"shipment_uid": uid,
					"status":       "searching",
				})
				if err != nil {
					logger.Error("failed to marshal body", slog.String("service", "delivery webhook"), slog.Any("error", err))
					os.Exit(1)
				}
				resp, err := http.Post("http://localhost:8080/webhook", "application/json", bytes.NewReader(body))
				if err != nil {
					logger.Error("failed to http post", slog.String("service", "delivery webhook"), slog.Any("error", err))
					os.Exit(1)
				}
				if resp.StatusCode != 200 {
					logger.Error("failed to http post", slog.Int("status_code", resp.StatusCode), slog.String("service", "delivery webhook"), slog.Any("error", err))
					os.Exit(1)
				}
			}()
		}
		wg.Wait()
	}
}

func startFindingWorker(logger *slog.Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	logger = logger.With(slog.String("worker", "finding"))

	runFindingWorker(logger)
	for range ticker.C {
		runFindingWorker(logger)
	}
}

func runFindingWorker(logger *slog.Logger) {
	for {
		tx, err := db.Begin()
		if err != nil {
			logger.Error("failed to begin transaction", slog.Any("error", err))
			os.Exit(1)
		}

		query := `
			SELECT shipment_uid, retries, status FROM shipments_3pl
			WHERE status = 'searching'
			  AND start_time <= NOW() + INTERVAL '5 minutes'
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		`

		rows, err := tx.Query(query, 100)
		if err != nil {
			tx.Rollback()
			logger.Error("failed to execute batch update", slog.Any("error", err))
			os.Exit(1)
		}

		shipments := make([]Shipment, 0, 100)
		for rows.Next() {
			var shipment Shipment
			if err := rows.Scan(&shipment.ShipmentUID, &shipment.Retries, &shipment.Status); err != nil {
				logger.Error("error scanning uid", slog.Any("error", err))
				continue
			}
			shipments = append(shipments, shipment)
		}
		rows.Close()

		if len(shipments) == 0 {
			if err := tx.Commit(); err != nil {
				logger.Error("transaction commit failed", slog.Any("error", err))
				os.Exit(1)
			}

			logger.Info("No more shipments to update in this cycle.")
			break
		}

		for _, shipment := range shipments {
			// if shipment.Retries > 2 {
			shipment.Status = "found"
			if _, err := tx.Exec(`update shipments_3pl set status = $1 where shipment_uid = $2`, shipment.Status, shipment.ShipmentUID); err != nil {
				logger.Error("failed to update status to found", slog.Any("error", err))
				tx.Rollback()
				os.Exit(1)
			}
			// }
		}

		if err := tx.Commit(); err != nil {
			logger.Error("transaction commit failed", slog.Any("error", err))
			os.Exit(1)
		}

		var wg sync.WaitGroup
		for _, shipment := range shipments {
			wg.Add(1)
			go func() {
				defer wg.Done()

				status := shipment.Status
				if shipment.Status != "found" {
					status = "not_found"
				}

				body, err := json.Marshal(map[string]any{
					"shipment_uid": shipment.ShipmentUID,
					"status":       status,
				})
				if err != nil {
					logger.Error("failed to marshal body", slog.String("service", "delivery webhook"), slog.Any("error", err))
					os.Exit(1)
				}
				resp, err := http.Post("http://localhost:8080/webhook", "application/json", bytes.NewReader(body))
				if err != nil {
					logger.Error("failed to http post", slog.String("service", "delivery webhook"), slog.Any("error", err))
					os.Exit(1)
				}
				if resp.StatusCode != 200 {
					logger.Error("failed to http post", slog.Int("status_code", resp.StatusCode), slog.String("service", "delivery webhook"), slog.Any("error", err))
					os.Exit(1)
				}
			}()
		}
		wg.Wait()

		logger.Info("Successfully updated batch shipments to found/not_found status", slog.Int("batch_length", len(shipments)))

	}
}

func startShippingWorker(logger *slog.Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	logger = logger.With(slog.String("worker", "shipping"))

	runShippingWorker(logger)
	for range ticker.C {
		runShippingWorker(logger)
	}
}

func runShippingWorker(logger *slog.Logger) {
	for {
		tx, err := db.Begin()
		if err != nil {
			logger.Error("failed to begin transaction", slog.Any("error", err))
			os.Exit(1)
		}

		query := `
		UPDATE shipments_3pl
		SET status = 'shipped', start_time = NOW()
		WHERE shipment_uid IN (
			SELECT shipment_uid FROM shipments_3pl
			WHERE status = 'found'
			  AND start_time <= NOW()
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING shipment_uid;
		`

		rows, err := tx.Query(query, 100)
		if err != nil {
			tx.Rollback()
			logger.Error("failed to execute batch update", slog.Any("error", err))
			os.Exit(1)
		}

		uids := make([]string, 0, 100)
		for rows.Next() {
			var uid string
			if err := rows.Scan(&uid); err != nil {
				logger.Error("error scanning uid", slog.Any("error", err))
				continue
			}
			uids = append(uids, uid)
		}
		rows.Close()

		if err := tx.Commit(); err != nil {
			logger.Error("transaction commit failed", slog.Any("error", err))
			os.Exit(1)
		}

		if len(uids) == 0 {
			logger.Info("No more shipments to update in this cycle.")
			break
		}

		logger.Info("Successfully updated batch shipments to shipped status", slog.Int("batch_length", len(uids)))

		var wg sync.WaitGroup
		for _, uid := range uids {
			wg.Add(1)
			go func() {
				defer wg.Done()

				body, err := json.Marshal(map[string]any{
					"shipment_uid": uid,
					"status":       "shipped",
				})
				if err != nil {
					logger.Error("failed to marshal body", slog.String("service", "delivery webhook"), slog.Any("error", err))
					os.Exit(1)
				}
				resp, err := http.Post("http://localhost:8080/webhook", "application/json", bytes.NewReader(body))
				if err != nil {
					logger.Error("failed to http post", slog.String("service", "delivery webhook"), slog.Any("error", err))
					os.Exit(1)
				}
				if resp.StatusCode != 200 {
					logger.Error("failed to http post", slog.Int("status_code", resp.StatusCode), slog.String("service", "delivery webhook"), slog.Any("error", err))
					os.Exit(1)
				}
			}()
		}
		wg.Wait()
	}
}
