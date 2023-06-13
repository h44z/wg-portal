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
	Debug              bool              `yaml:"debug"`
	SlowQueryThreshold time.Duration     `yaml:"slow_query_threshold"` // 0 means no logging of slow queries
	Type               SupportedDatabase `yaml:"type"`
	DSN                string            `yaml:"dsn"` // On SQLite: the database file-path, otherwise the dsn (see: https://gorm.io/docs/connecting_to_the_database.html)
}
