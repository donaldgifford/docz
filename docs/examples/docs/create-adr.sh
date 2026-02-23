#!/usr/bin/env bash
#
# Create a new ADR (Architecture Decision Record) from template
#
# Usage: make adr "Your ADR Title"
#    or: ./tools/create-adr.sh "Your ADR Title"

set -e

# Show deprecation notice
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../DEPRECATED.sh" 2>/dev/null || true
show_deprecation_notice "create-adr.sh" "rex adr create \"Your ADR Title\"" 2>/dev/null || true

TITLE="$1"

# Validate input
if [ -z "$TITLE" ]; then
    echo "Error: ADR title is required"
    echo "Usage: make adr \"Your ADR Title\""
    exit 1
fi

# Find the next ADR number by looking at existing files
LAST_NUM=$(find docs/adr -maxdepth 1 -name '[0-9][0-9][0-9][0-9]-*.md' -exec basename {} \; 2>/dev/null | cut -d- -f1 | sort -n | tail -1)

# If no ADRs exist yet, start at 0001, otherwise increment
if [ -z "$LAST_NUM" ]; then
    NEXT_NUM="0001"
else
    # Use 10# prefix to force base-10 (avoids octal issues with leading zeros)
    NEXT_NUM=$(printf "%04d" $((10#$LAST_NUM + 1)))
fi

# Convert title to kebab-case for filename
# 1. Convert to lowercase
# 2. Replace spaces with hyphens
# 3. Remove any characters that aren't alphanumeric or hyphens
SLUG=$(echo "$TITLE" | tr '[:upper:]' '[:lower:]' | tr -s ' ' '-' | sed 's/[^a-z0-9-]//g' | sed 's/^-//' | sed 's/-$//')

# Construct filename
FILENAME="docs/adr/${NEXT_NUM}-${SLUG}.md"
DATE=$(date +%Y-%m-%d)

# Check if file already exists
if [ -f "$FILENAME" ]; then
    echo "Error: File already exists: $FILENAME"
    exit 1
fi

# Read template line by line and replace placeholders
while IFS= read -r line; do
    # Replace [Number] with actual number
    line="${line//\[Number\]/$NEXT_NUM}"
    # Replace [Title] with actual title
    line="${line//\[Title\]/$TITLE}"
    # Replace YYYY-MM-DD with current date
    line="${line//YYYY-MM-DD/$DATE}"
    echo "$line"
done < docs/adr/template.md > "$FILENAME"

echo "✓ Created ADR: $FILENAME"

# Update the ADR README with the new entry
./tools/docs/update-adr-readme.sh

echo ""
echo "Next steps:"
echo "  1. Edit $FILENAME"
echo "  2. Update status from 'Proposed' to 'Accepted' when implemented"
echo "  3. Link from related RFC or ADRs in the 'References' section"
echo "  4. Run 'make adr-update' to refresh docs/adr/README.md if status changes"
