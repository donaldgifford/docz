# docz — task runner
#
# The root file is composition (ADR-0004 Decision 4): each half of the
# repository keeps its own recipes in its own file, and this one imports
# them and defines the gates that span them.
#
#   docz.just   the library and the CLI
#   api.just    the server (docz-api, IMPL-0019)
#   ui.just     the frontend (docz-site, IMPL-0020)
#   chart.just  charts/docz, the one Helm chart for both (IMPL-0021)
#
# docz.just is imported flat, so its recipes own the root namespace
# (just build, just test, just ci). The other three are optional *modules*
# (DESIGN-0016 OQ 1): docz-api's recipes share 27 names with docz.just and
# just refuses duplicate names across sibling imports, so each arriving half
# gets its own namespace — just api build, just ui build, just chart lint —
# and root gates reach them as api::lint. ui.just carries
# `set working-directory := "ui"` for its Bun commands.

set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

import 'docz.just'
mod? api 'api.just'
mod? ui 'ui.just'
mod? chart 'chart.just'

# Default: list recipes
_default:
    @just --list --unsorted

# ─── Composite gates ───────────────────────────────────────────────

# Full CI gate: lint + test + consumer + parity + validate + build + licences,
# then the server's lint, tests, and chart lint from the api module, then the
# frontend's chain from the ui module (DESIGN-0017 OQ 6). ui::e2e stays out:
# it needs Playwright's browsers, and CI runs it as its own job.
[group('gate')]
ci: lint test test-consumer parity validate build license-check api::lint api::test api::helm-lint ui::install ui::gen-api ui::lint ui::fmt-check ui::typecheck ui::test ui::test-server ui::build ui::bundle-budget ui::gen-api-check
    @echo "✓ CI pipeline complete"

# Pre-commit gate: lint + test
[group('gate')]
check: lint test
    @echo "✓ Pre-commit checks passed"
