package gusserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUpdate(t *testing.T) {
	testDBs := testDatabases()

	for _, tc := range testDBs {
		t.Run(tc.databaseType, func(t *testing.T) {
			srv, mux, err := newServer(tc.databaseType, tc.databaseSource, nil)
			if err != nil {
				t.Fatal(err)
			}

			if err := srv.db.Ping(); err != nil {
				t.Fatalf("unable to reach database %s", tc.databaseType)
			}

			testsrv := httptest.NewServer(mux)
			client := testsrv.Client()

			// Send a heartbeat to add a machine
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

			b, err = json.Marshal(ingestRequest{
				MachineIDPattern: "scan2drive",
				SBOMHash:         "abcdefg",
				RegistryType:     "localdisk",
				DownloadLink:     "/doesnotexist/full.gaf",
			})
			req, err = http.NewRequest("POST", testsrv.URL+"/api/v1/ingest", bytes.NewReader(b))
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
				MachineID: "scan2drive",
			})
			req, err = http.NewRequest("POST", testsrv.URL+"/api/v1/update", bytes.NewReader(b))
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
				DownloadLink: "/doesnotexist/full.gaf",
			}
			if diff := cmp.Diff(want, upResp); diff != "" {
				t.Fatalf("update: diff (-want +got):\n%s", diff)
			}
		})
	}
}
