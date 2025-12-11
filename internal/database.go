package internal

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/XSAM/otelsql"
	_ "github.com/lib/pq"
	userspb "github.com/zcking/go-api-template/gen/go/users/v1"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

type Database struct {
	db *sql.DB
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func NewDatabase(config DatabaseConfig) (*Database, error) {
	log.Printf("setting up database connection to %s:%s/%s...", config.Host, config.Port, config.DBName)

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

	ddb := &Database{
		db: db,
	}
	return ddb, nil
}

func (d *Database) Close() error {
	log.Println("shutting down database connection...")
	return d.db.Close()
}

func (d *Database) GetUsers(ctx context.Context) (*userspb.ListUsersResponse, error) {
	rows, err := d.db.QueryContext(ctx, "SELECT * FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]*userspb.User, 0)

	for rows.Next() {
		var user userspb.User
		err := rows.Scan(&user.Id, &user.Email, &user.Name)
		if err != nil {
			return nil, err
		}
		users = append(users, &user)
	}

	return &userspb.ListUsersResponse{Users: users}, nil
}

func (d *Database) CreateUser(ctx context.Context, req *userspb.CreateUserRequest) (*userspb.CreateUserResponse, error) {
	row := d.db.QueryRowContext(ctx, "INSERT INTO users (email, name) VALUES ($1, $2) RETURNING id;", req.GetEmail(), req.GetName())
	if row.Err() != nil {
		return nil, row.Err()
	}

	var userID int64
	if err := row.Scan(&userID); err != nil {
		return nil, err
	}

	user := &userspb.User{
		Id:    userID,
		Email: req.GetEmail(),
		Name:  req.GetName(),
	}

	return &userspb.CreateUserResponse{User: user}, nil
}
