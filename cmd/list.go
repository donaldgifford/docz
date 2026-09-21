package cmd

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

const formatJSON = "json"

var (
	listStatus string
	listFormat string
)

type listEntry struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Date    string `json:"date"`
	Author  string `json:"author"`
	Type    string `json:"type"`
	File    string `json:"file"`
	TypeDir string `json:"-"`
}

// listOpts holds the per-invocation flag values for `docz list`.
type listOpts struct {
	status string
	format string
}

var listCmd = &cobra.Command{
	Use:   "list [type]",
	Short: "List documents, optionally filtered by type",
	Long: `List all documents across all types, or filter by a specific type.

` + config.TypesHelp(),
	Args: cobra.MaximumNArgs(1),
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVar(&listStatus, "status", "", "filter by status (case-insensitive)")
	listCmd.Flags().StringVar(&listFormat, "format", "table", "output format: table, json, csv")
	rootCmd.AddCommand(listCmd)
}

// repoOrOpen returns the Repo this handler orchestrates through.
//
// Production sets Runner.Repo in PersistentPreRunE: loadAndValidateConfig
// calls repo.Open and then points Repo.Cfg back at Runner.Cfg, so for a
// real invocation this is the identity. cmd tests construct a Runner
// directly — a bytes.Buffer for Out, a t.TempDir for RepoRoot, no Repo —
// so the fallback builds one from the two fields that describe a
// repository anyway: the resolved config and the root its relative paths
// join under. Building it here rather than dereferencing a nil field is
// what keeps a test that only cares about formatting from having to know
// this field exists.
func (r *Runner) repoOrOpen() *repo.Repo {
	if r.Repo != nil {
		return r.Repo
	}
	return &repo.Repo{Root: r.RepoRoot, Cfg: &r.Cfg}
}

func runList(cmd *cobra.Command, args []string) error {
	return getRunner().List(
		cmdContext(cmd),
		listOpts{status: listStatus, format: listFormat},
		args,
	)
}

// List gathers documents across one or all types, applies any status
// filter, and emits them through r.Out in the requested format.
//
// The gathering is repo.List: a nil or empty args slice means every
// enabled type in EnabledTypes order, and a token is resolved there with
// the same precedence the rest of the CLI uses, so an alias or an
// id_prefix still names a type and an unresolvable one still reports the
// same "unknown document type" line config.ValidateType produced.
func (r *Runner) List(ctx context.Context, opts listOpts, args []string) error {
	rp := r.repoOrOpen()

	docs, err := rp.List(ctx, args)
	if err != nil && !listSkippableErr(err) {
		return err
	}

	// Nil rather than an empty slice when nothing matched: `--format json`
	// has always encoded an empty listing as `null`, and make() here would
	// silently change that to `[]`.
	var entries []listEntry
	for i := range docs {
		doc := &docs[i]
		entries = append(entries, listEntry{
			ID:      doc.ID,
			Title:   doc.Title,
			Status:  string(doc.Status),
			Date:    doc.Created,
			Author:  doc.Author,
			Type:    strings.ToUpper(doc.Type),
			File:    doc.Filename,
			TypeDir: rp.TypeDir(doc.Type),
		})
	}

	if opts.status != "" {
		entries = filterByStatus(entries, opts.status)
	}

	switch strings.ToLower(opts.format) {
	case formatJSON:
		return outputJSON(r.Out, entries)
	case "csv":
		return outputCSV(r.Out, entries)
	default:
		return outputTable(r.Out, entries)
	}
}

// listSkippableErr reports whether a repo.List failure is one `docz list`
// has never surfaced to the user.
//
// A disabled type is the only case. Before the swap the handler resolved
// its argument with config.ValidateType, which maps a token to a canonical
// name without consulting Enabled, so `docz list <disabled-type>` scanned
// the directory anyway. repo draws the distinction the rest of v2 wants —
// Scan on a disabled type is a TypeDisabledError rather than an empty
// slice, because the two mean different things — and no method scans past
// it, so swallowing the error is as close as the API gets: the type
// contributes no entries and the listing still prints with exit 0, which
// is byte for byte what the pre-swap handler produced for the dormant
// block a v1 repo actually carries (no directory was ever scaffolded for
// it, so there was nothing to list). The one case that does differ is a
// disabled type whose directory holds documents: those used to appear and
// now do not. Reporting the error instead would turn a successful
// invocation into exit 1, which is the larger change of the two.
func listSkippableErr(err error) bool {
	var disabled *repo.TypeDisabledError

	return errors.As(err, &disabled)
}

func filterByStatus(entries []listEntry, status string) []listEntry {
	var filtered []listEntry
	for i := range entries {
		if strings.EqualFold(entries[i].Status, status) {
			filtered = append(filtered, entries[i])
		}
	}
	return filtered
}

func outputTable(out io.Writer, entries []listEntry) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tDATE\tAUTHOR\tTYPE"); err != nil {
		return fmt.Errorf("writing table header: %w", err)
	}
	if _, err := fmt.Fprintln(w, "--\t-----\t------\t----\t------\t----"); err != nil {
		return fmt.Errorf("writing table separator: %w", err)
	}
	for i := range entries {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			entries[i].ID, entries[i].Title, entries[i].Status, entries[i].Date,
			entries[i].Author, entries[i].Type); err != nil {
			return fmt.Errorf("writing table row %d: %w", i, err)
		}
	}
	return w.Flush()
}

func outputJSON(out io.Writer, entries []listEntry) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func outputCSV(out io.Writer, entries []listEntry) error {
	w := csv.NewWriter(out)
	if err := w.Write([]string{"ID", "Title", "Status", "Date", "Author", "Type", "File"}); err != nil {
		return fmt.Errorf("writing csv header: %w", err)
	}
	for i := range entries {
		if err := w.Write([]string{
			entries[i].ID, entries[i].Title, entries[i].Status,
			entries[i].Date, entries[i].Author, entries[i].Type, entries[i].File,
		}); err != nil {
			return fmt.Errorf("writing csv row %d: %w", i, err)
		}
	}
	w.Flush()
	return w.Error()
}
