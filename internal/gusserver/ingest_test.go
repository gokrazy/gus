package gusserver

import (
	"context"
	"testing"

	"github.com/antihax/optional"
	"github.com/gokrazy/gokapi/gusapi"
)

func TestIngest(t *testing.T) {
	testDBs := testDatabases()

	for _, tc := range testDBs {
		t.Run(tc.databaseType, func(t *testing.T) {
			ctx := context.Background()
			ts := newTestServer(t, tc.databaseType)
			api := ts.API()

			const machineID = "scan2drive"

			// Ensure the images table is empty when a fresh server starts
			ts.ensureEmpty(t, "images")

			// Send a heartbeat to add a machine
			_, _, err := api.HeartbeatApi.Heartbeat(ctx, &gusapi.HeartbeatApiHeartbeatOpts{
				Body: optional.NewInterface(&gusapi.HeartbeatRequest{
					MachineId: machineID,
				}),
			})
			if err != nil {
				t.Fatal(err)
			}

			_, _, err = api.IngestApi.Ingest(ctx, &gusapi.IngestApiIngestOpts{
				Body: optional.NewInterface(&gusapi.IngestRequest{
					MachineIdPattern: "scan2drive",
					SbomHash:         "abcdefg",
					RegistryType:     "localdisk",
					DownloadLink:     "/doesnotexist/disk.gaf",
				}),
			})
			if err != nil {
				t.Fatal(err)
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
