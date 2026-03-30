package resilience

import (
	"time"

	"github.com/itau-ai-assistant/bfa-go/internal/observability"
	"github.com/sony/gobreaker/v2"
)

func NewCircuitBreaker(name string, failRatio float64, minRequests uint32, openTimeout time.Duration, metrics *observability.Metrics) *gobreaker.CircuitBreaker[any] {
	settings := gobreaker.Settings{
		Name:        name,
		Timeout:     openTimeout,
		MaxRequests: 1,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < minRequests {
				return false
			}
			failures := float64(counts.TotalFailures) / float64(counts.Requests)
			return failures >= failRatio
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			metrics.CircuitBreakerEvents.WithLabelValues(name, to.String()).Inc()
		},
	}

	return gobreaker.NewCircuitBreaker[any](settings)
}
