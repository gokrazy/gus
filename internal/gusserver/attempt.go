package gusserver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type attemptUpdateRequest struct {
	MachineID string `json:"machine_id"`
	SBOMHash  string `json:"sbom_hash"`
}

type attemptUpdateResponse struct{}

func (s *server) attempt(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	if r.Method != "POST" {
		return httpError(http.StatusBadRequest, fmt.Errorf("invalid method (expected POST)"))
	}
	var req attemptUpdateRequest
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &req); err != nil {
		return err
	}

	if req.MachineID == "" {
		return httpError(http.StatusBadRequest, fmt.Errorf("machine_id not set"))
	}

	if req.SBOMHash == "" {
		return httpError(http.StatusBadRequest, fmt.Errorf("sbom_hash not set"))
	}

	// Verify this AttemptUpdate request is for the desired image of the
	// machine, otherwise perhaps a new image has been ingested between
	// GetUpdate and AttemptUpdate.
	rows, err := s.queries.selectDesired.QueryContext(ctx, req.MachineID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type desired struct {
		DesiredImage string
		RegistryType string
		DownloadLink string
	}
	if !rows.Next() {
		return httpError(http.StatusNotFound, fmt.Errorf("machine_id not found"))
	}
	var d desired
	err = rows.Scan(
		&d.DesiredImage,
		&d.RegistryType,
		&d.DownloadLink)
	if err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	if d.DesiredImage == req.SBOMHash {
		if _, err := s.queries.updateUpdateState.ExecContext(ctx, "attempted", req.MachineID); err != nil {
			return err
		}
	} else {
		log.Printf("device %q is updating to %s (desired: %s)?!", req.MachineID, req.SBOMHash, d.DesiredImage)
	}

	b, err = json.Marshal(&attemptUpdateResponse{})
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
	return nil
}
