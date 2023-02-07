package gusserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type updateRequest struct {
	MachineID string `json:"machine_id"`
}

type updateResponse struct {
	SBOMHash     string `json:"sbom_hash"`
	RegistryType string `json:"registry_type"`
	DownloadLink string `json:"download_link"`
}

func (s *server) update(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "POST" {
		return httpError(http.StatusBadRequest, fmt.Errorf("invalid method (expected POST)"))
	}
	var req updateRequest
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

	rows, err := s.queries.selectDesired.QueryContext(r.Context(), req.MachineID)
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

	b, err = json.Marshal(&updateResponse{
		SBOMHash:     d.DesiredImage,
		RegistryType: d.RegistryType,
		DownloadLink: d.DownloadLink,
	})
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
	return nil
}
