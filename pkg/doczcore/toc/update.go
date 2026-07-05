package toc

import (
	"os"

	"github.com/donaldgifford/docz/pkg/doczcore/config"
)

// FileInput is a single document handed to UpdateFiles. Content is the
// in-memory bytes (typically cached by an earlier scan pass — see
// IMPL-0007's DocEntry.Content), so UpdateFiles never re-reads the file.
// Path is used for write-back and for the returned report.
type FileInput struct {
	Path    string
	Content []byte
}

// FileResult identifies a single file in the report and carries the
// number of headings the parser found in it. Headings is 0 for files
// that had ToC markers but no usable headings (which still counts as
// "Updated" if the previous ToC was non-empty).
type FileResult struct {
	Path     string
	Headings int
}

// FileError records a non-fatal failure encountered while writing a
// file. UpdateFiles continues processing the remaining files when one
// fails to write.
type FileError struct {
	Path string
	Err  error
}

// UpdateReport categorizes the outcome of an UpdateFiles call. Files
// without ToC markers go into Skipped; files with markers go into one
// of Updated, Unchanged, or WouldUpdate (dry-run mode). Write failures
// go into WriteErrors and are not also recorded as Updated.
type UpdateReport struct {
	Updated     []FileResult
	Unchanged   []FileResult
	WouldUpdate []FileResult
	Skipped     []string
	WriteErrors []FileError
}

// UpdateFiles regenerates the ToC inside each FileInput. Files without
// markers are reported as Skipped and left alone. When dryRun is true
// nothing is written to disk; would-be-updated files are reported in
// WouldUpdate. When dryRun is false the new content is written back via
// os.WriteFile and the file is recorded as Updated on success, or as a
// WriteError on failure. The error return is reserved for failures that
// stop processing; per-file write failures are non-fatal and live in
// the report.
func UpdateFiles(files []FileInput, minHeadings int, dryRun bool) (UpdateReport, error) {
	var report UpdateReport

	for _, f := range files {
		original := string(f.Content)
		res := UpdateToC(original, minHeadings)
		if !res.Found {
			report.Skipped = append(report.Skipped, f.Path)
			continue
		}

		if res.Updated == original {
			report.Unchanged = append(report.Unchanged, FileResult{
				Path:     f.Path,
				Headings: len(res.Headings),
			})
			continue
		}

		if dryRun {
			report.WouldUpdate = append(report.WouldUpdate, FileResult{
				Path:     f.Path,
				Headings: len(res.Headings),
			})
			continue
		}

		if err := os.WriteFile(f.Path, []byte(res.Updated), config.FileMode); err != nil {
			report.WriteErrors = append(report.WriteErrors, FileError{
				Path: f.Path,
				Err:  err,
			})
			continue
		}

		report.Updated = append(report.Updated, FileResult{
			Path:     f.Path,
			Headings: len(res.Headings),
		})
	}

	return report, nil
}
