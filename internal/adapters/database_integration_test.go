//go:build integration

package adapters

import (
	"database/sql"
	"fmt"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"testing"
)

func tempSqliteDb(t *testing.T) *gorm.DB {

	// github.com/mattn/go-sqlite3
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func Test_sqlRepo_migrate(t *testing.T) {
	db := tempSqliteDb(t)

	r := SqlRepo{db: db}

	err := r.migrate()
	assert.NoError(t, err)

	// check result
	var sqlStatement []sql.NullString
	db.Raw("SELECT sql FROM sqlite_master").Find(&sqlStatement)
	fmt.Println("Table Schemas:")
	for _, stm := range sqlStatement {
		if stm.Valid {
			fmt.Println(stm.String)
		}
	}
}
