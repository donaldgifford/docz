#!/usr/bin/env bash
# The fixture repository with html.theme set to classic, so the page's
# theme comes from .docz.yaml and not from a flag.
set -euo pipefail
bash "$(dirname "${BASH_SOURCE[0]}")/../fixture/setup.sh"
printf '\nhtml:\n  theme: classic\n' >> .docz.yaml
git -c user.name=eval -c user.email=eval@example.invalid \
  commit -q -am "theme: classic"
