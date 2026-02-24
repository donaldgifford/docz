#!/usr/bin/env bash
#
# Update docs/adr/README.md with auto-generated table of all ADRs
#
# Parses all ADR files and generates a table with:
# - ADR ID
# - Title
# - Status
# - Link to file

set -e

ADR_DIR="docs/adr"
README="$ADR_DIR/README.md"

# Function to extract status from ADR file
# Looks for "## Status" section and gets the next non-empty line
get_status() {
    local file="$1"
    awk '
        /^## Status/ { in_status = 1; next }
        in_status && /^[[:space:]]*$/ { next }
        in_status && /^[^#]/ { gsub(/\[|\]/, ""); print; exit }
    ' "$file"
}

# Function to extract title from ADR file
# Gets the first line (markdown h1) and extracts title after the number
get_title() {
    local file="$1"
    head -n 1 "$file" | sed -E 's/^#[[:space:]]*[0-9]+\.[[:space:]]*//'
}

# Start building the table
TABLE="| ID | Title | Status | Link |\n"
TABLE="${TABLE}|----|-------|--------|------|\n"

# Find all ADR files (excluding template and README)
for file in $(find "$ADR_DIR" -maxdepth 1 -name '[0-9][0-9][0-9][0-9]-*.md' | sort); do
    filename=$(basename "$file")

    # Extract ID from filename (first 4 digits)
    id=$(echo "$filename" | grep -o '^[0-9]\{4\}')

    # Extract title and status from file content
    title=$(get_title "$file")
    status=$(get_status "$file")

    # If status is empty, default to "Unknown"
    if [ -z "$status" ]; then
        status="Unknown"
    fi

    # Add row to table
    TABLE="${TABLE}| $id | $title | $status | [$filename]($filename) |\n"
done

# Read existing README if it exists, keeping content before the marker
HEADER=""
if [ -f "$README" ]; then
    # Extract everything before the auto-generated marker
    HEADER=$(awk '/<!-- BEGIN AUTO-GENERATED -->/ { exit } { print }' "$README")
fi

# If no header exists, create default header
if [ -z "$HEADER" ]; then
    HEADER="# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records documenting significant
technical decisions made during the development of hoomlab-api.

## What are ADRs?

ADRs document **technical implementation decisions** for specific architectural
components. Each ADR focuses on a single decision and includes:

- **Context**: The problem or constraint that led to this decision
- **Decision**: What was chosen and why
- **Consequences**: Trade-offs, pros, and cons
- **Alternatives**: Other options that were considered

## Creating a New ADR

\`\`\`bash
make adr \"Your ADR Title\"
\`\`\`

This will create a new ADR file with an auto-incremented ID.

## ADR Status

- **Proposed**: Under discussion, not yet approved
- **Accepted**: Approved and being implemented or already implemented
- **Deprecated**: No longer relevant or superseded
- **Superseded by ADR-XXXX**: Replaced by another ADR

"
fi

# Write the new README
{
    echo "$HEADER"
    echo "<!-- BEGIN AUTO-GENERATED -->"
    echo "## All ADRs"
    echo ""
    echo -e "$TABLE"
    echo "<!-- END AUTO-GENERATED -->"
} > "$README"

echo "✓ Updated $README"
