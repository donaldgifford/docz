#!/usr/bin/env bash
#
# PR gate for the generated API client. The output dir is gitignored, so
# there is no committed copy to `git diff` — instead: snapshot what's on
# disk, regenerate from the vendored spec, and fail if they differ. That
# catches a spec or orval-config edit whose regenerated client wasn't
# picked up, and nondeterministic generation in CI (where a prior
# `bun run gen-api` provides the snapshot).
set -euo pipefail

cd "$(dirname "$0")/.."

readonly GEN_DIR="src/api/__generated__"
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

bun run gen-api >/dev/null

if [[ -z "${tmp_dir}" ]]; then
  echo "gen-api-check: no existing ${GEN_DIR}; generated a fresh client."
  exit 0
fi

if ! diff -r "${tmp_dir}/before" "${GEN_DIR}" >/dev/null; then
  echo "gen-api-check: ${GEN_DIR} was stale — regeneration changed it." >&2
  echo "gen-api-check: the fresh output is now in place; review and re-run." >&2
  exit 1
fi

echo "gen-api-check: generated client is current."
