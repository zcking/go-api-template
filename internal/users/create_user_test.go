package users

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	userspb "github.com/zcking/go-api-template/gen/go/users/v1"
)

func TestService_CreateUser(t *testing.T) {
	tests := []struct {
		name          string
		req           *userspb.CreateUserRequest
		mockSetup     func(sqlmock.Sqlmock)
		expectedUser  *userspb.User
		expectedError bool
		errorContains string
	}{
		{
			name: "success - valid user creation",
			req: &userspb.CreateUserRequest{
				Name:  "John Doe",
				Email: "john.doe@example.com",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id"}).AddRow(1)
				mock.ExpectQuery(`INSERT INTO users \(email, name\) VALUES \(\$1, \$2\) RETURNING id`).
					WithArgs("john.doe@example.com", "John Doe").
					WillReturnRows(rows)
			},
			expectedUser: &userspb.User{
				Id:    1,
				Name:  "John Doe",
				Email: "john.doe@example.com",
			},
			expectedError: false,
		},
		{
			name: "error - database error during insert",
			req: &userspb.CreateUserRequest{
				Name:  "Jane Doe",
				Email: "jane.doe@example.com",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO users \(email, name\) VALUES \(\$1, \$2\) RETURNING id`).
					WithArgs("jane.doe@example.com", "Jane Doe").
					WillReturnError(errors.New("database connection failed"))
			},
			expectedError: true,
			errorContains: "database connection failed",
		},
		{
			name: "error - scan error",
			req: &userspb.CreateUserRequest{
				Name:  "Test User",
				Email: "test@example.com",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id"}).AddRow("invalid")
				mock.ExpectQuery(`INSERT INTO users \(email, name\) VALUES \(\$1, \$2\) RETURNING id`).
					WithArgs("test@example.com", "Test User").
					WillReturnRows(rows)
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			// Setup mock expectations
			tt.mockSetup(mock)

			// Create service with mock DB
			logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
			service := &Service{db: db, logger: logger}
			ctx := context.Background()

			// Execute test
			resp, err := service.CreateUser(ctx, tt.req)

			// Assert results
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.User)
				if tt.expectedUser != nil {
					assert.Equal(t, tt.expectedUser.Id, resp.User.Id)
					assert.Equal(t, tt.expectedUser.Name, resp.User.Name)
					assert.Equal(t, tt.expectedUser.Email, resp.User.Email)
				}
			}

			// Assert all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
