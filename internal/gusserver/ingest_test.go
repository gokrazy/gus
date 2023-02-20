package gusserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestIngest(t *testing.T) {
	testDBs := testDatabases()

	for _, tc := range testDBs {
		t.Run(tc.databaseType, func(t *testing.T) {
			ts := newTestServer(t, tc.databaseType)

			// Ensure the images table is empty when a fresh server starts
			ts.ensureEmpty(t, "images")

			client := ts.Client()

			// Send a heartbeat to add a machine
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

			b, err = json.Marshal(ingestRequest{
				MachineIDPattern: "scan2drive",
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

			// Ensure the images table has a corresponding entry now
			{
				want := []map[string]any{
					{"sbom_hash": "abcdefg"},
				}
				q := "SELECT sbom_hash FROM images"
				if diff := ts.diffQuery(t, want, q); diff != "" {
					t.Errorf("heartbeats table: unexpected diff (-want +got):\n%s", diff)
				}
			}

			// Ensure the machines table was updated for the new desired_image
			{
				want := []map[string]any{
					{
						"machine_id":    "scan2drive",
						"desired_image": "abcdefg",
					},
				}
				q := "SELECT machine_id, desired_image FROM machines"
				if diff := ts.diffQuery(t, want, q); diff != "" {
					t.Errorf("heartbeats table: unexpected diff (-want +got):\n%s", diff)
				}
			}
		})
	}
}
