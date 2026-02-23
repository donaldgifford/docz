#!/usr/bin/env bash
#
# Create a new RFC (Request for Comments) from template
#
# Usage: make rfc "Your RFC Title"
#    or: ./tools/create-rfc.sh "Your RFC Title"

set -e

# Show deprecation notice
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../DEPRECATED.sh" 2>/dev/null || true
show_deprecation_notice "create-rfc.sh" "rex rfc create \"Your RFC Title\"" 2>/dev/null || true

TITLE="$1"

# Validate input
if [ -z "$TITLE" ]; then
    echo "Error: RFC title is required"
    echo "Usage: make rfc \"Your RFC Title\""
    exit 1
fi

# Find the next RFC number by looking at existing files
LAST_NUM=$(find docs/rfc -maxdepth 1 -name '[0-9][0-9][0-9][0-9]-*.md' -exec basename {} \; 2>/dev/null | cut -d- -f1 | sort -n | tail -1)

# If no RFCs exist yet, start at 0001, otherwise increment
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
FILENAME="docs/rfc/${NEXT_NUM}-${SLUG}.md"
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
done < docs/rfc/template.md > "$FILENAME"

echo "✓ Created RFC: $FILENAME"

# Update the RFC README with the new entry
./tools/docs/update-rfc-readme.sh

echo ""
echo "Next steps:"
echo "  1. Edit $FILENAME"
echo "  2. Keep RFC high-level (problem, solution, phases)"
echo "  3. Create ADRs for technical implementation decisions"
echo "  4. Update status from 'Draft' to 'Proposed' when ready for review"
echo "  5. Run 'make rfc-update' to refresh docs/rfc/README.md if status changes"
