// Package index generates the README index table for a docz document
// directory and splices it between the BEGIN/END markers in each type's
// README.md.
//
// Promoted whole from internal/index (IMPL-0018 Phase 2, DESIGN-0014 §2.6).
// Scanning lives in pkg/doczcore/document; this package only builds the table
// and puts it where the markers say. Splice is the whole of it as a pure
// function, and UpdateReadme and DryRunReadme are the filesystem around it.
package index

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
)

// The marker pair that delimits the generated table.
//
// Exported because a consumer that writes its own README — docz-api
// scaffolding a repo it has no checkout of — has to put the pair in, and a
// consumer that renders one has to find it. They are also docparse's legacy
// "index" region kind, which is how Splice locates them.
const (
	BeginMarker = "<!-- BEGIN DOCZ AUTO-GENERATED -->"
	EndMarker   = "<!-- END DOCZ AUTO-GENERATED -->"
)

// UpdateAction names the kind of work UpdateReadme / DryRunReadme
// performed. The cmd layer switches on this value to format the
// user-facing message; index/* never produces English strings.
type UpdateAction int

const (
	// ActionCreated indicates UpdateReadme wrote a brand-new README
	// from the caller-provided index header.
	ActionCreated UpdateAction = iota + 1
	// ActionUpdated indicates UpdateReadme found existing markers
	// and rewrote the auto-generated table between them.
	ActionUpdated
	// ActionNoMarkers indicates the target README exists but has no
	// DOCZ auto-generated markers; the file is left unchanged.
	ActionNoMarkers
	// ActionDryRunCreated is the dry-run analogue of ActionCreated.
	// Body holds the README content that would have been written.
	ActionDryRunCreated
	// ActionDryRunUpdated is the dry-run analogue of ActionUpdated.
	// Body holds the README content that would have been written.
	ActionDryRunUpdated
)

// UpdateOutcome is the typed result of UpdateReadme / DryRunReadme. The
// Action discriminator drives caller-side message formatting; Body is
// populated for the two dry-run actions so the cmd layer can print the
// would-be content.
type UpdateOutcome struct {
	Action UpdateAction
	Path   string
	Body   string
}

// GenerateTable produces a markdown table from a list of document entries.
func GenerateTable(docs []document.DocEntry, heading string) string {
	var sb strings.Builder

	sb.WriteString("## " + heading + "\n\n")
	sb.WriteString("| ID | Title | Status | Date | Author | Link |\n")
	sb.WriteString("|----|-------|--------|------|--------|------|\n")

	for i := range docs {
		doc := &docs[i]

		fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s | [%s](%s) |\n",
			doc.ID, doc.Title, doc.Status, doc.Created, doc.Author,
			doc.Filename, doc.Filename)
	}

	return sb.String()
}

// Scaffold returns a README body for a header that has no index yet: the
// header followed by exactly one marker pair, with nothing between them.
//
// Exactly one pair, whatever the header ends with. That is issue #99: two of
// the embedded index headers end with their own pair, the creation path
// appended another, and every `docz init` produced a README with a second,
// permanently empty pair — the splice only ever touches the first. Checking the
// header rather than stripping the pair from those two files also covers a
// repo's own templates/index_<type>.md override, which is where the next one
// would come from.
//
// The result still needs a table spliced into it, which is what Splice does
// with it.
func Scaffold(header string) []byte {
	if hasTrailingPair(header) {
		return []byte(header)
	}

	return []byte(header + BeginMarker + "\n" + EndMarker + "\n")
}

// hasTrailingPair reports whether the header already closes with a marker
// pair, ignoring blank lines after it.
//
// By region rather than by string search, so a header whose pair is spelled
// leniently counts too — the same reason Splice walks regions. "Trailing"
// means the pair is the last thing in the header: a header that mentions the
// markers in prose partway through still needs one appended.
func hasTrailingPair(header string) bool {
	regions := docparse.Regions([]byte(header))
	if len(regions) == 0 {
		return false
	}

	last := regions[len(regions)-1]
	if last.Kind != docparse.IndexKind || !last.Closed {
		return false
	}

	lines := strings.Split(header, "\n")

	for _, line := range lines[last.End:] {
		if strings.TrimSpace(line) != "" {
			return false
		}
	}

	return true
}

// Splice puts table between the index markers of existing and returns the new
// body and what it did.
//
// This is the whole of what the package does, as a pure function: no path, no
// filesystem, no error. A consumer holding a README it fetched rather than a
// path it can write calls this and commits the result (DESIGN-0014 §2.6).
//
// A nil existing means there is no README, and the body is built from
// Scaffold(header) — ActionCreated. An existing body with no closed index
// region is left alone and reported as ActionNoMarkers with a nil body: a
// README somebody wrote by hand is not docz's to rewrite. Otherwise the table
// replaces whatever was between the markers, and the markers themselves are
// rewritten canonically — ActionUpdated.
//
// The pair is located with docparse.Regions, so a leniently spelled marker is
// found and normalised rather than making docz skip the file and say nothing
// (INV-0009 Finding 4). The first index region wins when a README carries more
// than one, which is what the old string scan did and what issue #99's doubled
// pair relies on.
func Splice(existing []byte, header, table string) ([]byte, UpdateAction) {
	if existing == nil {
		existing = Scaffold(header)

		body, ok := spliceRegion(existing, table)
		if !ok {
			// Scaffold guarantees a pair, so this is unreachable unless
			// Scaffold and the walker disagree.
			return existing, ActionCreated
		}

		return body, ActionCreated
	}

	body, ok := spliceRegion(existing, table)
	if !ok {
		return nil, ActionNoMarkers
	}

	return body, ActionUpdated
}

// spliceRegion rewrites the first closed index region of content to hold
// table, canonicalising the marker lines.
//
// Line-based because docparse reports lines, and reconstructed so the result
// is byte-identical to the string-cut splice this replaced for a canonically
// marked README: everything before the begin marker and after the end marker
// is preserved exactly, including whether the file ends in a newline.
func spliceRegion(content []byte, table string) ([]byte, bool) {
	var at docparse.Region

	for _, r := range docparse.Regions(content) {
		if r.Kind == docparse.IndexKind && r.Closed {
			at = r

			break
		}
	}

	if at.Kind == "" {
		return nil, false
	}

	lines := strings.Split(string(content), "\n")

	var sb strings.Builder

	if at.Start > 1 {
		sb.WriteString(strings.Join(lines[:at.Start-1], "\n"))
		sb.WriteString("\n")
	}

	sb.WriteString(BeginMarker + "\n" + table + EndMarker)

	if at.End < len(lines) {
		sb.WriteString("\n")
		sb.WriteString(strings.Join(lines[at.End:], "\n"))
	}

	return []byte(sb.String()), true
}

// UpdateReadme updates the auto-generated section of a README file between
// the DOCZ markers. If the file doesn't exist, it is created with the
// provided header (Action=ActionCreated). If it exists with markers, the
// table is rewritten (Action=ActionUpdated). If it exists without markers,
// nothing is written and Action=ActionNoMarkers — the caller decides how to
// surface that. The header is resolved by the caller (see
// doctemplate.ResolveIndexHeader) so this package stays a pure marker-splicer.
func UpdateReadme(readmePath, header, tableContent string) (UpdateOutcome, error) {
	existing, err := readExisting(readmePath)
	if err != nil {
		return UpdateOutcome{}, err
	}

	body, action := Splice(existing, header, tableContent)
	if action == ActionNoMarkers {
		return UpdateOutcome{Action: ActionNoMarkers, Path: readmePath}, nil
	}

	if action == ActionCreated {
		dir := filepath.Dir(readmePath)
		if err := os.MkdirAll(dir, config.DirMode); err != nil {
			return UpdateOutcome{}, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(readmePath, body, config.FileMode); err != nil {
		return UpdateOutcome{}, fmt.Errorf("writing %s: %w", readmePath, err)
	}

	return UpdateOutcome{Action: action, Path: readmePath}, nil
}

// DryRunReadme returns what UpdateReadme would write without modifying
// files. Action is one of ActionDryRunCreated, ActionDryRunUpdated, or
// ActionNoMarkers; Body holds the would-be content for the two dry-run
// success cases.
func DryRunReadme(readmePath, header, tableContent string) (UpdateOutcome, error) {
	existing, err := readExisting(readmePath)
	if err != nil {
		return UpdateOutcome{}, err
	}

	body, action := Splice(existing, header, tableContent)

	switch action {
	case ActionNoMarkers:
		return UpdateOutcome{Action: ActionNoMarkers, Path: readmePath}, nil
	case ActionCreated:
		action = ActionDryRunCreated
	case ActionUpdated:
		action = ActionDryRunUpdated
	case ActionDryRunCreated, ActionDryRunUpdated:
		// Splice never returns these; the cases exist so a new action
		// cannot be added without this switch failing to compile.
	}

	return UpdateOutcome{Action: action, Path: readmePath, Body: string(body)}, nil
}

// readExisting reads a README, returning a nil body for one that is not there.
//
// Nil rather than empty is the signal Splice reads: a README that exists and
// is empty has no markers, which docz leaves alone, while one that does not
// exist is docz's to create.
func readExisting(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	return nil, fmt.Errorf("reading %s: %w", path, err)
}
