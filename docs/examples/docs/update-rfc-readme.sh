#!/usr/bin/env bash
#
# Update docs/rfc/README.md with auto-generated table of all RFCs
#
# Parses all RFC files and generates a table with:
# - RFC ID
# - Title
# - Status
# - Link to file

set -e

RFC_DIR="docs/rfc"
README="$RFC_DIR/README.md"

# Function to extract status from RFC file
# Looks for "**Status**:" line
get_status() {
    local file="$1"
    grep -i "^\*\*Status\*\*:" "$file" | head -1 | sed -E 's/^\*\*Status\*\*:[[:space:]]*//' || echo "Unknown"
}

# Function to extract title from RFC file
# Gets line with "# RFC [Number]:" and extracts title
get_title() {
    local file="$1"
    grep "^# RFC" "$file" | head -1 | sed -E 's/^#[[:space:]]*RFC[[:space:]]*[0-9]+:[[:space:]]*//'
}

# Start building the table
TABLE="| ID | Title | Status | Link |\n"
TABLE="${TABLE}|----|-------|--------|------|\n"

# Find all RFC files (excluding template, README, and backup files)
for file in $(find "$RFC_DIR" -maxdepth 1 -name '[0-9][0-9][0-9][0-9]-*.md' ! -name '*-original.md' ! -name '*-backup.md' | sort); do
    filename=$(basename "$file")

    # Extract ID from filename (first 4 digits)
    id=$(echo "$filename" | grep -o '^[0-9]\{4\}')

    # Extract title and status from file content
    title=$(get_title "$file")
    status=$(get_status "$file")

    # If title or status is empty, use defaults
    if [ -z "$title" ]; then
        title=$(echo "$filename" | sed -E 's/^[0-9]+-//' | sed 's/-/ /g' | sed 's/.md$//')
    fi
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
    HEADER="# Requests for Comments (RFCs)

This directory contains RFCs documenting high-level proposals for major features
and system redesigns in hoomlab-api.

## What are RFCs?

RFCs document **high-level problem definitions and solution strategies**. Each
RFC focuses on:

- **Problem Statement**: The issue being addressed with evidence
- **Proposed Solution**: High-level approach and architecture
- **Implementation Phases**: Overview of how the solution will be built
- **Alternatives**: Other approaches that were considered
- **Risks and Success Criteria**: What could go wrong and how we measure success

RFCs are broader than ADRs and typically reference multiple ADRs for technical
implementation details.

## Creating a New RFC

\`\`\`bash
make rfc \"Your RFC Title\"
\`\`\`

This will create a new RFC file with an auto-incremented ID.

## RFC Status

- **Draft**: Initial draft, not yet ready for review
- **Proposed**: Ready for review and feedback
- **Accepted**: Approved and ready for implementation
- **Rejected**: Not moving forward with this proposal
- **Superseded**: Replaced by another RFC

"
fi

# Write the new README
{
    echo "$HEADER"
    echo "<!-- BEGIN AUTO-GENERATED -->"
    echo "## All RFCs"
    echo ""
    echo -e "$TABLE"
    echo "<!-- END AUTO-GENERATED -->"
} > "$README"

echo "✓ Updated $README"
