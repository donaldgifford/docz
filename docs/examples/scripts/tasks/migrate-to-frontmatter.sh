#!/usr/bin/env bash
# Task Management - Migrate existing tasks to YAML frontmatter format
# Extracts metadata from markdown and generates frontmatter

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

TASKS_DIR="docs/tasks"
BACKUP_DIR="docs/tasks.backup"

# Check if mise and yq are available
if ! command -v mise &> /dev/null; then
    echo -e "${RED}[ERROR]${NC} mise is not installed."
    exit 1
fi

if ! mise list yq &> /dev/null || [ -z "$(mise list yq)" ]; then
    echo -e "${RED}[ERROR]${NC} yq is not installed. Run 'mise install yq' first."
    exit 1
fi

YQ="mise exec -- yq"

# Track current task ID globally
CURRENT_TASK_ID=0
NEXT_TASK_ID=""

# Initialize CURRENT_TASK_ID from existing TASK-* files
init_task_id() {
    local max_id=0

    # Find all existing TASK-* IDs
    for file in $(find "$TASKS_DIR" -name "TASK-*.md" 2>/dev/null); do
        local id=$(basename "$file" | sed -E 's/TASK-([0-9]+)-.*/\1/')
        if [[ "$id" =~ ^[0-9]+$ ]] && [ "$id" -gt "$max_id" ]; then
            max_id=$id
        fi
    done

    CURRENT_TASK_ID=$max_id
}

# Get next available TASK-NNN ID and increment (sets NEXT_TASK_ID global)
get_next_task_id() {
    CURRENT_TASK_ID=$((CURRENT_TASK_ID + 1))
    NEXT_TASK_ID=$(printf "TASK-%03d" "$CURRENT_TASK_ID")
}

# Extract title from markdown
extract_title() {
    local file=$1
    # Get first line starting with #, remove # and trailing " - COMPLETE ✅"
    grep -m 1 "^# " "$file" 2>/dev/null | sed 's/^# //' | sed 's/ - COMPLETE ✅//' | sed 's/ - COMPLETE//' || echo "Untitled"
}

# Extract status from markdown
extract_status() {
    local file=$1
    local path=$2

    # Check if in completed directory
    if [[ "$path" == *"/complete/"* ]] || [[ "$path" == *"/completed/"* ]]; then
        echo "completed"
        return
    fi

    # Check for COMPLETE in title
    if grep -q "^# .*COMPLETE" "$file" 2>/dev/null; then
        echo "completed"
        return
    fi

    # Check for Status field in markdown
    local status_line=$(grep "^\*\*Status:\*\*" "$file" 2>/dev/null | head -1)
    if [ -n "$status_line" ]; then
        if echo "$status_line" | grep -qi "complete"; then
            echo "completed"
        elif echo "$status_line" | grep -qi "in progress"; then
            echo "in_progress"
        elif echo "$status_line" | grep -qi "not started"; then
            echo "planned"
        elif echo "$status_line" | grep -qi "blocked"; then
            echo "blocked"
        else
            echo "planned"
        fi
        return
    fi

    # Default to planned
    echo "planned"
}

# Extract priority from markdown
extract_priority() {
    local file=$1

    # Look for Priority field
    local priority_line=$(grep "^\*\*Priority:\*\*" "$file" 2>/dev/null | head -1)
    if [ -n "$priority_line" ]; then
        if echo "$priority_line" | grep -q "P0"; then
            echo "P0"
        elif echo "$priority_line" | grep -q "P1"; then
            echo "P1"
        elif echo "$priority_line" | grep -q "P2"; then
            echo "P2"
        elif echo "$priority_line" | grep -q "P3"; then
            echo "P3"
        else
            echo "P2"  # Default
        fi
        return
    fi

    # Default to P2
    echo "P2"
}

# Extract estimated hours from markdown
extract_estimated_hours() {
    local file=$1

    # Look for Estimated Time field
    local est_line=$(grep "^\*\*Estimated Time:\*\*" "$file" 2>/dev/null | head -1)
    if [ -n "$est_line" ]; then
        # Try to extract number of hours
        # Patterns: "4 hours", "2-3 hours", "1.5 hours", "3 weeks" (convert to hours)
        if echo "$est_line" | grep -qE "[0-9]+ hours?"; then
            echo "$est_line" | grep -oE "[0-9]+" | head -1
        elif echo "$est_line" | grep -qE "[0-9]+-[0-9]+ hours?"; then
            # Take average of range
            local low=$(echo "$est_line" | grep -oE "[0-9]+" | head -1)
            local high=$(echo "$est_line" | grep -oE "[0-9]+" | tail -1)
            echo $(( (low + high) / 2 ))
        elif echo "$est_line" | grep -qE "[0-9]+ weeks?"; then
            # Convert weeks to hours (assuming 40 hour work week)
            local weeks=$(echo "$est_line" | grep -oE "[0-9]+" | head -1)
            echo $(( weeks * 40 ))
        elif echo "$est_line" | grep -qE "[0-9]+ days?"; then
            # Convert days to hours (assuming 8 hour work day)
            local days=$(echo "$est_line" | grep -oE "[0-9]+" | head -1)
            echo $(( days * 8 ))
        else
            echo "4"  # Default
        fi
        return
    fi

    # Default
    echo "4"
}

# Determine type from path and filename
determine_type() {
    local file=$1
    local filename=$(basename "$file")

    # Check path first
    if [[ "$file" == *"/ui/"* ]]; then
        echo "ui"
    elif [[ "$file" == *"/plugins/"* ]] || [[ "$filename" == *"PLUGIN"* ]] || [[ "$filename" == *"plugin"* ]]; then
        echo "plugin"
    elif [[ "$filename" == *"PHASE"* ]] || [[ "$filename" == *"MVP"* ]] || [[ "$filename" == *"API"* ]]; then
        echo "core"
    else
        echo "other"
    fi
}

# Convert title to filename slug
title_to_slug() {
    local title="$1"
    echo "$title" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g' | sed -E 's/^-+|-+$//g' | cut -c1-50
}

# Migrate single file
migrate_file() {
    local file=$1
    local dry_run=$2

    echo ""
    echo -e "${BLUE}[MIGRATE]${NC} $(basename "$file")"

    # Skip if already has frontmatter (check if first line is ---)
    if head -1 "$file" 2>/dev/null | grep -q "^---$"; then
        echo -e "${YELLOW}[SKIP]${NC} Already has frontmatter"
        return 0
    fi

    # Extract metadata
    local title=$(extract_title "$file")
    local status=$(extract_status "$file" "$file")
    local priority=$(extract_priority "$file")
    local estimated_hours=$(extract_estimated_hours "$file")
    local type=$(determine_type "$file")

    # Get next task ID (updates NEXT_TASK_ID global)
    get_next_task_id
    local task_id="$NEXT_TASK_ID"

    echo "  Title: $title"
    echo "  Type: $type"
    echo "  Status: $status"
    echo "  Priority: $priority"
    echo "  Estimated: $estimated_hours hours"
    echo "  ID: $task_id"

    if [ "$dry_run" = "true" ]; then
        echo -e "${YELLOW}[DRY RUN]${NC} Would add frontmatter"
        return 0
    fi

    # Create temporary file with frontmatter + original content
    local temp_file=$(mktemp)

    cat > "$temp_file" << EOF
---
id: $task_id
title: $title
type: $type
status: $status
priority: $priority
estimated_hours: $estimated_hours
actual_hours: null
started_date: null
completed_date: null
blocked_by: []
blocks: []
related_to: []
assignee: null
tags: []
phase: null
---

EOF

    # Append original content
    cat "$file" >> "$temp_file"

    # Replace original file
    mv "$temp_file" "$file"

    echo -e "${GREEN}[SUCCESS]${NC} Frontmatter added"
}

# Main migration
main() {
    local dry_run=false

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                dry_run=true
                shift
                ;;
            *)
                echo -e "${RED}[ERROR]${NC} Unknown argument: $1"
                echo "Usage: $0 [--dry-run]"
                exit 1
                ;;
        esac
    done

    echo -e "${CYAN}╔══════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║       TASK MIGRATION TO YAML FRONTMATTER             ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════╝${NC}"
    echo ""

    if [ "$dry_run" = "true" ]; then
        echo -e "${YELLOW}[DRY RUN MODE]${NC} No files will be modified"
        echo ""
    fi

    # Verify backup exists
    if [ ! -d "$BACKUP_DIR" ]; then
        echo -e "${RED}[ERROR]${NC} Backup directory not found: $BACKUP_DIR"
        echo "Run: cp -r docs/tasks docs/tasks.backup"
        exit 1
    fi

    echo -e "${GREEN}[INFO]${NC} Backup exists: $BACKUP_DIR"
    echo ""

    # Initialize task ID counter
    init_task_id

    # Find all task files (excluding template and README)
    local task_files=()
    while IFS= read -r file; do
        task_files+=("$file")
    done < <(find "$TASKS_DIR" -name "*.md" ! -name "README.md" ! -name "template.md" -type f | sort)

    local total=${#task_files[@]}
    echo -e "${BLUE}[INFO]${NC} Found $total task file(s) to migrate"

    if [ "$total" -eq 0 ]; then
        echo -e "${YELLOW}[WARN]${NC} No tasks found to migrate"
        exit 0
    fi

    # Migrate each file
    local count=0
    local skipped=0

    for file in "${task_files[@]}"; do
        migrate_file "$file" "$dry_run"

        if head -1 "$file" 2>/dev/null | grep -q "^---$"; then
            count=$((count + 1))
        else
            skipped=$((skipped + 1))
        fi
    done

    echo ""
    echo -e "${CYAN}╚══════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${GREEN}[SUMMARY]${NC}"
    echo "  Total tasks: $total"

    if [ "$dry_run" = "true" ]; then
        echo "  Would migrate: $count"
        echo "  Would skip: $skipped"
        echo ""
        echo -e "${YELLOW}[INFO]${NC} Run without --dry-run to perform migration"
    else
        echo "  Migrated: $count"
        echo "  Skipped: $skipped"
        echo ""
        echo -e "${GREEN}[SUCCESS]${NC} Migration complete!"
        echo ""
        echo -e "${BLUE}[NEXT STEPS]${NC}"
        echo "  1. Review migrated tasks: git diff docs/tasks/"
        echo "  2. Adjust metadata manually if needed"
        echo "  3. Move tasks to domain directories (if not already)"
        echo "  4. Generate READMEs: make task-update"
        echo "  5. Commit: git add docs/tasks/ && git commit -m 'Migrate tasks to frontmatter'"
    fi
    echo ""
}

# Add cyan color
CYAN='\033[0;36m'

main "$@"
