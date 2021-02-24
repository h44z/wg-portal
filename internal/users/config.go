package users

type SupportedDatabase string

const (
	SupportedDatabaseMySQL  SupportedDatabase = "mysql"
	SupportedDatabaseSQLite SupportedDatabase = "sqlite"
)

type Config struct {
	Typ      SupportedDatabase `yaml:"typ" envconfig:"DATABASE_TYPE"` //mysql or sqlite
	Host     string            `yaml:"host" envconfig:"DATABASE_HOST"`
	Port     int               `yaml:"port" envconfig:"DATABASE_PORT"`
	Database string            `yaml:"database" envconfig:"DATABASE_NAME"` // On SQLite: the database file-path, otherwise the database name
	User     string            `yaml:"user" envconfig:"DATABASE_USERNAME"`
	Password string            `yaml:"password" envconfig:"DATABASE_PASSWORD"`
}
