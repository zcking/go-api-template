package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
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
	ServiceName        string
	WorkspaceURL       string
	Token              string
	UCTableName        string
	UCMetricsTableName string
	ShutdownTimeout    time.Duration
}

// InitOTel initializes OpenTelemetry with Databricks Zerobus exporter.
// If Databricks configuration is not provided, it returns nil and sets up a no-op TracerProvider.
// The app will continue to run without exporting traces.
func InitOTel(ctx context.Context, config OTelConfig) (*sdktrace.TracerProvider, error) {
	// Set default service name if not provided
	if config.ServiceName == "" {
		config.ServiceName = "go-api-template"
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 30 * time.Second
	}

	// Check if Databricks configuration is provided
	if config.WorkspaceURL == "" || config.Token == "" || config.UCTableName == "" {
		missing := []string{}
		if config.WorkspaceURL == "" {
			missing = append(missing, "DATABRICKS_WORKSPACE_URL")
		}
		if config.Token == "" {
			missing = append(missing, "DATABRICKS_TOKEN")
		}
		if config.UCTableName == "" {
			missing = append(missing, "DATABRICKS_UC_TABLE_NAME")
		}
		slog.Warn("OpenTelemetry Databricks exporter not configured",
			"missing_vars", strings.Join(missing, ", "),
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

	// Normalize workspace URL - strip protocol if present
	workspaceURL := strings.TrimSpace(config.WorkspaceURL)
	if strings.HasPrefix(workspaceURL, "https://") {
		workspaceURL = strings.TrimPrefix(workspaceURL, "https://")
	} else if strings.HasPrefix(workspaceURL, "http://") {
		workspaceURL = strings.TrimPrefix(workspaceURL, "http://")
	}
	// Remove trailing slash if present
	workspaceURL = strings.TrimSuffix(workspaceURL, "/")

	// Build the endpoint URL using net/url for proper construction
	endpointURL := &url.URL{
		Scheme: "https",
		Host:   workspaceURL,
		Path:   "/api/2.0/otel/v1/traces",
	}
	endpoint := endpointURL.String()

	// Create OTLP HTTP exporter with Databricks-specific headers
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(endpoint),
		otlptracehttp.WithHeaders(map[string]string{
			"content-type":               "application/x-protobuf",
			"X-Databricks-UC-Table-Name": config.UCTableName,
			"Authorization":              fmt.Sprintf("Bearer %s", config.Token),
		}),
		// Use HTTP/protobuf protocol (default for otlptracehttp)
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
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

// InitOTelMetrics initializes OpenTelemetry metrics with Databricks Zerobus exporter.
// If Databricks configuration is not provided, it returns nil and sets up a no-op MeterProvider.
// The app will continue to run without exporting metrics.
func InitOTelMetrics(ctx context.Context, config OTelConfig) (*metric.MeterProvider, error) {
	// Set default service name if not provided
	if config.ServiceName == "" {
		config.ServiceName = "go-api-template"
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 30 * time.Second
	}

	// Check if Databricks configuration is provided
	if config.WorkspaceURL == "" || config.Token == "" || config.UCMetricsTableName == "" {
		missing := []string{}
		if config.WorkspaceURL == "" {
			missing = append(missing, "DATABRICKS_WORKSPACE_URL")
		}
		if config.Token == "" {
			missing = append(missing, "DATABRICKS_TOKEN")
		}
		if config.UCMetricsTableName == "" {
			missing = append(missing, "DATABRICKS_UC_METRICS_TABLE_NAME")
		}
		slog.Warn("OpenTelemetry Databricks metrics exporter not configured",
			"missing_vars", strings.Join(missing, ", "),
			"message", "Application will continue without metrics export. Metrics will be collected but not exported.")

		// Set up a no-op MeterProvider so instrumentation still works
		// Create a MeterProvider with a manual reader that discards all metrics
		noopMeterProvider := metric.NewMeterProvider(
			metric.WithReader(metric.NewManualReader()),
		)
		otel.SetMeterProvider(noopMeterProvider)

		return nil, nil
	}

	// Normalize workspace URL - strip protocol if present
	workspaceURL := strings.TrimSpace(config.WorkspaceURL)
	if strings.HasPrefix(workspaceURL, "https://") {
		workspaceURL = strings.TrimPrefix(workspaceURL, "https://")
	} else if strings.HasPrefix(workspaceURL, "http://") {
		workspaceURL = strings.TrimPrefix(workspaceURL, "http://")
	}
	// Remove trailing slash if present
	workspaceURL = strings.TrimSuffix(workspaceURL, "/")

	// Build the endpoint URL using net/url for proper construction
	endpointURL := &url.URL{
		Scheme: "https",
		Host:   workspaceURL,
		Path:   "/api/2.0/otel/v1/metrics",
	}
	endpoint := endpointURL.String()

	// Create OTLP HTTP metrics exporter with Databricks-specific headers
	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpointURL(endpoint),
		otlpmetrichttp.WithHeaders(map[string]string{
			"content-type":               "application/x-protobuf",
			"X-Databricks-UC-Table-Name": config.UCMetricsTableName,
			"Authorization":              fmt.Sprintf("Bearer %s", config.Token),
		}),
		// Use HTTP/protobuf protocol (default for otlpmetrichttp)
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metrics exporter: %w", err)
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
