package gusserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHeartbeat(t *testing.T) {
	testDBs := testDatabases()

	for _, tc := range testDBs {
		t.Run(tc.databaseType, func(t *testing.T) {
			srv, mux, err := newServer(tc.databaseType, tc.databaseSource, "")
			if err != nil {
				t.Fatal(err)
			}

			if err := srv.db.Ping(); err != nil {
				t.Fatalf("unable to reach database %s", tc.databaseType)
			}

			// Ensure the heartbeats and machines tables are empty when a fresh
			// server starts
			ensureEmpty(t, srv, "heartbeats")
			ensureEmpty(t, srv, "machines")

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
			rows, err := srv.db.Query("SELECT machine_id FROM heartbeats")
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

			// Ensure the machines table has a corresponding entry now
			rows, err = srv.db.Query("SELECT machine_id FROM machines")
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			if !rows.Next() {
				t.Fatalf("machines table unexpectedly still contains no entries")
			}
			if err := rows.Scan(&machineID); err != nil {
				t.Fatal(err)
			}
			if got, want := machineID, "scan2drive"; got != want {
				t.Fatalf("machines table entry has unexpected machine_id: got %q, want %q", got, want)
			}
			if rows.Next() {
				t.Fatalf("machines table unexpectedly contains more than one entry")
			}
		})
	}
}
