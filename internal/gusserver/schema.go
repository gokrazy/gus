package gusserver

import (
	"database/sql"
	"fmt"
)

type queries struct {
	insertHeartbeat          *sql.Stmt
	insertMachine            *sql.Stmt
	selectMachinesForIndex   *sql.Stmt
	selectMachinesForDesired *sql.Stmt
	insertImage              *sql.Stmt
	selectImagesForDesired   *sql.Stmt
	updateDesiredImage       *sql.Stmt
}

func initDatabase(db *sql.DB, dbType string) (*queries, error) {
	const schemaTemplate = `
CREATE TABLE IF NOT EXISTS images (
	sbom_hash TEXT NOT NULL PRIMARY KEY,
	ingestion_timestamp %s NOT NULL,
	machine_id_pattern TEXT NOT NULL,
	registry_type TEXT NOT NULL,
	download_url TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS machines (
	machine_id TEXT NOT NULL PRIMARY KEY,
	desired_image TEXT NULL,
	update_state TEXT NULL,
	ingestion_policy TEXT NULL
);

CREATE TABLE IF NOT EXISTS heartbeats (
	machine_id TEXT NOT NULL PRIMARY KEY,
	timestamp %[1]s NOT NULL,
	sbom_hash TEXT NOT NULL,
	sbom TEXT NOT NULL,
	kernel TEXT NULL,
	model TEXT NULL,
	remote_ip TEXT NULL
);
	`

	var schema string
	switch dbType {
	case "sqlite":
		schema = fmt.Sprintf(schemaTemplate, "DATETIME")
	case "postgres":
		schema = fmt.Sprintf(schemaTemplate, "TIMESTAMPTZ")
	}

	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	insertImage, err := db.Prepare(`
INSERT INTO images (sbom_hash, ingestion_timestamp, machine_id_pattern, registry_type, download_url)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (sbom_hash) DO UPDATE SET ingestion_timestamp = $2, machine_id_pattern = $3, registry_type = $4, download_url = $5
`)
	if err != nil {
		return nil, err
	}

	insertHeartbeat, err := db.Prepare(`
INSERT INTO heartbeats (machine_id, timestamp, sbom_hash, sbom, kernel, model, remote_ip)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (machine_id) DO UPDATE SET timestamp = $2, sbom_hash = $3, sbom = $4, kernel = $5, model = $6, remote_ip = $7
`)
	if err != nil {
		return nil, err
	}

	insertMachine, err := db.Prepare(`
INSERT INTO machines (machine_id)
VALUES ($1)
ON CONFLICT (machine_id) DO NOTHING
`)
	if err != nil {
		return nil, err
	}

	selectMachinesForIndex, err := db.Prepare(`
SELECT machine_id, sbom_hash, timestamp, model, remote_ip FROM heartbeats
`)
	if err != nil {
		return nil, err
	}

	selectMachinesForDesired, err := db.Prepare(`
SELECT machine_id, desired_image, ingestion_policy FROM machines
`)
	if err != nil {
		return nil, err
	}

	selectImagesForDesired, err := db.Prepare(`
SELECT
  sbom_hash,
  machine_id_pattern
FROM images
ORDER BY ingestion_timestamp DESC
`)
	if err != nil {
		return nil, err
	}

	updateDesiredImage, err := db.Prepare(`
UPDATE machines
SET desired_image = $1
WHERE machine_id = $2
`)
	if err != nil {
		return nil, err
	}

	return &queries{
		insertHeartbeat:          insertHeartbeat,
		insertMachine:            insertMachine,
		selectMachinesForIndex:   selectMachinesForIndex,
		selectMachinesForDesired: selectMachinesForDesired,
		insertImage:              insertImage,
		selectImagesForDesired:   selectImagesForDesired,
		updateDesiredImage:       updateDesiredImage,
	}, nil
}
