package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/document"
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

func runStatusSet(_ *cobra.Command, args []string) error {
	opts := statusSetOpts{
		dryRun: statusDryRun,
		quiet:  statusQuiet,
		format: statusFormat,
	}
	return getRunner().statusSet(opts, args)
}

// statusSet runs the `docz status set` resolution algorithm: validate the
// type, locate the document by frontmatter id, validate the requested
// status against the type's lifecycle, then mutate the file unless the
// status is unchanged or --dry-run is set. See DESIGN-0005 §Resolution
// algorithm.
func (r *Runner) statusSet(opts statusSetOpts, args []string) error {
	typeArg, idArg, newStatus := args[0], args[1], args[2]

	format, err := resolveStatusFormat(opts.format)
	if err != nil {
		return err
	}

	typeName, err := r.Cfg.ValidateType(typeArg)
	if err != nil {
		return exitErrorf(errExitCode2, "%v", err)
	}
	statuses := r.Cfg.Types[typeName].Statuses

	typeDir := r.inRepo(r.Cfg.TypeDir(typeName))
	docs, err := document.ScanDocuments(typeDir)
	if err != nil {
		return exitErrorf(errExitCode1, "scanning %s: %v", r.relPath(typeDir), err)
	}

	entry := findByID(docs, idArg)
	if entry == nil {
		return exitErrorf(errExitCode1,
			"no %s document with id %q found in %s",
			typeName, idArg, r.relPath(typeDir))
	}

	if !slices.Contains(statuses, newStatus) {
		return exitErrorf(errExitCode2,
			"%q is not a valid status for %s.\nValid statuses: %s.",
			newStatus, typeName, strings.Join(statuses, ", "))
	}

	docPath := filepath.Join(typeDir, entry.Filename)
	res := statusResult{
		path:    r.relPath(docPath),
		from:    string(entry.Status),
		to:      newStatus,
		dryRun:  opts.dryRun,
		changed: string(entry.Status) != newStatus,
		quiet:   opts.quiet,
		format:  format,
	}

	if res.changed && !opts.dryRun {
		if _, err := document.SetStatus(docPath, newStatus); err != nil {
			return statusWriteError(err)
		}
	}
	return r.emitStatus(res)
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

// findByID returns the entry whose frontmatter id exactly matches id
// (case-sensitive, Decision 3), or nil when none does.
func findByID(docs []document.DocEntry, id string) *document.DocEntry {
	for i := range docs {
		if docs[i].ID == id {
			return &docs[i]
		}
	}
	return nil
}

// statusWriteError maps a document.SetStatus failure to the right exit
// code: unsupported line endings are a validation failure (exit 2,
// Decision 7); everything else (missing frontmatter, IO) is a lookup or
// write failure (exit 1).
func statusWriteError(err error) error {
	if errors.Is(err, document.ErrUnsupportedLineEndings) {
		return exitErrorf(errExitCode2, "%v", err)
	}
	return exitErrorf(errExitCode1, "%v", err)
}

// relPath renders p relative to r.RepoRoot for display (Decision 3),
// falling back to p when RepoRoot is unset or p is on another root.
func (r *Runner) relPath(p string) string {
	if r.RepoRoot == "" {
		return p
	}
	rel, err := filepath.Rel(r.RepoRoot, p)
	if err != nil {
		return p
	}
	return rel
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
