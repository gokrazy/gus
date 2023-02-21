package gusserver

import (
	"context"
	"testing"

	"github.com/antihax/optional"
	"github.com/gokrazy/gokapi/gusapi"
)

func TestHeartbeat(t *testing.T) {
	testDBs := testDatabases()

	for _, tc := range testDBs {
		t.Run(tc.databaseType, func(t *testing.T) {
			ctx := context.Background()
			ts := newTestServer(t, tc.databaseType)
			api := ts.API()

			const machineID = "scan2drive"

			// Ensure the heartbeats and machines tables are empty when a fresh
			// server starts
			ts.ensureEmpty(t, "heartbeats")
			ts.ensureEmpty(t, "machines")

			_, _, err := api.HeartbeatApi.Heartbeat(ctx, &gusapi.HeartbeatApiHeartbeatOpts{
				Body: optional.NewInterface(&gusapi.HeartbeatRequest{
					MachineId: machineID,
				}),
			})
			if err != nil {
				t.Fatal(err)
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
