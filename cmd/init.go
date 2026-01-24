// cmd/init.go
package cmd

import (
	"fmt"
	"os"

	"github.com/markb/sblite/internal/dashboard"
	"github.com/markb/sblite/internal/db"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Supabase Lite database",
	Long:  `Creates a new SQLite database with the auth schema tables.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")
		password, _ := cmd.Flags().GetString("password")

		// Check if file already exists
		if _, err := os.Stat(dbPath); err == nil {
			return fmt.Errorf("database already exists at %s", dbPath)
		}

		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		defer database.Close()

		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		// Set dashboard password if provided
		if password != "" {
			store := dashboard.NewStore(database.DB)
			auth := dashboard.NewAuth(store)
			if err := auth.SetupPassword(password); err != nil {
				return fmt.Errorf("failed to set dashboard password: %w", err)
			}
			fmt.Printf("Initialized database at %s with dashboard password\n", dbPath)
		} else {
			fmt.Printf("Initialized database at %s\n", dbPath)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().String("db", "data.db", "Path to database file")
	initCmd.Flags().String("password", "", "Set dashboard password during initialization")
}
