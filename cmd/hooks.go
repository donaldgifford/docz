package cmd

import (
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// hooks returns the repo.Hooks that narrate an operation onto this
// Runner's logger at debug level.
//
// This is the whole of how a library that prints nothing (DESIGN-0014
// rule R4) still produces `docz --verbose` output: the five events are
// the debug lines cmd/update.go and cmd/init.go used to emit inline, so
// wiring them here moves the lines rather than losing them. Nothing is
// logged above debug, because a hook firing is not news.
//
// Two of the five keep their old message verbatim (DESIGN-0014 §7 pins
// them): "scanning type" and "scan complete". The other three are
// reworded to the hook's shape, because no single old message matches
// them one-to-one — "type disabled, skipping" was the only reason a type
// was ever skipped, and both "config file exists, skipping" and "readme
// exists, skipping" are one FileSkipped with reason "already exists". The
// reason and kind carry what the old wording said, as attributes rather
// than as English. No golden or test pins any of these strings: the
// parity fixtures never pass --verbose.
//
// The closures capture the Runner, not its logger, so a caller that
// replaces r.Logger after wiring the hooks still gets its records.
func (r *Runner) hooks() *repo.Hooks {
	return &repo.Hooks{
		ScanStart: func(_, dir string) {
			r.Logger.Debug("scanning type", "dir", dir)
		},
		ScanDone: func(typeName string, docs int) {
			r.Logger.Debug("scan complete", "type", typeName, "count", docs)
		},
		TypeSkipped: func(typeName string, reason repo.SkipReason) {
			r.Logger.Debug("type skipped", "type", typeName, "reason", reason.String())
		},
		FileWritten: func(path string, kind repo.FileKind) {
			r.Logger.Debug("file written", "path", path, "kind", kind.String())
		},
		FileSkipped: func(path string, reason repo.SkipReason) {
			r.Logger.Debug("file skipped", "path", path, "reason", reason.String())
		},
	}
}
