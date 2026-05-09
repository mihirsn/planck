// Package cmd contains all Cobra CLI command definitions for Planck.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Build-time variables injected by goreleaser via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// rootCmd is the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "planck",
	Short: "Observe behavior at the smallest scale.",
	Long: `⚛️  Planck — Lightweight log analysis for developers.

Transforms raw application logs into actionable operational insights
without requiring heavyweight observability infrastructure.

Built for Docker users, small teams, and self-hosted applications.

Examples:
  planck analyze app.log
  planck analyze --docker invoice-api
  planck analyze --docker invoice-api --tail 1000 --since 1h`,
}

// versionCmd prints the current build version.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the Planck version",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("planck %s (commit: %s, built: %s)\n", version, commit, date)
	},
}

// Execute runs the root command. Called by main.main().
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
