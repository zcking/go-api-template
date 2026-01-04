package users

import (
	"context"

	userspb "github.com/zcking/go-api-template/gen/go/users/v1"
)

// CreateUser creates a new user in the database
func (s *Service) CreateUser(ctx context.Context, req *userspb.CreateUserRequest) (*userspb.CreateUserResponse, error) {
	// Insert user into database
	row := s.db.QueryRowContext(ctx, "INSERT INTO users (email, name) VALUES ($1, $2) RETURNING id;", req.GetEmail(), req.GetName())
	if row.Err() != nil {
		return nil, row.Err()
	}

	var userID int64
	if err := row.Scan(&userID); err != nil {
		return nil, err
	}

	// Build response
	user := &userspb.User{
		Id:    userID,
		Email: req.GetEmail(),
		Name:  req.GetName(),
	}

	return &userspb.CreateUserResponse{User: user}, nil
}
