package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/config"
	"github.com/donaldgifford/docz/internal/document"
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

func runList(_ *cobra.Command, args []string) error {
	return getRunner().List(listOpts{status: listStatus, format: listFormat}, args)
}

// List gathers documents across one or all types, applies any status
// filter, and emits them through r.Out in the requested format.
func (r *Runner) List(opts listOpts, args []string) error {
	types := r.Cfg.EnabledTypes()
	if len(args) > 0 {
		typeName, err := r.Cfg.ValidateType(args[0])
		if err != nil {
			return err
		}
		types = []string{typeName}
	}

	var entries []listEntry
	for _, typeName := range types {
		typeDir := r.Cfg.TypeDir(typeName)
		docs, err := document.ScanDocuments(typeDir)
		if err != nil {
			return fmt.Errorf("scanning %s: %w", typeDir, err)
		}
		for _, doc := range docs {
			entries = append(entries, listEntry{
				ID:      doc.ID,
				Title:   doc.Title,
				Status:  string(doc.Status),
				Date:    doc.Created,
				Author:  doc.Author,
				Type:    strings.ToUpper(typeName),
				File:    doc.Filename,
				TypeDir: typeDir,
			})
		}
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
