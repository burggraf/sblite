// cmd/migration.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/markb/sblite/internal/migration"
	"github.com/spf13/cobra"
)

var migrationCmd = &cobra.Command{
	Use:   "migration",
	Short: "Manage database migrations",
	Long:  `Commands for creating and listing database migrations.`,
}

var migrationNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new migration file",
	Long: `Create a new migration file with a timestamp prefix.

The name should be a short description using snake_case.

Examples:
  sblite migration new create_posts
  sblite migration new add_user_id_to_posts`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		migrationsDir, _ := cmd.Flags().GetString("migrations-dir")

		// Validate name (alphanumeric and underscores only)
		if !regexp.MustCompile(`^[a-z][a-z0-9_]*$`).MatchString(name) {
			return fmt.Errorf("migration name must be lowercase alphanumeric with underscores, starting with a letter")
		}

		// Create migrations directory if needed
		if err := os.MkdirAll(migrationsDir, 0755); err != nil {
			return fmt.Errorf("failed to create migrations directory: %w", err)
		}

		// Generate migration
		m := migration.Migration{
			Version: migration.GenerateVersion(),
			Name:    name,
		}

		// Create file with template
		filename := filepath.Join(migrationsDir, m.Filename())
		content := fmt.Sprintf(`-- Migration: %s
-- Created: %s

-- Write your SQL here

`, m.Name, m.Version)

		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create migration file: %w", err)
		}

		fmt.Printf("Created migration: %s\n", filename)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrationCmd)
	migrationCmd.AddCommand(migrationNewCmd)

	migrationNewCmd.Flags().String("migrations-dir", "./migrations", "Directory for migration files")
}
