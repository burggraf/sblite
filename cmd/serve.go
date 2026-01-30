// cmd/serve.go
package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/markb/sblite/internal/auth"
	"github.com/markb/sblite/internal/dashboard"
	"github.com/markb/sblite/internal/dashboard/migration"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/functions"
	"github.com/markb/sblite/internal/log"
	"github.com/markb/sblite/internal/mail"
	"github.com/markb/sblite/internal/observability"
	"github.com/markb/sblite/internal/pgwire"
	"github.com/markb/sblite/internal/server"
	"github.com/markb/sblite/internal/storage"
	"github.com/spf13/cobra"
)

// startTime records when the app started (initialized when package loads)
var startTime = time.Now()

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

		// Initialize OpenTelemetry
		otelCfg := buildOTelConfig(cmd)
		var tel *observability.Telemetry
		var otelCtx context.Context
		if otelCfg.ShouldEnable() {
			var otelCleanup func()
			var err error
			tel, otelCleanup, err = observability.Init(context.Background(), otelCfg)
			if err != nil {
				return fmt.Errorf("failed to initialize OpenTelemetry: %w", err)
			}
			defer otelCleanup()
			otelCtx = context.Background()

			log.Info("OpenTelemetry enabled",
				"exporter", otelCfg.Exporter,
				"endpoint", otelCfg.Endpoint,
				"metrics", otelCfg.MetricsEnabled,
				"traces", otelCfg.TracesEnabled,
				"sample_rate", otelCfg.SampleRate,
			)
		} else {
			otelCtx = context.Background()
		}

		dbPath, _ := cmd.Flags().GetString("db")
		port, _ := cmd.Flags().GetInt("port")
		host, _ := cmd.Flags().GetString("host")
		jwtSecret := os.Getenv("SBLITE_JWT_SECRET")

		if jwtSecret == "" {
			jwtSecret = "super-secret-jwt-key-please-change-in-production"
			log.Warn("using default JWT secret, set SBLITE_JWT_SECRET in production")
		}

		// HTTPS configuration
		httpsDomain, _ := cmd.Flags().GetString("https")
		httpPort, _ := cmd.Flags().GetInt("http-port")

		// Check environment variable for HTTPS domain
		if httpsDomain == "" {
			httpsDomain = os.Getenv("SBLITE_HTTPS_DOMAIN")
		}
		if !cmd.Flags().Changed("http-port") {
			if envPort := os.Getenv("SBLITE_HTTP_PORT"); envPort != "" {
				if p, err := strconv.Atoi(envPort); err == nil {
					httpPort = p
				} else {
					log.Warn("invalid SBLITE_HTTP_PORT, using default", "value", envPort, "error", err)
				}
			}
		}

		// Validate HTTPS domain if provided
		httpsEnabled := httpsDomain != ""
		if httpsEnabled {
			if err := server.ValidateDomain(httpsDomain); err != nil {
				return err
			}
		}

		// Auto-initialize database if it doesn't exist
		dbExists := true
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			dbExists = false
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

		// Create metrics tables for observability
		if err := db.CreateMetricsTables(database.DB); err != nil {
			return fmt.Errorf("failed to create metrics tables: %w", err)
		}

		// Set database connection for telemetry metrics storage
		if tel != nil {
			tel.SetDB(database.DB)
		}

		if !dbExists {
			log.Info("initialized new database", "path", dbPath)

			// Set dashboard password if provided during auto-init
			if password, _ := cmd.Flags().GetString("password"); password != "" {
				store := dashboard.NewStore(database.DB)
				auth := dashboard.NewAuth(store)
				if err := auth.SetupPassword(password); err != nil {
					return fmt.Errorf("failed to set dashboard password: %w", err)
				}
				log.Info("dashboard password set")
			}
		}

		// Build mail configuration
		mailConfig := buildMailConfig(cmd)
		migrationsDir, _ := cmd.Flags().GetString("migrations-dir")

		// Build storage configuration (pass db for dashboard settings)
		storageConfig := buildStorageConfig(cmd, database.DB)

		// Static file serving configuration
		staticDir, _ := cmd.Flags().GetString("static-dir")
		if envStaticDir := os.Getenv("SBLITE_STATIC_DIR"); envStaticDir != "" && !cmd.Flags().Changed("static-dir") {
			staticDir = envStaticDir
		}

		srv := server.NewWithConfig(database, server.ServerConfig{
			JWTSecret:     jwtSecret,
			MailConfig:    mailConfig,
			MigrationsDir: migrationsDir,
			StorageConfig: storageConfig,
			StaticDir:     staticDir,
		})

		// Set telemetry on server BEFORE setting up routes
		if tel != nil {
			srv.SetTelemetry(tel)
		}

		// Set up routes AFTER telemetry is set (so middleware can be applied)
		srv.SetupRoutes()

		// Set dashboard config for settings display
		srv.SetDashboardConfig(&dashboard.ServerConfig{
			Version: "0.1.1",
			Host:    host,
			Port:    port,
			DBPath:  dbPath,
			LogMode: logConfig.Mode,
			LogFile: logConfig.FilePath,
			LogDB:   logConfig.DBPath,
		})

		// Initialize migration service
		functionsDir, _ := cmd.Flags().GetString("functions-dir")
		migrationSvc := migration.NewService(database.DB, &migration.ServerConfig{
			FunctionsDir: functionsDir,
			StorageDir:   storageConfig.LocalPath,
			JWTSecret:    jwtSecret,
		})
		srv.SetMigrationService(migrationSvc)

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
		ctx, cancel := context.WithCancel(otelCtx) // Use otelCtx as parent
		defer cancel()

		if functionsEnabled {
			functionsDir, _ := cmd.Flags().GetString("functions-dir")
			functionsPort, _ := cmd.Flags().GetInt("functions-port")
			edgeRuntimeDir, _ := cmd.Flags().GetString("edge-runtime-dir")

			// Check environment variable if flag not set
			if edgeRuntimeDir == "" {
				edgeRuntimeDir = os.Getenv("SBLITE_EDGE_RUNTIME_DIR")
			}

			// Generate API keys for function environment
			anonKey := auth.GenerateAPIKey(jwtSecret, "anon")
			serviceKey := auth.GenerateAPIKey(jwtSecret, "service_role")

			// Determine base URL (HTTPS if enabled)
			baseURL := "http://" + addr
			if httpsEnabled {
				baseURL = "https://" + httpsDomain
			}

			cfg := &functions.Config{
				FunctionsDir:   functionsDir,
				RuntimePort:    functionsPort,
				JWTSecret:      jwtSecret,
				BaseURL:        baseURL,
				SblitePort:     port,
				AnonKey:        anonKey,
				ServiceKey:     serviceKey,
				DBPath:         dbPath,
				EdgeRuntimeDir: edgeRuntimeDir,
			}

			if err := srv.EnableFunctions(cfg); err != nil {
				return fmt.Errorf("failed to enable functions: %w", err)
			}

			// Start edge runtime
			if err := srv.StartFunctions(ctx); err != nil {
				log.Warn("failed to start edge runtime", "error", err)
				log.Info("functions API will return errors until runtime is available")
			} else {
				log.Info("edge functions enabled",
					"functions_api", "http://"+addr+"/functions/v1",
					"functions_dir", functionsDir,
				)
			}
		}

		// Enable realtime WebSocket support if requested
		realtimeEnabled, _ := cmd.Flags().GetBool("realtime")
		if realtimeEnabled {
			srv.EnableRealtime()
			log.Info("realtime enabled", "realtime_api", "ws://"+addr+"/realtime/v1")
		}

		// Start TUS cleanup routine for expired resumable uploads
		srv.StartTUSCleanup(ctx)

		// Start PostgreSQL wire protocol server if requested
		pgPort, _ := cmd.Flags().GetInt("pg-port")
		var pgServer *pgwire.Server
		if pgPort > 0 {
			pgPassword, _ := cmd.Flags().GetString("pg-password")
			pgNoAuth, _ := cmd.Flags().GetBool("pg-no-auth")

			// Check environment variables
			if pgPassword == "" {
				pgPassword = os.Getenv("SBLITE_PG_PASSWORD")
			}

			pgConfig := pgwire.Config{
				Address:       fmt.Sprintf("%s:%d", host, pgPort),
				Password:      pgPassword,
				NoAuth:        pgNoAuth,
				Logger:        log.Logger(),
				MigrationsDir: migrationsDir,
			}

			// If no explicit password provided, use dashboard password
			authMode := "disabled"
			if !pgNoAuth {
				if pgPassword != "" {
					authMode = "password (--pg-password)"
				} else {
					// Use dashboard auth - same password as web dashboard
					dashStore := dashboard.NewStore(database.DB)
					dashAuth := dashboard.NewAuth(dashStore)
					if dashAuth.NeedsSetup() {
						log.Warn("pgwire server requires dashboard password to be set first",
							"hint", "run 'sblite dashboard setup' or set --pg-password",
						)
						authMode = "rejected (no password configured)"
					} else {
						pgConfig.PasswordVerifier = dashAuth.VerifyPassword
						authMode = "dashboard password"
					}
				}
			}

			var err error
			pgServer, err = pgwire.NewServer(database.DB, pgConfig)
			if err != nil {
				return fmt.Errorf("failed to create pgwire server: %w", err)
			}

			go func() {
				log.Info("pgwire server listening", "addr", pgConfig.Address, "auth", authMode)
				if err := pgServer.ListenAndServe(); err != nil {
					log.Warn("pgwire server error", "error", err)
				}
			}()
		}

		// Handle graceful shutdown for all modes
		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			log.Info("shutting down...")

			// Create a timeout context for shutdown
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()

			// Stop functions first if enabled
			if functionsEnabled {
				srv.StopFunctions()
			}

			// Shutdown pgwire server if enabled
			if pgServer != nil {
				if err := pgServer.Shutdown(shutdownCtx); err != nil {
					log.Warn("pgwire server shutdown error", "error", err)
				}
			}

			// Shutdown HTTP server
			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Warn("HTTP server shutdown error", "error", err)
			}

			cancel()
		}()

		// Start HTTP server (blocks until shutdown)
		if httpsEnabled {
			// Determine certificate directory (alongside database)
			certDir := filepath.Join(filepath.Dir(dbPath), "certs")

			// Determine addresses
			httpsAddr := fmt.Sprintf("%s:%d", host, port)
			httpAddr := fmt.Sprintf("%s:%d", host, httpPort)

			// Override port to 443 if using default
			if port == 8080 {
				httpsAddr = fmt.Sprintf("%s:443", host)
				port = 443 // Update for logging
			}

			httpsCfg := server.HTTPSConfig{
				Domain:   httpsDomain,
				CertDir:  certDir,
				HTTPAddr: httpAddr,
			}

			log.Info("starting server with HTTPS",
				"https_addr", httpsAddr,
				"http_addr", httpAddr,
				"domain", httpsDomain,
				"auth_api", "https://"+httpsDomain+"/auth/v1",
				"rest_api", "https://"+httpsDomain+"/rest/v1",
			)

			// Print startup time
			startupDuration := time.Since(startTime)
			fmt.Printf("\n  Ready! Started in %s\n\n", startupDuration.Round(time.Millisecond))

			if err := srv.ListenAndServeTLS(httpsAddr, httpAddr, httpsCfg); err != nil && err != http.ErrServerClosed {
				return err
			}
		} else {
			// Print startup time
			startupDuration := time.Since(startTime)
			fmt.Printf("\n  Ready! Started in %s\n\n", startupDuration.Round(time.Millisecond))

			if err := srv.ListenAndServe(addr); err != nil && err != http.ErrServerClosed {
				return err
			}
		}
		return nil
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
	if bufferLines := os.Getenv("SBLITE_LOG_BUFFER_LINES"); bufferLines != "" {
		if v, err := strconv.Atoi(bufferLines); err == nil {
			cfg.BufferLines = v
		}
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
	if bufferLines, _ := cmd.Flags().GetInt("log-buffer-lines"); cmd.Flags().Changed("log-buffer-lines") {
		cfg.BufferLines = bufferLines
	}

	return cfg
}

// buildOTelConfig creates an observability.Config from environment variables and CLI flags.
// Priority: CLI flags > environment variables > defaults
func buildOTelConfig(cmd *cobra.Command) *observability.Config {
	cfg := observability.NewConfig()

	// Read environment variables first
	if exporter := os.Getenv("SBLITE_OTEL_EXPORTER"); exporter != "" {
		cfg.Exporter = exporter
	}
	if endpoint := os.Getenv("SBLITE_OTEL_ENDPOINT"); endpoint != "" {
		cfg.Endpoint = endpoint
	}
	if serviceName := os.Getenv("SBLITE_OTEL_SERVICE_NAME"); serviceName != "" {
		cfg.ServiceName = serviceName
	}
	if sampleRate := os.Getenv("SBLITE_OTEL_SAMPLE_RATE"); sampleRate != "" {
		if rate, err := strconv.ParseFloat(sampleRate, 64); err == nil {
			cfg.SampleRate = rate
		}
	}

	// CLI flags override environment variables
	if exporter, _ := cmd.Flags().GetString("otel-exporter"); exporter != "" {
		cfg.Exporter = exporter
	}
	if endpoint, _ := cmd.Flags().GetString("otel-endpoint"); endpoint != "" {
		cfg.Endpoint = endpoint
	}
	if serviceName, _ := cmd.Flags().GetString("otel-service-name"); serviceName != "" {
		cfg.ServiceName = serviceName
	}
	if cmd.Flags().Changed("otel-sample-rate") {
		if sampleRate, _ := cmd.Flags().GetFloat64("otel-sample-rate"); sampleRate >= 0 && sampleRate <= 1 {
			cfg.SampleRate = sampleRate
		}
	}

	// Enable metrics/traces if exporter is set (unless explicitly disabled)
	if cfg.ShouldEnable() {
		metricsEnabled, _ := cmd.Flags().GetBool("otel-metrics-enabled")
		tracesEnabled, _ := cmd.Flags().GetBool("otel-traces-enabled")

		// If flags are not set, default to true
		if !cmd.Flags().Changed("otel-metrics-enabled") {
			cfg.MetricsEnabled = true
		} else {
			cfg.MetricsEnabled = metricsEnabled
		}
		if !cmd.Flags().Changed("otel-traces-enabled") {
			cfg.TracesEnabled = true
		} else {
			cfg.TracesEnabled = tracesEnabled
		}
	}

	return cfg
}

// buildStorageConfig creates a storage.Config from dashboard settings, environment variables, and CLI flags.
// Priority: Dashboard settings > CLI flags > environment variables > defaults
func buildStorageConfig(cmd *cobra.Command, db *sql.DB) *storage.Config {
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

	// Dashboard settings have highest priority (if db is available)
	if db != nil {
		store := dashboard.NewStore(db)
		if backend, _ := store.Get("storage_backend"); backend != "" {
			cfg.Backend = backend
		}
		if localPath, _ := store.Get("storage_local_path"); localPath != "" {
			cfg.LocalPath = localPath
		}
		if s3Endpoint, _ := store.Get("storage_s3_endpoint"); s3Endpoint != "" {
			cfg.S3Endpoint = s3Endpoint
		}
		if s3Region, _ := store.Get("storage_s3_region"); s3Region != "" {
			cfg.S3Region = s3Region
		}
		if s3Bucket, _ := store.Get("storage_s3_bucket"); s3Bucket != "" {
			cfg.S3Bucket = s3Bucket
		}
		if s3AccessKey, _ := store.Get("storage_s3_access_key"); s3AccessKey != "" {
			cfg.S3AccessKey = s3AccessKey
		}
		if s3SecretKey, _ := store.Get("storage_s3_secret_key"); s3SecretKey != "" {
			cfg.S3SecretKey = s3SecretKey
		}
		if s3PathStyle, _ := store.Get("storage_s3_path_style"); s3PathStyle == "true" {
			cfg.S3ForcePathStyle = true
		}
	}

	return cfg
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().String("db", "data.db", "Path to database file")
	serveCmd.Flags().String("password", "", "Set dashboard password (only used when auto-initializing database)")
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
	serveCmd.Flags().Int("log-buffer-lines", 500, "Number of log lines to keep in memory buffer (0 to disable)")

	// Edge functions flags
	serveCmd.Flags().Bool("functions", false, "Enable edge functions support")
	serveCmd.Flags().String("functions-dir", "./functions", "Path to functions directory")
	serveCmd.Flags().Int("functions-port", 8081, "Internal port for edge runtime")
	serveCmd.Flags().String("edge-runtime-dir", "", "Directory for edge runtime binary (default: <db-dir>/edge-runtime/)")

	// Realtime flags
	serveCmd.Flags().Bool("realtime", false, "Enable realtime WebSocket support")

	// HTTPS flags
	serveCmd.Flags().String("https", "", "Domain for automatic Let's Encrypt HTTPS")
	serveCmd.Flags().Int("http-port", 80, "HTTP port for ACME challenges (only used with --https)")

	// PostgreSQL wire protocol flags
	serveCmd.Flags().Int("pg-port", 0, "PostgreSQL wire protocol port (0 = disabled)")
	serveCmd.Flags().String("pg-password", "", "PostgreSQL wire protocol password (empty = no auth)")
	serveCmd.Flags().Bool("pg-no-auth", false, "Disable PostgreSQL wire protocol authentication")

	// Static file serving flags
	serveCmd.Flags().String("static-dir", "./public", "Directory for static file hosting")

	// OpenTelemetry flags
	serveCmd.Flags().String("otel-exporter", "", "OpenTelemetry exporter: none (default), stdout, otlp")
	serveCmd.Flags().String("otel-endpoint", "", "OpenTelemetry OTLP endpoint (default: localhost:4317)")
	serveCmd.Flags().String("otel-service-name", "", "OpenTelemetry service name (default: sblite)")
	serveCmd.Flags().Float64("otel-sample-rate", 0.1, "OpenTelemetry trace sampling rate 0.0-1.0 (default: 0.1)")
	serveCmd.Flags().Bool("otel-metrics-enabled", true, "Enable OpenTelemetry metrics")
	serveCmd.Flags().Bool("otel-traces-enabled", true, "Enable OpenTelemetry traces")
}
