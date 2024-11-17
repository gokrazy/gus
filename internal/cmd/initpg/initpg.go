// initpg is a small test helper command which starts a Postgres
// instance and makes it available to the wrapped 'go test' command.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/stapelberg/postgrestest"

	_ "github.com/lib/pq"
)

func runWrappedCommand(pgurl string) error {
	// os.Args[0] is initpg
	// os.Args[1] is --
	// os.Args[2] is go
	// os.Args[3] is test
	// etc.
	wrapped := exec.Command(os.Args[2], os.Args[3:]...)
	wrapped.Stdin = os.Stdin
	wrapped.Stdout = os.Stdout
	wrapped.Stderr = os.Stderr
	wrapped.Env = append(os.Environ(), "PGURL="+pgurl)
	if err := wrapped.Run(); err != nil {
		return fmt.Errorf("%v: %v", wrapped.Args, err)
	}
	return nil
}

func initpg() error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	pgt, err := postgrestest.Start(context.Background(),
		postgrestest.WithDir(filepath.Join(cacheDir, "pg_tmp.gus")))
	if err != nil {
		return err
	}
	defer pgt.Cleanup()
	// Run the wrapped command ('go test', typically)
	return runWrappedCommand(pgt.DefaultDatabase())
}

func main() {
	if err := initpg(); err != nil {
		log.Fatal(err)
	}
}
