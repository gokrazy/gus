package gusserver

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
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
		"printIngestion": func(ingestion time.Time) string {
			return ingestion.Format("2006-01-02 15:04:05")
		},
		"humanizeBytes": func(b uint64) string {
			return humanize.Bytes(b)
		},
	}).
	ParseFS(assets.Assets, "*.tmpl.html"))

var versionBrief = version.ReadBrief()

type image struct {
	imageDir string

	SBOMHash           string
	IngestionTimestamp time.Time
	MachineIDPattern   string
	RegistryType       string
	DownloadURL        string
}

func (i *image) Size() uint64 {
	dl := i.DownloadURL
	if !strings.HasPrefix(dl, "/images/") {
		// TODO: implement fetching the size of remote images using HTTP HEAD
		return 0
	}
	if i.imageDir == "" {
		return 0
	}
	base := strings.TrimPrefix(dl, "/images/")
	if idx := strings.IndexRune(base, '/'); idx > 0 {
		base = base[:idx]
	}
	dirents, err := os.ReadDir(i.imageDir)
	if err != nil {
		log.Print(err)
		return 0
	}
	for _, ent := range dirents {
		if ent.Name() != base {
			continue
		}
		st, err := os.Stat(filepath.Join(i.imageDir, ent.Name(), "disk.gaf"))
		if err != nil {
			log.Print(err)
			return 0
		}
		return uint64(st.Size())
	}
	return 0 // not found
}

func (s *server) index(w http.ResponseWriter, r *http.Request) error {
	if r.URL.Path != "/" && r.URL.Path != "" {
		return httpError(http.StatusNotFound, fmt.Errorf("not found"))
	}
	rows, err := s.queries.selectMachinesForIndex.QueryContext(r.Context())
	if err != nil {
		return err
	}
	defer rows.Close()
	type machine struct {
		MachineID       string
		DesiredImage    sql.NullString
		UpdateState     sql.NullString
		IngestionPolicy sql.NullString

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
			&m.DesiredImage,
			&m.UpdateState,
			&m.IngestionPolicy,
			&m.SBOMHash,
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
	if err := rows.Close(); err != nil {
		return err
	}

	rows, err = s.queries.selectImagesForIndex.QueryContext(r.Context())
	if err != nil {
		return err
	}
	defer rows.Close()
	var images []image
	for rows.Next() {
		i := image{
			imageDir: s.cfg.imageDir,
		}
		err := rows.Scan(
			&i.SBOMHash,
			&i.IngestionTimestamp,
			&i.MachineIDPattern,
			&i.RegistryType,
			&i.DownloadURL)
		if err != nil {
			return err
		}
		images = append(images, i)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, "index.tmpl.html", struct {
		Version  string
		Machines []machine
		Images   []image
	}{
		Version:  versionBrief,
		Machines: machines,
		Images:   images,
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
	mux.Handle("/api/v1/update", handleError(s.update))
	mux.Handle("/api/v1/attempt", handleError(s.attempt))
	if s.cfg.imageDir != "" {
		// TODO: start periodic s.imageDir+"/tmp" cleanup

		// TODO: add a handler that explicitly only allows access to disk.gaf
		// and sets Content-Type: application/zip without sniffing. verify that
		// resume still works.
		mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir(s.cfg.imageDir))))
	}
	return s, mux, nil
}

func (s *server) Close() error {
	return s.db.Close()
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
