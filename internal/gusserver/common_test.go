package gusserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

type testServer struct {
	srv     *server
	mux     *http.ServeMux
	httpsrv *httptest.Server
}

func newTestServer(t *testing.T, databaseType string) *testServer {
	srv, mux, err := newServer("txdb/"+databaseType, t.Name(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })

	if err := srv.db.Ping(); err != nil {
		t.Fatalf("unable to reach database %s", databaseType)
	}

	return &testServer{
		srv:     srv,
		mux:     mux,
		httpsrv: httptest.NewServer(mux),
	}
}

func (ts *testServer) Client() *http.Client {
	return ts.httpsrv.Client()
}

func (ts *testServer) URL() string {
	return ts.httpsrv.URL
}

func (ts *testServer) ensureEmpty(t *testing.T, table string) {
	rows, err := ts.srv.db.Query("SELECT * FROM " + table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatalf("%s table unexpectedly contains entries", table)
	}
}
