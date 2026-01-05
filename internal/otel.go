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
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace/noop"
)

// OTelConfig holds configuration for OpenTelemetry
type OTelConfig struct {
	ServiceName     string
	WorkspaceURL    string
	Token           string
	UCTableName     string
	ShutdownTimeout time.Duration
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
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(getVersion()),
		),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
	)
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

// getVersion returns the service version, defaulting to "unknown"
func getVersion() string {
	if version := os.Getenv("SERVICE_VERSION"); version != "" {
		return version
	}
	return "unknown"
}
