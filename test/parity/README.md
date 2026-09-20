# Functional parity suite

This suite is the evidence behind ADR-0002 Decision 4. The v2 line rebuilds
docz as an API and moves the CLI onto it; "moved, not changed" is a claim
until a byte comparison backs it. So the goldens here record what the
**v1.2.2 release binary** printed, exited with, and wrote, and every later
build has to reproduce it.

```bash
make parity                     # build build/bin/docz, replay the goldens
make parity BIN=/path/to/docz   # replay against another binary
make parity-capture             # re-install v1.2.2 and re-capture (rare, see below)
```

The driver is a Go test behind the `parity` build tag, so `go test ./...`
stays a source-only run and nothing in CI needs a binary until Phase 5 wires
`make parity` into `make ci`.

## Capture provenance

| Fact | Value |
| ---- | ----- |
| Source tag | `v1.2.2` |
| Module | `github.com/donaldgifford/docz` (the pre-`/v2` path) |
| Package | `github.com/donaldgifford/docz/cmd/docz` |
| Module checksum | `h1:0jzdvuGSG33DaBAv7BvAeFcqkVahXk0cKE+AOSpCYwA=` |
| go.mod checksum | `h1:YWffd55Zk1BGmaaw1cm4sPaIbnJuzi9wbRaBYtV7EnQ=` |
| Toolchain | `go1.26.4` |
| Platform | `darwin/arm64` |
| Captured | 2026-09-20, IMPL-0018 Phase 0; re-captured the same day in Phase 1 |
| Cases | 213 across 7 fixtures |

The Phase 1 re-capture changed only the size and digest on each recorded file:
the `markers` normaliser learned to take a marker's blank line with it, and
sizes moved onto the normalised body. Every content line of every golden is
byte-identical to the Phase 0 capture, and the goldens still come from the
v1.2.2 binary, not from a v2 build.

`make parity-capture` installs the tag into a temporary `GOBIN` rather than
trusting whatever `docz` is on `PATH`. The installed binary reports
`docz dev (commit: none)` because `go install` applies no ldflags; that is
expected and is why `version` is not one of the recorded cases.

Set `DOCZ_PARITY_BIN` to capture from a binary you already have instead of
installing the tag.

## The one rule

**Goldens come from v1.2.2 and from nowhere else.** Never re-run
`make parity-capture` against a v2 build: the suite would then agree with
whatever it happens to be measuring, and the only thing it proves is that
the code equals itself.

A golden is edited by hand only under a permitted delta, one file at a time,
with the reason in the pull request that does it.

## Permitted deltas

DESIGN-0014 §4 allows three, and IMPL-0018 Open Question 8 adds a fourth:

| Delta | Why | How it is handled |
| ----- | --- | ----------------- |
| Region marker lines in `create` and `template` output | Phase 1 adds markers to every template section | The `markers` normaliser drops whole marker lines before comparison |
| New commands and flags | `docz validate` did not exist in v1.2.2 | No golden covers them; they get their own tests |
| New findings printed by existing commands | warnings the v1 CLI could not produce | Argued for per case in the PR that adds them |
| Every trace of the `plan` document type | ADR-0003 removes the built-in on the v2 line | The `plan` normaliser, applied to **both** sides at comparison time |

Anything else that differs is a regression until someone shows otherwise.

## Normalisers

Each is named, lives outside the build tag, and has unit tests that run in
`make test`:

- **root** replaces the fixture's temporary root with `$ROOT`. Both the
  `/var/folders/...` and `/private/var/folders/...` spellings are replaced,
  because a temp dir is handed out as one and reported as the other on macOS.
- **date** replaces today with `$DATE`. `docz create` stamps today into
  frontmatter, so without this every golden would expire overnight.
- **markers** drops whole region-marker lines. A marker with text beside it
  is kept, which is also what the walker does with it. A marker alone between
  two blank lines takes one of them with it: the templates put every marker on
  its own line and separate sections with a single blank, so keeping both
  blanks would turn every section break into a false difference.
- **plan** removes every trace of the document type ADR-0003 dropped. Five
  traces: the `plan:` block under `types:`, the `plan: Plans` entry under
  `wiki.nav_titles`, the `config declares non-built-in type "plan"` warning a
  repo keeping its block now gets on every command, the comment preamble of a
  generated `.docz.yaml` (v2 rewrote it to say five built-in types and to
  explain the fallback), and the `PLAN-XXXX` placeholder in the IMPL and INV
  templates' *Implements* and *Triggered by* hints. Every rule is anchored on a
  spelling only the type uses, so `impl: Implementation Plans` and
  `## Testing Plan` are left alone.

**plan is the one normaliser that runs on both sides**, in `runCase` rather
than in the `norms` list, because the golden is the side carrying the removed
type: normalising only the captured output would leave every trace as a
difference. Two consequences follow from that symmetry.

A `stdout` or `stderr` block the pass empties is rewritten to `(empty)`, so a
legacy fixture whose only stderr was the plan warning matches a run that
printed nothing.

A file whose body the golden records loses its recorded size and digest to
`$SIZE` and `$SUM`. A byte count computed before a line was dropped cannot be
recomputed from the golden's text, and the rule has to be the same on both
sides — the side that no longer carries the trace cannot know one was there.
Nothing is lost: a recorded body is compared line by line, so its digest is
the redundant half of that check, and a file with no recorded body keeps the
digest as its only one.

A recorded size and digest are computed from the normalised body, not the raw
one, so the number beside a body describes the body the golden shows. Both
tree snapshots go through the same pass, so the before/after comparison that
decides which bodies to record still compares like with like.

## The run environment is an allowlist

The driver does not hand `os.Environ()` to the binary under test. It builds
the environment from scratch: `PATH`, `HOME` pointed at the fixture copy,
`TZ=UTC`, the two `GIT_CONFIG_*` nulls, and `NO_COLOR`.

That is not belt-and-braces. docz reads a global `~/.docz.yaml` and
deep-merges it *under* the repo's, so an inherited `HOME` would merge
whoever ran the capture into all 213 cases and commit their author name,
paths, and custom types into `testdata/*/config.golden`. `TZ` is pinned
because the driver computes today's date for the `date` normaliser while the
binary stamps its own; a run spanning midnight in another zone would miss.
Nothing CI puts in the environment, `GITHUB_TOKEN` included, reaches the
binary.

Each invocation is bounded at 30 seconds with a one-second `WaitDelay`, so a
hung case names itself instead of consuming the whole `go test` deadline.

## Known limits

- **Fixture file modes are not preserved.** `copyTree` writes 0o644 files and
  0o755 directories, so a fixture cannot pin a permission-error path such as
  a read-only `docs/` or an unreadable `.docz.yaml`. Preserving the source
  mode is the fix if a case ever needs one.
- **Only regular files may live in `fixtures/`.** A symlink or FIFO fails the
  copy with an error rather than being skipped: `os.ReadFile` follows a
  symlink, so an entry pointing at a file outside the repo would be copied
  in, read back, and embedded in a committed golden, and symlink mode is
  invisible in a diff.
- **Both tree snapshots hold every file body in memory.** Fine at 148K of
  fixtures; it does mean `Tree` is not something to point at an arbitrary
  directory.

## Fixtures

Seven repositories under `fixtures/`, each with `author.from_git: false` and
a pinned author so `docz create` output does not depend on who runs the
suite. Document `created:` dates are pinned to 2026-03-04.

| Fixture | What it holds | Why |
| ------- | ------------- | --- |
| `rfc`, `adr`, `design`, `impl`, `investigation` | one document straight from the type's template, one hand-written in the fleet's messier style | the template path and the real-world path differ, and both have to survive |
| `custom` | a `frameworks` type with `docs/templates/frameworks.md`, alias `fw`, prefix `FW`, beside a built-in `adr` | custom-type resolution by name, alias, and prefix (IMPL-0012), and a built-in and custom type in one repo |
| `legacy` | a config that still carries the pre-v1.1.1 `plan:` block, dormant | the block must keep loading and keep appearing in `docz config` after ADR-0003 removes the built-in |

Every fixture document already carries canonical region markers, so no later
phase migrates a fixture and no golden moves for that reason.

The `legacy` fixture deliberately records no `create plan` or
`template show|export plan` case. Those three are the only places where
ADR-0003's removal is an expected change rather than a regression, so they
are left out instead of being normalised away.

Fixture documents are not held to this repo's markdown standards. Their
messiness is the point.

## Golden format

One file per case at `testdata/<fixture>/<case>.golden`:

```text
$ docz create rfc Parity created document
exit 0

=== stdout
...
=== stderr
(empty)

=== files
docs/rfc/0001-....md (1695 bytes, 587c4679eeef)
...

=== written docs/rfc/0003-parity-created-document.md
<the full bytes of what the command wrote>
```

Every file in the tree is listed with its size and a digest, so a deletion
or an unexpected edit fails the golden. Full bodies are recorded only for
files the case created or changed, which keeps a golden readable: a reviewer
sees the new document, not nine unchanged ones.
