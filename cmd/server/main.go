package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	gatewayruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	userspb "github.com/zcking/go-api-template/gen/go/users/v1"
	"github.com/zcking/go-api-template/internal"
	"github.com/zcking/go-api-template/internal/users"
)

var (
	dbHost                   = flag.String("db-host", getEnvOrDefault("DB_HOST", "localhost"), "Database host")
	dbPort                   = flag.String("db-port", getEnvOrDefault("DB_PORT", "5432"), "Database port")
	dbUser                   = flag.String("db-user", getEnvOrDefault("DB_USER", "postgres"), "Database user")
	dbPassword               = flag.String("db-password", getEnvOrDefault("DB_PASSWORD", "postgres"), "Database password")
	dbName                   = flag.String("db-name", getEnvOrDefault("DB_NAME", "go_api_template"), "Database name")
	dbSSLMode                = flag.String("db-ssl-mode", getEnvOrDefault("DB_SSLMODE", "disable"), "Database SSL mode")
	otelServiceName          = flag.String("otel-service-name", getEnvOrDefault("OTEL_SERVICE_NAME", "go-api-template"), "OpenTelemetry service name")
	databricksURL            = flag.String("databricks-workspace-url", getEnvOrDefault("DATABRICKS_WORKSPACE_URL", ""), "Databricks workspace URL (e.g., myworkspace.databricks.com)")
	databricksToken          = flag.String("databricks-token", getEnvOrDefault("DATABRICKS_TOKEN", ""), "Databricks authentication token")
	databricksUCTable        = flag.String("databricks-uc-table-name", getEnvOrDefault("DATABRICKS_UC_TABLE_NAME", ""), "Databricks Unity Catalog table name for traces (e.g., catalog.schema.prefix_otel_spans)")
	databricksUCMetricsTable = flag.String("databricks-uc-metrics-table-name", getEnvOrDefault("DATABRICKS_UC_METRICS_TABLE_NAME", ""), "Databricks Unity Catalog table name for metrics (e.g., catalog.schema.prefix_otel_metrics)")
)

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	flag.Parse()

	// Initialize slog with JSON handler
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	})
	// Wrap with trace context handler to inject trace/span IDs
	traceHandler := internal.NewTraceContextHandler(jsonHandler)
	logger := slog.New(traceHandler)
	slog.SetDefault(logger)

	// Initialize OpenTelemetry (optional - will use no-op if Databricks config is missing)
	ctx := context.Background()
	otelConfig := internal.OTelConfig{
		ServiceName:        *otelServiceName,
		WorkspaceURL:       *databricksURL,
		Token:              *databricksToken,
		UCTableName:        *databricksUCTable,
		UCMetricsTableName: *databricksUCMetricsTable,
		ShutdownTimeout:    30 * time.Second,
	}
	tp, err := internal.InitOTel(ctx, otelConfig)
	if err != nil {
		slog.Error("failed to initialize OpenTelemetry traces", "error", err)
		os.Exit(1)
	}
	// Only defer shutdown if TracerProvider was created (tp != nil)
	if tp != nil {
		defer func() {
			if err := internal.ShutdownOTel(ctx, tp, otelConfig.ShutdownTimeout); err != nil {
				slog.Error("failed to shutdown OpenTelemetry traces", "error", err)
			}
		}()
	}

	// Initialize OpenTelemetry metrics (optional - will use no-op if Databricks config is missing)
	mp, err := internal.InitOTelMetrics(ctx, otelConfig)
	if err != nil {
		slog.Error("failed to initialize OpenTelemetry metrics", "error", err)
		os.Exit(1)
	}
	// Only defer shutdown if MeterProvider was created (mp != nil)
	if mp != nil {
		defer func() {
			if err := internal.ShutdownOTelMetrics(ctx, mp, otelConfig.ShutdownTimeout); err != nil {
				slog.Error("failed to shutdown OpenTelemetry metrics", "error", err)
			}
		}()

		// Start runtime metrics collection (goroutines, memory, GC stats, CPU usage)
		// This will automatically export metrics via the MeterProvider
		if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second)); err != nil {
			slog.Warn("failed to start runtime metrics collection", "error", err)
		}
	}

	// Setup database configuration
	dbConfig := users.Config{
		Host:     *dbHost,
		Port:     *dbPort,
		User:     *dbUser,
		Password: *dbPassword,
		DBName:   *dbName,
		SSLMode:  *dbSSLMode,
	}

	// Run database migrations
	if err := runMigrations(dbConfig); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Create a TCP listener for the gRPC server
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		slog.Error("failed to create (gRPC) listener", "error", err)
		os.Exit(1)
	}

	// Create a gRPC server and attach our implementation
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(
			grpc_logging.UnaryServerInterceptor(interceptorLogger(logger)),
		),
		grpc.StreamInterceptor(
			grpc_logging.StreamServerInterceptor(interceptorLogger(logger)),
		),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}
	grpcServer := grpc.NewServer(opts...)
	impl, err := users.NewService(dbConfig, logger)
	if err != nil {
		slog.Error("failed to create users service instance", "error", err)
		os.Exit(1)
	}
	userspb.RegisterUserServiceServer(grpcServer, impl)

	// Serve the gRPC server, in a separate goroutine to avoid blocking
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Now setup the gRPC Gateway, a REST proxy to the gRPC server
	conn, err := grpc.NewClient(
		"0.0.0.0:8080",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		slog.Error("failed to create gRPC client", "error", err)
		os.Exit(1)
	}

	mux := gatewayruntime.NewServeMux()
	err = userspb.RegisterUserServiceHandler(context.Background(), mux, conn)
	if err != nil {
		slog.Error("failed to register gRPC gateway", "error", err)
		os.Exit(1)
	}

	// Wrap HTTP handler with OpenTelemetry instrumentation
	otelHandler := otelhttp.NewHandler(mux, "grpc-gateway",
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
	)

	// Start HTTP server to proxy requests to gRPC server
	gwServer := &http.Server{
		Addr:    ":8081",
		Handler: otelHandler,
	}

	// Catch interrupt signal to gracefully shutdown the server
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-signalChan
		slog.Info("received signal, shutting down servers", "signal", sig.String())

		// Shutdown HTTP server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := gwServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shutdown HTTP server", "error", err)
		}

		// Shutdown gRPC server
		grpcServer.GracefulStop()

		// Close database connection
		if err := impl.Close(); err != nil {
			slog.Error("failed to properly close users service", "error", err)
			os.Exit(1)
		}

		// Shutdown OpenTelemetry traces
		if err := internal.ShutdownOTel(context.Background(), tp, otelConfig.ShutdownTimeout); err != nil {
			slog.Error("failed to shutdown OpenTelemetry traces", "error", err)
		}

		// Shutdown OpenTelemetry metrics
		if mp != nil {
			if err := internal.ShutdownOTelMetrics(context.Background(), mp, otelConfig.ShutdownTimeout); err != nil {
				slog.Error("failed to shutdown OpenTelemetry metrics", "error", err)
			}
		}
	}()

	slog.Info("gRPC Gateway listening", "address", "http://0.0.0.0:8081")
	if err := gwServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}

func runMigrations(config users.Config) error {
	// Build database URL for migrations
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		config.User, config.Password, config.Host, config.Port, config.DBName, config.SSLMode)

	// Create migration instance
	// TODO: ensure migrations are only run once, not on every pod?
	m, err := migrate.New("file://migrations", dbURL)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("Database migrations completed successfully")
	return nil
}

// interceptorLogger adapts slog.Logger to grpc_logging.Logger.
// This code is simple enough to be copied and not imported.
// Based on: https://github.com/grpc-ecosystem/go-grpc-middleware/blob/main/interceptors/logging/examples/slog/example_test.go
func interceptorLogger(l *slog.Logger) grpc_logging.Logger {
	return grpc_logging.LoggerFunc(func(ctx context.Context, lvl grpc_logging.Level, msg string, fields ...any) {
		l.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}
