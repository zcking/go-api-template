package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace/noop"
)

// OTelConfig holds configuration for OpenTelemetry
type OTelConfig struct {
	ServiceName     string
	ShutdownTimeout time.Duration
}

// InitOTel initializes OpenTelemetry with OTLP exporter.
// Configuration is read from standard OpenTelemetry environment variables:
// - OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_TRACES_ENDPOINT
// - OTEL_EXPORTER_OTLP_HEADERS or OTEL_EXPORTER_OTLP_TRACES_HEADERS
// If no endpoint is configured, it returns nil and sets up a no-op TracerProvider.
// The app will continue to run without exporting traces.
func InitOTel(ctx context.Context, config OTelConfig) (*sdktrace.TracerProvider, error) {
	// Set default service name if not provided
	if config.ServiceName == "" {
		config.ServiceName = "go-api-template"
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 30 * time.Second
	}

	// Create OTLP HTTP exporter - reads configuration from environment variables automatically
	// OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_TRACES_ENDPOINT
	// OTEL_EXPORTER_OTLP_HEADERS or OTEL_EXPORTER_OTLP_TRACES_HEADERS
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		// If exporter creation fails (e.g., invalid endpoint config), fall back to no-op
		slog.Warn("OpenTelemetry trace exporter not configured or failed to initialize",
			"error", err,
			"message", "Application will continue without trace export. Traces will be collected but not exported.")

		// Set up a no-op TracerProvider so instrumentation still works
		noopTracerProvider := noop.NewTracerProvider()
		otel.SetTracerProvider(noopTracerProvider)

		// Set global propagator for context propagation (still useful even without export)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))

		return nil, nil
	}

	// Create resource with service information
	res, err := createResource(ctx, config.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global TracerProvider
	otel.SetTracerProvider(tp)

	// Set global propagator for context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

// ShutdownOTel gracefully shuts down the TracerProvider
func ShutdownOTel(ctx context.Context, tp *sdktrace.TracerProvider, timeout time.Duration) error {
	if tp == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return tp.Shutdown(shutdownCtx)
}

// InitOTelMetrics initializes OpenTelemetry metrics with OTLP exporter.
// Configuration is read from standard OpenTelemetry environment variables:
// - OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_METRICS_ENDPOINT
// - OTEL_EXPORTER_OTLP_HEADERS or OTEL_EXPORTER_OTLP_METRICS_HEADERS
// If no endpoint is configured, it returns nil and sets up a no-op MeterProvider.
// The app will continue to run without exporting metrics.
func InitOTelMetrics(ctx context.Context, config OTelConfig) (*metric.MeterProvider, error) {
	// Set default service name if not provided
	if config.ServiceName == "" {
		config.ServiceName = "go-api-template"
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 30 * time.Second
	}

	// Create OTLP HTTP metrics exporter - reads configuration from environment variables automatically
	// OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_METRICS_ENDPOINT
	// OTEL_EXPORTER_OTLP_HEADERS or OTEL_EXPORTER_OTLP_METRICS_HEADERS
	exporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		// If exporter creation fails (e.g., invalid endpoint config), fall back to no-op
		slog.Warn("OpenTelemetry metrics exporter not configured or failed to initialize",
			"error", err,
			"message", "Application will continue without metrics export. Metrics will be collected but not exported.")

		// Set up a no-op MeterProvider so instrumentation still works
		// Create a MeterProvider with a manual reader that discards all metrics
		noopMeterProvider := metric.NewMeterProvider(
			metric.WithReader(metric.NewManualReader()),
		)
		otel.SetMeterProvider(noopMeterProvider)

		return nil, nil
	}

	// Create resource with service information
	res, err := createResource(ctx, config.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create MeterProvider with periodic reader (default 60s export interval)
	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(exporter)),
	)

	// Set global MeterProvider
	otel.SetMeterProvider(mp)

	return mp, nil
}

// ShutdownOTelMetrics gracefully shuts down the MeterProvider
func ShutdownOTelMetrics(ctx context.Context, mp *metric.MeterProvider, timeout time.Duration) error {
	if mp == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return mp.Shutdown(shutdownCtx)
}

// createResource creates a resource with service information
// This is shared between traces and metrics
func createResource(ctx context.Context, serviceName string) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(getVersion()),
		),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
	)
}

// getVersion returns the service version, defaulting to "unknown"
func getVersion() string {
	if version := os.Getenv("SERVICE_VERSION"); version != "" {
		return version
	}
	return "unknown"
}
