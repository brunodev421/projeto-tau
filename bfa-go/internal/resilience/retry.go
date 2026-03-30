package resilience

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/itau-ai-assistant/bfa-go/internal/observability"
)

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	Metrics     *observability.Metrics
}

func NewRetryPolicy(maxAttempts int, metrics *observability.Metrics) RetryPolicy {
	return RetryPolicy{
		MaxAttempts: maxAttempts,
		BaseDelay:   100 * time.Millisecond,
		Metrics:     metrics,
	}
}

func (p RetryPolicy) Do(ctx context.Context, dependency string, fn func() error) error {
	if p.MaxAttempts <= 1 {
		return fn()
	}

	var lastErr error
	for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			if !IsRetryable(err) || attempt == p.MaxAttempts {
				return err
			}
			p.Metrics.RetryCount.WithLabelValues(dependency).Inc()

			backoff := p.BaseDelay * time.Duration(1<<(attempt-1))
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("retry exhausted: %w", lastErr)
}

func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var httpErr *HTTPStatusError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode >= http.StatusInternalServerError || httpErr.StatusCode == http.StatusTooManyRequests
	}
	return false
}
