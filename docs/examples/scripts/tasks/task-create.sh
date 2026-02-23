#!/usr/bin/env bash
# Task Management - Create new task file with YAML frontmatter
# Prompts for frontmatter fields, auto-generates TASK-ID, places in correct domain directory

set -e

# Show deprecation notice
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../../../tools/DEPRECATED.sh" 2>/dev/null || true
show_deprecation_notice "task-create.sh" "rex task create" 2>/dev/null || true

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

TASKS_DIR="docs/tasks"
TEMPLATE_FILE="$TASKS_DIR/template.md"

# Check if mise and yq are available
if ! command -v mise &> /dev/null; then
    echo -e "${RED}[ERROR]${NC} mise is not installed."
    exit 1
fi

if ! mise list yq &> /dev/null || [ -z "$(mise list yq)" ]; then
    echo -e "${RED}[ERROR]${NC} yq is not installed. Run 'mise install yq' first."
    exit 1
fi

# Use yq through mise
YQ="mise exec -- yq"

# Prompt for input with default
prompt() {
    local prompt_text=$1
    local default=$2
    local result

    if [ -n "$default" ]; then
        read -p "$prompt_text [$default]: " result
        echo "${result:-$default}"
    else
        read -p "$prompt_text: " result
        echo "$result"
    fi
}

# Generate next available TASK-NNN ID
generate_task_id() {
    local max_id=0

    # Find all task files across all domains and extract IDs
    for file in $(find "$TASKS_DIR" -name "TASK-*.md" 2>/dev/null); do
        local id=$(basename "$file" | sed -E 's/TASK-([0-9]+)-.*/\1/')
        if [[ "$id" =~ ^[0-9]+$ ]] && [ "$id" -gt "$max_id" ]; then
            max_id=$id
        fi
    done

    # Next ID (zero-padded to 3 digits)
    local next_id=$((max_id + 1))
    printf "TASK-%03d" "$next_id"
}

# Convert title to filename slug
title_to_slug() {
    local title="$1"
    echo "$title" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g' | sed -E 's/^-+|-+$//g'
}

# Main task creation
main() {
    echo -e "${BLUE}=== Shitwiz Task Creator (YAML Frontmatter) ===${NC}"
    echo ""

    # Auto-generate Task ID
    TASK_ID=$(generate_task_id)
    echo -e "${GREEN}[INFO]${NC} Auto-generated ID: $TASK_ID"
    echo ""

    # Gather required fields
    echo -e "${YELLOW}Required Fields${NC}"
    echo ""

    title=$(prompt "Task title (e.g., Implement IAM Collector)" "")
    if [ -z "$title" ]; then
        echo -e "${RED}[ERROR]${NC} Title is required"
        exit 1
    fi

    echo ""
    echo "Type options: core, plugin, ui, other"
    type=$(prompt "Task type" "core")
    if [[ ! "$type" =~ ^(core|plugin|ui|other)$ ]]; then
        echo -e "${RED}[ERROR]${NC} Invalid type. Must be: core, plugin, ui, or other"
        exit 1
    fi

    echo ""
    echo "Priority levels: P0 (blocker), P1 (high), P2 (medium), P3 (low)"
    priority=$(prompt "Priority" "P1")
    if [[ ! "$priority" =~ ^P[0-3]$ ]]; then
        echo -e "${RED}[ERROR]${NC} Invalid priority. Must be: P0, P1, P2, or P3"
        exit 1
    fi

    estimated_hours=$(prompt "Estimated hours" "4")
    if ! [[ "$estimated_hours" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
        echo -e "${RED}[ERROR]${NC} Estimated hours must be a number"
        exit 1
    fi

    # Optional fields
    echo ""
    echo -e "${YELLOW}Optional Fields (press Enter to skip)${NC}"
    echo ""

    assignee=$(prompt "Assignee (optional)" "null")
    if [ "$assignee" = "null" ] || [ -z "$assignee" ]; then
        assignee="null"
    fi

    phase=$(prompt "Phase number (optional)" "null")
    if [ "$phase" = "null" ] || [ -z "$phase" ]; then
        phase="null"
    fi

    # Tags (comma-separated)
    tags_input=$(prompt "Tags (comma-separated, e.g., 'aws,security,collector')" "")
    if [ -n "$tags_input" ]; then
        # Convert comma-separated to YAML array
        tags="["
        IFS=',' read -ra TAG_ARRAY <<< "$tags_input"
        for i in "${!TAG_ARRAY[@]}"; do
            tag=$(echo "${TAG_ARRAY[$i]}" | xargs) # trim whitespace
            if [ $i -gt 0 ]; then
                tags+=", "
            fi
            tags+="$tag"
        done
        tags+="]"
    else
        tags="[]"
    fi

    # Relationships (optional, comma-separated TASK-IDs)
    echo ""
    echo -e "${YELLOW}Relationships (optional)${NC}"
    echo ""

    blocked_by_input=$(prompt "Blocked by (TASK-IDs, comma-separated)" "")
    if [ -n "$blocked_by_input" ]; then
        blocked_by="["
        IFS=',' read -ra BLOCKED_ARRAY <<< "$blocked_by_input"
        for i in "${!BLOCKED_ARRAY[@]}"; do
            task_id=$(echo "${BLOCKED_ARRAY[$i]}" | xargs)
            if [ $i -gt 0 ]; then
                blocked_by+=", "
            fi
            blocked_by+="$task_id"
        done
        blocked_by+="]"
    else
        blocked_by="[]"
    fi

    blocks_input=$(prompt "Blocks (TASK-IDs, comma-separated)" "")
    if [ -n "$blocks_input" ]; then
        blocks="["
        IFS=',' read -ra BLOCKS_ARRAY <<< "$blocks_input"
        for i in "${!BLOCKS_ARRAY[@]}"; do
            task_id=$(echo "${BLOCKS_ARRAY[$i]}" | xargs)
            if [ $i -gt 0 ]; then
                blocks+=", "
            fi
            blocks+="$task_id"
        done
        blocks+="]"
    else
        blocks="[]"
    fi

    related_to_input=$(prompt "Related to (TASK-IDs, comma-separated)" "")
    if [ -n "$related_to_input" ]; then
        related_to="["
        IFS=',' read -ra RELATED_ARRAY <<< "$related_to_input"
        for i in "${!RELATED_ARRAY[@]}"; do
            task_id=$(echo "${RELATED_ARRAY[$i]}" | xargs)
            if [ $i -gt 0 ]; then
                related_to+=", "
            fi
            related_to+="$task_id"
        done
        related_to+="]"
    else
        related_to="[]"
    fi

    # Generate filename slug from title
    slug=$(title_to_slug "$title")
    filename="${TASK_ID}-${slug}.md"

    # Determine target directory
    target_dir="$TASKS_DIR/$type/active"
    filepath="$target_dir/$filename"

    # Create directory if it doesn't exist
    mkdir -p "$target_dir"

    # Check if file exists
    if [ -f "$filepath" ]; then
        echo -e "${YELLOW}[WARN]${NC} File already exists: $filepath"
        overwrite=$(prompt "Overwrite? (yes/no)" "no")
        if [ "$overwrite" != "yes" ]; then
            echo "Cancelled."
            exit 0
        fi
    fi

    echo ""
    echo -e "${BLUE}[INFO]${NC} Creating task file: $filename"
    echo -e "${BLUE}[INFO]${NC} Location: $filepath"
    echo ""

    # Create task file with YAML frontmatter
    cat > "$filepath" << EOF
---
id: $TASK_ID
title: $title
type: $type
status: planned
priority: $priority
estimated_hours: $estimated_hours
actual_hours: null
started_date: null
completed_date: null
blocked_by: $blocked_by
blocks: $blocks
related_to: $related_to
assignee: $assignee
tags: $tags
phase: $phase
---

# $title

## Description

Clear description of what needs to be done and why.

## Context

Background information, related decisions, or constraints.

## Requirements

- Requirement 1
- Requirement 2
- Requirement 3

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3

## Implementation Notes

Implementation approach, key decisions, or technical details.

## Testing

How to verify the task is complete.

## References

- Related ADRs, docs, or external resources
EOF

    echo -e "${GREEN}[SUCCESS]${NC} Task file created!"
    echo ""
    echo -e "${BLUE}[INFO]${NC} Task Summary:"
    echo "  ID: $TASK_ID"
    echo "  Title: $title"
    echo "  Type: $type"
    echo "  Priority: $priority"
    echo "  Estimated Hours: $estimated_hours"
    echo "  Location: $filepath"
    echo ""
    echo -e "${BLUE}[INFO]${NC} Next steps:"
    echo "  1. Edit the task file to add details: \$EDITOR $filepath"
    echo "  2. Update task READMEs: make task-update"
    echo "  3. Commit: git add $filepath && git commit -m 'Add task: $title'"
}

main "$@"
