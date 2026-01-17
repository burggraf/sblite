// cmd/db.go
package cmd

import (
	"fmt"
	"os"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/migration"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
	Long:  `Commands for managing the database schema via migrations.`,
}

var dbPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Apply pending migrations",
	Long: `Apply all pending migrations to the database.

Migrations are applied in order by their timestamp version.
Each migration runs in a transaction - if it fails, the migration
is rolled back and no further migrations are applied.

Examples:
  sblite db push
  sblite db push --db data.db`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")
		migrationsDir, _ := cmd.Flags().GetString("migrations-dir")

		// Check if database exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found at %s (run 'sblite init' first)", dbPath)
		}

		// Open database
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		runner := migration.NewRunner(database.DB)

		// Get pending migrations
		pending, err := runner.GetPending(migrationsDir)
		if err != nil {
			return fmt.Errorf("failed to get pending migrations: %w", err)
		}

		if len(pending) == 0 {
			fmt.Println("No pending migrations")
			return nil
		}

		fmt.Printf("Applying %d migration(s)...\n\n", len(pending))

		for _, m := range pending {
			fmt.Printf("  Applying %s_%s... ", m.Version, m.Name)
			if err := runner.Apply(m); err != nil {
				fmt.Println("FAILED")
				return fmt.Errorf("migration failed: %w", err)
			}
			fmt.Println("done")
		}

		fmt.Printf("\nApplied %d migration(s)\n", len(pending))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(dbPushCmd)

	dbPushCmd.Flags().String("db", "data.db", "Database path")
	dbPushCmd.Flags().String("migrations-dir", "./migrations", "Directory for migration files")
}
