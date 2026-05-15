package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version and Commit are set via ldflags at build time:
//
//	-X github.com/donaldgifford/docz/cmd.Version=...
//	-X github.com/donaldgifford/docz/cmd.Commit=...
var (
	Version = "dev"
	Commit  = "none"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE: func(_ *cobra.Command, _ []string) error {
		fmt.Printf("docz %s (commit: %s)\n", Version, Commit)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
