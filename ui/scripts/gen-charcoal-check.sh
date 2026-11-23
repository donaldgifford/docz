#!/usr/bin/env bash
#
# Drift gate for pkg/theme's generated charcoal theme (DESIGN-0028 §11),
# the frontend's half of two checks: regenerate to a temporary file from
# the installed design system and fail if the committed file differs.
# The Go half, TestCharcoalMatchesPinnedDesignSystem, holds the version
# in the file's header to ui/bun.lock; this one holds the bytes, and
# needs node_modules to do it.
#
# Unlike the other gen-*-check scripts it never rewrites the tree: the
# file is committed, so a difference is left for `just gen-charcoal`.
set -euo pipefail

cd "$(dirname "$0")/.."

readonly COMMITTED="../pkg/theme/tokens/charcoal.css"
tmp_dir="$(mktemp -d)"

cleanup() {
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT

bun scripts/gen-charcoal.ts --out "${tmp_dir}/charcoal.css" >/dev/null

if ! diff -u "${COMMITTED}" "${tmp_dir}/charcoal.css" >&2; then
  echo "gen-charcoal-check: ${COMMITTED} is not what the generator writes." >&2
  echo "gen-charcoal-check: it is generated; run 'just gen-charcoal' and commit the result." >&2
  exit 1
fi

echo "gen-charcoal-check: charcoal.css is current."
