package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage document templates",
	Long: `View, export, and override document templates.

Subcommands:
  show      Print the resolved template for a type
  export    Write the resolved template to a file
  override  Copy the resolved template into the local overrides directory`,
}

var templateShowCmd = &cobra.Command{
	Use:   "show <type>",
	Short: "Print the resolved template for a document type",
	Long: `Print the resolved template for the given document type to stdout.

The template is resolved in order: config path > local override > embedded default.

Types: ` + strings.Join(config.DocTypeNames(), ", "),
	Args: cobra.ExactArgs(1),
	RunE: runTemplateShow,
}

var templateExportCmd = &cobra.Command{
	Use:   "export <type> [path]",
	Short: "Export the resolved template to a file",
	Long: `Write the resolved template for the given type to a file.

If no path is specified, the file is written to ./<type>.md in the current directory.

Types: ` + strings.Join(config.DocTypeNames(), ", "),
	Args: cobra.RangeArgs(1, 2),
	RunE: runTemplateExport,
}

var templateOverrideCmd = &cobra.Command{
	Use:   "override <type>",
	Short: "Copy the resolved template into the local overrides directory",
	Long: `Copy the resolved template into <docs_dir>/templates/<type>.md so you can
edit it locally. Future document creation will use this override.

Fails if the override file already exists.

Types: ` + strings.Join(config.DocTypeNames(), ", "),
	Args: cobra.ExactArgs(1),
	RunE: runTemplateOverride,
}

func init() {
	templateCmd.AddCommand(templateShowCmd)
	templateCmd.AddCommand(templateExportCmd)
	templateCmd.AddCommand(templateOverrideCmd)
	rootCmd.AddCommand(templateCmd)
}

func runTemplateShow(cmd *cobra.Command, args []string) error {
	return getRunner().TemplateShow(cmdContext(cmd), args)
}

func runTemplateExport(cmd *cobra.Command, args []string) error {
	return getRunner().TemplateExport(cmdContext(cmd), args)
}

func runTemplateOverride(cmd *cobra.Command, args []string) error {
	return getRunner().TemplateOverride(cmdContext(cmd), args)
}

// TemplateShow prints the resolved template for the given document
// type to r.Out.
func (r *Runner) TemplateShow(ctx context.Context, args []string) error {
	docType, err := r.Cfg.ValidateType(args[0])
	if err != nil {
		return err
	}

	content, err := r.tmplResolve(ctx, docType)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(r.Out, content)
	return err
}

// TemplateExport writes the resolved template to a file (default
// ./<type>.md) and reports the path on r.Out.
//
// The destination is always passed to repo.ExportTemplate explicitly, so
// the scaffold-the-generic-pair branch (which only fires for an empty
// dest) is out of reach here: an unresolvable template is the same
// failure it has always been. Overwrite is set because os.WriteFile
// truncated an existing destination before the swap and `docz template
// export` run twice must keep working.
func (r *Runner) TemplateExport(ctx context.Context, args []string) error {
	docType, err := r.Cfg.ValidateType(args[0])
	if err != nil {
		return err
	}

	outPath := r.inRepo(docType + ".md")
	if len(args) > 1 {
		outPath = args[1]
	}

	if _, err := r.repoOrOpen().ExportTemplate(
		ctx, docType, outPath, repo.ExportOptions{Overwrite: true},
	); err != nil {
		if cause := tmplWriteCause(err); cause != nil {
			return fmt.Errorf("writing template to %s: %w", outPath, cause)
		}

		return err
	}

	_, err = fmt.Fprintf(r.Out, "Exported %s template to %s\n", docType, outPath)
	return err
}

// TemplateOverride copies the resolved template into
// <docs_dir>/templates/<type>.md, failing if the override file already
// exists.
//
// repo.ExportTemplate scaffolds the generic template-and-schema pair for
// a type whose template resolves nowhere (IMPL-0018 Phase 3), which is
// not what this command does: it copies a *resolved* template, and a
// type with none has nothing to copy. So the template is resolved first
// and a failure returned as it stands — the same error, naming the same
// path, that `docz create` names for the same type. Turning that failure
// into a scaffolded pair is a behaviour change, and
// TestLegacyPlan_TemplateOverrideNamesTheSamePath is where it would be
// decided.
func (r *Runner) TemplateOverride(ctx context.Context, args []string) error {
	docType, err := r.Cfg.ValidateType(args[0])
	if err != nil {
		return err
	}

	overridePath := r.tmplOverridePath(docType)

	if _, err := os.Stat(overridePath); err == nil {
		return tmplExistsErr(overridePath)
	}

	// The resolve that decides whether there is anything to copy. Its
	// error is repo.Template's, already wrapped "resolving <type>
	// template: …", so the message is unchanged by going through it.
	if _, err := r.tmplResolve(ctx, docType); err != nil {
		return err
	}

	rp := r.repoOrOpen()
	if _, err := rp.ExportTemplate(ctx, docType, "", repo.ExportOptions{}); err != nil {
		var exists *repo.ExistsError
		if errors.As(err, &exists) {
			return tmplExistsErr(overridePath)
		}

		if cause := tmplWriteCause(err); cause != nil {
			return fmt.Errorf("writing override template: %w", cause)
		}

		return err
	}

	_, err = fmt.Fprintf(r.Out, "Created override template: %s\n", overridePath)
	return err
}

// tmplResolve resolves a type's body template through the repo API,
// keeping the debug line the command emitted when it called
// doctemplate.Resolve itself.
func (r *Runner) tmplResolve(ctx context.Context, docType string) (string, error) {
	r.Logger.Debug("resolving template",
		"type", docType,
		"config_template_path", r.Cfg.Types[docType].Template,
		"docs_dir", r.Cfg.DocsDir,
	)

	return r.repoOrOpen().Template(ctx, docType)
}

// tmplOverridePath is the file `template override` writes:
// <docs_dir>/templates/<type>.md.
//
// inRepo mirrors repo.Path exactly — an absolute path passes through, an
// empty RepoRoot leaves the path relative — so the path this names is
// the path repo.ExportTemplate writes, and the message cannot describe a
// different file than the one on disk.
func (r *Runner) tmplOverridePath(docType string) string {
	return filepath.Join(r.inRepo(r.Cfg.DocsDir), config.TemplatesDir, docType+".md")
}

// tmplExistsErr is the refusal to clobber an existing override, worded
// as it was before the swap and naming the absolute path rather than the
// repo-relative one repo.ExistsError carries.
func tmplExistsErr(path string) error {
	return fmt.Errorf("override file already exists: %s", path)
}

// tmplWriteCause unwraps a *repo.WriteError to the failure underneath,
// reporting false for anything else.
//
// It exists so each caller can keep the wording it had before the swap:
// repo names the repo-relative path and says only "writing", while these
// commands said which operation failed and, for an export, which
// absolute destination. A resolution error is not a write error and is
// returned untouched — repo.Template already wraps one exactly the way
// this file used to, so a nil result means "not a write error, report it
// as it came".
func tmplWriteCause(err error) error {
	var write *repo.WriteError
	if errors.As(err, &write) {
		return write.Err
	}

	return nil
}
