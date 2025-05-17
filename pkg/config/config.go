package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// DBConfig holds the database connection configuration
type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// ConnectionString returns a PostgreSQL connection string
func (c *DBConfig) ConnectionString() string {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "require" // Default to require instead of prefer
	}

	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, sslMode)
}

// PgDumpConnectionString returns a connection string for pg_dump
func (c *DBConfig) PgDumpConnectionString() string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

// LoadSourceConfig loads source database configuration from flags or environment variables
func LoadSourceConfig(host string, port int, user string, password string, database string, sslMode string) (*DBConfig, error) {
	config := &DBConfig{}
	// Try to get values from command-line arguments first
	config.Host = host
	config.Port = port
	config.User = user
	config.Password = password
	config.Database = database
	config.SSLMode = sslMode

	// If not provided via command-line, try environment variables
	if config.Host == "" {
		config.Host = os.Getenv("SOURCE_DB_HOST")
	}
	if config.Port == 0 {
		portStr := os.Getenv("SOURCE_DB_PORT")
		if portStr != "" {
			var err error
			config.Port, err = strconv.Atoi(portStr)
			if err != nil {
				return nil, fmt.Errorf("invalid SOURCE_DB_PORT: %v", err)
			}
		}
	}
	if config.User == "" {
		config.User = os.Getenv("SOURCE_DB_USER")
	}
	if config.Password == "" {
		config.Password = os.Getenv("SOURCE_DB_PASSWORD")
	}
	if config.Database == "" {
		config.Database = os.Getenv("SOURCE_DB_NAME")
	}
	if config.SSLMode == "" {
		config.SSLMode = os.Getenv("SOURCE_DB_SSLMODE")
	}

	// Set default port if still not provided
	if config.Port == 0 {
		config.Port = 5432
	}

	// Set default SSL mode if still not provided
	if config.SSLMode == "" {
		config.SSLMode = "require"
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// LoadTargetConfig loads target database configuration from flags or environment variables
func LoadTargetConfig(host string, port int, user string, password string, database string, sslMode string) (*DBConfig, error) {
	config := &DBConfig{}
	// Try to get values from command-line arguments first
	config.Host = host
	config.Port = port
	config.User = user
	config.Password = password
	config.Database = database
	config.SSLMode = sslMode

	// If not provided via command-line, try environment variables
	if config.Host == "" {
		config.Host = os.Getenv("TARGET_DB_HOST")
	}
	if config.Port == 0 {
		portStr := os.Getenv("TARGET_DB_PORT")
		if portStr != "" {
			var err error
			config.Port, err = strconv.Atoi(portStr)
			if err != nil {
				return nil, fmt.Errorf("invalid TARGET_DB_PORT: %v", err)
			}
		}
	}
	if config.User == "" {
		config.User = os.Getenv("TARGET_DB_USER")
	}
	if config.Password == "" {
		config.Password = os.Getenv("TARGET_DB_PASSWORD")
	}
	if config.Database == "" {
		config.Database = os.Getenv("TARGET_DB_NAME")
	}
	if config.SSLMode == "" {
		config.SSLMode = os.Getenv("TARGET_DB_SSLMODE")
	}

	// Set default port if still not provided
	if config.Port == 0 {
		config.Port = 5432
	}

	// Set default SSL mode if still not provided
	if config.SSLMode == "" {
		config.SSLMode = "require"
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// validateConfig validates that all required configuration parameters are provided
func validateConfig(config *DBConfig) error {
	if config.Host == "" {
		return errors.New("database host is required")
	}
	if config.User == "" {
		return errors.New("database user is required")
	}
	if config.Password == "" {
		return errors.New("database password is required")
	}
	if config.Database == "" {
		return errors.New("database name is required")
	}
	return nil
}