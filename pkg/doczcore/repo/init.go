package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/index"
)

// InitOptions is what a caller can vary about scaffolding a repository.
type InitOptions struct {
	// Force rewrites an index README that is already there. Without it an
	// existing file is reported as InitSkipped, because running init twice is
	// a normal thing to do and the second run must not eat the first run's
	// edits.
	//
	// Force does not reach .docz.yaml, which is never overwritten. See
	// initConfig.
	Force bool
}

// InitAction names what Init did with one file.
//
// Three outcomes rather than a bool, because "skipped" and "overwritten" are
// the two a user needs to be told apart: one preserved their work and the
// other replaced it.
type InitAction int

const (
	// InitCreated means the file was absent and Init wrote it.
	InitCreated InitAction = iota + 1
	// InitSkipped means the file was already there and Force was not set, so
	// it was left exactly as it was.
	InitSkipped
	// InitOverwritten means the file was already there and Force replaced it.
	InitOverwritten
)

// String returns the lower-case action name, or "unknown" for a zero or
// out-of-range value.
//
// Exported behaviour rather than a caller-side switch, so two consumers
// cannot word the same run differently.
func (a InitAction) String() string {
	switch a {
	case InitCreated:
		return "created"
	case InitSkipped:
		return "skipped"
	case InitOverwritten:
		return "overwritten"
	default:
		return "unknown"
	}
}

// InitFile is one file Init considered, and what happened to it.
type InitFile struct {
	// Path is relative to Repo.Root.
	Path string
	// Action is what Init did.
	Action InitAction
}

// InitReport is everything Init did, in a stable order: .docz.yaml first,
// then one README per enabled type in Cfg.EnabledTypes order.
//
// Directories are not listed. A directory is a means rather than an outcome —
// nobody needs telling that docs/adr/ exists — and a report that mixed the
// two would make "how many files did init write" a question about which
// entries to filter out.
type InitReport struct {
	Files []InitFile
}

// Init scaffolds a repository: the configuration file, a directory per
// enabled type, and an index README in each.
//
// Every enabled type is scaffolded, custom types included, because the list
// comes from Cfg.EnabledTypes (DESIGN-0014 §2.8). A type switched off in the
// config gets no directory, which is the whole point of switching it off.
//
// The context is checked between types, and a cancelled run returns the
// report completed so far together with ctx.Err(). What was already written
// stays written: a half-scaffolded repository is fixed by running init again,
// and unwinding it would delete files the user may have edited in between.
func (r *Repo) Init(ctx context.Context, opts InitOptions) (InitReport, error) {
	var report InitReport

	// Checked before the first write as well as between types, so a run whose
	// context was already cancelled writes nothing at all rather than one
	// file.
	if err := ctx.Err(); err != nil {
		return report, err
	}

	cfgFile, err := r.initConfig(ctx)
	if err != nil {
		return report, err
	}

	report.Files = append(report.Files, cfgFile)

	for _, typeName := range r.Cfg.EnabledTypes() {
		if err := ctx.Err(); err != nil {
			return report, err
		}

		readme, err := r.initType(ctx, typeName, opts.Force)
		if err != nil {
			return report, err
		}

		report.Files = append(report.Files, readme)
	}

	return report, nil
}

// initConfig writes .docz.yaml from the same defaults every other consumer
// renders, and never overwrites one that is already there.
//
// Not even with Force, which is the one file that rule applies to. A
// configuration is the thing in a repository most likely to have been edited
// by hand and least likely to be reconstructible from defaults: whatever a
// user ran `init --force` to fix, it was not their own `.docz.yaml`. Deleting
// the file is how you ask for a fresh one, and that is an explicit act.
//
// doctemplate.DefaultConfigYAML rather than a literal here, so a new config
// key reaches a scaffolded repository by being added to config.DefaultConfig
// and nowhere else.
func (r *Repo) initConfig(ctx context.Context) (InitFile, error) {
	path := r.Path(config.ConfigFileName)
	rel := r.RelPath(path)

	action, write := initAction(path, false)
	if !write {
		fireFileSkipped(ctx, rel, SkipExists)

		return InitFile{Path: rel, Action: action}, nil
	}

	body, err := doctemplate.DefaultConfigYAML()
	if err != nil {
		return InitFile{}, fmt.Errorf("rendering default config: %w", err)
	}

	if err := r.writeAndFire(ctx, path, []byte(body), FileConfig); err != nil {
		return InitFile{}, err
	}

	return InitFile{Path: rel, Action: action}, nil
}

// initType creates one type's directory and its index README.
//
// The directory is created even when the README is skipped: a repository that
// lost a directory but kept its README is not a state worth preserving, and
// MkdirAll on a directory that is already there does nothing.
func (r *Repo) initType(ctx context.Context, typeName string, force bool) (InitFile, error) {
	dir := r.TypeDir(typeName)
	if err := os.MkdirAll(dir, config.DirMode); err != nil {
		return InitFile{}, &WriteError{Path: r.RelPath(dir), Err: err}
	}

	path := r.ReadmePath(typeName)
	rel := r.RelPath(path)

	action, write := initAction(path, force)
	if !write {
		fireFileSkipped(ctx, rel, SkipExists)

		return InitFile{Path: rel, Action: action}, nil
	}

	header, err := doctemplate.ResolveIndexHeader(typeName, r.Path(r.Cfg.DocsDir), doctemplate.IndexHeaderData{
		TypeName:    typeName,
		PluralLabel: r.indexLabelFor(typeName),
	})
	if err != nil {
		return InitFile{}, fmt.Errorf("resolving index header for %s: %w", typeName, err)
	}

	// index.Scaffold and nothing else. The body is the header plus exactly one
	// marker pair, and appending a pair here as cmd/init.go does today is
	// issue #99: two of the embedded headers end with their own pair, so every
	// new repository got a second, permanently empty one that no splice ever
	// touches again.
	if err := r.writeAndFire(ctx, path, index.Scaffold(header), FileIndex); err != nil {
		return InitFile{}, err
	}

	return InitFile{Path: rel, Action: action}, nil
}

// initAction decides what to do with one destination and reports both the
// action to record and whether to write.
//
// Shared with ExportTemplate's claimDest, so there is one rule in the package
// for what an existing file means.
//
// A stat that fails for any reason counts as absent. The alternative is
// deciding a permission error means "skip", which would report a file as
// preserved that Init never looked inside; letting the write attempt fail
// instead surfaces the real error with the real path.
func initAction(path string, force bool) (action InitAction, write bool) {
	if _, err := os.Stat(path); err != nil {
		return InitCreated, true
	}

	if force {
		return InitOverwritten, true
	}

	return InitSkipped, false
}

// writeAndFire creates the parent directory, writes the file, and fires the
// FileWritten hook.
//
// The single write path for the operations in this package, so the file mode,
// the parent-directory creation, and the narration cannot drift between them.
// The path in the returned WriteError is repo-relative, like every other path
// the package reports; a caller that needs the absolute one joins it back
// under Root.
func (r *Repo) writeAndFire(ctx context.Context, path string, body []byte, kind FileKind) error {
	rel := r.RelPath(path)

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, config.DirMode); err != nil {
			return &WriteError{Path: rel, Err: err}
		}
	}

	if err := os.WriteFile(path, body, config.FileMode); err != nil {
		return &WriteError{Path: rel, Err: err}
	}

	fireFileWritten(ctx, rel, kind)

	return nil
}
