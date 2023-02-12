package config

type SupportedDatabase string

const (
	DatabaseMySQL    SupportedDatabase = "mysql"
	DatabaseMsSQL    SupportedDatabase = "mssql"
	DatabasePostgres SupportedDatabase = "postgres"
	DatabaseSQLite   SupportedDatabase = "sqlite"
)

type DatabaseConfig struct {
	Type SupportedDatabase `yaml:"type"`
	DSN  string            `yaml:"dsn"` // On SQLite: the database file-path, otherwise the dsn (see: https://gorm.io/docs/connecting_to_the_database.html)
}
