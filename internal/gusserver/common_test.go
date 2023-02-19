package gusserver

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-txdb"
)

type testDatabase struct {
	databaseType   string
	databaseSource string
}

var registerOnce sync.Once

func testDatabases() []testDatabase {
	pgHost := os.Getenv("POSTGRES_HOST")
	pgPort := os.Getenv("POSTGRES_PORT")
	pgUser := os.Getenv("POSTGRES_USER")
	pgPassword := os.Getenv("POSTGRES_PASSWORD")
	pgDBName := os.Getenv("POSTGRES_DBNAME")

	dbs := []testDatabase{
		{"sqlite", ":memory:"},
		{"postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			pgHost, pgPort, pgUser, pgPassword, pgDBName)},
	}

	registerOnce.Do(func() {
		for _, db := range dbs {
			if db.databaseType == "sqlite" {
				txdb.Register("txdb/"+db.databaseType, db.databaseType, db.databaseSource, txdb.SavePointOption(nil))
			} else {
				txdb.Register("txdb/"+db.databaseType, db.databaseType, db.databaseSource)
			}
		}
	})

	return dbs
}

func ensureEmpty(t *testing.T, srv *server, table string) {
	rows, err := srv.db.Query("SELECT * FROM " + table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatalf("%s table unexpectedly contains entries", table)
	}
}
