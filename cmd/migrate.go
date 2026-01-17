// cmd/migrate.go
package cmd

import (
	"fmt"
	"os"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/migrate"
	"github.com/markb/sblite/internal/schema"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migration commands",
	Long:  `Commands for migrating sblite data to Supabase/PostgreSQL.`,
}

var migrateExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export schema as PostgreSQL DDL",
	Long: `Export the sblite schema as PostgreSQL DDL for migration to Supabase.

The export reads table metadata from the _columns table and generates
CREATE TABLE statements with proper PostgreSQL types, NOT NULL constraints,
DEFAULT values, and PRIMARY KEY definitions.

Examples:
  # Export to stdout
  sblite migrate export --db data.db

  # Export to a file
  sblite migrate export --db data.db -o schema.sql

  # Export and review before migration
  sblite migrate export --db data.db -o schema.sql && cat schema.sql`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")
		outputPath, _ := cmd.Flags().GetString("output")

		// Check if database exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found at %s", dbPath)
		}

		// Open database
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Create schema and exporter
		sch := schema.New(database.DB)
		exporter := migrate.New(sch)

		// Export DDL
		ddl, err := exporter.ExportDDL()
		if err != nil {
			return fmt.Errorf("failed to export DDL: %w", err)
		}

		// Write output
		if outputPath != "" {
			if err := os.WriteFile(outputPath, []byte(ddl), 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			fmt.Printf("Schema exported to %s\n", outputPath)
		} else {
			fmt.Print(ddl)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.AddCommand(migrateExportCmd)

	migrateExportCmd.Flags().String("db", "data.db", "Database path")
	migrateExportCmd.Flags().StringP("output", "o", "", "Output file (stdout if not specified)")
}
