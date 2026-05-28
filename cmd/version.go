package cmd

import (
	"fmt"
	"io"

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
		return getRunner().Version()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// Version prints "docz <version> (commit: <commit>)" to r.Out.
func (r *Runner) Version() error {
	return printVersion(r.Out)
}

// printVersion is the shared write path used by both (*Runner).Version
// and any future caller that needs to format the version banner. Kept
// separate to make the writer dependency explicit.
func printVersion(w io.Writer) error {
	_, err := fmt.Fprintf(w, "docz %s (commit: %s)\n", Version, Commit)
	return err
}
