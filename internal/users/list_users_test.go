package users

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	userspb "github.com/zcking/go-api-template/gen/go/users/v1"
)

func TestService_ListUsers(t *testing.T) {
	tests := []struct {
		name          string
		mockSetup     func(sqlmock.Sqlmock)
		expectedUsers []*userspb.User
		expectedError bool
		errorContains string
	}{
		{
			name: "success - returns multiple users",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "email", "name"}).
					AddRow(1, "john.doe@example.com", "John Doe").
					AddRow(2, "jane.smith@example.com", "Jane Smith")
				mock.ExpectQuery(`SELECT \* FROM users`).
					WillReturnRows(rows)
			},
			expectedUsers: []*userspb.User{
				{
					Id:    1,
					Name:  "John Doe",
					Email: "john.doe@example.com",
				},
				{
					Id:    2,
					Name:  "Jane Smith",
					Email: "jane.smith@example.com",
				},
			},
			expectedError: false,
		},
		{
			name: "success - returns empty list",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "email", "name"})
				mock.ExpectQuery(`SELECT \* FROM users`).
					WillReturnRows(rows)
			},
			expectedUsers: []*userspb.User{},
			expectedError: false,
		},
		{
			name: "error - database query fails",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM users`).
					WillReturnError(errors.New("failed to query database"))
			},
			expectedError: true,
			errorContains: "failed to query database",
		},
		{
			name: "error - scan error",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "email", "name"}).
					AddRow("invalid", "test@example.com", "Test")
				mock.ExpectQuery(`SELECT \* FROM users`).
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
			service := &Service{db: db}
			ctx := context.Background()

			// Execute test
			resp, err := service.ListUsers(ctx, &userspb.ListUsersRequest{})

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
				assert.Equal(t, len(tt.expectedUsers), len(resp.Users))
				for i, expectedUser := range tt.expectedUsers {
					if i < len(resp.Users) {
						assert.Equal(t, expectedUser.Id, resp.Users[i].Id)
						assert.Equal(t, expectedUser.Name, resp.Users[i].Name)
						assert.Equal(t, expectedUser.Email, resp.Users[i].Email)
					}
				}
			}

			// Assert all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
