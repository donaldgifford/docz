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
# The last two are optional imports, so they start working the moment the
# file exists and cost nothing until then.

set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

import 'docz.just'
import? 'api.just'
import? 'ui.just'

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
