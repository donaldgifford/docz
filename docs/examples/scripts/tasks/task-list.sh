#!/usr/bin/env bash
# Task Management - List and filter tasks
# Filters tasks by TYPE, STATUS, and PRIORITY using YAML frontmatter

set -e

# Show deprecation notice
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../../../tools/DEPRECATED.sh" 2>/dev/null || true
show_deprecation_notice "task-list.sh" "rex task list" 2>/dev/null || true

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

TASKS_DIR="docs/tasks"

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

# Parse command-line arguments
TYPE_FILTER=""
STATUS_FILTER=""
PRIORITY_FILTER=""

while [[ $# -gt 0 ]]; do
    case $1 in
        TYPE=*)
            TYPE_FILTER="${1#*=}"
            shift
            ;;
        STATUS=*)
            STATUS_FILTER="${1#*=}"
            shift
            ;;
        PRIORITY=*)
            PRIORITY_FILTER="${1#*=}"
            shift
            ;;
        *)
            echo -e "${RED}[ERROR]${NC} Unknown argument: $1"
            echo "Usage: $0 [TYPE=core|plugin|ui|other] [STATUS=planned|in_progress|blocked|completed|cancelled] [PRIORITY=P0,P1,P2,P3]"
            exit 1
            ;;
    esac
done

# Extract frontmatter field from task file using yq
get_field() {
    local file=$1
    local field=$2
    local default=$3

    local value=$($YQ eval --front-matter=extract ".${field}" "$file" 2>/dev/null || echo "$default")
    if [ "$value" = "null" ]; then
        echo "$default"
    else
        echo "$value"
    fi
}

# Get status emoji
status_emoji() {
    case "$1" in
        planned) echo "📋" ;;
        in_progress) echo "🔄" ;;
        blocked) echo "🚫" ;;
        completed) echo "✅" ;;
        cancelled) echo "❌" ;;
        *) echo "❓" ;;
    esac
}

# Get priority emoji
priority_emoji() {
    case "$1" in
        P0) echo "🔴" ;;
        P1) echo "🟠" ;;
        P2) echo "🟡" ;;
        P3) echo "🟢" ;;
        *) echo "⚪" ;;
    esac
}

# Check if value matches filter (handles comma-separated lists)
matches_filter() {
    local value=$1
    local filter=$2

    if [ -z "$filter" ]; then
        return 0  # No filter means match all
    fi

    # Split filter by comma
    IFS=',' read -ra FILTER_ARRAY <<< "$filter"
    for filter_val in "${FILTER_ARRAY[@]}"; do
        if [ "$value" = "$filter_val" ]; then
            return 0  # Match found
        fi
    done

    return 1  # No match
}

# Find and filter tasks
list_tasks() {
    local found_count=0

    # Print header
    echo ""
    echo -e "${BLUE}=== Task List ===${NC}"
    echo ""

    if [ -n "$TYPE_FILTER" ]; then
        echo "Type filter: $TYPE_FILTER"
    fi
    if [ -n "$STATUS_FILTER" ]; then
        echo "Status filter: $STATUS_FILTER"
    fi
    if [ -n "$PRIORITY_FILTER" ]; then
        echo "Priority filter: $PRIORITY_FILTER"
    fi

    if [ -n "$TYPE_FILTER" ] || [ -n "$STATUS_FILTER" ] || [ -n "$PRIORITY_FILTER" ]; then
        echo ""
    fi

    # Table header
    printf "%-12s %-40s %-15s %-10s %-10s %-20s\n" "ID" "Title" "Status" "Priority" "Est. Hrs" "Domain"
    printf "%-12s %-40s %-15s %-10s %-10s %-20s\n" "------------" "----------------------------------------" "---------------" "----------" "----------" "--------------------"

    # Find all task files (excluding archive and reports)
    for file in $(find "$TASKS_DIR" -name "TASK-*.md" -o \( -name "*.md" ! -name "README.md" ! -name "template.md" ! -path "*/.*" ! -path "*/archive/*" ! -path "*/reports/*" \) 2>/dev/null | sort); do
        [ -f "$file" ] || continue

        # Skip if no frontmatter
        if ! grep -q "^---$" "$file" 2>/dev/null; then
            continue
        fi

        # Extract fields
        local id=$(get_field "$file" "id" "")
        local title=$(get_field "$file" "title" "Untitled")
        local status=$(get_field "$file" "status" "planned")
        local priority=$(get_field "$file" "priority" "P2")
        local type=$(get_field "$file" "type" "other")
        local estimated=$(get_field "$file" "estimated_hours" "-")

        # Apply filters
        if ! matches_filter "$type" "$TYPE_FILTER"; then
            continue
        fi

        if ! matches_filter "$status" "$STATUS_FILTER"; then
            continue
        fi

        if ! matches_filter "$priority" "$PRIORITY_FILTER"; then
            continue
        fi

        # Match found, display task
        found_count=$((found_count + 1))

        # Truncate title if too long
        if [ ${#title} -gt 38 ]; then
            title="${title:0:35}..."
        fi

        local status_emoji_val=$(status_emoji "$status")
        local priority_emoji_val=$(priority_emoji "$priority")

        printf "%-12s %-40s %-15s %-10s %-10s %-20s\n" \
            "$id" \
            "$title" \
            "$status_emoji_val $status" \
            "$priority_emoji_val $priority" \
            "$estimated" \
            "$type"
    done

    echo ""
    echo -e "${GREEN}[INFO]${NC} Found $found_count task(s)"
    echo ""
}

# Main execution
main() {
    list_tasks
}

main "$@"
