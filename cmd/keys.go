// cmd/keys.go
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage API keys",
	Long:  `Commands for managing API keys for sblite.`,
}

var keysGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate anon and service_role API keys",
	Long:  `Generates both anon and service_role API keys using the configured JWT secret.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jwtSecret := os.Getenv("SBLITE_JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "super-secret-jwt-key-please-change-in-production"
			fmt.Fprintln(os.Stderr, "Warning: Using default JWT secret. Set SBLITE_JWT_SECRET in production.")
		}

		anonKey, err := generateAPIKey(jwtSecret, "anon")
		if err != nil {
			return fmt.Errorf("failed to generate anon key: %w", err)
		}

		serviceKey, err := generateAPIKey(jwtSecret, "service_role")
		if err != nil {
			return fmt.Errorf("failed to generate service key: %w", err)
		}

		fmt.Printf("SBLITE_ANON_KEY=%s\n", anonKey)
		fmt.Printf("SBLITE_SERVICE_KEY=%s\n", serviceKey)

		return nil
	},
}

func generateAPIKey(jwtSecret, role string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"role": role,
		"iss":  "sblite",
		"iat":  now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

func init() {
	rootCmd.AddCommand(keysCmd)
	keysCmd.AddCommand(keysGenerateCmd)
}
