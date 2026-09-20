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
| Captured | 2026-09-20, IMPL-0018 Phase 0 |
| Cases | 213 across 7 fixtures |

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
| The `types.plan` block in `docz config` and `docz init` output | ADR-0003 removes the built-in on the v2 line, so the rendered config changes | A named normaliser, added in Phase 4 with the removal |

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
  is kept, which is also what the walker does with it.

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
