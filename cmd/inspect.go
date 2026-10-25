package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	clischema "github.com/donaldgifford/docz/v2/api/cli"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// openQuestionsKind is the region kind whose body inspect reads questions
// from.
const openQuestionsKind = "open-questions"

var inspectFormat string

// The DTOs below are the contract api/cli/inspect.schema.json describes.
// They are inspect's own rather than json tags on the frozen document,
// docparse, and kinds structs (DESIGN-0024 §3), so the library's field names
// and this output's can move independently.

// inspectOutput is the whole of `docz inspect --format json`.
type inspectOutput struct {
	SchemaVersion int                 `json:"schema_version"`
	DoczVersion   string              `json:"docz_version"`
	Path          string              `json:"path"`
	Type          string              `json:"type"`
	ID            string              `json:"id"`
	Schema        string              `json:"schema"`
	ContentSHA256 string              `json:"content_sha256"`
	Frontmatter   *inspectFrontmatter `json:"frontmatter"`
	Statuses      []string            `json:"statuses"`
	Inferred      bool                `json:"inferred"`
	Regions       []inspectRegion     `json:"regions"`
	Headings      []inspectHeading    `json:"headings"`
	OpenQuestions []inspectQuestion   `json:"open_questions"`
	Tasks         []inspectTask       `json:"tasks"`
	Content       string              `json:"content"`
}

// inspectFrontmatter is the parsed frontmatter, null when there is none.
type inspectFrontmatter struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Author  string `json:"author"`
	Created string `json:"created"`
	Schema  string `json:"schema,omitempty"`
}

// inspectRegion is one region, marked or inferred.
type inspectRegion struct {
	Kind   string `json:"kind"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Depth  int    `json:"depth"`
	Closed bool   `json:"closed"`
}

// inspectHeading is one H2–H6 heading with the anchor docparse assigned it.
type inspectHeading struct {
	Level  int    `json:"level"`
	Text   string `json:"text"`
	Anchor string `json:"anchor"`
	Line   int    `json:"line"`
}

// inspectQuestion is one open question, its lines document lines.
type inspectQuestion struct {
	Number   int                `json:"number"`
	Title    string             `json:"title"`
	Line     int                `json:"line"`
	Options  []inspectOption    `json:"options"`
	Resolved *inspectResolution `json:"resolved"`
}

// inspectOption is one lettered option.
type inspectOption struct {
	Letter      string `json:"letter"`
	Text        string `json:"text"`
	Recommended bool   `json:"recommended"`
	Line        int    `json:"line"`
}

// inspectResolution is a question's resolution quote.
type inspectResolution struct {
	Date   string `json:"date"`
	Choice string `json:"choice"`
	Note   string `json:"note"`
	Line   int    `json:"line"`
}

// inspectTask is one checkbox item.
type inspectTask struct {
	Line    int    `json:"line"`
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
	Indent  int    `json:"indent"`
}

var inspectCmd = &cobra.Command{
	Use:   "inspect <id|path>",
	Short: "Report one document's structure",
	Long: `Inspect one document: its frontmatter, regions, headings, open questions,
tasks, and the statuses its type allows, read from a single read of the file.

The argument is a document id (DESIGN-0024) or a path, repo-relative or
absolute. A type name is refused, because inspect takes a document. A path
must resolve, symlinks followed, under the repository root and under an
enabled type's directory; it need not follow the docz filename convention.

--format json prints the contract described by api/cli/inspect.schema.json,
including the document's bytes and their SHA-256.

Exit codes:
  0  the document was inspected
  1  no such document
  2  a type name, a path the repository does not own, or a bad --format`,
	Args: cobra.ExactArgs(1),
	RunE: runInspect,
}

func init() {
	inspectCmd.Flags().StringVar(&inspectFormat, "format", formatText,
		"output format: text or json")
	rootCmd.AddCommand(inspectCmd)
}

func runInspect(cmd *cobra.Command, args []string) error {
	return getRunner().inspect(cmdContext(cmd), inspectFormat, args[0])
}

// inspect resolves the argument, reads the document once, and prints it.
func (r *Runner) inspect(ctx context.Context, format, arg string) error {
	format, err := resolveStatusFormat(format)
	if err != nil {
		return err
	}

	rp := r.repoOrOpen()

	report, err := r.resolveDocument(ctx, rp, arg)
	if err != nil {
		return err
	}

	out, err := buildInspect(ctx, rp, &report)
	if err != nil {
		return err
	}

	if format == formatJSON {
		enc := json.NewEncoder(r.Out)
		enc.SetIndent("", "  ")

		return enc.Encode(out)
	}

	return r.printInspectText(out)
}

// resolveDocument turns a command's document argument into a validated
// single-document report, in the order DESIGN-0024 §3 fixes: a type name is
// refused, then the argument is tried as an id, then as a path.
//
// A type name first, so a file named like a type can never shadow it. An
// argument that looks like a path (a separator or a .md suffix) skips the id
// lookup, since no id contains either.
func (r *Runner) resolveDocument(ctx context.Context, rp *repo.Repo, arg string) (repo.DocumentReport, error) {
	if _, err := r.Cfg.ValidateType(arg); err == nil {
		return repo.DocumentReport{}, exitErrorf(errExitCode2,
			"%q is a document type; this command takes a document id or path", arg)
	}

	path := arg

	if !looksLikePath(arg) {
		entry, err := rp.Find(ctx, arg)
		if err != nil {
			return repo.DocumentReport{}, documentLookupError(arg, err)
		}

		path = entry.Path
	}

	report, err := rp.ValidateDocument(ctx, path)
	if err != nil {
		return repo.DocumentReport{}, documentLookupError(arg, err)
	}

	return report, nil
}

// looksLikePath reports whether an argument can only be a path.
func looksLikePath(arg string) bool {
	return strings.ContainsRune(arg, '/') ||
		strings.ContainsRune(arg, filepath.Separator) ||
		strings.HasSuffix(strings.ToLower(arg), ".md")
}

// documentLookupError maps a resolution failure onto an exit code: a path the
// repository does not own is a usage error (2); a document that is not there
// is not found (1).
func documentLookupError(arg string, err error) error {
	var pathErr *repo.PathError
	if errors.As(err, &pathErr) {
		return exitErrorf(errExitCode2, "%v", pathErr)
	}

	var notFound *repo.NotFoundError
	var unknown *repo.UnknownTypeError

	switch {
	case errors.As(err, &notFound), errors.As(err, &unknown), errors.Is(err, fs.ErrNotExist):
		return exitErrorf(errExitCode1, "no document %q", arg)
	}

	return err
}

// buildInspect composes the output from the readers in pkg/doczcore over the
// report's bytes. Nothing here reads the file again.
func buildInspect(ctx context.Context, rp *repo.Repo, report *repo.DocumentReport) (*inspectOutput, error) {
	content := report.Content

	tmpl, _, err := rp.Template(ctx, report.Type)
	if err != nil {
		return nil, fmt.Errorf("resolving the %s template: %w", report.Type, err)
	}

	regions, inferred := kinds.ResolveRegions(content, kinds.SpecFromTemplate([]byte(tmpl)))

	out := &inspectOutput{
		SchemaVersion: clischema.SchemaVersionInspect,
		DoczVersion:   Version,
		Path:          filepath.ToSlash(report.Path),
		Type:          report.Type,
		ID:            documentID(rp, report),
		Schema:        report.Schema,
		ContentSHA256: report.ContentSHA256,
		Statuses:      append([]string{}, rp.Cfg.Types[report.Type].Statuses...),
		Inferred:      inferred,
		Regions:       make([]inspectRegion, 0, len(regions)),
		Headings:      []inspectHeading{},
		OpenQuestions: []inspectQuestion{},
		Tasks:         []inspectTask{},
		Content:       string(content),
	}

	if fm, err := document.ParseFrontmatter(content); err == nil {
		out.Frontmatter = &inspectFrontmatter{
			ID: fm.ID, Title: fm.Title, Status: string(fm.Status),
			Author: fm.Author, Created: fm.Created, Schema: fm.Schema,
		}
	}

	for _, rg := range regions {
		out.Regions = append(out.Regions, inspectRegion{
			Kind: rg.Kind, Start: rg.Start, End: rg.End, Depth: rg.Depth, Closed: rg.Closed,
		})

		if rg.Kind == openQuestionsKind {
			questions := kinds.ShiftQuestions(kinds.OpenQuestions(kinds.RegionBytes(content, rg)), rg)
			out.OpenQuestions = append(out.OpenQuestions, toInspectQuestions(questions)...)
		}
	}

	for _, h := range docparse.Headings(content) {
		out.Headings = append(out.Headings, inspectHeading{
			Level: h.Level, Text: h.Text, Anchor: h.Slug, Line: h.Line,
		})
	}

	for _, task := range docparse.TaskItems(content) {
		out.Tasks = append(out.Tasks, inspectTask{
			Line: task.Line, Text: task.Text, Checked: task.Checked, Indent: task.Indent,
		})
	}

	return out, nil
}

// documentID is the frontmatter id, or one derived from the filename when the
// frontmatter has none: the type's prefix and the filename's number.
func documentID(rp *repo.Repo, report *repo.DocumentReport) string {
	if report.ID != "" {
		return report.ID
	}

	base := filepath.Base(report.Path)
	if !document.IsDoczFile(base) {
		return ""
	}

	number, _, _ := strings.Cut(base, "-")

	return rp.Cfg.Types[report.Type].IDPrefix + "-" + number
}

func toInspectQuestions(questions []kinds.Question) []inspectQuestion {
	out := make([]inspectQuestion, 0, len(questions))

	for i := range questions {
		q := &questions[i]

		iq := inspectQuestion{
			Number: q.Number, Title: q.Title, Line: q.Line,
			Options: make([]inspectOption, 0, len(q.Options)),
		}

		for _, o := range q.Options {
			iq.Options = append(iq.Options, inspectOption{
				Letter: strings.ToLower(o.Letter), Text: o.Text, Recommended: o.Recommended, Line: o.Line,
			})
		}

		if q.Resolved != nil {
			iq.Resolved = &inspectResolution{
				Date: q.Resolved.Date, Choice: strings.ToLower(q.Resolved.Choice),
				Note: q.Resolved.Note, Line: q.Resolved.Line,
			}
		}

		out = append(out, iq)
	}

	return out
}

// printInspectText is the short summary for a person.
func (r *Runner) printInspectText(out *inspectOutput) error {
	status := "(no frontmatter)"
	if out.Frontmatter != nil {
		status = out.Frontmatter.Status
	}

	checked := 0

	for _, t := range out.Tasks {
		if t.Checked {
			checked++
		}
	}

	var open []inspectQuestion

	for _, q := range out.OpenQuestions {
		if q.Resolved == nil {
			open = append(open, q)
		}
	}

	if _, err := fmt.Fprintf(r.Out,
		"%s (%s) %s\n%s\nstatus: %s\nregions: %d, headings: %d, tasks: %d/%d, open questions: %d unresolved of %d\n",
		out.ID, out.Type, out.Path, frontmatterTitle(out), status,
		len(out.Regions), len(out.Headings), checked, len(out.Tasks), len(open), len(out.OpenQuestions),
	); err != nil {
		return err
	}

	for _, q := range open {
		if _, err := fmt.Fprintf(r.Out, "  Q%d %s (line %d)\n", q.Number, q.Title, q.Line); err != nil {
			return err
		}
	}

	return nil
}

func frontmatterTitle(out *inspectOutput) string {
	if out.Frontmatter == nil {
		return ""
	}

	return out.Frontmatter.Title
}
