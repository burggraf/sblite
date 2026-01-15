package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sblite",
	Short: "Supabase Lite - lightweight Supabase-compatible backend",
	Long:  `A single-binary backend with SQLite, providing Supabase-compatible auth and REST APIs.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
