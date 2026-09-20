package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docwrite"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// formatText is the default --format value for `status set`; formatJSON
// is defined in list.go and reused here.
const formatText = "text"

// errExitCode1 and errExitCode2 are exit-code marker sentinels returned
// (wrapped) by statusSet. Execute in root.go reads the marker with
// errors.Is to select os.Exit(1) for lookup or write failures and
// os.Exit(2) for validation failures (DESIGN-0005 §Exit codes,
// Decision 6).
var (
	errExitCode1 = errors.New("status set lookup or write failed")
	errExitCode2 = errors.New("status set validation failed")
)

// exitCodeError pairs a user-facing message with an exit-code marker.
// Cobra prints Error() to stderr in the standard "Error: <message>"
// form, while Execute unwraps the marker to choose the process exit
// code. Carrying the code in the error rather than calling os.Exit in
// the handler keeps statusSet unit-testable (Decision 6).
type exitCodeError struct {
	msg    string
	marker error
}

func (e *exitCodeError) Error() string { return e.msg }

func (e *exitCodeError) Unwrap() error { return e.marker }

// exitErrorf builds an exitCodeError carrying marker and a formatted
// user-facing message.
func exitErrorf(marker error, format string, a ...any) error {
	return &exitCodeError{msg: fmt.Sprintf(format, a...), marker: marker}
}

var (
	statusDryRun bool
	statusQuiet  bool
	statusFormat string
)

// statusSetOpts captures the per-invocation flag values for
// `docz status set`, packed by runStatusSet so the handler method never
// reads a package global (matching createOpts / listOpts).
type statusSetOpts struct {
	dryRun bool
	quiet  bool
	format string
}

// statusResult is the outcome of a status set, shared by the text and
// JSON emitters.
type statusResult struct {
	path    string
	from    string
	to      string
	dryRun  bool
	changed bool
	quiet   bool
	format  string
}

// statusJSON is the machine-readable shape emitted by
// `status set --format=json`. On a no-op From == To and Changed is false;
// on --dry-run DryRun is true and Changed still reports what would have
// happened (DESIGN-0005 §Output format, Decision 7).
type statusJSON struct {
	Path    string `json:"path"`
	From    string `json:"from"`
	To      string `json:"to"`
	DryRun  bool   `json:"dry_run"`
	Changed bool   `json:"changed"`
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Inspect and mutate document status",
	Long: `Parent command for status operations.

The set subcommand mutates a document's frontmatter status field with
lifecycle validation against .docz.yaml.`,
}

var statusSetCmd = &cobra.Command{
	Use:   "set <type> <id> <new-status>",
	Short: "Set a document's status, validated against its lifecycle",
	Long: `Set the status field in a document's YAML frontmatter.

The document is located by its frontmatter id (case-sensitive exact
match), and the new status must appear in the type's configured statuses
list. Only the status value bytes change; the rest of the frontmatter is
preserved.

Exit codes: 0 on success, no-op, or --dry-run; 1 when the id is not found
or a write fails; 2 for an unknown type, an invalid status, or a file
with unsupported line endings.`,
	Args: cobra.ExactArgs(3),
	RunE: runStatusSet,
}

func init() {
	statusSetCmd.Flags().BoolVar(&statusDryRun, "dry-run", false,
		"print the change without writing the file")
	statusSetCmd.Flags().BoolVar(&statusQuiet, "quiet", false,
		"suppress success output on stdout (exit code unchanged)")
	statusSetCmd.Flags().StringVar(&statusFormat, "format", formatText,
		"output format: text or json")
	statusCmd.AddCommand(statusSetCmd)
	rootCmd.AddCommand(statusCmd)
}

func runStatusSet(cmd *cobra.Command, args []string) error {
	opts := statusSetOpts{
		dryRun: statusDryRun,
		quiet:  statusQuiet,
		format: statusFormat,
	}
	return getRunner().statusSetCtx(cmdContext(cmd), opts, args)
}

// statusSet is the context-free entry point `docz status set`'s tests call.
// The signature predates the repo tier and is kept: a handler that only
// ever ran to completion had nothing to cancel, so there was no context to
// take. statusSetCtx below is the real handler.
func (r *Runner) statusSet(opts statusSetOpts, args []string) error {
	return r.statusSetCtx(context.Background(), opts, args)
}

// statusSetCtx runs the `docz status set` resolution algorithm through
// repo.SetStatus, which performs it in the order DESIGN-0005 §Resolution
// algorithm fixed: resolve the type, locate the document by frontmatter id,
// validate the requested status against the type's lifecycle, then mutate
// the file unless the status is unchanged or --dry-run is set.
//
// Everything left here is cmd's own: --format membership, the wording and
// exit code of each failure (statusExitError), and the emitted line. The
// `changed` field is computed from the returned pair rather than read off
// StatusResult.Changed, because repo reports Changed: false for a dry run —
// it describes whether bytes moved — while this command's output and JSON
// report what *would* have happened (DESIGN-0005 Decision 7).
func (r *Runner) statusSetCtx(ctx context.Context, opts statusSetOpts, args []string) error {
	typeArg, idArg, newStatus := args[0], args[1], args[2]

	format, err := resolveStatusFormat(opts.format)
	if err != nil {
		return err
	}

	rp := r.repoOrOpen()

	out, err := rp.SetStatus(ctx, typeArg, idArg, newStatus, repo.StatusOptions{
		DryRun: opts.dryRun,
	})
	if err != nil {
		return statusExitError(rp, err)
	}

	// out.Path is already relative to the repo root, which is the form this
	// command has always printed — relativizing it again would strip a
	// leading path element.
	return r.emitStatus(statusResult{
		path:    out.Path,
		from:    out.Old,
		to:      out.New,
		dryRun:  opts.dryRun,
		changed: out.Old != out.New,
		quiet:   opts.quiet,
		format:  format,
	})
}

// statusExitError maps a repo failure onto the message and exit code
// `docz status set` has always produced (DESIGN-0005 §Exit codes).
//
// Exit 2 is the validation family — an unresolvable type, a disabled type,
// a status outside the lifecycle, unsupported line endings — and exit 1 is
// the lookup-or-write family. The wording is assembled from the typed
// errors' fields rather than from their Error() strings wherever the two
// differ: repo's NotFoundError cannot name the directory it searched (only
// cmd knows how to shorten a path for display) and its InvalidStatusError
// lists the valid statuses on one line where this command uses two.
func statusExitError(rp *repo.Repo, err error) error {
	var unknown *repo.UnknownTypeError
	if errors.As(err, &unknown) {
		// UnknownTypeError renders exactly what config.ValidateType did.
		return exitErrorf(errExitCode2, "%v", unknown)
	}

	var disabled *repo.TypeDisabledError
	if errors.As(err, &disabled) {
		return exitErrorf(errExitCode2, "%v", disabled)
	}

	var notFound *repo.NotFoundError
	if errors.As(err, &notFound) {
		return exitErrorf(errExitCode1,
			"no %s document with id %q found in %s",
			notFound.Type, notFound.ID, rp.RelPath(rp.TypeDir(notFound.Type)))
	}

	var badStatus *repo.InvalidStatusError
	if errors.As(err, &badStatus) {
		return exitErrorf(errExitCode2,
			"%q is not a valid status for %s.\nValid statuses: %s.",
			badStatus.Status, badStatus.Type, strings.Join(badStatus.Allowed, ", "))
	}

	// The wrapped cause, not the WriteError: docwrite already names the file
	// it could not write, and repo's "writing <path>: " prefix would say it
	// a second time.
	var write *repo.WriteError
	if errors.As(err, &write) {
		return statusWriteError(write.Err)
	}

	// A scan failure or a cancelled context. repo.Scan's message already
	// reads "scanning <relpath>: …", which is what this command printed.
	return exitErrorf(errExitCode1, "%v", err)
}

// resolveStatusFormat validates --format membership (Decision 2),
// defaulting an empty value to text and rejecting anything other than
// text or json with an exit-code-2 error.
func resolveStatusFormat(format string) (string, error) {
	switch strings.ToLower(format) {
	case "", formatText:
		return formatText, nil
	case formatJSON:
		return formatJSON, nil
	default:
		return "", exitErrorf(errExitCode2,
			"invalid --format %q (want %q or %q)", format, formatText, formatJSON)
	}
}

// statusWriteError maps a docwrite.SetStatus failure to the right exit
// code: unsupported line endings are a validation failure (exit 2,
// Decision 7); everything else (missing frontmatter, IO) is a lookup or
// write failure (exit 1).
func statusWriteError(err error) error {
	if errors.Is(err, docwrite.ErrUnsupportedLineEndings) {
		return exitErrorf(errExitCode2, "%v", err)
	}
	return exitErrorf(errExitCode1, "%v", err)
}

// emitStatus writes the result through r.Out in the configured format.
// --quiet suppresses all stdout (the exit code is the contract); errors
// always go to stderr as plain text, never JSON (DESIGN-0005 §Output
// format).
func (r *Runner) emitStatus(res statusResult) error {
	if res.quiet {
		return nil
	}
	if res.format == formatJSON {
		return r.emitStatusJSON(res)
	}
	_, err := fmt.Fprintln(r.Out, formatStatusText(res))
	return err
}

// emitStatusJSON writes a single-line JSON object terminated by a newline
// (DESIGN-0005 §Output format). It uses json.Marshal so the object is one
// line; consumers branch on the `changed` field.
func (r *Runner) emitStatusJSON(res statusResult) error {
	data, err := json.Marshal(statusJSON{
		Path:    res.path,
		From:    res.from,
		To:      res.to,
		DryRun:  res.dryRun,
		Changed: res.changed,
	})
	if err != nil {
		return fmt.Errorf("marshaling status json: %w", err)
	}
	_, err = fmt.Fprintf(r.Out, "%s\n", data)
	return err
}

// formatStatusText renders the human-readable status line:
//
//	<relpath>: status <old> -> <new>   (changed)
//	<relpath>: already at <status>     (no-op)
//
// prefixed with "[dry-run] " when dryRun is set.
func formatStatusText(res statusResult) string {
	prefix := ""
	if res.dryRun {
		prefix = "[dry-run] "
	}
	if res.changed {
		return fmt.Sprintf("%s%s: status %s -> %s", prefix, res.path, res.from, res.to)
	}
	return fmt.Sprintf("%s%s: already at %s", prefix, res.path, res.to)
}
