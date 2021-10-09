package persistence

import (
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

type SupportedDatabase string

const (
	SupportedDatabaseMySQL    SupportedDatabase = "mysql"
	SupportedDatabaseMsSQL    SupportedDatabase = "mssql"
	SupportedDatabasePostgres SupportedDatabase = "postgres"
	SupportedDatabaseSQLite   SupportedDatabase = "sqlite"
)

type DatabaseFilterCondition func(tx *gorm.DB) *gorm.DB

type DatabaseConfig struct {
	Type SupportedDatabase
	DSN  string // On SQLite: the database file-path, otherwise the dsn (see: https://gorm.io/docs/connecting_to_the_database.html)
}

type Database struct {
	db *gorm.DB
}

func NewDatabase(cfg DatabaseConfig) (*Database, error) {
	d := &Database{}

	var gormDb *gorm.DB
	var err error

	switch cfg.Type {
	case SupportedDatabaseMySQL:
		gormDb, err = gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
		if err != nil {
			return nil, errors.WithMessage(err, "failed to open MySQL database")
		}

		sqlDB, _ := gormDb.DB()
		sqlDB.SetConnMaxLifetime(time.Minute * 5)
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetMaxOpenConns(10)
		err = sqlDB.Ping() // This DOES open a connection if necessary. This makes sure the database is accessible
		if err != nil {
			return nil, errors.WithMessage(err, "failed to ping MySQL database")
		}
	case SupportedDatabaseMsSQL:
		gormDb, err = gorm.Open(sqlserver.Open(cfg.DSN), &gorm.Config{})
		if err != nil {
			return nil, errors.WithMessage(err, "failed to open sqlserver database")
		}
	case SupportedDatabasePostgres:
		gormDb, err = gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
		if err != nil {
			return nil, errors.WithMessage(err, "failed to open Postgres database")
		}
	case SupportedDatabaseSQLite:
		if _, err = os.Stat(filepath.Dir(cfg.DSN)); os.IsNotExist(err) {
			if err = os.MkdirAll(filepath.Dir(cfg.DSN), 0700); err != nil {
				return nil, errors.WithMessage(err, "failed to create database base directory")
			}
		}
		gormDb, err = gorm.Open(sqlite.Open(cfg.DSN), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
		if err != nil {
			return nil, errors.WithMessage(err, "failed to open sqlite database")
		}
	}

	d.db = gormDb

	return d, nil
}
