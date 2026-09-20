package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize docz in the current repository",
	Long: `Create a .docz.yaml configuration file and set up the documentation
directory structure with default README index files for each document type.

If .docz.yaml already exists and declares a top-level "types:" block,
only the types listed there are scaffolded. Omit the "types:" block (or
delete .docz.yaml entirely and let init regenerate it) to scaffold all
five built-in types.

Existing README files are not overwritten unless --force is passed.
An existing .docz.yaml is never overwritten, with or without --force.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&forceInit, "force", false, "overwrite existing README index files")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, _ []string) error {
	return getRunner().initRepo(cmdContext(cmd), forceInit)
}

// Init scaffolds .docz.yaml plus a README index per enabled doc type.
// Existing files are skipped unless force is true.
func (r *Runner) Init(force bool) error {
	return r.initRepo(context.Background(), force)
}

// initRepo hands the scaffolding to repo.Init and prints its report.
//
// The report is printed even when the operation failed partway: repo.Init
// fills in every file it got to, and those files are on disk whether or
// not the run finished.
func (r *Runner) initRepo(ctx context.Context, force bool) error {
	rp := r.repoOrOpen()

	report, err := rp.Init(ctx, repo.InitOptions{Force: force})

	// The scaffolding error outranks a failure to print the report about it.
	// A write to r.Out failing is worth reporting when it is the only thing
	// that went wrong, and worth dropping when it is not: "cannot write to
	// stdout" in place of "could not create docs/rfc/README.md" would name
	// the symptom and lose the cause.
	if perr := r.printInitReport(rp, report); perr != nil && err == nil {
		err = perr
	}

	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(r.Out, "Initialized docz successfully.")

	return err
}

// printInitReport writes one line per file that was actually written.
//
// A skipped file gets no line, only the FileSkipped hook's debug record —
// which is what `docz init` on an already-initialised repository has
// always done, and why it prints a single success line there rather than a
// wall of "left alone" notices.
//
// Paths are absolutized back before printing. repo reports every path
// relative to the root, because that is the useful form for a consumer
// rendering a report; the CLI has always printed the absolute one, and a
// user who copies a line into an editor needs it to resolve.
func (r *Runner) printInitReport(rp *repo.Repo, report repo.InitReport) error {
	for _, f := range report.Files {
		switch f.Action {
		case repo.InitCreated, repo.InitOverwritten:
			if _, err := fmt.Fprintf(r.Out, "Created %s\n", rp.Path(f.Path)); err != nil {
				return err
			}
		case repo.InitSkipped:
		}
	}

	return nil
}
