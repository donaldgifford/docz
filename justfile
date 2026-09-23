# docz — task runner
#
# The root file is composition (ADR-0004 Decision 4): each half of the
# repository keeps its own recipes in its own file, and this one imports
# them and defines the gates that span them.
#
#   docz.just   the library and the CLI
#   api.just    the server, when docz-api moves in
#   ui.just     the frontend, when docz-site moves in
#
# docz.just is imported flat, so its recipes own the root namespace
# (just build, just test, just ci). The other two are optional *modules*
# (DESIGN-0016 OQ 1): docz-api's recipes share 27 names with docz.just and
# just refuses duplicate names across sibling imports, so each arriving half
# gets its own namespace — just api build, just ui build — and root gates
# reach them as api::lint. `mod?` means neither has to exist yet. ui.just
# will carry `set working-directory := "ui"` for its Bun commands.

set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

import 'docz.just'
mod? api 'api.just'
mod? ui 'ui.just'

# Default: list recipes
_default:
    @just --list --unsorted

# ─── Composite gates ───────────────────────────────────────────────

# Full CI gate: lint + test + consumer + parity + validate + build + licences
[group('gate')]
ci: lint test test-consumer parity validate build license-check
    @echo "✓ CI pipeline complete"

# Pre-commit gate: lint + test
[group('gate')]
check: lint test
    @echo "✓ Pre-commit checks passed"
