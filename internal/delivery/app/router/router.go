package router

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/domain"
	internal_error "github.com/aria3ppp/delivery-service-simulator/internal/delivery/error"
	"github.com/aria3ppp/delivery-service-simulator/internal/delivery/usecase"

	goccy_json "github.com/goccy/go-json"
)

type router struct {
	uc     usecase.UseCase
	logger *slog.Logger
	mux    *http.ServeMux
}

var _ http.Handler = (*router)(nil)

func NewRouter(
	uc usecase.UseCase,
	logger *slog.Logger,
) *router {
	router := &router{
		uc:     uc,
		logger: logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /request", router.request)
	mux.HandleFunc("POST /webhook", router.webhook)

	router.mux = mux
	return router
}

func (r *router) request(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	defer req.Body.Close()

	logger := r.logger.With(slog.String("method", req.Method), slog.String("url", req.URL.Path))

	var requestInput domain.RequestInput
	if err := goccy_json.NewDecoder(req.Body).Decode(&requestInput); err != nil {
		logger.Error("failed to decode request", slog.Any("error", err))

		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); err != nil {
			http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		}

		return
	}

	response, err := r.uc.Request(req.Context(), &requestInput)
	if err != nil {
		logger.Error("failed to uc.RequestDelivery", slog.Any("error", err))

		if _, ok := err.(internal_error.ValidationError); ok {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); err != nil {
			http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		}

		return
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("failed to encode response", slog.Any("error", err))
		http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (r *router) webhook(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	defer req.Body.Close()

	logger := r.logger.With(slog.String("method", req.Method), slog.String("url", req.URL.Path))

	var webhookInput domain.WebhookInput
	if err := goccy_json.NewDecoder(req.Body).Decode(&webhookInput); err != nil {
		logger.Error("failed to decode request", slog.Any("error", err))

		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); err != nil {
			http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		}

		return
	}

	response, err := r.uc.Webhook(req.Context(), &webhookInput)
	if err != nil {
		logger.Error("failed to uc.Webhook", slog.Any("error", err))

		if _, ok := err.(internal_error.ValidationError); ok {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); err != nil {
			http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		}

		return
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("failed to encode response", slog.Any("error", err))
		http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (r *router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
