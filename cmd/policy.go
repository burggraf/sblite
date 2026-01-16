// cmd/policy.go
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/rls"
	"github.com/spf13/cobra"
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage RLS policies",
	Long:  `Commands for managing Row Level Security (RLS) policies.`,
}

var policyAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new RLS policy",
	Long: `Create a new Row Level Security policy for a table.

The policy can specify USING expressions (for SELECT, UPDATE, DELETE) and
CHECK expressions (for INSERT, UPDATE) to control row-level access.

Examples:
  # Allow users to only see their own rows
  sblite policy add --table todos --name user_isolation --using "user_id = auth.uid()"

  # Allow inserts only for authenticated user's own data
  sblite policy add --table todos --name user_insert --command INSERT --check "user_id = auth.uid()"

  # Policy for all operations
  sblite policy add --table todos --name full_isolation --using "user_id = auth.uid()" --check "user_id = auth.uid()"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath, _ := cmd.Flags().GetString("db")
		tableName, _ := cmd.Flags().GetString("table")
		policyName, _ := cmd.Flags().GetString("name")
		command, _ := cmd.Flags().GetString("command")
		usingExpr, _ := cmd.Flags().GetString("using")
		checkExpr, _ := cmd.Flags().GetString("check")

		if tableName == "" || policyName == "" {
			return fmt.Errorf("--table and --name are required")
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

		service := rls.NewService(database)
		policy, err := service.CreatePolicy(tableName, policyName, command, usingExpr, checkExpr)
		if err != nil {
			return fmt.Errorf("failed to create policy: %w", err)
		}

		fmt.Printf("Created policy '%s' on table '%s' (ID: %d)\n", policy.PolicyName, policy.TableName, policy.ID)
		return nil
	},
}

var policyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all RLS policies",
	Long:  `Display all Row Level Security policies in the database.`,
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

		service := rls.NewService(database)
		policies, err := service.ListAllPolicies()
		if err != nil {
			return fmt.Errorf("failed to list policies: %w", err)
		}

		if len(policies) == 0 {
			fmt.Println("No policies found")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTABLE\tNAME\tCOMMAND\tENABLED\tUSING\tCHECK")
		for _, p := range policies {
			enabled := "yes"
			if !p.Enabled {
				enabled = "no"
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
				p.ID, p.TableName, p.PolicyName, p.Command, enabled, p.UsingExpr, p.CheckExpr)
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(policyCmd)
	policyCmd.AddCommand(policyAddCmd)
	policyCmd.AddCommand(policyListCmd)

	// Add --db flag to parent policy command so it's inherited by subcommands
	policyCmd.PersistentFlags().String("db", "data.db", "Path to database file")

	// Flags for policy add
	policyAddCmd.Flags().String("table", "", "Table name (required)")
	policyAddCmd.Flags().String("name", "", "Policy name (required)")
	policyAddCmd.Flags().String("command", "ALL", "Command (SELECT, INSERT, UPDATE, DELETE, ALL)")
	policyAddCmd.Flags().String("using", "", "USING expression for SELECT/UPDATE/DELETE")
	policyAddCmd.Flags().String("check", "", "CHECK expression for INSERT/UPDATE")
}
