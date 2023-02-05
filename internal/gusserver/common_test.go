package gusserver

import (
	"fmt"
	"os"
	"testing"
)

type testDatabase struct {
	databaseType   string
	databaseSource string
}

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
