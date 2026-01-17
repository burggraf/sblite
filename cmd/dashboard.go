// cmd/dashboard.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/markb/sblite/internal/dashboard"
	"github.com/markb/sblite/internal/db"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Manage the dashboard",
	Long:  `Commands for managing the sblite dashboard.`,
}

var dashboardSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up the dashboard password",
	Long:  `Set the initial dashboard password. Only works if no password has been set.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")

		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		store := dashboard.NewStore(database.DB)
		auth := dashboard.NewAuth(store)

		if !auth.NeedsSetup() {
			return fmt.Errorf("dashboard password already set, use 'dashboard reset-password' to change it")
		}

		password, err := promptPassword("Enter dashboard password: ")
		if err != nil {
			return err
		}

		confirm, err := promptPassword("Confirm password: ")
		if err != nil {
			return err
		}

		if password != confirm {
			return fmt.Errorf("passwords do not match")
		}

		if err := auth.SetupPassword(password); err != nil {
			return fmt.Errorf("failed to set password: %w", err)
		}

		fmt.Println("Dashboard password set successfully")
		return nil
	},
}

var dashboardResetPasswordCmd = &cobra.Command{
	Use:   "reset-password",
	Short: "Reset the dashboard password",
	Long:  `Change the dashboard password. This will invalidate any existing sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")

		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		if err := database.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		store := dashboard.NewStore(database.DB)
		auth := dashboard.NewAuth(store)

		password, err := promptPassword("Enter new dashboard password: ")
		if err != nil {
			return err
		}

		confirm, err := promptPassword("Confirm password: ")
		if err != nil {
			return err
		}

		if password != confirm {
			return fmt.Errorf("passwords do not match")
		}

		if err := auth.ResetPassword(password); err != nil {
			return fmt.Errorf("failed to reset password: %w", err)
		}

		// Destroy any existing sessions
		sessions := dashboard.NewSessionManager(store)
		sessions.Destroy()

		fmt.Println("Dashboard password reset successfully")
		return nil
	},
}

// stdinReader is reused for non-terminal input to avoid losing buffered data
var stdinReader *bufio.Reader

func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	// Try to read password securely (hides input)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // Add newline after hidden input
		if err != nil {
			return "", err
		}
		return string(password), nil
	}

	// Fallback for non-terminal (e.g., piped input)
	// Reuse reader to avoid losing buffered data
	if stdinReader == nil {
		stdinReader = bufio.NewReader(os.Stdin)
	}
	password, err := stdinReader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(password), nil
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
	dashboardCmd.AddCommand(dashboardSetupCmd)
	dashboardCmd.AddCommand(dashboardResetPasswordCmd)

	dashboardSetupCmd.Flags().String("db", "data.db", "Path to the database file")
	dashboardResetPasswordCmd.Flags().String("db", "data.db", "Path to the database file")
}
