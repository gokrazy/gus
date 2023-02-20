package gusserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAttempt(t *testing.T) {
	testDBs := testDatabases()

	for _, tc := range testDBs {
		t.Run(tc.databaseType, func(t *testing.T) {
			ts := newTestServer(t, tc.databaseType)

			client := ts.Client()

			const machineID = "scan2drive"

			// Send a heartbeat to add a machine
			b, err := json.Marshal(heartbeatRequest{
				MachineID: machineID,
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

			b, err = json.Marshal(ingestRequest{
				MachineIDPattern: machineID,
				SBOMHash:         "abcdefg",
				RegistryType:     "localdisk",
				DownloadLink:     "/doesnotexist/disk.gaf",
			})
			req, err = http.NewRequest("POST", ts.URL()+"/api/v1/ingest", bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			resp, err = client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := resp.StatusCode, http.StatusOK; got != want {
				t.Fatalf("unexpected HTTP status code: got %v, want %v", resp.Status, want)
			}

			// Ensure the update API returns the new image now
			b, err = json.Marshal(updateRequest{
				MachineID: machineID,
			})
			req, err = http.NewRequest("POST", ts.URL()+"/api/v1/update", bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			resp, err = client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := resp.StatusCode, http.StatusOK; got != want {
				t.Fatalf("unexpected HTTP status code: got %v, want %v", resp.Status, want)
			}
			b, err = io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			var upResp updateResponse
			if err := json.Unmarshal(b, &upResp); err != nil {
				t.Fatal(err)
			}
			want := updateResponse{
				SBOMHash:     "abcdefg",
				RegistryType: "localdisk",
				DownloadLink: "/doesnotexist/disk.gaf",
			}
			if diff := cmp.Diff(want, upResp); diff != "" {
				t.Fatalf("update: diff (-want +got):\n%s", diff)
			}

			// Ensure the attempt API changes the state to attempted
			b, err = json.Marshal(attemptUpdateRequest{
				MachineID: machineID,
				SBOMHash:  upResp.SBOMHash,
			})
			req, err = http.NewRequest("POST", ts.URL()+"/api/v1/attempt", bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			resp, err = client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := resp.StatusCode, http.StatusOK; got != want {
				t.Fatalf("unexpected HTTP status code: got %v, want %v", resp.Status, want)
			}
			b, err = io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}

			{
				want := []map[string]any{
					{"update_state": "attempted"},
				}
				q := "SELECT update_state FROM machines WHERE machine_id = $1"
				if diff := ts.diffQuery(t, want, q, machineID); diff != "" {
					t.Errorf("heartbeats table: unexpected diff (-want +got):\n%s", diff)
				}
			}
		})
	}
}
