// cmd/serve.go
package cmd

import (
	"fmt"
	"os"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/server"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Supabase Lite server",
	Long:  `Starts the HTTP server with auth and REST API endpoints.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")
		port, _ := cmd.Flags().GetInt("port")
		host, _ := cmd.Flags().GetString("host")
		jwtSecret := os.Getenv("SBLITE_JWT_SECRET")

		if jwtSecret == "" {
			jwtSecret = "super-secret-jwt-key-please-change-in-production"
			fmt.Println("Warning: Using default JWT secret. Set SBLITE_JWT_SECRET in production.")
		}

		// Check if database exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found at %s. Run 'sblite init' first", dbPath)
		}

		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Run migrations in case schema is outdated
		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		srv := server.New(database, jwtSecret)
		addr := fmt.Sprintf("%s:%d", host, port)
		fmt.Printf("Starting Supabase Lite on %s\n", addr)
		fmt.Printf("  Auth API: http://%s/auth/v1\n", addr)
		fmt.Printf("  REST API: http://%s/rest/v1\n", addr)

		return srv.ListenAndServe(addr)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().String("db", "data.db", "Path to database file")
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	serveCmd.Flags().String("host", "0.0.0.0", "Host to bind to")
}
