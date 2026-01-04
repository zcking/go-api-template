package users

import (
	"context"

	userspb "github.com/zcking/go-api-template/gen/go/users/v1"
)

// ListUsers retrieves all users from the database
func (s *Service) ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	// Query all users
	rows, err := s.db.QueryContext(ctx, "SELECT * FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]*userspb.User, 0)

	// Scan each row into a user
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
