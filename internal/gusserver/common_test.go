package gusserver

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gokrazy/gokapi/gusapi"
	"github.com/google/go-cmp/cmp"
	"github.com/stapelberg/postgrestest"
)

var dbc *postgrestest.DBCreator

func TestMain(m *testing.M) {
	// It is best to specify the PGURL environment variable so that only
	// one PostgreSQL instance is used for all tests.
	pgurl := os.Getenv("PGURL")
	if pgurl == "" {
		// 'go test' was started directly, start one Postgres per process:
		pgt, err := postgrestest.Start(context.Background())
		if err != nil {
			panic(err)
		}
		defer pgt.Cleanup()
		pgurl = pgt.DefaultDatabase()
	}

	var err error
	dbc, err = postgrestest.NewDBCreator(pgurl)
	if err != nil {
		panic(err)
	}

	m.Run()
}

type testDatabase struct {
	databaseType string
}

func testDatabases() []testDatabase {
	return []testDatabase{
		{"sqlite"},
		{"postgres"},
	}
}

type testServer struct {
	srv     *server
	mux     *http.ServeMux
	httpsrv *httptest.Server
}

func newTestServer(t *testing.T, databaseType string) *testServer {
	pgurl := t.Name()
	switch databaseType {
	case "postgres":
		var err error
		start := time.Now()
		pgurl, err = dbc.CreateDatabase(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "CreateDatabase in %v\n", time.Since(start))

	case "sqlite":
		pgurl = ":memory:"

	default:
		t.Fatalf("BUG: unknown database type %q", databaseType)
	}

	srv, mux, err := newServer(databaseType, pgurl, nil)
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

func (ts *testServer) API() *gusapi.APIClient {
	cfg := gusapi.NewConfiguration()
	cfg.BasePath = ts.URL() + "/api/v1"
	return gusapi.NewAPIClient(cfg)
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
