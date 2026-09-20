// Package parity holds the functional-parity suite: it drives a docz binary
// over fixture repositories and compares everything the binary produced —
// stdout, stderr, exit code, and the resulting file tree — against goldens
// captured from the v1.2.2 release.
//
// The suite is the evidence for ADR-0002 Decision 4: the CLI is the first
// tool moved onto the API, and "moved, not changed" is only a claim until a
// byte comparison backs it. Goldens come from v1.2.2 and from nowhere else
// (see README.md); a v2 build that differs is either a bug or a
// permitted delta argued for in a pull request.
//
// The driver itself lives in parity_test.go behind the "parity" build tag,
// because it needs a built binary. Everything in this file is ordinary code
// with ordinary unit tests, so the normalisers are covered by `make test`.
package parity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// A Normalizer rewrites one run- or machine-specific detail out of captured
// output, so that a golden recorded on one checkout replays on another. Each
// carries a Name because a golden mismatch is easier to read when the suite
// can say which normalisers were applied.
type Normalizer struct {
	Name  string
	Apply func(string) string
}

// Normalize applies every normalizer in order.
func Normalize(s string, ns ...Normalizer) string {
	for _, n := range ns {
		s = n.Apply(s)
	}

	return s
}

// RootNormalizer replaces a fixture's absolute root with $ROOT. Both the path
// as given and its symlink-resolved form are replaced: on macOS a temporary
// directory is handed out as /var/folders/... while a process that resolves
// its own working directory reports /private/var/folders/..., and output that
// mixes the two would otherwise never match.
func RootNormalizer(root string) Normalizer {
	forms := []string{root}
	if resolved, err := filepath.EvalSymlinks(root); err == nil && resolved != root {
		forms = append(forms, resolved)
	}

	// Longest first, so a prefix never shadows the fuller form.
	sort.Slice(forms, func(i, j int) bool { return len(forms[i]) > len(forms[j]) })

	return Normalizer{
		Name: "root",
		Apply: func(s string) string {
			for _, f := range forms {
				s = strings.ReplaceAll(s, f, "$ROOT")
			}

			return s
		},
	}
}

// DateNormalizer replaces the given YYYY-MM-DD date with $DATE. `docz create`
// stamps today into frontmatter, so without this every golden would expire
// overnight.
func DateNormalizer(date string) Normalizer {
	return Normalizer{
		Name:  "date",
		Apply: func(s string) string { return strings.ReplaceAll(s, date, "$DATE") },
	}
}

// markerLine matches a region marker alone on its line, in the canonical
// spelling the templates carry.
var markerLine = regexp.MustCompile(`^[ \t]*<!--docz:[a-z0-9-]+:(start|end)-->[ \t]*$`)

// MarkerNormalizer drops region-marker lines. Marker lines in `create` and
// `template` output are the one intended difference between v1.2.2 and the v2
// line (DESIGN-0014 §4), so they are removed before comparison rather than
// argued about in every golden.
//
// Only whole marker lines go. A marker with text beside it is not a marker to
// the walker either, and leaving it in keeps that agreement visible.
//
// A marker sitting alone between two blank lines takes one of them with it.
// The templates separate their sections with a single blank line and put the
// markers on lines of their own, so dropping a marker without its blank would
// leave a double blank where v1.2.2 has one and turn every section break into
// a false difference. Only that shape is collapsed: a blank line not adjacent
// to a dropped marker is untouched.
func MarkerNormalizer() Normalizer {
	return Normalizer{
		Name: "markers",
		Apply: func(s string) string {
			if !strings.Contains(s, "<!--docz:") {
				return s
			}

			lines := strings.Split(s, "\n")
			kept := make([]string, 0, len(lines))

			for i := 0; i < len(lines); i++ {
				if !markerLine.MatchString(lines[i]) {
					kept = append(kept, lines[i])

					continue
				}

				surrounded := len(kept) > 0 && kept[len(kept)-1] == "" &&
					i+1 < len(lines) && lines[i+1] == ""
				if surrounded {
					i++
				}
			}

			return strings.Join(kept, "\n")
		},
	}
}

// NormalizeFiles applies norms to every recorded body and recomputes the size
// and digest from the result.
//
// Recording a raw size beside a normalised body would make the golden argue
// with itself: the marker lines the body no longer shows would still be in its
// byte count, so a v2 tree and the v1.2.2 tree it matches line for line would
// differ on the one number a reviewer cannot check by eye. Both snapshots go
// through this, so the before/after comparison that decides which bodies to
// record still compares like with like.
func NormalizeFiles(files []File, norms ...Normalizer) []File {
	out := make([]File, 0, len(files))

	for _, f := range files {
		f.Body = Normalize(f.Body, norms...)
		sum := sha256.Sum256([]byte(f.Body))
		f.Size = int64(len(f.Body))
		f.Sum = hex.EncodeToString(sum[:])[:12]

		out = append(out, f)
	}

	return out
}

// A File is one entry of a recorded file tree.
type File struct {
	Path string // slash-separated, relative to the fixture root
	Size int64
	Body string // recorded only when the case created or changed the file
	Sum  string // sha256 prefix, so a same-size edit still fails the golden
}

// A Result is everything one command invocation produced.
type Result struct {
	Args     []string
	ExitCode int
	Stdout   string
	Stderr   string
	Files    []File
}

// Tree walks dir and returns every regular file with its size and content
// digest, sorted by path. Directories are not recorded on their own: an empty
// one that a command created shows up in the command's own output, and
// recording them doubles the golden for no extra signal.
func Tree(dir string) ([]File, error) {
	// Walked through an os.Root rather than filepath.WalkDir: the walk and the
	// reads then resolve inside dir by construction, so a symlink planted
	// mid-walk cannot send a read outside the fixture copy.
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}

	out, err := walkTree(root.FS())

	// The walk error is the interesting one; a close failure on a read-only
	// root only matters when the walk itself succeeded.
	if closeErr := root.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })

	return out, nil
}

func walkTree(fsys fs.FS) ([]File, error) {
	var out []File

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// A gitlink is a *file* named .git holding an absolute host path, which
		// no normaliser would catch, so the name is skipped at either type.
		if d.Name() == ".git" {
			if d.IsDir() {
				return fs.SkipDir
			}

			return nil
		}

		if d.IsDir() {
			return nil
		}

		if !d.Type().IsRegular() {
			return nil
		}

		body, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}

		sum := sha256.Sum256(body)

		out = append(out, File{
			Path: path,
			Size: int64(len(body)),
			Body: string(body),
			Sum:  hex.EncodeToString(sum[:])[:12],
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

// Changed returns after's entries with Body kept only where the file is new or
// its content moved. Every file is still listed with its size and digest, so a
// deletion or an unexpected edit fails the golden; bodies are reserved for
// what the command actually wrote, which is what a reviewer reads.
func Changed(before, after []File) []File {
	prior := make(map[string]string, len(before))
	for _, f := range before {
		prior[f.Path] = f.Sum
	}

	out := make([]File, 0, len(after))

	for _, f := range after {
		if sum, ok := prior[f.Path]; ok && sum == f.Sum {
			f.Body = ""
		}

		out = append(out, f)
	}

	return out
}

// Deleted returns the paths present before the command and gone after it.
func Deleted(before, after []File) []string {
	live := make(map[string]struct{}, len(after))
	for _, f := range after {
		live[f.Path] = struct{}{}
	}

	var gone []string

	for _, f := range before {
		if _, ok := live[f.Path]; !ok {
			gone = append(gone, f.Path)
		}
	}

	return gone
}

// Format renders a Result as the golden text: a header per section, one line
// per unchanged file, and the full body of anything the command wrote. The
// shape is chosen to read as a diff when it fails.
func Format(r *Result, deleted []string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "$ docz %s\n", strings.Join(r.Args, " "))
	fmt.Fprintf(&b, "exit %d\n", r.ExitCode)

	writeBlock(&b, "stdout", r.Stdout)
	writeBlock(&b, "stderr", r.Stderr)

	b.WriteString("\n=== files\n")

	for _, f := range r.Files {
		fmt.Fprintf(&b, "%s (%d bytes, %s)\n", f.Path, f.Size, f.Sum)
	}

	for _, p := range deleted {
		fmt.Fprintf(&b, "%s (deleted)\n", p)
	}

	for _, f := range r.Files {
		if f.Body == "" {
			continue
		}

		fmt.Fprintf(&b, "\n=== written %s\n", f.Path)
		b.WriteString(f.Body)

		if !strings.HasSuffix(f.Body, "\n") {
			b.WriteString("\n\\ no trailing newline\n")
		}
	}

	return b.String()
}

func writeBlock(b *strings.Builder, name, body string) {
	fmt.Fprintf(b, "\n=== %s\n", name)

	if body == "" {
		b.WriteString("(empty)\n")

		return
	}

	b.WriteString(body)

	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n\\ no trailing newline\n")
	}
}

// planNavTitle matches the wiki nav-title entry the removed built-in
// contributed. Anchored on the exact label so `impl: Implementation Plans`,
// which also contains the word, is untouched.
var planNavTitle = regexp.MustCompile(`^[ \t]+plan: Plans[ \t]*$`)

// planTypeKey matches the `plan:` key of a type block, capturing its indent so
// the block's more-deeply-indented body can be dropped with it.
var planTypeKey = regexp.MustCompile(`^([ \t]+)plan:[ \t]*$`)

// planWarning matches the validation warning a repo that kept its `types.plan`
// block now gets on every command, because plan is a custom type there.
var planWarning = regexp.MustCompile(
	`^Warning: config declares non-built-in type "plan" \(typo\?\)[ \t]*$`)

// recordedFile matches an entry of the "=== files" list, capturing the path so
// a touched file's size and digest can be replaced.
var recordedFile = regexp.MustCompile(`^(\S+) \(\d+ bytes, [0-9a-f]+\)$`)

// PlanNormalizer removes every trace of the `plan` document type, which
// ADR-0003 drops from the built-in catalogue on the v2 line.
//
// This is the fourth permitted delta (IMPL-0018 Open Question 8) and the first
// one that is not additive: a v1.2.2 golden shows a type the v2 binary does not
// have, so the difference cannot be argued away per case. Unlike the other
// normalisers it runs on the formatted text of **both** sides at comparison
// time rather than on the captured output alone, because the golden is the side
// carrying the type.
//
// Four traces, each anchored tightly enough that a type whose label merely
// contains the word — `impl: Implementation Plans` — is left alone:
//
//   - the `plan:` block under `types:`, key line and indented body;
//   - the `plan: Plans` entry under `wiki.nav_titles`;
//   - the "non-built-in type" warning, which a repo keeping its `types.plan`
//     block now gets on every command because plan is a custom type there;
//   - the comment preamble of a generated `.docz.yaml`, which v2 rewrote to
//     say five built-in types instead of six and to explain the fallback.
//
// A fifth trace is the `PLAN-XXXX` placeholder in the IMPL and INV templates'
// "Implements" and "Triggered by" hints, which name an id prefix docz can no
// longer issue.
//
// The recorded size and digest become placeholders for every file whose body the
// golden records, and for no others. A byte count computed before a line was
// dropped cannot be recomputed from the golden's text, and the rule has to be
// the same on both sides — the side that no longer carries the trace cannot
// know a trace was there. Nothing is lost: a recorded body is compared line by
// line, so its digest is the redundant half of that check, and a file with no
// recorded body keeps the digest as its only one.
func PlanNormalizer() Normalizer {
	return Normalizer{
		Name: "plan",
		Apply: func(s string) string {
			lines, bodies := dropPlanTraces(strings.Split(s, "\n"))

			return strings.Join(restoreBlocks(lines, bodies), "\n")
		},
	}
}

// planHints are the template placeholder references to the removed type, as
// pure substring deletions in the order they have to be applied.
//
// Deletions rather than rewrites: the v1 hint listed PLAN beside RFC and
// DESIGN, and v2 lists the other two, so removing the token from the v1 side
// produces the v2 line exactly. A rewrite would need the normaliser to know
// both spellings, which is how a normaliser starts hiding real differences.
// The prose elsewhere in those templates that happens to use the word "plan"
// is untouched, in the templates and here: it means a planning document, not
// the type, and `## Testing Plan` is a heading a substring rule would eat.
var planHints = []string{" / PLAN-XXXX", "/PLAN"}

// dropPlanTraces removes every plan trace and reports the paths whose bodies
// the golden records.
//
// The set decides which size and digest lines are comparable, and it is read
// off the format rather than from what the pass edited: the two sides disagree
// about which bodies carry a trace, so a rule that depended on that would
// neutralise different entries on each side.
func dropPlanTraces(lines []string) (kept []string, bodies map[string]bool) {
	kept = make([]string, 0, len(lines))
	bodies = make(map[string]bool)

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if path, ok := writtenBlockPath(line); ok {
			bodies[path] = true
		}

		if planNavTitle.MatchString(line) || planWarning.MatchString(line) {
			continue
		}

		if m := planTypeKey.FindStringSubmatch(line); m != nil {
			i = skipIndentedUnder(lines, i, len(m[1]))

			continue
		}

		if isConfigPreambleStart(lines, i) {
			i = skipCommentPreamble(lines, i)

			continue
		}

		trimmed := line
		for _, hint := range planHints {
			trimmed = strings.ReplaceAll(trimmed, hint, "")
		}

		kept = append(kept, trimmed)
	}

	return kept, bodies
}

// The index README's auto-generated marker lines, as the whole line. Compared
// by equality rather than by pattern because these two spellings are the only
// ones index.UpdateReadme will splice into.
const (
	indexBeginLine = "<!-- BEGIN DOCZ AUTO-GENERATED -->"
	indexEndLine   = "<!-- END DOCZ AUTO-GENERATED -->"
)

// IndexPairNormalizer collapses a repeated, empty index marker pair down to
// one.
//
// The fifth permitted delta, and the second non-additive one. Issue #99: two of
// the embedded index headers end with their own marker pair, and v1's `init`
// appended another unconditionally, so a new repository got a second pair that
// no splice would ever touch again — the table always fills the first. repo.Init
// writes index.Scaffold, which appends a pair only when the header lacks one, so
// the duplicate is gone.
//
// That makes a v1.2.2 `init` golden for the investigation type differ from the
// v2 binary by two lines it should differ by. Like the plan normaliser this runs
// on the formatted text of **both** sides, because the golden is the side
// carrying the duplicate and the side without it cannot know one was there.
//
// Only an empty pair immediately following another pair's end is collapsed:
// `END, BEGIN, END` becomes `END`. A pair with a table between its markers is
// never touched, so a golden that records a spliced README still compares its
// table line by line. The recorded size and digest need no special handling
// here — the plan normaliser already replaces them for every file whose body the
// golden records, which is every file this one can change.
func IndexPairNormalizer() Normalizer {
	return Normalizer{
		Name: "index-pair",
		Apply: func(s string) string {
			if !strings.Contains(s, indexBeginLine) {
				return s
			}

			return strings.Join(collapseIndexPairs(strings.Split(s, "\n")), "\n")
		},
	}
}

// collapseIndexPairs drops every empty marker pair that directly follows
// another pair's end line.
//
// One pass suffices for any number of consecutive pairs: each collapse consumes
// the follower and leaves the same end line in place to be compared against the
// next one.
func collapseIndexPairs(lines []string) []string {
	out := make([]string, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		if len(out) > 0 && out[len(out)-1] == indexEndLine &&
			lines[i] == indexBeginLine && i+1 < len(lines) && lines[i+1] == indexEndLine {
			i++

			continue
		}

		out = append(out, lines[i])
	}

	return out
}

// writtenBlockPath returns the path a "=== written <path>" header names.
func writtenBlockPath(line string) (string, bool) {
	const prefix = "=== written "
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}

	return strings.TrimPrefix(line, prefix), true
}

// restoreBlocks repairs what line removal broke in the format itself: a stdout
// or stderr block emptied by the pass reads "(empty)" as it would have if the
// binary had printed nothing, and a recorded body's size and digest become
// placeholders.
//
// Without the first half, a legacy fixture whose only stderr was the plan
// warning would differ from an identical run that printed nothing at all,
// which is the opposite of what the normaliser is for.
func restoreBlocks(lines []string, bodies map[string]bool) []string {
	out := make([]string, 0, len(lines))

	for i, line := range lines {
		if line == "=== stdout" || line == "=== stderr" {
			out = append(out, line)

			if blockIsEmpty(lines, i) {
				out = append(out, "(empty)")
			}

			continue
		}

		if m := recordedFile.FindStringSubmatch(line); len(m) > 1 && bodies[m[1]] {
			out = append(out, m[1]+" ($SIZE bytes, $SUM)")

			continue
		}

		out = append(out, line)
	}

	return out
}

// blockIsEmpty reports whether the block opened at the header on line i has no
// content left: nothing but blank lines before the next header or the end.
func blockIsEmpty(lines []string, header int) bool {
	for j := header + 1; j < len(lines); j++ {
		line := lines[j]
		if strings.TrimSpace(line) == "" {
			continue
		}

		return strings.HasPrefix(line, "=== ")
	}

	return true
}

// skipIndentedUnder returns the index of the last line belonging to the block
// opened at start, whose key is indented by indent characters.
//
// A block ends at the first line indented no more deeply than its key. A blank
// line inside it belongs to it, because the rendered config puts one between
// type blocks and dropping the key without it would leave a double blank where
// v1.2.2 has one.
func skipIndentedUnder(lines []string, start, indent int) int {
	last := start

	for j := start + 1; j < len(lines); j++ {
		line := lines[j]

		if strings.TrimSpace(line) == "" {
			last = j

			continue
		}

		if len(line)-len(strings.TrimLeft(line, " \t")) <= indent {
			return last
		}

		last = j
	}

	return last
}

// configPreambleFirstLine is the opening comment of the generated .docz.yaml,
// which is what identifies the preamble rather than any comment anywhere.
const configPreambleFirstLine = "# .docz.yaml -- configuration for the docz CLI."

// isConfigPreambleStart reports whether the line at i opens the generated
// config's comment header.
func isConfigPreambleStart(lines []string, i int) bool {
	return lines[i] == configPreambleFirstLine
}

// skipCommentPreamble returns the index of the last line of the comment block
// opened at start: every following line that is a comment or blank, stopping at
// the first line of actual configuration.
func skipCommentPreamble(lines []string, start int) int {
	last := start

	for j := start + 1; j < len(lines); j++ {
		trimmed := strings.TrimSpace(lines[j])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			last = j

			continue
		}

		return last
	}

	return last
}
