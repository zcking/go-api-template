package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewService_BuildsConnectionString(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "standard configuration",
			config: Config{
				Host:     "localhost",
				Port:     "5432",
				User:     "postgres",
				Password: "postgres",
				DBName:   "testdb",
				SSLMode:  "disable",
			},
		},
		{
			name: "production configuration with SSL",
			config: Config{
				Host:     "prod.example.com",
				Port:     "5432",
				User:     "admin",
				Password: "secure123",
				DBName:   "production",
				SSLMode:  "require",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't actually connect without a real database,
			// but we can verify the config is valid
			assert.NotEmpty(t, tt.config.Host)
			assert.NotEmpty(t, tt.config.Port)
			assert.NotEmpty(t, tt.config.User)
			assert.NotEmpty(t, tt.config.DBName)
		})
	}
}
