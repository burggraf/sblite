// cmd/functions.go
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/markb/sblite/internal/auth"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/functions"
	"github.com/markb/sblite/internal/log"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var functionsCmd = &cobra.Command{
	Use:   "functions",
	Short: "Manage edge functions",
	Long:  `Create, list, and manage edge functions for serverless TypeScript/JavaScript execution.`,
}

var functionsNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new edge function",
	Long:  `Creates a new edge function from a template.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		functionsDir, _ := cmd.Flags().GetString("functions-dir")
		template, _ := cmd.Flags().GetString("template")

		// Validate function name
		if err := functions.ValidateFunctionName(name); err != nil {
			return err
		}

		// Create service to manage functions
		svc, err := functions.NewService(nil, &functions.Config{
			FunctionsDir: functionsDir,
		})
		if err != nil {
			return fmt.Errorf("failed to initialize functions: %w", err)
		}

		// Check if function already exists
		if svc.FunctionExists(name) {
			return fmt.Errorf("function %q already exists", name)
		}

		// Create function directory
		if err := svc.CreateFunction(name); err != nil {
			return err
		}

		// If a specific template was requested, overwrite with that template
		if template != "" && template != "default" {
			tmpl := functions.GetTemplate(functions.TemplateType(template), name)
			path := fmt.Sprintf("%s/%s/index.ts", functionsDir, name)
			if err := os.WriteFile(path, []byte(tmpl), 0644); err != nil {
				return fmt.Errorf("failed to write template: %w", err)
			}
		}

		fmt.Printf("Created function %q in %s/%s\n", name, functionsDir, name)
		fmt.Println("\nTo run your function locally:")
		fmt.Printf("  sblite serve --functions\n")
		fmt.Println("\nTo invoke your function:")
		fmt.Printf("  curl -X POST http://localhost:8080/functions/v1/%s \\\n", name)
		fmt.Printf("    -H 'Content-Type: application/json' \\\n")
		fmt.Printf("    -d '{\"name\": \"World\"}'\n")
		return nil
	},
}

var functionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all edge functions",
	Long:  `Lists all discovered edge functions in the functions directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		functionsDir, _ := cmd.Flags().GetString("functions-dir")

		svc, err := functions.NewService(nil, &functions.Config{
			FunctionsDir: functionsDir,
		})
		if err != nil {
			return fmt.Errorf("failed to initialize functions: %w", err)
		}

		funcs, err := svc.ListFunctions()
		if err != nil {
			return fmt.Errorf("failed to list functions: %w", err)
		}

		if len(funcs) == 0 {
			fmt.Println("No functions found.")
			fmt.Printf("Create one with: sblite functions new <name>\n")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tENTRYPOINT\tMODIFIED")
		for _, fn := range funcs {
			fmt.Fprintf(w, "%s\t%s\t%s\n", fn.Name, fn.Entrypoint, fn.ModTime.Format("2006-01-02 15:04:05"))
		}
		w.Flush()
		return nil
	},
}

var functionsDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete an edge function",
	Long:  `Deletes an edge function and its directory.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		functionsDir, _ := cmd.Flags().GetString("functions-dir")
		force, _ := cmd.Flags().GetBool("force")

		svc, err := functions.NewService(nil, &functions.Config{
			FunctionsDir: functionsDir,
		})
		if err != nil {
			return fmt.Errorf("failed to initialize functions: %w", err)
		}

		if !svc.FunctionExists(name) {
			return fmt.Errorf("function %q not found", name)
		}

		if !force {
			fmt.Printf("Delete function %q? This cannot be undone. [y/N]: ", name)
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := svc.DeleteFunction(name); err != nil {
			return err
		}

		fmt.Printf("Deleted function %q\n", name)
		return nil
	},
}

var functionsServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the edge runtime for local development",
	Long: `Starts the edge runtime for local development.
This is useful for testing functions without running the full sblite server.
For production use, use 'sblite serve --functions' instead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize logging
		logConfig := log.DefaultConfig()
		logConfig.Level = "debug"
		if err := log.Init(logConfig); err != nil {
			return fmt.Errorf("failed to initialize logging: %w", err)
		}

		functionsDir, _ := cmd.Flags().GetString("functions-dir")
		port, _ := cmd.Flags().GetInt("port")
		dbPath, _ := cmd.Flags().GetString("db")
		jwtSecret := os.Getenv("SBLITE_JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "super-secret-jwt-key-please-change-in-production"
		}

		// Generate API keys
		anonKey := auth.GenerateAPIKey(jwtSecret, "anon")
		serviceKey := auth.GenerateAPIKey(jwtSecret, "service_role")

		// Open database if it exists (for secrets)
		var database *db.DB
		if _, err := os.Stat(dbPath); err == nil {
			database, err = db.New(dbPath)
			if err != nil {
				log.Warn("failed to open database", "error", err)
			} else {
				defer database.Close()
				// Run migrations to ensure functions tables exist
				if err := database.RunMigrations(); err != nil {
					log.Warn("failed to run migrations", "error", err)
				}
			}
		}

		cfg := &functions.Config{
			FunctionsDir: functionsDir,
			RuntimePort:  port,
			JWTSecret:    jwtSecret,
			AnonKey:      anonKey,
			ServiceKey:   serviceKey,
			DBPath:       dbPath,
		}

		// Create service with database for secrets
		var svc *functions.Service
		var err error
		if database != nil {
			svc, err = functions.NewService(database.DB, cfg)
		} else {
			svc, err = functions.NewService(nil, cfg)
		}
		if err != nil {
			return fmt.Errorf("failed to initialize functions service: %w", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle graceful shutdown
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigCh
			fmt.Println("\nShutting down...")
			cancel()
			svc.Stop()
		}()

		fmt.Printf("Starting edge runtime on port %d...\n", port)
		fmt.Printf("Functions directory: %s\n", functionsDir)
		if database != nil {
			fmt.Printf("Database: %s (secrets will be loaded)\n", dbPath)
		}
		fmt.Println("Press Ctrl+C to stop.")

		if err := svc.Start(ctx); err != nil {
			return fmt.Errorf("failed to start edge runtime: %w", err)
		}

		// Wait for context cancellation
		<-ctx.Done()
		return nil
	},
}

var functionsDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download the edge runtime binary",
	Long:  `Downloads the Supabase Edge Runtime binary for the current platform.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !functions.IsSupported() {
			return fmt.Errorf("platform %s is not supported", functions.PlatformString())
		}

		downloader := functions.NewDownloader(functions.DefaultDownloadDir())

		fmt.Printf("Downloading edge runtime for %s...\n", functions.PlatformString())
		if err := downloader.Download(); err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		fmt.Printf("Downloaded to: %s\n", downloader.BinaryPath())
		return nil
	},
}

// Config management commands
var functionsConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage per-function configuration",
	Long:  `Manage per-function configuration like JWT verification, memory limits, and timeouts.`,
}

var functionsConfigSetJWTCmd = &cobra.Command{
	Use:   "set-jwt <function-name> <enabled|disabled>",
	Short: "Enable or disable JWT verification for a function",
	Long: `Enables or disables JWT verification for a specific function.
When disabled, the function can be invoked without a valid JWT token.
This is useful for public functions like webhooks.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		setting := args[1]
		dbPath, _ := cmd.Flags().GetString("db")

		// Get JWT secret for encryption
		jwtSecret := os.Getenv("SBLITE_JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "super-secret-jwt-key-please-change-in-production"
		}

		// Validate setting
		var verifyJWT bool
		switch setting {
		case "enabled", "true", "1", "on":
			verifyJWT = true
		case "disabled", "false", "0", "off":
			verifyJWT = false
		default:
			return fmt.Errorf("invalid setting %q: use 'enabled' or 'disabled'", setting)
		}

		// Open database
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Run migrations to ensure tables exist
		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		store := functions.NewStore(database.DB, jwtSecret)

		// Get existing metadata (or defaults)
		meta, err := store.GetMetadata(name)
		if err != nil {
			return fmt.Errorf("failed to get function metadata: %w", err)
		}

		// Update JWT verification setting
		meta.VerifyJWT = verifyJWT

		if err := store.SetMetadata(meta); err != nil {
			return fmt.Errorf("failed to save function metadata: %w", err)
		}

		status := "enabled"
		if !verifyJWT {
			status = "disabled"
		}
		fmt.Printf("JWT verification %s for function %q.\n", status, name)
		if !verifyJWT {
			fmt.Println("Warning: This function can now be invoked without authentication.")
		}
		return nil
	},
}

var functionsConfigShowCmd = &cobra.Command{
	Use:   "show <function-name>",
	Short: "Show configuration for a function",
	Long:  `Shows the current configuration for a specific function.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		dbPath, _ := cmd.Flags().GetString("db")

		// Get JWT secret for encryption
		jwtSecret := os.Getenv("SBLITE_JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "super-secret-jwt-key-please-change-in-production"
		}

		// Open database
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Run migrations to ensure tables exist
		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		store := functions.NewStore(database.DB, jwtSecret)

		meta, err := store.GetMetadata(name)
		if err != nil {
			return fmt.Errorf("failed to get function metadata: %w", err)
		}

		fmt.Printf("Function: %s\n", meta.Name)
		fmt.Printf("  JWT Verification: %v\n", meta.VerifyJWT)
		if meta.MemoryMB > 0 {
			fmt.Printf("  Memory Limit: %d MB\n", meta.MemoryMB)
		}
		if meta.TimeoutMS > 0 {
			fmt.Printf("  Timeout: %d ms\n", meta.TimeoutMS)
		}
		if meta.ImportMap != "" {
			fmt.Printf("  Import Map: %s\n", meta.ImportMap)
		}
		if len(meta.EnvVars) > 0 {
			fmt.Println("  Environment Variables:")
			for k := range meta.EnvVars {
				fmt.Printf("    - %s\n", k)
			}
		}
		return nil
	},
}

// Secrets management commands
var functionsSecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage function secrets",
	Long:  `Manage encrypted secrets that are injected as environment variables to edge functions.`,
}

var functionsSecretsSetCmd = &cobra.Command{
	Use:   "set <name>",
	Short: "Set a secret value",
	Long: `Sets a secret value. The value will be encrypted and stored in the database.
Secrets are injected as environment variables when the edge runtime starts.

If --value is not provided, you will be prompted to enter the value securely.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		dbPath, _ := cmd.Flags().GetString("db")
		value, _ := cmd.Flags().GetString("value")

		// Get JWT secret for encryption
		jwtSecret := os.Getenv("SBLITE_JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "super-secret-jwt-key-please-change-in-production"
		}

		// Open database
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Run migrations to ensure tables exist
		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		// If value not provided, prompt for it securely
		if value == "" {
			fmt.Printf("Enter value for secret %q: ", name)
			if term.IsTerminal(int(os.Stdin.Fd())) {
				// Read password without echo
				byteValue, err := term.ReadPassword(int(os.Stdin.Fd()))
				if err != nil {
					return fmt.Errorf("failed to read secret value: %w", err)
				}
				value = string(byteValue)
				fmt.Println() // Print newline after hidden input
			} else {
				// Read from stdin (non-terminal, e.g., pipe)
				reader := bufio.NewReader(os.Stdin)
				value, err = reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read secret value: %w", err)
				}
				value = strings.TrimSpace(value)
			}
		}

		if value == "" {
			return fmt.Errorf("secret value cannot be empty")
		}

		// Create store and set secret
		store := functions.NewStore(database.DB, jwtSecret)
		if err := store.SetSecret(name, value); err != nil {
			return fmt.Errorf("failed to set secret: %w", err)
		}

		fmt.Printf("Secret %q has been set.\n", name)
		fmt.Println("Note: Restart the edge runtime for changes to take effect.")
		return nil
	},
}

var functionsSecretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secrets",
	Long:  `Lists all secret names (values are never displayed).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")

		// Get JWT secret for encryption
		jwtSecret := os.Getenv("SBLITE_JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "super-secret-jwt-key-please-change-in-production"
		}

		// Open database
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Run migrations to ensure tables exist
		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		store := functions.NewStore(database.DB, jwtSecret)
		secrets, err := store.ListSecrets()
		if err != nil {
			return fmt.Errorf("failed to list secrets: %w", err)
		}

		if len(secrets) == 0 {
			fmt.Println("No secrets found.")
			fmt.Println("Set a secret with: sblite functions secrets set <name>")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tCREATED\tUPDATED")
		for _, secret := range secrets {
			created := secret.CreatedAt.Format("2006-01-02 15:04:05")
			updated := secret.UpdatedAt.Format("2006-01-02 15:04:05")
			if secret.CreatedAt.IsZero() {
				created = "-"
			}
			if secret.UpdatedAt.IsZero() {
				updated = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", secret.Name, created, updated)
		}
		w.Flush()
		return nil
	},
}

var functionsSecretsDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a secret",
	Long:  `Deletes a secret from the database.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		dbPath, _ := cmd.Flags().GetString("db")
		force, _ := cmd.Flags().GetBool("force")

		// Get JWT secret for encryption
		jwtSecret := os.Getenv("SBLITE_JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "super-secret-jwt-key-please-change-in-production"
		}

		// Open database
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Run migrations to ensure tables exist
		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		store := functions.NewStore(database.DB, jwtSecret)

		if !force {
			fmt.Printf("Delete secret %q? This cannot be undone. [y/N]: ", name)
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := store.DeleteSecret(name); err != nil {
			return fmt.Errorf("failed to delete secret: %w", err)
		}

		fmt.Printf("Secret %q has been deleted.\n", name)
		fmt.Println("Note: Restart the edge runtime for changes to take effect.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(functionsCmd)

	// Subcommands
	functionsCmd.AddCommand(functionsNewCmd)
	functionsCmd.AddCommand(functionsListCmd)
	functionsCmd.AddCommand(functionsDeleteCmd)
	functionsCmd.AddCommand(functionsServeCmd)
	functionsCmd.AddCommand(functionsDownloadCmd)
	functionsCmd.AddCommand(functionsSecretsCmd)
	functionsCmd.AddCommand(functionsConfigCmd)

	// Secrets subcommands
	functionsSecretsCmd.AddCommand(functionsSecretsSetCmd)
	functionsSecretsCmd.AddCommand(functionsSecretsListCmd)
	functionsSecretsCmd.AddCommand(functionsSecretsDeleteCmd)

	// Config subcommands
	functionsConfigCmd.AddCommand(functionsConfigSetJWTCmd)
	functionsConfigCmd.AddCommand(functionsConfigShowCmd)

	// Persistent flags for all functions subcommands
	functionsCmd.PersistentFlags().String("functions-dir", "./functions", "Path to functions directory")
	functionsCmd.PersistentFlags().String("db", "data.db", "Path to database file")

	// Flags for 'new' command
	functionsNewCmd.Flags().String("template", "default", "Function template: default, supabase, cors")

	// Flags for 'delete' command
	functionsDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	// Flags for 'serve' command
	functionsServeCmd.Flags().Int("port", 8081, "Port for edge runtime")

	// Flags for 'secrets set' command
	functionsSecretsSetCmd.Flags().String("value", "", "Secret value (if not provided, will prompt)")

	// Flags for 'secrets delete' command
	functionsSecretsDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}
