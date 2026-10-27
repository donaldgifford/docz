#!/usr/bin/env bash
#
# PostToolUse hook on Bash: hands the request to `docz-review arm
# --from-hook`, which reads it from stdin and records any docz document the
# command created. Without the binary it tells the person once per
# `docz create`, through a systemMessage (stderr on exit 0 reaches only
# the debug log), and lets the session carry on.

set -euo pipefail

readonly INSTALL_URL="https://github.com/donaldgifford/docz/tree/main/plugins/docz-review#install"

if command -v docz-review >/dev/null 2>&1; then
  exec docz-review arm --from-hook
fi

if grep -q 'docz create'; then
  printf '{"systemMessage":"docz-review is not installed, so this document will not be reviewed; see %s"}\n' "${INSTALL_URL}"
fi
exit 0
