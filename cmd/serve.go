// cmd/serve.go
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/markb/sblite/internal/auth"
	"github.com/markb/sblite/internal/dashboard"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/functions"
	"github.com/markb/sblite/internal/log"
	"github.com/markb/sblite/internal/mail"
	"github.com/markb/sblite/internal/server"
	"github.com/markb/sblite/internal/storage"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Supabase Lite server",
	Long:  `Starts the HTTP server with auth and REST API endpoints.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize logging first
		logConfig := buildLogConfig(cmd)
		if err := log.Init(logConfig); err != nil {
			return fmt.Errorf("failed to initialize logging: %w", err)
		}

		dbPath, _ := cmd.Flags().GetString("db")
		port, _ := cmd.Flags().GetInt("port")
		host, _ := cmd.Flags().GetString("host")
		jwtSecret := os.Getenv("SBLITE_JWT_SECRET")

		if jwtSecret == "" {
			jwtSecret = "super-secret-jwt-key-please-change-in-production"
			log.Warn("using default JWT secret, set SBLITE_JWT_SECRET in production")
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
		migrationsDir, _ := cmd.Flags().GetString("migrations-dir")

		// Build storage configuration
		storageConfig := buildStorageConfig(cmd)

		srv := server.NewWithConfig(database, server.ServerConfig{
			JWTSecret:     jwtSecret,
			MailConfig:    mailConfig,
			MigrationsDir: migrationsDir,
			StorageConfig: storageConfig,
		})

		// Set dashboard config for settings display
		srv.SetDashboardConfig(&dashboard.ServerConfig{
			Version: "0.1.0",
			Host:    host,
			Port:    port,
			DBPath:  dbPath,
			LogMode: logConfig.Mode,
			LogFile: logConfig.FilePath,
			LogDB:   logConfig.DBPath,
		})

		addr := fmt.Sprintf("%s:%d", host, port)
		log.Info("starting server",
			"addr", addr,
			"auth_api", "http://"+addr+"/auth/v1",
			"rest_api", "http://"+addr+"/rest/v1",
			"mail_mode", mailConfig.Mode,
		)
		if mailConfig.Mode == mail.ModeCatch {
			log.Info("mail UI available", "url", "http://"+addr+"/mail")
		}

		// Enable edge functions if requested
		functionsEnabled, _ := cmd.Flags().GetBool("functions")
		if functionsEnabled {
			functionsDir, _ := cmd.Flags().GetString("functions-dir")
			functionsPort, _ := cmd.Flags().GetInt("functions-port")

			// Generate API keys for function environment
			anonKey := auth.GenerateAPIKey(jwtSecret, "anon")
			serviceKey := auth.GenerateAPIKey(jwtSecret, "service_role")

			cfg := &functions.Config{
				FunctionsDir: functionsDir,
				RuntimePort:  functionsPort,
				JWTSecret:    jwtSecret,
				BaseURL:      "http://" + addr,
				SblitePort:   port,
				AnonKey:      anonKey,
				ServiceKey:   serviceKey,
				DBPath:       dbPath,
			}

			if err := srv.EnableFunctions(cfg); err != nil {
				return fmt.Errorf("failed to enable functions: %w", err)
			}

			// Start edge runtime
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := srv.StartFunctions(ctx); err != nil {
				log.Warn("failed to start edge runtime", "error", err)
				log.Info("functions API will return errors until runtime is available")
			} else {
				log.Info("edge functions enabled",
					"functions_api", "http://"+addr+"/functions/v1",
					"functions_dir", functionsDir,
				)
			}

			// Handle graceful shutdown
			go func() {
				sigCh := make(chan os.Signal, 1)
				signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
				<-sigCh
				log.Info("shutting down...")
				srv.StopFunctions()
				cancel()
			}()
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

// buildLogConfig creates a log.Config from environment variables and CLI flags.
// Priority: CLI flags > environment variables > defaults
func buildLogConfig(cmd *cobra.Command) *log.Config {
	cfg := log.DefaultConfig()

	// Read environment variables first
	if mode := os.Getenv("SBLITE_LOG_MODE"); mode != "" {
		cfg.Mode = mode
	}
	if level := os.Getenv("SBLITE_LOG_LEVEL"); level != "" {
		cfg.Level = level
	}
	if format := os.Getenv("SBLITE_LOG_FORMAT"); format != "" {
		cfg.Format = format
	}
	if filePath := os.Getenv("SBLITE_LOG_FILE"); filePath != "" {
		cfg.FilePath = filePath
	}
	if dbPath := os.Getenv("SBLITE_LOG_DB"); dbPath != "" {
		cfg.DBPath = dbPath
	}
	if maxSize := os.Getenv("SBLITE_LOG_MAX_SIZE"); maxSize != "" {
		if v, err := strconv.Atoi(maxSize); err == nil {
			cfg.MaxSizeMB = v
		}
	}
	if maxAge := os.Getenv("SBLITE_LOG_MAX_AGE"); maxAge != "" {
		if v, err := strconv.Atoi(maxAge); err == nil {
			cfg.MaxAgeDays = v
		}
	}
	if maxBackups := os.Getenv("SBLITE_LOG_MAX_BACKUPS"); maxBackups != "" {
		if v, err := strconv.Atoi(maxBackups); err == nil {
			cfg.MaxBackups = v
		}
	}
	if fields := os.Getenv("SBLITE_LOG_FIELDS"); fields != "" {
		cfg.Fields = strings.Split(fields, ",")
	}

	// CLI flags override environment variables
	if mode, _ := cmd.Flags().GetString("log-mode"); mode != "" {
		cfg.Mode = mode
	}
	if level, _ := cmd.Flags().GetString("log-level"); level != "" {
		cfg.Level = level
	}
	if format, _ := cmd.Flags().GetString("log-format"); format != "" {
		cfg.Format = format
	}
	if filePath, _ := cmd.Flags().GetString("log-file"); filePath != "" {
		cfg.FilePath = filePath
	}
	if dbPath, _ := cmd.Flags().GetString("log-db"); dbPath != "" {
		cfg.DBPath = dbPath
	}
	if maxSize, _ := cmd.Flags().GetInt("log-max-size"); maxSize > 0 {
		cfg.MaxSizeMB = maxSize
	}
	if maxAge, _ := cmd.Flags().GetInt("log-max-age"); maxAge > 0 {
		cfg.MaxAgeDays = maxAge
	}
	if maxBackups, _ := cmd.Flags().GetInt("log-max-backups"); maxBackups > 0 {
		cfg.MaxBackups = maxBackups
	}
	if fields, _ := cmd.Flags().GetStringSlice("log-fields"); len(fields) > 0 {
		cfg.Fields = fields
	}

	return cfg
}

// buildStorageConfig creates a storage.Config from environment variables and CLI flags.
// Priority: CLI flags > environment variables > defaults
func buildStorageConfig(cmd *cobra.Command) *storage.Config {
	cfg := &storage.Config{
		Backend:   "local",
		LocalPath: "./storage",
	}

	// Read environment variables first
	if backend := os.Getenv("SBLITE_STORAGE_BACKEND"); backend != "" {
		cfg.Backend = backend
	}
	if localPath := os.Getenv("SBLITE_STORAGE_PATH"); localPath != "" {
		cfg.LocalPath = localPath
	}
	if s3Endpoint := os.Getenv("SBLITE_S3_ENDPOINT"); s3Endpoint != "" {
		cfg.S3Endpoint = s3Endpoint
	}
	if s3Region := os.Getenv("SBLITE_S3_REGION"); s3Region != "" {
		cfg.S3Region = s3Region
	}
	if s3Bucket := os.Getenv("SBLITE_S3_BUCKET"); s3Bucket != "" {
		cfg.S3Bucket = s3Bucket
	}
	if s3AccessKey := os.Getenv("SBLITE_S3_ACCESS_KEY"); s3AccessKey != "" {
		cfg.S3AccessKey = s3AccessKey
	}
	if s3SecretKey := os.Getenv("SBLITE_S3_SECRET_KEY"); s3SecretKey != "" {
		cfg.S3SecretKey = s3SecretKey
	}
	if pathStyle := os.Getenv("SBLITE_S3_PATH_STYLE"); pathStyle == "true" || pathStyle == "1" {
		cfg.S3ForcePathStyle = true
	}

	// CLI flags override environment variables
	if backend, _ := cmd.Flags().GetString("storage-backend"); backend != "" {
		cfg.Backend = backend
	}
	if localPath, _ := cmd.Flags().GetString("storage-path"); localPath != "" {
		cfg.LocalPath = localPath
	}
	if s3Endpoint, _ := cmd.Flags().GetString("s3-endpoint"); s3Endpoint != "" {
		cfg.S3Endpoint = s3Endpoint
	}
	if s3Region, _ := cmd.Flags().GetString("s3-region"); s3Region != "" {
		cfg.S3Region = s3Region
	}
	if s3Bucket, _ := cmd.Flags().GetString("s3-bucket"); s3Bucket != "" {
		cfg.S3Bucket = s3Bucket
	}
	if s3AccessKey, _ := cmd.Flags().GetString("s3-access-key"); s3AccessKey != "" {
		cfg.S3AccessKey = s3AccessKey
	}
	if s3SecretKey, _ := cmd.Flags().GetString("s3-secret-key"); s3SecretKey != "" {
		cfg.S3SecretKey = s3SecretKey
	}
	if pathStyle, _ := cmd.Flags().GetBool("s3-path-style"); pathStyle {
		cfg.S3ForcePathStyle = true
	}

	return cfg
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().String("db", "data.db", "Path to database file")
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	serveCmd.Flags().String("host", "0.0.0.0", "Host to bind to")
	serveCmd.Flags().String("migrations-dir", "./migrations", "Path to migrations directory")

	// Storage flags
	serveCmd.Flags().String("storage-backend", "local", "Storage backend: local or s3")
	serveCmd.Flags().String("storage-path", "./storage", "Path to storage directory (local backend)")
	serveCmd.Flags().String("s3-endpoint", "", "S3 endpoint URL (for S3-compatible services like MinIO)")
	serveCmd.Flags().String("s3-region", "", "S3 region (e.g., us-east-1)")
	serveCmd.Flags().String("s3-bucket", "", "S3 bucket name")
	serveCmd.Flags().String("s3-access-key", "", "S3 access key ID")
	serveCmd.Flags().String("s3-secret-key", "", "S3 secret access key")
	serveCmd.Flags().Bool("s3-path-style", false, "Use path-style addressing (required for MinIO)")

	// Mail flags
	serveCmd.Flags().String("mail-mode", "", "Email mode: log, catch, or smtp (default: log)")
	serveCmd.Flags().String("mail-from", "", "Default sender email address")
	serveCmd.Flags().String("site-url", "", "Base URL for email links")

	// Logging flags
	serveCmd.Flags().String("log-mode", "", "Logging output: console, file, database (default: console)")
	serveCmd.Flags().String("log-level", "", "Log level: debug, info, warn, error (default: info)")
	serveCmd.Flags().String("log-format", "", "Log format: text, json (default: text)")
	serveCmd.Flags().String("log-file", "", "Log file path (default: sblite.log)")
	serveCmd.Flags().String("log-db", "", "Log database path (default: log.db)")
	serveCmd.Flags().Int("log-max-size", 0, "Max log file size in MB (default: 100)")
	serveCmd.Flags().Int("log-max-age", 0, "Max age of logs in days (default: 7)")
	serveCmd.Flags().Int("log-max-backups", 0, "Max backup files to keep (default: 3)")
	serveCmd.Flags().StringSlice("log-fields", nil, "DB log fields: source,request_id,user_id,extra")

	// Edge functions flags
	serveCmd.Flags().Bool("functions", false, "Enable edge functions support")
	serveCmd.Flags().String("functions-dir", "./functions", "Path to functions directory")
	serveCmd.Flags().Int("functions-port", 8081, "Internal port for edge runtime")
}
