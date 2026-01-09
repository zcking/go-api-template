# Go API Template

This is a template I created for a simple, clean API implementation using the following tech stack:  

- [Go](https://go.dev/) - programming language of choice
- [gRPC](https://grpc.io) - modern open source high performance Remote Procedure Call (RPC) framework
- [gRPC-gateway](https://github.com/grpc-ecosystem/grpc-gateway) - gRPC to JSON proxy generator
- [buf](https://buf.build/docs/introduction) - Protocol buffers build tool
- [PostgreSQL](https://www.postgresql.org/) - powerful, open source object-relational database system
- [golang-migrate](https://github.com/golang-migrate/migrate) - database migration tool

Everything in this list are technology choices I would consider very standard/common and versatile to create any modern API. PostgreSQL provides a robust, production-ready database solution that scales well from development to production environments.

---

## Getting Started: Docker Compose

To build and run the application with PostgreSQL using Docker Compose:

```shell
make compose/up
```

This will:
- Start a PostgreSQL database container
- Build and start the API service
- Automatically run database migrations
- Make the API available on ports 8080 (gRPC) and 8081 (REST)

You should see an output like the following:  

```
docker-compose up --build
Building api
...
Starting go-api-template-postgres ... done
Starting go-api-template-api ... done
Attaching to go-api-template-postgres, go-api-template-api
go-api-template-api | 2024/06/06 01:24:05 Database migrations completed successfully
go-api-template-api | 2024/06/06 01:24:05 setting up database connection to postgres:5432/go_api_template...
go-api-template-api | 2024/06/06 01:24:05 gRPC Gateway listening on http://0.0.0.0:8081
```

You can call the REST API to create a user like so:  

```shell
curl --location 'http://localhost:8081/api/v1/users' \
--header 'Content-Type: application/json' \
--header 'Accept: application/json' \
--data-raw '{
  "name": "John Doe",
  "email": "jdoe@userapi.com"
}'
```

And list users with:  

```shell
curl --location 'http://localhost:8081/api/v1/users' \
--header 'Accept: application/json'
```

## Environment Variables

The application supports the following environment variables for database configuration:

- `DB_HOST` - Database host (default: localhost)
- `DB_PORT` - Database port (default: 5432)
- `DB_USER` - Database user (default: postgres)
- `DB_PASSWORD` - Database password (default: postgres)
- `DB_NAME` - Database name (default: go_api_template)
- `DB_SSLMODE` - SSL mode (default: disable for local, require for production)

The following environment variables are optional and configure OpenTelemetry trace and metrics export via OTLP. These use standard OpenTelemetry environment variables and work with any OTLP-compatible backend (e.g., Databricks Zerobus Ingest, Honeycomb, Grafana Cloud).

- `OTEL_EXPORTER_OTLP_ENDPOINT` - Base URL for OTLP export. The trace exporter automatically appends "/v1/traces" and the metrics exporter automatically appends "/v1/metrics"
- `OTEL_EXPORTER_OTLP_METRICS_HEADERS` - Headers to include with OTLP metric export requests (format: `key1=value1,key2=value2`)
- `OTEL_EXPORTER_OTLP_TRACES_HEADERS` - Headers to include with OTLP traces export requests (format: `key1=value1,key2=value2`)
- `OTEL_SERVICE_NAME` - Service name for OpenTelemetry resource attributes (default: `go-api-template`)

**Example: Databricks Zerobus Ingest**

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=https://workspace.databricks.com/api/2.0/otel
OTEL_EXPORTER_OTLP_METRICS_HEADERS="Authorization=Bearer dapi...,X-Databricks-UC-Table-Name=catalog.schema.metrics"
OTEL_EXPORTER_OTLP_TRACES_HEADERS="Authorization=Bearer dapi...,X-Databricks-UC-Table-Name=catalog.schema.traces"
OTEL_SERVICE_NAME=go-api-template
```

This will automatically export to:
- Traces: `https://workspace.databricks.com/api/2.0/otel/v1/traces`
- Metrics: `https://workspace.databricks.com/api/2.0/otel/v1/metrics`

If the OTel configurations are not set, the API will continue to run as normal, but not export traces or metrics.

**Example: Splunk Observability Cloud**

```bash
OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=https://ingest.us1.signalfx.com/v2/datapoint/otlp
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=https://ingest.us1.signalfx.com/v2/trace/otlp
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_EXPORTER_OTLP_METRICS_HEADERS="X-SF-TOKEN=YOUR_SPLUNK_OBSERVABILITY_API_KEY"
OTEL_EXPORTER_OTLP_TRACES_HEADERS="X-SF-TOKEN=YOUR_SPLUNK_OBSERVABILITY_API_KEY"
OTEL_SERVICE_NAME=go-api-template
```

## Logging

The application uses Go's native [`slog`](https://pkg.go.dev/log/slog) package for structured logging. All logs are emitted as JSON to stdout/stderr with trace context automatically injected for correlation with OpenTelemetry traces.

### Log Format

Logs are emitted in JSON format with the following structure:

```json
{
  "time": "2026-01-04T12:00:00.123Z",
  "level": "INFO",
  "msg": "user created",
  "trace_id": "80f198ee56343ba864fe8b2a57d3eff7",
  "span_id": "e457b5a2e4d86bd1",
  "user_id": 123
}
```

### Log Collection (External)

Following cloud-native principles, the application emits structured logs to stdout/stderr, and log collection/export is handled by platform infrastructure. This separation of concerns allows you to change log destinations without modifying application code.

### Kubernetes Deployment

For Kubernetes deployments, I recommend deploying [Fluentd](https://www.fluentd.org/) or [Fluent Bit](https://fluentbit.io/) as a DaemonSet to collect logs from all pods and forward them to your observability platform (e.g., DataDog, NewRelic, Databricks Zerobus).

This is a work in progress, but I am currently working on sharing Kubernetes manifests (using Kustomize) and a deployment guide in [`deployment/kustomize/`](deployment/kustomize/README.md); however, please note I only intend to provide these resources for Kubernetes deployments to cover the majority of cloud-native audiences.


### Trace-Log Correlation

Logs automatically include `trace_id` and `span_id` fields when a request is within an OpenTelemetry trace context. This enables correlation between logs and traces in your observability platform:

- **Logs**: Structured JSON with trace context
- **Traces**: Exported via OTLP to your configured backend
- **Correlation**: Use `trace_id` to link logs and traces together

## Metrics

The application automatically collects and exports OpenTelemetry metrics to Databricks Unity Catalog via Zerobus Ingest. Metrics are exported every 60 seconds by default.

### Collected Metrics

The following metrics are automatically collected:

- **Runtime Metrics** (Go runtime):
  - Goroutine count
  - Memory allocation (heap, stack, system)
  - Garbage collection statistics
  - CPU usage

- **HTTP Server Metrics** (gRPC Gateway):
  - Request duration
  - Request size
  - Response size
  - Active requests

- **gRPC Server Metrics**:
  - RPC duration
  - Request/response message counts
  - Status codes

- **Database Metrics** (PostgreSQL):
  - Connection pool statistics (idle, in-use, wait duration)
  - Query duration
  - Query errors

### Metrics Export

Metrics are exported via OTLP/HTTP to the endpoint configured by `OTEL_EXPORTER_OTLP_ENDPOINT` (or `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` if you need separate endpoints for traces and metrics).

### Custom Metrics

To add custom business metrics, obtain a Meter from the global MeterProvider:

```go
import "go.opentelemetry.io/otel"

meter := otel.Meter("service-name")

// Create a counter
counter, _ := meter.Int64Counter("custom.counter")
counter.Add(ctx, 1, attribute.String("key", "value"))

// Create a histogram
histogram, _ := meter.Int64Histogram("custom.duration")
histogram.Record(ctx, durationMs, attribute.String("operation", "create_user"))

// Create a gauge
gauge, _ := meter.Int64ObservableGauge("custom.gauge")
```

Custom metrics will automatically be exported along with the built-in metrics.

## Database Migrations

Database migrations are **automatically run** when the application starts via `make compose/up`. No manual intervention needed for normal development!

### Development Workflow

```shell
# Start everything (PostgreSQL + API + auto-migrations)
make compose/up

# When you need to make schema changes:
# 1. Create a new migration
make migrate/create

# 2. Edit the generated .up.sql and .down.sql files in migrations/

# 3. Restart services to apply the new migration
make compose/down
make compose/up
```

### Bring your own Postgres

To connect to your own Postgres instance instead of the docker-compose service, copy the `.env` file to create `.env.local` and change as needed.

For example, in production, we can use [Lakebase](https://www.databricks.com/product/lakebase) as our postgres database backend.

```shell
cp .env .env.local

# Edit .env.local

# Start only the API
make compose/up/api
```

### Manual Migration Commands

You can also run migrations manually when needed:

```shell
# Run migrations
make migrate/up

# Rollback migrations
make migrate/down

# Check migration version
make migrate/version

# Create a new migration
make migrate/create
```

## Production Deployment

For production deployment, configure the environment variables to connect to your PostgreSQL instance (e.g., Databricks Lakebase):

```shell
export DB_HOST=your-production-host
export DB_PORT=5432
export DB_USER=your-production-user
export DB_PASSWORD=your-production-password
export DB_NAME=your-production-database
export DB_SSLMODE=require
```

### Creating a Postgres Service User

The following queries will create a service user on the postgres server, and grant it
the necessary permissions for the API runtime as well as database migrations:  

```sql
-- Connect to the database first
-- \c go_api_template

-- Create the service user
CREATE USER go_api_service WITH PASSWORD 'your_secure_password_here';

-- Grant connection to the database
GRANT CONNECT ON DATABASE go_api_template TO go_api_service;

-- Grant schema usage and creation privileges (needed for migrations)
GRANT USAGE, CREATE ON SCHEMA public TO go_api_service;

-- Grant privileges on existing tables
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO go_api_service;

-- Grant privileges on existing sequences
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO go_api_service;

-- Grant privileges on future tables (for migrations)
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO go_api_service;

-- Grant privileges on future sequences (for migrations)
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO go_api_service;
```

## Testing

The project includes comprehensive unit tests for the API endpoints using [testify](https://github.com/stretchr/testify) for assertions and mocks. Tests follow Go best practices and cover both happy and unhappy paths.

### Running Tests

```shell
# Run all tests
make test

# Generate coverage report
make test/coverage
```

The `test/coverage` command generates:
- `coverage.out` - Coverage data file
- `coverage.html` - HTML report (open in browser to view)

**Note:** Generated code in the `gen/` folder is automatically excluded from coverage reports.

### Test Structure

Tests are organized by feature using **go-sqlmock** for database mocking:  

- `internal/users/create_user_test.go` - Unit tests for CreateUser endpoint
- `internal/users/list_users_test.go` - Unit tests for ListUsers endpoint
- `internal/users/service_test.go` - Unit tests for service configuration

### Code Organization

The codebase follows a **vertical slice architecture** where each feature owns its complete implementation:

```
internal/
├── otel.go                      # OpenTelemetry setup (shared)
└── users/                       # Users feature domain
    ├── service.go               # Service struct, DB connection, Config
    ├── create_user.go           # CreateUser RPC + database logic
    ├── list_users.go            # ListUsers RPC + database logic
    ├── create_user_test.go      # CreateUser tests
    ├── list_users_test.go       # ListUsers tests
    └── service_test.go          # Service tests
```

**Benefits of this structure:**

- Each endpoint file contains both the handler and its database queries
- Easy to find all code related to a specific feature
- Natural boundaries for splitting into microservices later
- No need for separate repository interfaces or mocks
- Tests use go-sqlmock for fast, isolated database testing

### Testing Philosophy

Tests use **go-sqlmock** to mock database interactions directly:  

- Fast, isolated unit tests without real database connections
- Tests verify both handler logic and SQL queries
- Easy to set up expectations for database behavior
- No need for complex mocking frameworks or interfaces

Test coverage includes:
- ✅ RPC success paths
- ✅ RPC with database errors
- ✅ RPC with invalid input (empty fields)
- ✅ Proper context handling
- ✅ Error propagation
- 🔜 Benchmark tests for performance-critical utils
- 🔜 Fuzzy tests

### Example Test Output

```shell
$ make test
go test ./... -v
=== RUN   TestService_CreateUser
=== RUN   TestService_CreateUser/success_-_valid_user_creation
=== RUN   TestService_CreateUser/error_-_database_error_during_insert
=== RUN   TestService_CreateUser/error_-_scan_error
=== RUN   TestService_ListUsers
=== RUN   TestService_ListUsers/success_-_returns_multiple_users
=== RUN   TestService_ListUsers/success_-_returns_empty_list
=== RUN   TestService_ListUsers/error_-_database_query_fails
=== RUN   TestService_ListUsers/error_-_scan_error
PASS
ok      github.com/zcking/go-api-template/internal/users    0.285s
```

## Changing Protobuf

You can change the protobuf at [proto/users/v1/users.proto](./proto/users/v1/users.proto). Then use `make generate` to generate all new stubs, which are written to the [gen/](./gen/) directory.

```shell
make generate
```

## Adding New Endpoints

To add a new RPC endpoint to the users service:

1. Update the protobuf: `proto/users/v1/users.proto`
2. Run `make generate` to regenerate gRPC stubs
3. Create a new file: `internal/users/<endpoint_name>.go`
4. Implement the RPC handler with its database logic
5. Create tests: `internal/users/<endpoint_name>_test.go`

