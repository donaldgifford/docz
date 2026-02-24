#!/usr/bin/env bash
#
# Deprecation notice for bash scripts
# This script is sourced by other scripts to show deprecation warnings

show_deprecation_notice() {
    local script_name="$1"
    local rex_command="$2"

    cat >&2 << EOF

╔════════════════════════════════════════════════════════════════╗
║                     ⚠️  DEPRECATION NOTICE                     ║
╠════════════════════════════════════════════════════════════════╣
║                                                                ║
║  This bash script is DEPRECATED and will be removed in v2.0   ║
║                                                                ║
║  Please use the rex CLI instead:                              ║
║                                                                ║
║    $ ${rex_command}
║                                                                ║
║  Migration Guide: docs/MIGRATION.md                           ║
║  Rex Documentation: README.md                                 ║
║                                                                ║
║  Install rex:                                                 ║
║    $ go install github.com/donaldgifford/rex@latest           ║
║                                                                ║
╚════════════════════════════════════════════════════════════════╝

EOF

    # Give user time to see the notice
    sleep 2
}
EOF