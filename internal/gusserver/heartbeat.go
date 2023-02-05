package gusserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

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
	// TODO: store remote ip address as well
	now := time.Now()
	if _, err := s.queries.insertHeartbeat.ExecContext(r.Context(), req.MachineID, now, req.SBOMHash, sbom); err != nil {
		return err
	}
	// TODO: insert into machines, too
	// TODO: update desired image
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{}")
	return nil
}
