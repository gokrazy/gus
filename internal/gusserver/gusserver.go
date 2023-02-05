package gusserver

import (
	"bytes"
	"database/sql"
	"flag"
	"html/template"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gokrazy/gus/internal/assets"
	"github.com/gokrazy/gus/internal/version"

	// modernc.org/sqlite is a cgo-free SQLite implementation (that uses a
	// custom C compiler targeting Go!).
	//
	// It’s twice as slow compared to using github.com/mattn/go-sqlite3 (SQLite
	// with cgo), but that’s still good enough for what we need:
	// https://datastation.multiprocess.io/blog/2022-05-12-sqlite-in-go-with-and-without-cgo.html
	_ "modernc.org/sqlite"

	_ "github.com/lib/pq"
)

type server struct {
	db       *sql.DB
	queries  *queries
	imageDir string
}

var templates = template.Must(template.New("root").ParseFS(assets.Assets, "*.tmpl.html"))

var versionBrief = version.ReadBrief()

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
	if err := templates.ExecuteTemplate(&buf, "index.tmpl.html", struct {
		Version  string
		Machines []machine
	}{
		Version:  versionBrief,
		Machines: machines,
	}); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = io.Copy(w, &buf)
	return err
}

func newServer(databaseType, databaseSource, imageDir string) (*server, *http.ServeMux, error) {
	log.Printf("using database: %s", databaseType)

	db, err := sql.Open(databaseType, databaseSource)
	if err != nil {
		return nil, nil, err
	}

	queries, err := initDatabase(db, databaseType)
	if err != nil {
		return nil, nil, err
	}

	s := &server{
		db:       db,
		queries:  queries,
		imageDir: imageDir,
	}
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets.Assets))))
	mux.Handle("/", handleError(s.index))
	mux.Handle("/api/v1/heartbeat", handleError(s.heartbeat))
	mux.Handle("/api/v1/push", handleError(s.push))
	if s.imageDir != "" {
		// TODO: start periodic s.imageDir+"/tmp" cleanup

		// TODO: add a handler that explicitly only allows access to full.gaf
		// and sets Content-Type: application/zip without sniffing. verify that
		// resume still works.
		mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir(s.imageDir))))
	}
	return s, mux, nil
}

func Main() error {
	var (
		listen         = flag.String("listen", "localhost:8655", "[host]:port listen address")
		databaseType   = flag.String("database_type", "sqlite", "can be one of: sqlite, postgres")
		databaseSource = flag.String("database_source", ":memory:", "database source for GUS internal state. can be :memory: (default. stores state in memory), directory path (sqlite) or an connection DSN (postgres. reference: https://pkg.go.dev/github.com/lib/pq#hdr-Connection_String_Parameters)")
		imageDir       = flag.String("image_dir", "", "if non-empty, a directory on disk in which to storage gokrazy disk images (consuming dozens to hundreds of megabytes each)")
	)
	flag.Parse()

	if *databaseType == "sqlite" && *databaseSource != ":memory:" {
		*databaseSource = filepath.Join(*databaseSource, "gus.db"+"?mode=rwc")
	}

	_, mux, err := newServer(*databaseType, *databaseSource, *imageDir)
	if err != nil {
		return err
	}
	log.Printf("GUS server listening on %s", *listen)
	return http.ListenAndServe(*listen, mux)
}
