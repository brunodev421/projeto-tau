package resilience

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker/v2"

	"github.com/itau-ai-assistant/bfa-go/internal/observability"
)

func TestCircuitBreakerOpensAfterThreshold(t *testing.T) {
	metrics := observability.NewMetrics(prometheus.NewRegistry())
	breaker := NewCircuitBreaker("agent_service", 0.5, 2, 50*time.Millisecond, metrics)

	for range 2 {
		_, _ = breaker.Execute(func() (any, error) { return nil, errors.New("boom") })
	}

	_, err := breaker.Execute(func() (any, error) { return nil, nil })
	if !errors.Is(err, gobreaker.ErrOpenState) {
		t.Fatalf("expected open state error, got %v", err)
	}
}
