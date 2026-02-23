package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/config"
	"github.com/donaldgifford/docz/internal/document"
)

var (
	createStatus   string
	createAuthor   string
	createNoUpdate bool
)

var createCmd = &cobra.Command{
	Use:   "create <type> <title>",
	Short: "Create a new document from a template",
	Long: `Create a new document of the specified type with an auto-incremented ID.

Types: rfc, adr, design, impl

Examples:
  docz create rfc "API Rate Limiting Strategy"
  docz create adr "Use PostgreSQL for Primary Storage"
  docz create design "User Authentication Flow"
  docz create impl "Migrate to gRPC"`,
	Args: cobra.ExactArgs(2),
	RunE: runCreate,
}

func init() {
	createCmd.Flags().StringVar(&createStatus, "status", "", "initial status (default varies by type)")
	createCmd.Flags().StringVar(&createAuthor, "author", "", "document author (default: git user.name)")
	createCmd.Flags().BoolVar(&createNoUpdate, "no-update", false, "skip automatic index update after creation")
	rootCmd.AddCommand(createCmd)
}

func runCreate(_ *cobra.Command, args []string) error {
	docType := strings.ToLower(args[0])
	title := args[1]

	tc, ok := appCfg.Types[docType]
	if !ok {
		return fmt.Errorf("unknown document type %q (valid types: %s)",
			docType, strings.Join(config.ValidTypes(), ", "))
	}

	if !tc.Enabled {
		return fmt.Errorf("document type %q is disabled in configuration", docType)
	}

	author := resolveAuthor()
	status := createStatus
	if status == "" && len(tc.Statuses) > 0 {
		status = tc.Statuses[0]
	}

	opts := document.CreateOptions{
		Type:         docType,
		Title:        title,
		Author:       author,
		Status:       status,
		Prefix:       tc.IDPrefix,
		IDWidth:      tc.IDWidth,
		DocsDir:      appCfg.DocsDir,
		TypeDir:      tc.Dir,
		TemplatePath: tc.Template,
	}

	result, err := document.Create(&opts)
	if err != nil {
		return err
	}

	fmt.Printf("Created %s: %s\n", strings.ToUpper(docType), result.FilePath)

	if !createNoUpdate && appCfg.Index.AutoUpdate {
		if err := updateType(docType); err != nil {
			return fmt.Errorf("auto-updating index: %w", err)
		}
	}

	return nil
}

// resolveAuthor determines the document author from (in order):
// flag, config default, git config user.name, fallback.
func resolveAuthor() string {
	if createAuthor != "" {
		return createAuthor
	}

	if appCfg.Author.Default != "" {
		return appCfg.Author.Default
	}

	if appCfg.Author.FromGit {
		if name := gitUserName(); name != "" {
			return name
		}
	}

	return "Unknown"
}

func gitUserName() string {
	out, err := exec.CommandContext(context.Background(), "git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
