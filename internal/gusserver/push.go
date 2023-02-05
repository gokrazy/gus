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
//	% curl --data-binary @/tmp/gokrazy.gaf -X PUT -v http://localhost:8655/api/v1/push
func (s *server) push(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "PUT" {
		return httpError(http.StatusBadRequest, fmt.Errorf("invalid method (expected PUT)"))
	}

	if s.imageDir == "" {
		return httpError(http.StatusForbidden, fmt.Errorf("no --image_dir configured on this GUS server"))
	}

	timePrefix := time.Now().Format(time.RFC3339)
	dir, err := os.MkdirTemp(s.imageDir, timePrefix+"-")
	if err != nil {
		return err
	}

	tempDir := filepath.Join(s.imageDir, "tmp")
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

	rel := strings.TrimPrefix(dir, filepath.Clean(s.imageDir)+"/")
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