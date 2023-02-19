package gusserver

import (
	"database/sql"
	"fmt"
	"strings"
)

type queries struct {
	insertHeartbeat          *sql.Stmt
	insertMachine            *sql.Stmt
	selectMachinesForIndex   *sql.Stmt
	selectMachinesForDesired *sql.Stmt
	selectDesired            *sql.Stmt
	insertImage              *sql.Stmt
	selectImagesForIndex     *sql.Stmt
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
	remote_ip TEXT NULL,
	hostname TEXT NULL
);
	`

	var schema string
	switch strings.TrimPrefix(dbType, "txdb/") {
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
INSERT INTO heartbeats (machine_id, timestamp, sbom_hash, sbom, kernel, model, remote_ip, hostname)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (machine_id) DO UPDATE SET timestamp = $2, sbom_hash = $3, sbom = $4, kernel = $5, model = $6, remote_ip = $7, hostname = $8
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
SELECT
  machines.machine_id,
  machines.desired_image,
  machines.update_state,
  machines.ingestion_policy,
  heartbeats.sbom_hash,
  heartbeats.timestamp,
  heartbeats.model,
  heartbeats.remote_ip,
  heartbeats.hostname
FROM machines
LEFT JOIN heartbeats ON (machines.machine_id = heartbeats.machine_id)
ORDER BY heartbeats.hostname, heartbeats.machine_id ASC
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

	selectDesired, err := db.Prepare(`
SELECT
  machines.desired_image,
  images.registry_type,
  images.download_url
FROM machines
INNER JOIN images ON (machines.desired_image = images.sbom_hash)
WHERE machine_id = $1
`)
	if err != nil {
		return nil, err
	}

	selectImagesForIndex, err := db.Prepare(`
SELECT
  sbom_hash,
  ingestion_timestamp,
  machine_id_pattern,
  registry_type,
  download_url
FROM images
ORDER BY ingestion_timestamp DESC
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
		selectDesired:            selectDesired,
		insertImage:              insertImage,
		selectImagesForIndex:     selectImagesForIndex,
		selectImagesForDesired:   selectImagesForDesired,
		updateDesiredImage:       updateDesiredImage,
	}, nil
}
