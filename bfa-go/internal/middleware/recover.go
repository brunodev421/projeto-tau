package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"

	apierrors "github.com/itau-ai-assistant/bfa-go/internal/errors"
)

func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered", "panic", rec)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(apierrors.ErrInternalFailure)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
