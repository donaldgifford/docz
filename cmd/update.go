package cmd

import (
	"fmt"
	"path/filepath"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/config"
	"github.com/donaldgifford/docz/internal/document"
	"github.com/donaldgifford/docz/internal/index"
	doctemplate "github.com/donaldgifford/docz/internal/template"
	"github.com/donaldgifford/docz/internal/toc"
)

var updateDryRun bool

var updateCmd = &cobra.Command{
	Use:   "update [type]",
	Short: "Update index/README for a document type (or all types)",
	Long: `Regenerate the auto-generated table in the README.md for the specified
document type directory. If no type is given, all types are updated.

` + config.TypesHelp(),
	Args: cobra.MaximumNArgs(1),
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "show what would change without writing")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(_ *cobra.Command, args []string) error {
	return getRunner().Update(updateDryRun, args)
}

// Update is the `docz update` handler. With no args it iterates every
// enabled type; with one arg it updates only that type.
func (r *Runner) Update(dryRun bool, args []string) error {
	var types []string
	if len(args) > 0 {
		typeName, err := r.Cfg.ValidateType(args[0])
		if err != nil {
			return err
		}
		types = []string{typeName}
	} else {
		types = r.Cfg.EnabledTypes()
	}

	for _, typeName := range types {
		tc, ok := r.Cfg.Types[typeName]
		if !ok || !tc.Enabled {
			r.Logger.Debug("type disabled, skipping", "type", typeName)
			continue
		}

		if err := r.updateType(typeName, dryRun); err != nil {
			return fmt.Errorf("updating %s: %w", typeName, err)
		}
	}

	return nil
}

func (r *Runner) updateType(typeName string, dryRun bool) error {
	tc := r.Cfg.Types[typeName]
	typeDir := r.Cfg.TypeDir(typeName)
	readmePath := filepath.Join(typeDir, config.IndexFileName)

	r.Logger.Debug("scanning type", "dir", typeDir)

	docs, err := document.ScanDocuments(typeDir)
	if err != nil {
		return fmt.Errorf("scanning %s: %w", typeDir, err)
	}

	r.Logger.Debug("scan complete", "type", typeName, "count", len(docs))

	if r.Cfg.TOC.Enabled {
		r.runToCUpdate(typeDir, docs, dryRun)
	}

	label := indexLabel(tc.PluralLabel, typeName)
	heading := "All " + label
	tableContent := index.GenerateTable(docs, heading)

	header, err := doctemplate.ResolveIndexHeader(typeName, r.Cfg.DocsDir, doctemplate.IndexHeaderData{
		TypeName:    typeName,
		PluralLabel: label,
	})
	if err != nil {
		return fmt.Errorf("resolving index header for %s: %w", typeName, err)
	}

	if dryRun {
		outcome, err := index.DryRunReadme(readmePath, header, tableContent)
		if err != nil {
			return fmt.Errorf("dry-run readme %s: %w", readmePath, err)
		}
		return r.printIndexOutcome(outcome)
	}

	outcome, err := index.UpdateReadme(readmePath, header, tableContent)
	if err != nil {
		return fmt.Errorf("updating readme %s: %w", readmePath, err)
	}
	return r.printIndexOutcome(outcome)
}

// indexLabel is the display label for a type's index header and table
// heading: the configured plural_label, or a Title-cased type name when the
// type (typically a custom one) declares no plural_label (DESIGN-0006
// Decision 3).
func indexLabel(pluralLabel, typeName string) string {
	if pluralLabel != "" {
		return pluralLabel
	}
	if typeName == "" {
		return typeName
	}
	r := []rune(typeName)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// printIndexOutcome translates the typed index.UpdateOutcome into a
// user-facing message on r.Out. The internal/index package is
// intentionally silent on English wording — that lives here.
func (r *Runner) printIndexOutcome(o index.UpdateOutcome) error {
	switch o.Action {
	case index.ActionCreated:
		_, err := fmt.Fprintf(r.Out, "Created %s\n", o.Path)
		return err
	case index.ActionUpdated:
		_, err := fmt.Fprintf(r.Out, "Updated %s\n", o.Path)
		return err
	case index.ActionNoMarkers:
		_, err := fmt.Fprintf(
			r.Out,
			"Warning: %s has no DOCZ auto-generated markers. "+
				"Run 'docz init --force' or manually add markers to update it.\n",
			o.Path,
		)
		return err
	case index.ActionDryRunCreated, index.ActionDryRunUpdated:
		_, err := fmt.Fprintln(r.Out, o.Body)
		return err
	}
	return nil
}

// runToCUpdate builds the toc.FileInput list from cached scan results,
// delegates to toc.UpdateFiles, and formats user-facing messages so the
// internal/toc package stays free of I/O-shaped strings.
func (r *Runner) runToCUpdate(typeDir string, docs []document.DocEntry, dryRun bool) {
	if len(docs) == 0 {
		return
	}

	files := make([]toc.FileInput, len(docs))
	for i, doc := range docs {
		files[i] = toc.FileInput{
			Path:    filepath.Join(typeDir, doc.Filename),
			Content: doc.Content,
		}
	}

	report, err := toc.UpdateFiles(files, r.Cfg.TOC.MinHeadings, dryRun)
	if err != nil {
		//nolint:errcheck // warning to stderr; nothing actionable
		// if the warning itself fails to print.
		fmt.Fprintf(r.Err, "Warning: ToC update failed: %v\n", err)
		return
	}

	for _, fr := range report.WouldUpdate {
		//nolint:errcheck // user-facing dry-run line; write failures
		// would surface again on the next normal write.
		fmt.Fprintf(r.Out, "Would update ToC in %s (%d headings)\n", fr.Path, fr.Headings)
	}

	for _, fr := range report.Updated {
		r.Logger.Debug("ToC updated", "path", fr.Path)
	}
	for _, fr := range report.Unchanged {
		r.Logger.Debug("ToC unchanged", "path", fr.Path)
	}

	for _, fe := range report.WriteErrors {
		//nolint:errcheck // warning to stderr; see above.
		fmt.Fprintf(r.Err, "Warning: writing ToC to %s: %v\n", fe.Path, fe.Err)
	}
}
