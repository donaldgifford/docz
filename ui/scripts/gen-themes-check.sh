#!/usr/bin/env bash
#
# Drift gate for the generated theme list, the twin of gen-cli-check.sh:
# snapshot src/theme/__generated__, regenerate from ../pkg/theme/tokens,
# and fail if they differ.
set -euo pipefail

cd "$(dirname "$0")/.."

readonly GEN_DIR="src/theme/__generated__"
tmp_dir=""

cleanup() {
  if [[ -n "${tmp_dir}" && -d "${tmp_dir}" ]]; then
    rm -rf "${tmp_dir}"
  fi
}
trap cleanup EXIT

if [[ -d "${GEN_DIR}" ]]; then
  tmp_dir="$(mktemp -d)"
  mkdir -p "${tmp_dir}/before"
  cp -R "${GEN_DIR}/." "${tmp_dir}/before/"
fi

bun run gen-themes >/dev/null

if [[ -z "${tmp_dir}" ]]; then
  echo "gen-themes-check: no existing ${GEN_DIR}; generated it fresh."
  exit 0
fi

if ! diff -r "${tmp_dir}/before" "${GEN_DIR}" >/dev/null; then
  echo "gen-themes-check: ${GEN_DIR} was stale — regeneration changed it." >&2
  echo "gen-themes-check: the fresh output is now in place; review and re-run." >&2
  exit 1
fi

echo "gen-themes-check: generated theme list is current."
