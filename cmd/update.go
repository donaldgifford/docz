package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/index"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/toc"
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

func runUpdate(cmd *cobra.Command, args []string) error {
	return getRunner().update(cmdContext(cmd), updateDryRun, args)
}

// Update is the `docz update` handler. With no args it iterates every
// enabled type; with one arg it updates only that type.
//
// Kept at this signature for the tests that call it; the work is in
// update, which takes the context the RunE wrapper has and this one does
// not.
func (r *Runner) Update(dryRun bool, args []string) error {
	return r.update(context.Background(), dryRun, args)
}

// update resolves the type argument and hands the whole operation to
// repo.Update, then prints its report.
//
// The resolution stays here rather than being left to repo for two
// reasons, both about preserving what the CLI already does:
//
//   - config.ValidateType's "unknown document type" error lists the
//     built-in catalogue, where repo.UnknownTypeError lists the enabled
//     set. The former is the message users have been reading.
//   - A type named explicitly but switched off has always been a quiet
//     no-op here. repo.Update reports TypeDisabledError for it, on the
//     grounds that the user asked for that type by name — the right call
//     for a library, and not the CLI's existing behaviour.
//
// A nil types slice is what tells repo.Update to walk every enabled type,
// so the no-argument path does not enumerate them here either.
func (r *Runner) update(ctx context.Context, dryRun bool, args []string) error {
	var types []string

	if len(args) > 0 {
		typeName, err := r.Cfg.ValidateType(args[0])
		if err != nil {
			return err
		}

		if tc, ok := r.Cfg.Types[typeName]; !ok || !tc.Enabled {
			r.Logger.Debug("type skipped",
				"type", typeName,
				"reason", repo.SkipTypeDisabled.String(),
			)

			return nil
		}

		types = []string{typeName}
	}

	report, err := r.repoOrOpen().Update(ctx, types, repo.UpdateOptions{DryRun: dryRun})

	// Printed before the error is returned, and from the report rather
	// than as the work happens: a failed or cancelled run fills in every
	// type that finished, and those are types whose README really was
	// rewritten. Swallowing their lines would leave the user with a
	// failure and no idea how far it got.
	if perr := r.printUpdateReport(report); perr != nil {
		return perr
	}

	return err
}

// updateType updates a single type by canonical name.
//
// A thin wrapper over the same repo.Update the no-argument path uses, so
// `docz update rfc` and one iteration of `docz update` cannot drift. Kept
// at this signature because the update tests and BenchmarkCmdUpdate call
// it directly.
func (r *Runner) updateType(typeName string, dryRun bool) error {
	return r.update(context.Background(), dryRun, []string{typeName})
}

// printUpdateReport writes the user-facing lines for every type the
// operation completed, in the order it processed them.
func (r *Runner) printUpdateReport(report repo.UpdateReport) error {
	for i := range report.Types {
		if err := r.printTypeReport(&report.Types[i]); err != nil {
			return err
		}
	}

	return nil
}

// printTypeReport writes one type's lines: the table-of-contents pass
// first, then the README index outcome.
//
// The order matters and is the order the operation itself ran in — a
// dry run that listed the index diff before the documents it would also
// rewrite would read as though the documents were an afterthought.
//
// Taken by pointer because TypeReport embeds two nested reports and
// gocritic counts the bytes.
func (r *Runner) printTypeReport(tr *repo.TypeReport) error {
	if tr.ToC != nil {
		r.printToCReport(tr.ToC)
	}

	return r.printIndexOutcome(tr.Index)
}

// printToCReport writes the table-of-contents pass's user-facing lines
// and logs the rest.
//
// Only dry-run lines and write failures are worth a user's attention; a
// document whose ToC was rewritten is reported by the FileWritten hook at
// debug level, and one that was already current is logged here. Neither
// is news at the default level, which is why `docz update` on a clean
// repository prints one line per type rather than one per document.
func (r *Runner) printToCReport(report *toc.UpdateReport) {
	for _, fr := range report.WouldUpdate {
		//nolint:errcheck // user-facing dry-run line; write failures
		// would surface again on the next normal write.
		fmt.Fprintf(r.Out, "Would update ToC in %s (%d headings)\n", fr.Path, fr.Headings)
	}

	for _, fr := range report.Unchanged {
		r.Logger.Debug("ToC unchanged", "path", fr.Path)
	}

	for _, fe := range report.WriteErrors {
		//nolint:errcheck // warning to stderr; nothing actionable if the
		// warning itself fails to print.
		fmt.Fprintf(r.Err, "Warning: writing ToC to %s: %v\n", fe.Path, fe.Err)
	}
}

// printIndexOutcome translates the typed index.UpdateOutcome into a
// user-facing message on r.Out. The index package is
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
