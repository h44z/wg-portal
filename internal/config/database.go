package config

import "time"

type SupportedDatabase string

const (
	DatabaseMySQL    SupportedDatabase = "mysql"
	DatabaseMsSQL    SupportedDatabase = "mssql"
	DatabasePostgres SupportedDatabase = "postgres"
	DatabaseSQLite   SupportedDatabase = "sqlite"
)

type DatabaseConfig struct {
	// Debug enables logging of all database statements
	Debug bool `yaml:"debug"`
	// SlowQueryThreshold enables logging of slow queries which take longer than the specified duration
	SlowQueryThreshold time.Duration `yaml:"slow_query_threshold"` // 0 means no logging of slow queries
	// Type is the database type. Supported: mysql, mssql, postgres, sqlite
	Type SupportedDatabase `yaml:"type"`
	// DSN is the database connection string.
	// For SQLite, it is the path to the database file.
	// For other databases, it is the connection string, see: https://gorm.io/docs/connecting_to_the_database.html
	DSN string `yaml:"dsn"`
}
