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
