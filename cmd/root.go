package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version information set via ldflags at build time
var (
	Version   = "dev"
	BuildTime = ""
	GitCommit = ""
)

var rootCmd = &cobra.Command{
	Use:     "sblite",
	Short:   "Supabase Lite - lightweight Supabase-compatible backend",
	Long:    `A single-binary backend with SQLite, providing Supabase-compatible auth and REST APIs.`,
	Version: Version,
}

func init() {
	// Set version template to include build info when available
	versionTmpl := "sblite version {{.Version}}"
	if BuildTime != "" {
		versionTmpl += " (built " + BuildTime
		if GitCommit != "" {
			versionTmpl += ", commit " + GitCommit
		}
		versionTmpl += ")"
	}
	versionTmpl += "\n"
	rootCmd.SetVersionTemplate(versionTmpl)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
