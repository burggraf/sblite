// cmd/user.go
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/markb/sblite/internal/auth"
	"github.com/markb/sblite/internal/db"
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage users",
	Long:  `Commands for managing users in the authentication system.`,
}

var userCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new user",
	Long: `Create a new user with the specified email and password.

Examples:
  # Create a new user
  sblite user create --email user@example.com --password secret123

  # Create a user with a custom database path
  sblite user create --email user@example.com --password secret123 --db mydata.db`,
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		dbPath, _ := cmd.Flags().GetString("db")

		if email == "" || password == "" {
			return fmt.Errorf("--email and --password are required")
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

		service := auth.NewService(database, "not-needed-for-create")
		user, err := service.CreateUser(email, password, nil)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		fmt.Printf("Created user: %s (ID: %s)\n", user.Email, user.ID)
		return nil
	},
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	Long:  `Display all users in the authentication system.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")

		// Check if database exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return fmt.Errorf("database not found at %s. Run 'sblite init' first", dbPath)
		}

		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		rows, err := database.Query(`
			SELECT id, email, role, created_at FROM auth_users WHERE deleted_at IS NULL
		`)
		if err != nil {
			return fmt.Errorf("failed to query users: %w", err)
		}
		defer rows.Close()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tEMAIL\tROLE\tCREATED")

		count := 0
		for rows.Next() {
			var id, email, role, createdAt string
			if err := rows.Scan(&id, &email, &role, &createdAt); err != nil {
				continue
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", id, email, role, createdAt)
			count++
		}
		w.Flush()

		if count == 0 {
			fmt.Println("No users found")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userCreateCmd)
	userCmd.AddCommand(userListCmd)

	// Add --db flag to user parent command (inherited by subcommands)
	userCmd.PersistentFlags().String("db", "data.db", "Path to database file")

	// Flags for user create
	userCreateCmd.Flags().String("email", "", "User email (required)")
	userCreateCmd.Flags().String("password", "", "User password (required)")
}
