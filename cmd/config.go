package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print the resolved configuration",
	Long: `Print the fully resolved configuration (merged repo + global + defaults)
as YAML to stdout. Useful for debugging configuration issues.`,
	RunE: runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func runConfig(_ *cobra.Command, _ []string) error {
	enc := yaml.NewEncoder(os.Stdout)
	defer func() {
		//nolint:errcheck,gosec // best-effort flush; encoder writes to
		// os.Stdout so Close failure is not actionable for a CLI invocation.
		enc.Close()
	}()
	enc.SetIndent(2)
	if err := enc.Encode(appCfg); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	return nil
}
