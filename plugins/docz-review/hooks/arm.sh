#!/usr/bin/env bash
#
# PostToolUse hook on Bash: hands the request to `docz-review arm
# --from-hook`, which reads it from stdin and records any docz document the
# command created. Without the binary it prints one line and lets the
# session carry on.

set -euo pipefail

readonly INSTALL_URL="https://github.com/donaldgifford/docz/tree/main/plugins/docz-review#install"

if command -v docz-review >/dev/null 2>&1; then
  exec docz-review arm --from-hook
fi

echo "docz-review is not installed; see ${INSTALL_URL}" >&2
exit 0
