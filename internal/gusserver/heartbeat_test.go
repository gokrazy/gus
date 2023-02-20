package gusserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestHeartbeat(t *testing.T) {
	testDBs := testDatabases()

	for _, tc := range testDBs {
		t.Run(tc.databaseType, func(t *testing.T) {
			ts := newTestServer(t, tc.databaseType)

			// Ensure the heartbeats and machines tables are empty when a fresh
			// server starts
			ts.ensureEmpty(t, "heartbeats")
			ts.ensureEmpty(t, "machines")

			client := ts.Client()
			b, err := json.Marshal(heartbeatRequest{
				MachineID: "scan2drive",
			})
			req, err := http.NewRequest("POST", ts.URL()+"/api/v1/heartbeat", bytes.NewReader(b))
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
			{
				want := []map[string]any{
					{"machine_id": "scan2drive"},
				}
				q := "SELECT machine_id FROM heartbeats"
				if diff := ts.diffQuery(t, want, q); diff != "" {
					t.Errorf("heartbeats table: unexpected diff (-want +got):\n%s", diff)
				}
			}

			// Ensure the machines table has a corresponding entry now
			{
				want := []map[string]any{
					{"machine_id": "scan2drive"},
				}
				q := "SELECT machine_id FROM machines"
				if diff := ts.diffQuery(t, want, q); diff != "" {
					t.Errorf("heartbeats table: unexpected diff (-want +got):\n%s", diff)
				}
			}
		})
	}
}
