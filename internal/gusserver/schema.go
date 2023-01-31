package gusserver

import (
	"database/sql"
)

type queries struct {
	insertHeartbeat *sql.Stmt
	selectMachines  *sql.Stmt
}

func initDatabase(db *sql.DB) (*queries, error) {
	const schema = `
CREATE TABLE IF NOT EXISTS images (
	sbom_hash TEXT NOT NULL PRIMARY KEY,
	ingestion_timestamp DATETIME NOT NULL,
	machine_id_pattern TEXT NOT NULL,
	registry_type TEXT NOT NULL,
	download_url TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS machines (
	machine_id TEXT NOT NULL PRIMARY KEY,
	desired_image TEXT NULL,
	update_state TEXT NOT NULL,
	ingestion_policy TEXT NULL
);

CREATE TABLE IF NOT EXISTS heartbeats (
	machine_id TEXT NOT NULL PRIMARY KEY,
	timestamp DATETIME NOT NULL,
	sbom_hash TEXT NOT NULL,
	sbom TEXT NOT NULL
);
	`

	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	insertHeartbeat, err := db.Prepare(`
INSERT INTO heartbeats (machine_id, timestamp, sbom_hash, sbom)
VALUES ($1, $2, $3, $4)
ON CONFLICT (machine_id) DO UPDATE SET timestamp = $2, sbom_hash = $3, sbom = $4
`)
	if err != nil {
		return nil, err
	}

	selectMachines, err := db.Prepare(`
SELECT machine_id, timestamp FROM heartbeats
`)
	if err != nil {
		return nil, err
	}

	return &queries{
		insertHeartbeat: insertHeartbeat,
		selectMachines:  selectMachines,
	}, nil
}
