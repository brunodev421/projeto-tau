package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/itau-ai-assistant/bfa-go/internal/observability"
)

type contextKey string

const (
	RouteKey      contextKey = "route"
	CustomerIDKey contextKey = "customer_id"
)

func RequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		ctx := observability.WithRequestID(r.Context(), requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func WithRoute(ctx context.Context, route string) context.Context {
	return context.WithValue(ctx, RouteKey, route)
}

func Route(ctx context.Context) string {
	value, _ := ctx.Value(RouteKey).(string)
	return value
}

func WithCustomerID(ctx context.Context, customerID string) context.Context {
	return context.WithValue(ctx, CustomerIDKey, customerID)
}

func CustomerID(ctx context.Context) string {
	value, _ := ctx.Value(CustomerIDKey).(string)
	return value
}
