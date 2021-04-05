package common

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type SupportedDatabase string

const (
	SupportedDatabaseMySQL  SupportedDatabase = "mysql"
	SupportedDatabaseSQLite SupportedDatabase = "sqlite"
)

type DatabaseConfig struct {
	Typ      SupportedDatabase `yaml:"typ" envconfig:"DATABASE_TYPE"` //mysql or sqlite
	Host     string            `yaml:"host" envconfig:"DATABASE_HOST"`
	Port     int               `yaml:"port" envconfig:"DATABASE_PORT"`
	Database string            `yaml:"database" envconfig:"DATABASE_NAME"` // On SQLite: the database file-path, otherwise the database name
	User     string            `yaml:"user" envconfig:"DATABASE_USERNAME"`
	Password string            `yaml:"password" envconfig:"DATABASE_PASSWORD"`
}

func GetDatabaseForConfig(cfg *DatabaseConfig) (db *gorm.DB, err error) {
	switch cfg.Typ {
	case SupportedDatabaseSQLite:
		if _, err = os.Stat(filepath.Dir(cfg.Database)); os.IsNotExist(err) {
			if err = os.MkdirAll(filepath.Dir(cfg.Database), 0700); err != nil {
				return
			}
		}
		db, err = gorm.Open(sqlite.Open(cfg.Database), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
		if err != nil {
			return
		}
	case SupportedDatabaseMySQL:
		connectionString := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
		db, err = gorm.Open(mysql.Open(connectionString), &gorm.Config{})
		if err != nil {
			return
		}

		sqlDB, _ := db.DB()
		sqlDB.SetConnMaxLifetime(time.Minute * 5)
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetMaxOpenConns(10)
		err = sqlDB.Ping() // This DOES open a connection if necessary. This makes sure the database is accessible
		if err != nil {
			return nil, errors.Wrap(err, "failed to ping mysql authentication database")
		}
	}

	// Enable Logger (logrus)
	logCfg := logger.Config{
		SlowThreshold: time.Second, // all slower than one second
		Colorful:      false,
		LogLevel:      logger.Silent, // default: log nothing
	}

	if logrus.StandardLogger().GetLevel() == logrus.TraceLevel {
		logCfg.LogLevel = logger.Info
		logCfg.SlowThreshold = 500 * time.Millisecond // all slower than half a second
	}

	db.Config.Logger = logger.New(logrus.StandardLogger(), logCfg)
	return
}

type DatabaseMigrationInfo struct {
	Version string `gorm:"primaryKey"`
	Applied time.Time
}

func MigrateDatabase(db *gorm.DB, version string) error {
	if err := db.AutoMigrate(&DatabaseMigrationInfo{}); err != nil {
		return errors.Wrap(err, "failed to migrate version database")
	}

	newVersion := DatabaseMigrationInfo{
		Version: version,
		Applied: time.Now(),
	}

	existingMigration := DatabaseMigrationInfo{}
	db.Where("version = ?", version).FirstOrInit(&existingMigration)

	if existingMigration.Version == "" {
		lastVersion := DatabaseMigrationInfo{}
		db.Order("applied desc, version desc").FirstOrInit(&lastVersion)

		// TODO: migrate database

		res := db.Create(&newVersion)
		if res.Error != nil {
			return errors.Wrap(res.Error, "failed to write version to database")
		}
	}

	return nil
}
