package jobs

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/tern/v2/migrate"
)

//go:embed pg/migrations/*.sql
var migrations embed.FS

// MigrateDatabase runs the migrations using Tern. ccc
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
	// Convert embed.FS to fs.FS
	// filesystem := fs.FS(migrations) // Type conversion

	// Load migrations from the converted fs.FS
	err = migrator.LoadMigrations(filesystem)
	if err != nil {
		return fmt.Errorf("failed to load migrations: %v", err)
	}

	// Run the migrations
	err = migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %v", err)
	}

	return nil
}
