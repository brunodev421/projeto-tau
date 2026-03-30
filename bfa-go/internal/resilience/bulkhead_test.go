package resilience

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/itau-ai-assistant/bfa-go/internal/observability"
)

func TestBulkheadRejectsWhenSaturated(t *testing.T) {
	metrics := observability.NewMetrics(prometheus.NewRegistry())
	bulkhead := NewBulkhead(1, metrics)

	blockerDone := make(chan struct{})
	go func() {
		_ = bulkhead.Run(context.Background(), "profile_api", func() error {
			time.Sleep(150 * time.Millisecond)
			close(blockerDone)
			return nil
		})
	}()

	time.Sleep(20 * time.Millisecond)
	err := bulkhead.Run(context.Background(), "profile_api", func() error { return nil })
	if err == nil {
		t.Fatal("expected saturation error")
	}
	<-blockerDone
}
