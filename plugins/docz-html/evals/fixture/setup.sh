#!/usr/bin/env bash
# Seeds a case's empty workspace with the fixture docz repository.
#
# Each case's scaffold.sh calls this, because a scaffold script runs in
# the workspace, not beside its case. With "exported", the pages are also
# built, for a case that starts from a page already made.
#
# docz comes from the scaffold's PATH (`just plugin-eval docz-html` puts
# this checkout's build/bin first) and is copied into the run's home. The
# agent's sandboxed Bash does not inherit that PATH: it gets a login PATH
# whose first entry is $HOME/.nix-profile/bin, with HOME the run's own
# temporary home, and it cannot read the real home or /tmp. The copy is
# what lets the skill's plain `docz ...` commands resolve.
set -euo pipefail

fixture="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/repo"

bin="${HOME}/.nix-profile/bin"
mkdir -p "${bin}"
cp "$(command -v docz)" "${bin}/docz"

cp -R "${fixture}/." .
git init -q
git add -A
git -c user.name=eval -c user.email=eval@example.invalid \
  commit -q -m "fixture"

if [[ "${1:-}" == "exported" ]]; then
  docz export html >/dev/null
fi
