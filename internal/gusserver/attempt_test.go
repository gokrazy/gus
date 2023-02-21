package gusserver

import (
	"context"
	"testing"

	"github.com/antihax/optional"
	"github.com/gokrazy/gokapi/gusapi"
	"github.com/google/go-cmp/cmp"
)

func TestAttempt(t *testing.T) {
	testDBs := testDatabases()

	for _, tc := range testDBs {
		t.Run(tc.databaseType, func(t *testing.T) {
			ctx := context.Background()
			ts := newTestServer(t, tc.databaseType)
			api := ts.API()

			const machineID = "scan2drive"

			// Send a heartbeat to add a machine
			_, _, err := api.HeartbeatApi.Heartbeat(ctx, &gusapi.HeartbeatApiHeartbeatOpts{
				Body: optional.NewInterface(&gusapi.HeartbeatRequest{
					MachineId: machineID,
				}),
			})
			if err != nil {
				t.Fatal(err)
			}

			_, _, err = ts.API().IngestApi.Ingest(ctx, &gusapi.IngestApiIngestOpts{
				Body: optional.NewInterface(&gusapi.IngestRequest{
					MachineIdPattern: machineID,
					SbomHash:         "abcdefg",
					RegistryType:     "localdisk",
					DownloadLink:     "/doesnotexist/disk.gaf",
				}),
			})
			if err != nil {
				t.Fatal(err)
			}

			// Ensure the update API returns the new image now
			upResp, _, err := ts.API().UpdateApi.Update(ctx, &gusapi.UpdateApiUpdateOpts{
				Body: optional.NewInterface(&gusapi.UpdateRequest{
					MachineId: machineID,
				}),
			})
			if err != nil {
				t.Fatal(err)
			}

			want := gusapi.UpdateResponse{
				SbomHash:     "abcdefg",
				RegistryType: "localdisk",
				DownloadLink: "/doesnotexist/disk.gaf",
			}
			if diff := cmp.Diff(want, upResp); diff != "" {
				t.Fatalf("update: diff (-want +got):\n%s", diff)
			}

			// Ensure the attempt API changes the state to attempted
			_, _, err = ts.API().UpdateApi.Attempt(ctx, &gusapi.UpdateApiAttemptOpts{
				Body: optional.NewInterface(&gusapi.AttemptRequest{
					MachineId: machineID,
					SbomHash:  upResp.SbomHash,
				}),
			})
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
