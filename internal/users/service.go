package users

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/XSAM/otelsql"
	_ "github.com/lib/pq"
	userspb "github.com/zcking/go-api-template/gen/go/users/v1"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

// Service handles gRPC requests for user operations
type Service struct {
	userspb.UnimplementedUserServiceServer
	db     *sql.DB
	logger *slog.Logger
}

// Config holds configuration for database connections
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// NewService creates a new user service with a database connection
func NewService(config Config, logger *slog.Logger) (*Service, error) {
	logger.Info("setting up database connection",
		"host", config.Host,
		"port", config.Port,
		"dbname", config.DBName,
	)

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode)

	// Open database connection with OpenTelemetry instrumentation
	db, err := otelsql.Open("postgres", connStr,
		otelsql.WithAttributes(
			semconv.DBSystemPostgreSQL,
			attribute.String("db.name", config.DBName),
			attribute.String("db.user", config.User),
			attribute.String("net.peer.name", config.Host),
			attribute.String("net.peer.port", config.Port),
		),
	)
	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Service{
		db:     db,
		logger: logger,
	}, nil
}

// Close closes the database connection
func (s *Service) Close() error {
	s.logger.Info("shutting down database connection")
	return s.db.Close()
}
