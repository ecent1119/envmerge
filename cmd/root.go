package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "envmerge",
	Short: "Environment Resolution Inspector",
	Long: `envmerge explains what environment variables actually resolve to and why.

It traces precedence across:
  - .env files
  - .env.local overrides
  - .env.example templates
  - compose env_file references
  - compose inline environment blocks

Use it to understand silent misconfigurations before they cause problems.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("envmerge %s\n", version)
	},
}
