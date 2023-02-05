package gusserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHeartbeat(t *testing.T) {
	srv, mux, err := newServer("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	// Ensure the heartbeats table is empty when a fresh server starts
	rows, err := srv.db.Query("SELECT machine_id FROM heartbeats")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatalf("heartbeats table unexpectedly contains entries")
	}

	testsrv := httptest.NewServer(mux)
	client := testsrv.Client()
	b, err := json.Marshal(heartbeatRequest{
		MachineID: "scan2drive",
	})
	req, err := http.NewRequest("POST", testsrv.URL+"/api/v1/heartbeat", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected HTTP status code: got %v, want %v", resp.Status, want)
	}

	// Ensure the heartbeats table has a corresponding entry now
	rows, err = srv.db.Query("SELECT machine_id FROM heartbeats")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatalf("heartbeats table unexpectedly still contains no entries")
	}
	var machineID string
	if err := rows.Scan(&machineID); err != nil {
		t.Fatal(err)
	}
	if got, want := machineID, "scan2drive"; got != want {
		t.Fatalf("heartbeats table entry has unexpected machine_id: got %q, want %q", got, want)
	}
	if rows.Next() {
		t.Fatalf("heartbeats table unexpectedly contains more than one entry")
	}
}
