package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/itau-ai-assistant/bfa-go/internal/handlers"
	"github.com/itau-ai-assistant/bfa-go/internal/middleware"
	"github.com/itau-ai-assistant/bfa-go/internal/observability"
)

func NewRouter(logger *slog.Logger, metrics *observability.Metrics, registry *prometheus.Registry, assistantHandler http.Handler) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestContext)
	router.Use(middleware.Recover(logger))
	router.Use(middleware.Logging(logger, metrics))
	router.Use(middleware.Tracing("bfa-router"))

	router.Get("/healthz", handlers.Healthz)
	router.Get("/readyz", handlers.Readyz)
	router.Handle("/metrics", observability.Handler(registry))
	router.Get("/v1/assistant/{customerId}", assistantHandler.ServeHTTP)

	return router
}
