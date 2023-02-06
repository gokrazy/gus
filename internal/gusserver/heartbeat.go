package gusserver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

type heartbeatRequest struct {
	MachineID     string          `json:"machine_id"`
	SBOMHash      string          `json:"sbom_hash"`
	SBOM          json.RawMessage `json:"sbom"`
	HumanReadable struct {
		Kernel string `json:"kernel"`
		Model  string `json:"model"`
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

	remoteAddr := r.RemoteAddr
	if s.cfg.reverseProxied {
		if ff := r.Header.Get("X-Forwarded-For"); ff != "" {
			// We use net.JoinHostPort so that we do not need to distinguish
			// between the two cases (X-Forwarded-For, without a port, and
			// RemoteAddr, with a port) in the code below.
			remoteAddr = net.JoinHostPort(ff, "0")
		}
	}
	addr, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return fmt.Errorf("invalid X-Forwarded-For or r.RemoteAddr (%q)", remoteAddr)
	}
	names, err := net.LookupAddr(addr)
	if err != nil {
		// TODO: rate limit this message
		log.Printf("could not look up address %q: %v (ignoring)", addr, err)
	} else {
		addr = names[0]
	}

	_, err = s.queries.insertHeartbeat.ExecContext(r.Context(),
		req.MachineID,
		now,
		req.SBOMHash,
		sbom,
		req.HumanReadable.Kernel,
		req.HumanReadable.Model,
		addr)
	if err != nil {
		return err
	}

	if _, err := s.queries.insertMachine.ExecContext(r.Context(), req.MachineID); err != nil {
		return err
	}

	// TODO(optimization): only update the desired image for machine req.MachineID
	if err := s.updateDesired(); err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{}")
	return nil
}
