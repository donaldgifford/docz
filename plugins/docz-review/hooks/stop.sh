#!/usr/bin/env bash
#
# Stop hook: opens a review of every pending document through
# `docz-review --pending --hook`, which blocks until the reviewer is done
# and prints the block decision when they asked for changes. Without the
# binary nothing can have been armed, so it lets the turn end with one
# line on stderr, which Claude Code shows only in its debug log; arm.sh
# is the hook that tells the person.

set -euo pipefail

readonly INSTALL_URL="https://github.com/donaldgifford/docz/tree/main/plugins/docz-review#install"

if command -v docz-review >/dev/null 2>&1; then
  exec docz-review --pending --hook
fi

echo "docz-review is not installed; see ${INSTALL_URL}" >&2
exit 0
