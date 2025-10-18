package internal

import (
	"context"
	"database/sql"
	"log"

	_ "github.com/marcboeker/go-duckdb"
	userspb "github.com/zcking/go-api-template/gen/go/users/v1"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(databaseLocation string) (*Database, error) {
	log.Printf("setting up database at %s...", databaseLocation)
	db, err := sql.Open("duckdb", databaseLocation)
	if err != nil {
		return nil, err
	}

	ddb := &Database{
		db: db,
	}
	return ddb, ddb.setup()
}

func (d *Database) setup() error {
	_, err := d.db.Exec("CREATE SEQUENCE IF NOT EXISTS seq_users_id START 1;")
	if err != nil {
		return err
	}

	_, err = d.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY DEFAULT nextval('seq_users_id'),
			email TEXT NOT NULL,
			name TEXT NOT NULL
		);
	`)
	if err != nil {
		return err
	}

	return nil
}

func (d *Database) Close() error {
	log.Println("shutting down database connection...")
	if _, err := d.db.Exec("CHECKPOINT"); err != nil {
		return err
	}

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
	row := d.db.QueryRowContext(ctx, "INSERT INTO users (email, name) VALUES (?, ?) RETURNING (id);", req.GetEmail(), req.GetName())
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
