package observability

import (
	"context"
	"log/slog"
	"os"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func NewLogger(serviceName string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})
	return slog.New(handler).With("service", serviceName)
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}
