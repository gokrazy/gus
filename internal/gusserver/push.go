package gusserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/renameio/v2"
)

type pushResponse struct {
	DownloadLink string `json:"download_link"`
}

// For local testing, use:
//
//	% gok -i gokrazy overwrite --gaf /tmp/gokrazy.gaf
//	% gok -i gokrazy push --gaf /tmp/gokrazy.gaf --server http://localhost:8655
func (s *server) push(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "PUT" {
		return httpError(http.StatusBadRequest, fmt.Errorf("invalid method (expected PUT)"))
	}

	if s.cfg.imageDir == "" {
		return httpError(http.StatusForbidden, fmt.Errorf("no --image_dir configured on this GUS server"))
	}

	if err := os.MkdirAll(s.cfg.imageDir, 0700); err != nil {
		return err
	}
	timePrefix := time.Now().Format(time.RFC3339)
	dir, err := os.MkdirTemp(s.cfg.imageDir, timePrefix+"-")
	if err != nil {
		return err
	}

	tempDir := filepath.Join(s.cfg.imageDir, "tmp")
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return err
	}
	out, err := renameio.NewPendingFile(filepath.Join(dir, "full.gaf"), renameio.WithTempDir(tempDir))
	if err != nil {
		return err
	}
	defer out.Cleanup()
	if _, err := io.Copy(out, r.Body); err != nil {
		return err
	}
	if err := out.CloseAtomicallyReplace(); err != nil {
		return err
	}

	rel := strings.TrimPrefix(dir, filepath.Clean(s.cfg.imageDir)+"/")
	resp, err := json.Marshal(pushResponse{
		DownloadLink: "/images/" + rel + "/full.gaf",
	})
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(resp)
	return err
}
