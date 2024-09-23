package jobs

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/tern/v2/migrate"
)

//go:embed pg/migrations/*.sql
var migrations embed.FS

// MigrateDatabase runs the migrations using Tern.
func MigrateDatabase(conn *pgx.Conn) error {
	ctx := context.Background()

	// Create a new migrator instance
	migrator, err := migrate.NewMigrator(ctx, conn, "schema_version")
	if err != nil {
		return fmt.Errorf("failed to create migrator: %v", err)
	}

	// Convert embed.FS to fs.FS
	filesystem, err := fs.Sub(migrations, "pg/migrations")
	if err != nil {
		return fmt.Errorf("failed to create sub-filesystem: %v", err)
	}

	// List and log the migration files
	files, err := fs.ReadDir(filesystem, ".")
	if err != nil {
		return fmt.Errorf("failed to read migration directory: %v", err)
	}
	log.Println("Migration files found:")
	for _, file := range files {
		log.Printf("- %s", file.Name())

		// Print the contents of the migration file
		content, err := fs.ReadFile(filesystem, file.Name())
		if err != nil {
			log.Printf("Error reading file %s: %v", file.Name(), err)
		} else {
			log.Printf("Contents of %s:\n%s", file.Name(), string(content))
		}
	}

	// Load migrations from the converted fs.FS
	err = migrator.LoadMigrations(filesystem)
	if err != nil {
		return fmt.Errorf("failed to load migrations: %v", err)
	}

	// Log the number of migrations loaded
	log.Printf("Loaded %d migrations", len(migrator.Migrations))

	// Run the migrations
	err = migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %v", err)
	}

	// Log the applied migrations
	log.Println("Applied migrations:")
	for _, m := range migrator.Migrations {
		log.Printf("- %s", m.Name)
	}

	return nil
}
