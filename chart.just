# charts/docz — the one Helm chart for docz (DESIGN-0018, IMPL-0021)
#
# Loaded by the root justfile as an optional module (`mod? chart`), so its
# recipes run as `just chart lint` and root gates reach them as
# `chart::lint`. The chart is neither the server's nor the frontend's — it
# installs both — so its recipes live here rather than in api.just or
# ui.just (DESIGN-0018 OQ 13). The deprecated charts/docz-api and
# charts/docz-site keep their `just api helm-*` / `just ui helm-*` recipes
# until the v2.0.0 cut deletes them.

set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

chart_dir := "charts/docz"
ci_values := chart_dir / "ci/ci-values.yaml"

# Default: list recipes
_default:
    @just --list chart --unsorted

# Lint the chart (ci-values supplies the required values with no defaults)
[group('helm')]
lint:
    @helm lint {{ chart_dir }} -f {{ ci_values }}
    @echo "✓ charts/docz lints"

# Render the chart with the ci values (fast "does it template" check)
[group('helm')]
template:
    @helm template docz {{ chart_dir }} -f {{ ci_values }}

# The suites sit in tests/{api,site,chart}/, which the default glob misses.
# Run the chart's helm-unittest suites (needs the helm-unittest plugin)
[group('helm')]
unittest:
    @helm unittest -f 'tests/**/*_test.yaml' {{ chart_dir }}

# Regenerate the chart README from README.md.gotmpl + values.yaml
[group('helm')]
docs:
    @helm-docs --chart-search-root={{ chart_dir }}
