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
	"strings"
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

type config struct {
	imageDir       string
	reverseProxied bool
}

type server struct {
	db      *sql.DB
	queries *queries
	cfg     *config
}

var templates = template.Must(template.New("root").
	Funcs(template.FuncMap{
		"printSBOMHash": func(sbomHash string) string {
			const sbomHashLen = 10
			if len(sbomHash) < sbomHashLen {
				return sbomHash
			}
			return sbomHash[:sbomHashLen]
		},
		"printHeartbeat": func(heartbeat time.Time) string {
			if time.Since(heartbeat) < 24*time.Hour {
				return heartbeat.Format("15:04:05")
			}
			return heartbeat.Format("2006-01-02 15:04:05")
		},
		"URLForIP": func(ip string) string {
			if strings.ContainsRune(ip, ':') {
				return "http://[" + ip + "]"
			}
			return "http://" + ip
		},
	}).
	ParseFS(assets.Assets, "*.tmpl.html"))

var versionBrief = version.ReadBrief()

func (s *server) index(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.queries.selectMachinesForIndex.QueryContext(r.Context())
	if err != nil {
		return err
	}
	defer rows.Close()
	type machine struct {
		MachineID       string
		SBOMHash        string
		DesiredSBOMHash string
		LastHeartbeat   time.Time
		Model           string
		RemoteIP        string
		Hostname        string
	}
	var machines []machine
	for rows.Next() {
		var m machine
		err := rows.Scan(
			&m.MachineID,
			&m.SBOMHash,
			// TODO: desired
			&m.LastHeartbeat,
			&m.Model,
			&m.RemoteIP,
			&m.Hostname)
		if err != nil {
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

func newServer(databaseType, databaseSource string, cfg *config) (*server, *http.ServeMux, error) {
	log.Printf("using database: %s", databaseType)

	db, err := sql.Open(databaseType, databaseSource)
	if err != nil {
		return nil, nil, err
	}

	queries, err := initDatabase(db, databaseType)
	if err != nil {
		return nil, nil, err
	}

	if cfg == nil {
		cfg = &config{}
	}

	s := &server{
		db:      db,
		queries: queries,
		cfg:     cfg,
	}
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets.Assets))))
	mux.Handle("/", handleError(s.index))
	mux.Handle("/api/v1/heartbeat", handleError(s.heartbeat))
	mux.Handle("/api/v1/push", handleError(s.push))
	mux.Handle("/api/v1/ingest", handleError(s.ingest))
	if s.cfg.imageDir != "" {
		// TODO: start periodic s.imageDir+"/tmp" cleanup

		// TODO: add a handler that explicitly only allows access to full.gaf
		// and sets Content-Type: application/zip without sniffing. verify that
		// resume still works.
		mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir(s.cfg.imageDir))))
	}
	return s, mux, nil
}

func Main() error {
	var (
		listen         = flag.String("listen", "localhost:8655", "[host]:port listen address")
		databaseType   = flag.String("database_type", "sqlite", "can be one of: sqlite, postgres")
		databaseSource = flag.String("database_source", ":memory:", "database source for GUS internal state. can be :memory: (default. stores state in memory), directory path (sqlite) or an connection DSN (postgres. reference: https://pkg.go.dev/github.com/lib/pq#hdr-Connection_String_Parameters)")
		imageDir       = flag.String("image_dir", "", "if non-empty, a directory on disk in which to storage gokrazy disk images (consuming dozens to hundreds of megabytes each)")
		reverseProxied = flag.Bool("reverse_proxied", false, "use X-Forwarded-For header instead of remote address")
	)
	flag.Parse()

	if *databaseType == "sqlite" && *databaseSource != ":memory:" {
		*databaseSource = filepath.Join(*databaseSource, "gus.db"+"?mode=rwc")
	}

	_, mux, err := newServer(*databaseType, *databaseSource, &config{
		imageDir:       *imageDir,
		reverseProxied: *reverseProxied,
	})
	if err != nil {
		return err
	}
	log.Printf("GUS server listening on %s", *listen)
	return http.ListenAndServe(*listen, mux)
}
