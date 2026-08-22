package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	databaseURL := flag.String("database-url", "", "PostgreSQL URL for a disposable validation database")
	migrations := flag.String("migrations", "file://migrations", "golang-migrate source URL")
	baselineVersion := flag.Uint("baseline-version", 0, "optional published production baseline to migrate through before latest")
	flag.Parse()
	if *databaseURL == "" {
		log.Fatal("-database-url is required")
	}

	m, err := migrate.New(*migrations, *databaseURL)
	if err != nil {
		log.Fatalf("create migrate instance: %v", err)
	}
	defer m.Close()

	if *baselineVersion > 0 {
		if err := m.Migrate(*baselineVersion); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("production baseline migration failed at version %d: %v", *baselineVersion, err)
		}
		version, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("read production baseline migration version: %v", err)
		}
		if dirty || version != *baselineVersion {
			log.Fatalf("production baseline mismatch: want=%d got=%d dirty=%v", *baselineVersion, version, dirty)
		}
		fmt.Printf("PRODUCTION_BASELINE=PASS version=%d\n", version)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("full migration up failed: %v", err)
	}
	latest, dirty, err := m.Version()
	if err != nil {
		log.Fatalf("read latest migration version: %v", err)
	}
	if dirty {
		log.Fatalf("database is dirty at migration %d", latest)
	}
	if *baselineVersion > 0 && latest <= *baselineVersion {
		log.Fatalf("latest migration %d does not advance production baseline %d", latest, *baselineVersion)
	}
	fmt.Printf("FULL_UP=PASS version=%d\n", latest)

	if err := m.Steps(-1); err != nil {
		log.Fatalf("latest migration down failed: %v", err)
	}
	previous, dirty, err := m.Version()
	if err != nil {
		log.Fatalf("read version after down: %v", err)
	}
	if dirty || previous >= latest {
		log.Fatalf("invalid version after down: latest=%d previous=%d dirty=%v", latest, previous, dirty)
	}
	fmt.Printf("LATEST_DOWN=PASS version=%d\n", previous)

	if err := m.Steps(1); err != nil {
		log.Fatalf("latest migration replay up failed: %v", err)
	}
	replayed, dirty, err := m.Version()
	if err != nil {
		log.Fatalf("read replayed migration version: %v", err)
	}
	if dirty || replayed != latest {
		log.Fatalf("replay did not restore latest version: want=%d got=%d dirty=%v", latest, replayed, dirty)
	}
	fmt.Printf("LATEST_REPLAY_UP=PASS version=%d\n", replayed)
}
