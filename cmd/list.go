package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/donaldgifford/docz/internal/config"
	"github.com/donaldgifford/docz/internal/index"
)

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

var listCmd = &cobra.Command{
	Use:   "list [type]",
	Short: "List documents, optionally filtered by type",
	Long: `List all documents across all types, or filter by a specific type.

Types: rfc, adr, design, impl`,
	Args: cobra.MaximumNArgs(1),
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVar(&listStatus, "status", "", "filter by status (case-insensitive)")
	listCmd.Flags().StringVar(&listFormat, "format", "table", "output format: table, json, csv")
	rootCmd.AddCommand(listCmd)
}

func runList(_ *cobra.Command, args []string) error {
	types := config.ValidTypes()
	if len(args) > 0 {
		typeName := strings.ToLower(args[0])
		if _, ok := appCfg.Types[typeName]; !ok {
			return fmt.Errorf("unknown document type %q (valid types: %s)",
				typeName, strings.Join(config.ValidTypes(), ", "))
		}
		types = []string{typeName}
	}

	var entries []listEntry
	for _, typeName := range types {
		typeDir := appCfg.TypeDir(typeName)
		docs, err := index.ScanDocuments(typeDir)
		if err != nil {
			return fmt.Errorf("scanning %s: %w", typeDir, err)
		}
		for _, doc := range docs {
			entries = append(entries, listEntry{
				ID:      doc.ID,
				Title:   doc.Title,
				Status:  doc.Status,
				Date:    doc.Created,
				Author:  doc.Author,
				Type:    strings.ToUpper(typeName),
				File:    doc.Filename,
				TypeDir: typeDir,
			})
		}
	}

	if listStatus != "" {
		entries = filterByStatus(entries, listStatus)
	}

	switch strings.ToLower(listFormat) {
	case "json":
		return outputJSON(entries)
	case "csv":
		return outputCSV(entries)
	default:
		return outputTable(entries)
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

func outputTable(entries []listEntry) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tDATE\tAUTHOR\tTYPE"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "--\t-----\t------\t----\t------\t----"); err != nil {
		return err
	}
	for i := range entries {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			entries[i].ID, entries[i].Title, entries[i].Status, entries[i].Date,
			entries[i].Author, entries[i].Type); err != nil {
			return err
		}
	}
	return w.Flush()
}

func outputJSON(entries []listEntry) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func outputCSV(entries []listEntry) error {
	w := csv.NewWriter(os.Stdout)
	if err := w.Write([]string{"ID", "Title", "Status", "Date", "Author", "Type", "File"}); err != nil {
		return err
	}
	for i := range entries {
		if err := w.Write([]string{
			entries[i].ID, entries[i].Title, entries[i].Status,
			entries[i].Date, entries[i].Author, entries[i].Type, entries[i].File,
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
