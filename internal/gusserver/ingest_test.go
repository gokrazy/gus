package gusserver

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIngest(t *testing.T) {
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

			// Ensure the images table is empty when a fresh server starts
			ensureEmpty(t, srv, "images")

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
				DownloadLink:     "/doesnotexist/disk.gaf",
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

			// Ensure the images table has a corresponding entry now
			rows, err := srv.db.Query("SELECT sbom_hash FROM images")
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			if !rows.Next() {
				t.Fatalf("images table unexpectedly still contains no entries")
			}
			var sbomHash string
			if err := rows.Scan(&sbomHash); err != nil {
				t.Fatal(err)
			}
			if got, want := sbomHash, "abcdefg"; got != want {
				t.Fatalf("heartbeats table entry has unexpected sbom_hash: got %q, want %q", got, want)
			}
			if rows.Next() {
				t.Fatalf("images table unexpectedly contains more than one entry")
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}

			// Ensure the machines table was updated for the new desired_image
			rows, err = srv.db.Query("SELECT machine_id, desired_image FROM machines")
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			if !rows.Next() {
				t.Fatalf("images table unexpectedly still contains no entries")
			}
			var machineID string
			var desiredImage sql.NullString
			if err := rows.Scan(&machineID, &desiredImage); err != nil {
				t.Fatal(err)
			}
			if got, want := desiredImage.String, "abcdefg"; got != want {
				t.Fatalf("machines table entry has unexpected desired_image: got %q, want %q", got, want)
			}
			if rows.Next() {
				t.Fatalf("machines table unexpectedly contains more than one entry")
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}

		})
	}
}
