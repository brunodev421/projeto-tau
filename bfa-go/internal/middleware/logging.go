package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/itau-ai-assistant/bfa-go/internal/observability"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func Logging(logger *slog.Logger, metrics *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r)

			duration := time.Since(start)
			route := Route(r.Context())
			if route == "" {
				if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
					route = routeContext.RoutePattern()
				}
			}
			if route == "" {
				route = r.URL.Path
			}
			metrics.ObserveRequest(route, recorder.status, duration)
			customerID := CustomerID(r.Context())
			if customerID == "" {
				customerID = chi.URLParam(r, "customerId")
			}

			log := logger.With(
				"timestamp", time.Now().UTC().Format(time.RFC3339Nano),
				"request_id", observability.RequestID(r.Context()),
				"customer_id", customerID,
				"route", route,
				"latency_ms", duration.Milliseconds(),
			)

			if recorder.status >= http.StatusBadRequest {
				log.Error("request failed", "error_code", strconv.Itoa(recorder.status))
				return
			}

			log.Info("request completed")
		})
	}
}
