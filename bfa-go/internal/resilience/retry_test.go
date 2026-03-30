package resilience

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/itau-ai-assistant/bfa-go/internal/observability"
)

func TestRetryPolicyRetriesTransientErrors(t *testing.T) {
	metrics := observability.NewMetrics(prometheus.NewRegistry())
	policy := NewRetryPolicy(3, metrics)

	attempts := 0
	err := policy.Do(context.Background(), "profile_api", func() error {
		attempts++
		if attempts < 3 {
			return context.DeadlineExceeded
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryPolicyStopsForNonRetryableError(t *testing.T) {
	metrics := observability.NewMetrics(prometheus.NewRegistry())
	policy := NewRetryPolicy(3, metrics)

	attempts := 0
	err := policy.Do(context.Background(), "profile_api", func() error {
		attempts++
		return errors.New("validation failed")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt for non-retryable error, got %d", attempts)
	}
}
