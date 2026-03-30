package resilience

import (
	"context"
	"fmt"

	"github.com/itau-ai-assistant/bfa-go/internal/observability"
	"golang.org/x/sync/semaphore"
)

type Bulkhead struct {
	limiter *semaphore.Weighted
	metrics *observability.Metrics
}

func NewBulkhead(limit int64, metrics *observability.Metrics) *Bulkhead {
	return &Bulkhead{
		limiter: semaphore.NewWeighted(limit),
		metrics: metrics,
	}
}

func (b *Bulkhead) Run(ctx context.Context, dependency string, fn func() error) error {
	if !b.limiter.TryAcquire(1) {
		b.metrics.ConcurrencySaturation.WithLabelValues(dependency).Inc()
		return fmt.Errorf("bulkhead saturated for %s", dependency)
	}
	defer b.limiter.Release(1)
	return fn()
}
