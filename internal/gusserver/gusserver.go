package gusserver

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	// modernc.org/sqlite is a cgo-free SQLite implementation (that uses a
	// custom C compiler targeting Go!).
	//
	// It’s twice as slow compared to using github.com/mattn/go-sqlite3 (SQLite
	// with cgo), but that’s still good enough for what we need:
	// https://datastation.multiprocess.io/blog/2022-05-12-sqlite-in-go-with-and-without-cgo.html
	_ "modernc.org/sqlite"
)

type server struct {
	db      *sql.DB
	queries *queries
}

var indexTmpl = template.Must(template.New("").Parse(`<!DOCTYPE html>
<html>
<head>
<title>GUS</title>
</head>
<body>
<h1>Machines</h1>
{{ range $mach := .Machines }}
{{ $mach.MachineID }} - {{ $mach.LastHeartbeat }}
{{ end }}
`))

func (s *server) index(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.queries.selectMachines.QueryContext(r.Context())
	if err != nil {
		return err
	}
	defer rows.Close()
	type machine struct {
		MachineID     string
		LastHeartbeat time.Time
	}
	var machines []machine
	for rows.Next() {
		var m machine
		if err := rows.Scan(&m.MachineID, &m.LastHeartbeat); err != nil {
			return err
		}
		machines = append(machines, m)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := indexTmpl.Execute(&buf, struct {
		Machines []machine
	}{
		Machines: machines,
	}); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = io.Copy(w, &buf)
	return err
}

type heartbeatRequest struct {
	MachineID     string          `json:"machine_id"`
	SBOMHash      string          `json:"sbom_hash"`
	SBOM          json.RawMessage `json:"sbom"`
	HumanReadable struct {
		Kernel   string `json:"kernel"`
		Firmware string `json:"firmware"`
	} `json:"human_readable"`
}

func (s *server) heartbeat(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "POST" {
		return httpError(http.StatusBadRequest, fmt.Errorf("invalid method (expected POST)"))
	}
	var req heartbeatRequest
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &req); err != nil {
		return err
	}

	sbom, err := req.SBOM.MarshalJSON()
	if err != nil {
		return err
	}
	now := time.Now()
	if _, err := s.queries.insertHeartbeat.ExecContext(r.Context(), req.MachineID, now, req.SBOMHash, sbom); err != nil {
		return err
	}
	// TODO: insert into machines, too
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{}")
	return nil
}

func newServer(databaseDir string) (*server, *http.ServeMux, error) {
	db, err := sql.Open("sqlite", databaseDir+"?mode=rwc")
	if err != nil {
		return nil, nil, err
	}

	queries, err := initDatabase(db)
	if err != nil {
		return nil, nil, err
	}

	s := &server{
		db:      db,
		queries: queries,
	}
	mux := http.NewServeMux()
	mux.Handle("/", handleError(s.index))
	mux.Handle("/api/v1/heartbeat", handleError(s.heartbeat))
	return s, mux, nil
}

func Main() error {
	var (
		listen      = flag.String("listen", "localhost:8655", "[host]:port listen address")
		databaseDir = flag.String("database_dir", "/var/lib/gus", "database directory for GUS internal state. the special value :memory: stores state in memory")
	)
	flag.Parse()

	if *databaseDir != ":memory:" {
		*databaseDir = filepath.Join(*databaseDir, "gus.db")
	}
	_, mux, err := newServer(*databaseDir)
	if err != nil {
		return err
	}
	log.Printf("GUS server listening on %s", *listen)
	return http.ListenAndServe(*listen, mux)
}
