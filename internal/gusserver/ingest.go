package gusserver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type ingestRequest struct {
	MachineIDPattern string `json:"machine_id_pattern"`
	SBOMHash         string `json:"sbom_hash"`
	RegistryType     string `json:"registry_type"`
	DownloadLink     string `json:"download_link"`
}

func (s *server) ingest(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "POST" {
		return httpError(http.StatusBadRequest, fmt.Errorf("invalid method (expected POST)"))
	}
	var req ingestRequest
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &req); err != nil {
		return err
	}

	if req.MachineIDPattern == "" {
		return httpError(http.StatusBadRequest, fmt.Errorf("machine_id_pattern not set"))
	}

	if req.SBOMHash == "" {
		return httpError(http.StatusBadRequest, fmt.Errorf("sbom_hash not set"))
	}

	if req.RegistryType == "" {
		return httpError(http.StatusBadRequest, fmt.Errorf("registry_type not set"))
	}

	if req.RegistryType != "localdisk" {
		return httpError(http.StatusBadRequest, fmt.Errorf("invalid registry_type: must be one of [localdisk]"))
	}

	if req.DownloadLink == "" {
		return httpError(http.StatusBadRequest, fmt.Errorf("download_link not set"))
	}

	// TODO: validate downloadlink actually exists (at least for registrytype == localdisk)

	now := time.Now()
	_, err = s.queries.insertImage.ExecContext(r.Context(),
		req.SBOMHash,
		now,
		req.MachineIDPattern,
		req.RegistryType,
		req.DownloadLink)
	if err != nil {
		return err
	}

	log.Printf("Ingested image %q (matching %q)", req.SBOMHash, req.MachineIDPattern)

	if err := s.updateDesired(); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{}")
	return nil
}
