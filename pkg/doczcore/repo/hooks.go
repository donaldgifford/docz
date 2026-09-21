package repo

import "context"

// Hooks and the context they ride in (DESIGN-0014 §7, rule R8).
//
// The net/http/httptrace.ClientTrace pattern: a struct of optional
// callbacks carried in the context, called synchronously when non-nil,
// never retained. A struct of funcs rather than an interface (R6), so
// adding an event does not break every existing implementation and a
// caller wires only the events it cares about.
//
// This is how a library that prints nothing (R4) still narrates. The five
// events are exactly the debug lines cmd/update.go, cmd/init.go, and
// cmd/create.go emit today, so the Phase 5 swap wires them back
// one-to-one and the existing debug-log test pins the mapping from the
// other side.

// unknownName is what every String method here reports for a zero or
// out-of-range value. One spelling, so a narration that hits an enum value
// added after the consumer was built says the same thing everywhere.
const unknownName = "unknown"

// FileKind names what a written file is, so a hook can report "wrote the
// index" rather than pattern-matching a path.
type FileKind int

const (
	// FileToC is a document rewritten by the table-of-contents splice.
	FileToC FileKind = iota + 1
	// FileIndex is a type directory's README.
	FileIndex
	// FileDocument is a document created or mutated as a whole.
	FileDocument
	// FileConfig is .docz.yaml.
	FileConfig
)

// String returns the lower-case kind name, or "unknown" for a zero or
// out-of-range value.
func (k FileKind) String() string {
	switch k {
	case FileToC:
		return "toc"
	case FileIndex:
		return "index"
	case FileDocument:
		return "document"
	case FileConfig:
		return "config"
	default:
		return unknownName
	}
}

// SkipReason names why a type or file was passed over.
//
// A reason rather than a bool, because "skipped" on its own is the least
// useful thing a narration can say: a disabled type and a README nobody
// marked are both skips, and only one of them is worth telling the user
// about.
type SkipReason int

const (
	// SkipTypeDisabled means the type is switched off in the config.
	SkipTypeDisabled SkipReason = iota + 1
	// SkipExists means the file was already there and no overwrite was
	// requested.
	SkipExists
	// SkipNoMarkers means the file has no docz marker pair to splice into,
	// so it is somebody's own content and not docz's to rewrite.
	SkipNoMarkers
	// SkipNotDoczFile means the filename does not match the docz
	// convention, so it is not a document.
	SkipNotDoczFile
)

// String returns a short reason name, or "unknown" for a zero or
// out-of-range value.
func (s SkipReason) String() string {
	switch s {
	case SkipTypeDisabled:
		return "type disabled"
	case SkipExists:
		return "already exists"
	case SkipNoMarkers:
		return "no markers"
	case SkipNotDoczFile:
		return "not a docz file"
	default:
		return unknownName
	}
}

// Hooks carries optional callbacks. Every field may be nil, and a nil
// Hooks is itself valid — the zero value is all no-ops.
//
// Callbacks run synchronously on the calling goroutine, so a slow one
// slows the operation down. That is deliberate: a hook that buffers or
// spawns is the caller's choice to make, and a library that decided it for
// them could not be used from a Temporal activity that must heartbeat in
// order.
type Hooks struct {
	// ScanStart fires before a type directory is read.
	ScanStart func(typeName, dir string)
	// ScanDone fires after a type directory is read, with the document count.
	ScanDone func(typeName string, docs int)
	// TypeSkipped fires instead of ScanStart when a type is passed over.
	TypeSkipped func(typeName string, reason SkipReason)
	// FileWritten fires after each successful write. It does not fire on a
	// dry run, because nothing was written.
	FileWritten func(path string, kind FileKind)
	// FileSkipped fires when a file that was a candidate for writing was
	// left alone.
	FileSkipped func(path string, reason SkipReason)
}

// hooksKey is the context key for Hooks. An unexported struct type, so no
// other package can collide with it and no string constant can be guessed.
type hooksKey struct{}

// WithHooks returns a copy of ctx carrying h.
//
// A nil h is stored as nil and HooksFrom hands back the no-op set, so
// WithHooks(ctx, nil) is a legal way to clear hooks a caller inherited.
func WithHooks(ctx context.Context, h *Hooks) context.Context {
	return context.WithValue(ctx, hooksKey{}, h)
}

// HooksFrom returns the hooks carried by ctx, never nil.
//
// A non-nil return for every context is what lets the call sites read
// HooksFrom(ctx).ScanStart without a guard at each one; only the field
// needs checking, and fire does that.
func HooksFrom(ctx context.Context) *Hooks {
	if h, ok := ctx.Value(hooksKey{}).(*Hooks); ok && h != nil {
		return h
	}

	return &Hooks{}
}

// The five fire helpers. Each is one guarded call, so a call site reads as
// the event it is reporting and no method body grows a nil check.

func fireScanStart(ctx context.Context, typeName, dir string) {
	if fn := HooksFrom(ctx).ScanStart; fn != nil {
		fn(typeName, dir)
	}
}

func fireScanDone(ctx context.Context, typeName string, docs int) {
	if fn := HooksFrom(ctx).ScanDone; fn != nil {
		fn(typeName, docs)
	}
}

func fireTypeSkipped(ctx context.Context, typeName string, reason SkipReason) {
	if fn := HooksFrom(ctx).TypeSkipped; fn != nil {
		fn(typeName, reason)
	}
}

func fireFileWritten(ctx context.Context, path string, kind FileKind) {
	if fn := HooksFrom(ctx).FileWritten; fn != nil {
		fn(path, kind)
	}
}

func fireFileSkipped(ctx context.Context, path string, reason SkipReason) {
	if fn := HooksFrom(ctx).FileSkipped; fn != nil {
		fn(path, reason)
	}
}
