#!/usr/bin/env bash
#
# Drift gate for the generated CLI types and open schemas, the twin of
# gen-api-check.sh: snapshot src/cli/__generated__, regenerate from
# ../api/cli, and fail if they differ.
set -euo pipefail

cd "$(dirname "$0")/.."

readonly GEN_DIR="src/cli/__generated__"
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

bun run gen-cli >/dev/null

if [[ -z "${tmp_dir}" ]]; then
  echo "gen-cli-check: no existing ${GEN_DIR}; generated fresh types."
  exit 0
fi

if ! diff -r "${tmp_dir}/before" "${GEN_DIR}" >/dev/null; then
  echo "gen-cli-check: ${GEN_DIR} was stale — regeneration changed it." >&2
  echo "gen-cli-check: the fresh output is now in place; review and re-run." >&2
  exit 1
fi

echo "gen-cli-check: generated CLI types are current."
