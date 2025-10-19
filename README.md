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

## Changing Protobuf

You can change the protobuf at [proto/users/v1/users.proto](./proto/users/v1/users.proto). Then use `make generate` to generate all new stubs, which are written to the [gen/](./gen/) directory.
