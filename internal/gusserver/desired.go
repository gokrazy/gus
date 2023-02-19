package gusserver

import (
	"context"
	"database/sql"
	"log"
)

func (s *server) updateDesired() error {
	// Intentionally not using a passed-in context so that this request keeps
	// running even if a client terminates the connection early.
	ctx := context.Background()

	// TODO: check if ingestion policy is auto-update

	rows, err := s.queries.selectMachinesForDesired.QueryContext(ctx)
	if err != nil {
		return err
	}
	defer rows.Close()
	type machine struct {
		MachineID       string
		DesiredImage    sql.NullString
		IngestionPolicy sql.NullString
	}
	var machines []machine
	for rows.Next() {
		var m machine
		if err := rows.Scan(&m.MachineID, &m.DesiredImage, &m.IngestionPolicy); err != nil {
			return err
		}
		machines = append(machines, m)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	rows, err = s.queries.selectImagesForDesired.QueryContext(ctx)
	if err != nil {
		return err
	}
	defer rows.Close()
	type image struct {
		SBOMHash         string
		MachineIDPattern string
	}
	seen := make(map[string]bool)
	var images []image
	for rows.Next() {
		var i image
		if err := rows.Scan(&i.SBOMHash, &i.MachineIDPattern); err != nil {
			return err
		}
		// While Postgres has a DISTINCT ON feature, SQLite lacks it, so it is
		// easier to do grouping ourselves: we only use the latest image sbom
		// hash per machine id pattern.
		if !seen[i.MachineIDPattern] {
			images = append(images, i)
			seen[i.MachineIDPattern] = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, img := range images {
		for _, mach := range machines {
			// TODO: pattern matching
			if mach.MachineID != img.MachineIDPattern {
				continue
			}
			if mach.DesiredImage.String == img.SBOMHash {
				continue // machine is already on the desired image
			}
			log.Printf("Setting desired image for machine %q to %q", mach.MachineID, img.SBOMHash)

			_, err := s.queries.updateDesiredImage.ExecContext(ctx, img.SBOMHash, mach.MachineID)
			if err != nil {
				return err
			}

		}
	}

	return nil
}
