package app

import (
	"flag"
	"github.com/h44z/wg-portal/internal/config"
	"gorm.io/gorm"
)

func HandleProgramArgs(cfg *config.Config, db *gorm.DB) (exit bool, err error) {
	migrationSource := flag.String("migrateFrom", "", "path to v1 database file or DSN")
	migrationDbType := flag.String("migrateFromType", string(config.DatabaseSQLite), "old database type, either mysql, mssql, postgres or sqlite")
	flag.Parse()

	if *migrationSource != "" {
		err = migrateFromV1(cfg, db, *migrationSource, *migrationDbType)
		exit = true
	}

	return
}
