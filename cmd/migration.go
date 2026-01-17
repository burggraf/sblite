// cmd/migration.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/markb/sblite/internal/db"
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

var migrationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all migrations and their status",
	Long: `Show all migrations from the migrations directory and whether they've been applied.

Examples:
  sblite migration list
  sblite migration list --db data.db`,
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

		// Get all migrations from filesystem
		all, err := migration.ReadFromDir(migrationsDir)
		if err != nil {
			return fmt.Errorf("failed to read migrations: %w", err)
		}

		// Get applied migrations
		applied, err := runner.GetApplied()
		if err != nil {
			return fmt.Errorf("failed to get applied migrations: %w", err)
		}

		// Build applied set
		appliedMap := make(map[string]migration.Migration)
		for _, m := range applied {
			appliedMap[m.Version] = m
		}

		if len(all) == 0 {
			fmt.Println("No migrations found in", migrationsDir)
			return nil
		}

		// Print table
		fmt.Printf("%-16s %-30s %s\n", "VERSION", "NAME", "STATUS")
		fmt.Println(strings.Repeat("-", 60))

		pendingCount := 0
		for _, m := range all {
			status := "pending"
			if a, ok := appliedMap[m.Version]; ok {
				status = fmt.Sprintf("applied %s", a.AppliedAt.Format("2006-01-02 15:04"))
			} else {
				pendingCount++
			}
			fmt.Printf("%-16s %-30s %s\n", m.Version, m.Name, status)
		}

		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("%d applied, %d pending\n", len(applied), pendingCount)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrationCmd)
	migrationCmd.AddCommand(migrationNewCmd)
	migrationCmd.AddCommand(migrationListCmd)

	migrationNewCmd.Flags().String("migrations-dir", "./migrations", "Directory for migration files")

	migrationListCmd.Flags().String("db", "data.db", "Database path")
	migrationListCmd.Flags().String("migrations-dir", "./migrations", "Directory for migration files")
}
