package gusserver

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-txdb"
	"github.com/google/go-cmp/cmp"
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

// beware: when calling decodeRows() explicitly, you are responsible for calling rows.Close() when done.
func (ts *testServer) decodeRows(t *testing.T, rows *sql.Rows) []map[string]any {
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}

	var results []map[string]any
	for rows.Next() {
		result := make(map[string]any)
		var dest []any
		for _, ct := range colTypes {
			switch tn := ct.DatabaseTypeName(); tn {
			case "TEXT":
				var val string
				dest = append(dest, &val)
			default:
				t.Fatalf("BUG: column type %q not implemented", tn)
			}
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatal(err)
		}
		for idx, ct := range colTypes {
			name := ct.Name()
			switch ct.DatabaseTypeName() {
			case "TEXT":
				val := dest[idx].(*string)
				result[name] = *val
			}
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return results
}

// diffQuery returns a (-want +got) diff between the want parameter and
// evaluating the specified database query.
func (ts *testServer) diffQuery(t *testing.T, want any, query string, args ...any) string {
	rows, err := ts.srv.db.Query(query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := ts.decodeRows(t, rows)
	return cmp.Diff(want, got)
}
