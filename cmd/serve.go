// cmd/serve.go
package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/mail"
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

		// Build mail configuration
		mailConfig := buildMailConfig(cmd)

		srv := server.New(database, jwtSecret, mailConfig)
		addr := fmt.Sprintf("%s:%d", host, port)
		fmt.Printf("Starting Supabase Lite on %s\n", addr)
		fmt.Printf("  Auth API: http://%s/auth/v1\n", addr)
		fmt.Printf("  REST API: http://%s/rest/v1\n", addr)
		fmt.Printf("  Mail Mode: %s\n", mailConfig.Mode)
		if mailConfig.Mode == mail.ModeCatch {
			fmt.Printf("  Mail UI: http://%s/mail\n", addr)
		}

		return srv.ListenAndServe(addr)
	},
}

// buildMailConfig creates a mail.Config from environment variables and CLI flags.
// Priority: CLI flags > environment variables > defaults
func buildMailConfig(cmd *cobra.Command) *mail.Config {
	cfg := mail.DefaultConfig()

	// Read environment variables first
	if mode := os.Getenv("SBLITE_MAIL_MODE"); mode != "" {
		cfg.Mode = mode
	}
	if from := os.Getenv("SBLITE_MAIL_FROM"); from != "" {
		cfg.From = from
	}
	if siteURL := os.Getenv("SBLITE_SITE_URL"); siteURL != "" {
		cfg.SiteURL = siteURL
	}
	if smtpHost := os.Getenv("SBLITE_SMTP_HOST"); smtpHost != "" {
		cfg.SMTPHost = smtpHost
	}
	if smtpPort := os.Getenv("SBLITE_SMTP_PORT"); smtpPort != "" {
		if port, err := strconv.Atoi(smtpPort); err == nil {
			cfg.SMTPPort = port
		}
	}
	if smtpUser := os.Getenv("SBLITE_SMTP_USER"); smtpUser != "" {
		cfg.SMTPUser = smtpUser
	}
	if smtpPass := os.Getenv("SBLITE_SMTP_PASS"); smtpPass != "" {
		cfg.SMTPPass = smtpPass
	}

	// CLI flags override environment variables
	if mailMode, _ := cmd.Flags().GetString("mail-mode"); mailMode != "" {
		cfg.Mode = mailMode
	}
	if mailFrom, _ := cmd.Flags().GetString("mail-from"); mailFrom != "" {
		cfg.From = mailFrom
	}
	if siteURL, _ := cmd.Flags().GetString("site-url"); siteURL != "" {
		cfg.SiteURL = siteURL
	}

	return cfg
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().String("db", "data.db", "Path to database file")
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	serveCmd.Flags().String("host", "0.0.0.0", "Host to bind to")
	serveCmd.Flags().String("mail-mode", "", "Email mode: log, catch, or smtp (default: log)")
	serveCmd.Flags().String("mail-from", "", "Default sender email address")
	serveCmd.Flags().String("site-url", "", "Base URL for email links")
}
