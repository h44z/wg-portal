package app

import (
	"flag"

	"gorm.io/gorm"

	"github.com/fedor-git/wg-portal-2/internal/config"
)

// HandleProgramArgs handles program arguments and returns true if the program should exit.
func HandleProgramArgs(db *gorm.DB) (exit bool, err error) {
	migrationSource := flag.String("migrateFrom", "", "path to v1 database file or DSN")
	migrationDbType := flag.String("migrateFromType", string(config.DatabaseSQLite),
		"old database type, either mysql, mssql, postgres or sqlite")
	flag.Parse()

	if *migrationSource != "" {
		err = migrateFromV1(db, *migrationSource, *migrationDbType)
		exit = true
	}

	return
}
