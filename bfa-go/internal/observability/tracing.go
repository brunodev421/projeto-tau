package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

func InitTracer(ctx context.Context, serviceName, otlpEndpoint string) (func(context.Context) error, error) {
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(serviceName),
	))
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	if otlpEndpoint != "" {
		exporter, err = otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(otlpEndpoint))
		if err != nil {
			return nil, fmt.Errorf("create otlp exporter: %w", err)
		}
	} else {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("create stdout exporter: %w", err)
		}
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))),
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(1*time.Second)),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}
