#!/usr/bin/env bash
# Task Management - Update READMEs with task status from YAML frontmatter
# Generates root README and domain READMEs using yq to parse frontmatter
# Requires: bash 4+ for associative arrays

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

TASKS_DIR="docs/tasks"
ROOT_README="$TASKS_DIR/README.md"

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

# Extract frontmatter field from task file using yq
get_field() {
    local file=$1
    local field=$2
    local default=$3

    # yq returns 'null' for missing fields, convert to our default
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

# Find all task files
find_task_files() {
    local domain=$1
    if [ -n "$domain" ]; then
        find "$TASKS_DIR/$domain" -name "TASK-*.md" -o \( -name "*.md" ! -name "README.md" ! -name "template.md" ! -path "*/archive/*" ! -path "*/reports/*" \) 2>/dev/null | sort
    else
        find "$TASKS_DIR" -name "TASK-*.md" -o \( -name "*.md" ! -name "README.md" ! -name "template.md" ! -path "*/.*" ! -path "*/archive/*" ! -path "*/reports/*" \) 2>/dev/null | sort
    fi
}

# Check if task was completed in last N days
completed_recently() {
    local completed_date=$1
    local days_ago=$2

    if [ "$completed_date" = "null" ] || [ -z "$completed_date" ]; then
        return 1
    fi

    local date_threshold=$(date -v-${days_ago}d +%Y-%m-%d 2>/dev/null || date -d "${days_ago} days ago" +%Y-%m-%d 2>/dev/null)

    if [[ "$completed_date" > "$date_threshold" ]] || [[ "$completed_date" == "$date_threshold" ]]; then
        return 0
    else
        return 1
    fi
}

# Generate statistics for all tasks
generate_stats() {
    local total_tasks=0
    local by_domain=()
    local by_status=()
    local by_priority=()
    local total_estimated=0
    local total_actual=0

    # Initialize counters
    declare -A domain_count
    declare -A status_count
    declare -A priority_count

    # Pre-initialize common keys to avoid issues
    domain_count[core]=0
    domain_count[plugin]=0
    domain_count[ui]=0
    domain_count[other]=0

    status_count[planned]=0
    status_count[in_progress]=0
    status_count[blocked]=0
    status_count[completed]=0
    status_count[cancelled]=0

    priority_count[P0]=0
    priority_count[P1]=0
    priority_count[P2]=0
    priority_count[P3]=0

    for file in $(find_task_files); do
        [ -f "$file" ] || continue

        # Skip if no frontmatter
        if ! grep -q "^---$" "$file" 2>/dev/null; then
            continue
        fi

        total_tasks=$((total_tasks + 1))

        local type=$(get_field "$file" "type" "other")
        local status=$(get_field "$file" "status" "planned")
        local priority=$(get_field "$file" "priority" "P2")
        local estimated=$(get_field "$file" "estimated_hours" "0")
        local actual=$(get_field "$file" "actual_hours" "0")

        # Count by domain
        if [[ -n "$type" ]]; then
            domain_count["$type"]=$((${domain_count["$type"]:-0} + 1))
        fi

        # Count by status
        if [[ -n "$status" ]]; then
            status_count["$status"]=$((${status_count["$status"]:-0} + 1))
        fi

        # Count by priority
        if [[ -n "$priority" ]]; then
            priority_count["$priority"]=$((${priority_count["$priority"]:-0} + 1))
        fi

        # Sum hours (handle null values)
        if [ "$estimated" != "null" ] && [ "$estimated" != "0" ]; then
            total_estimated=$(echo "$total_estimated + $estimated" | bc 2>/dev/null || echo "$total_estimated")
        fi

        if [ "$actual" != "null" ] && [ "$actual" != "0" ]; then
            total_actual=$(echo "$total_actual + $actual" | bc 2>/dev/null || echo "$total_actual")
        fi
    done

    # Print statistics
    echo "**Total Tasks:** $total_tasks"
    echo ""
    echo "**By Domain:**"
    for domain in core plugin ui other; do
        count=${domain_count["$domain"]:-0}
        echo "- ${domain}: $count"
    done
    echo ""
    echo "**By Status:**"
    for status in planned in_progress blocked completed cancelled; do
        count=${status_count["$status"]:-0}
        emoji=$(status_emoji "$status")
        echo "- $emoji ${status}: $count"
    done
    echo ""
    echo "**By Priority:**"
    for priority in P0 P1 P2 P3; do
        count=${priority_count["$priority"]:-0}
        emoji=$(priority_emoji "$priority")
        echo "- $emoji ${priority}: $count"
    done
    echo ""
    echo "**Time Tracking:**"
    echo "- Estimated: ${total_estimated} hours"
    echo "- Actual: ${total_actual} hours"
}

# Generate active tasks table grouped by domain
generate_active_tasks() {
    echo "### Active Tasks by Domain"
    echo ""

    for domain in core plugin ui other; do
        local domain_files=$(find "$TASKS_DIR/$domain/active" -name "*.md" 2>/dev/null | sort)
        local count=$(echo "$domain_files" | grep -c . || true)

        if [ "$count" -gt 0 ]; then
            echo "#### ${domain^} ($count)"
            echo ""
            echo "| ID | Title | Status | Priority | Est. Hours | Tags |"
            echo "|----|-------|--------|----------|------------|------|"

            for file in $domain_files; do
                [ -f "$file" ] || continue

                local id=$(get_field "$file" "id" "")
                local title=$(get_field "$file" "title" "Untitled")
                local status=$(get_field "$file" "status" "planned")
                local priority=$(get_field "$file" "priority" "P2")
                local estimated=$(get_field "$file" "estimated_hours" "-")
                local tags=$(get_field "$file" "tags" "[]" | tr -d '[]' | tr ',' ' ')

                local status_emoji_val=$(status_emoji "$status")
                local priority_emoji_val=$(priority_emoji "$priority")
                local rel_path=$(echo "$file" | sed "s|$TASKS_DIR/||")

                echo "| [$id]($rel_path) | $title | $status_emoji_val $status | $priority_emoji_val $priority | $estimated | $tags |"
            done
            echo ""
        fi
    done
}

# Generate blocked tasks section
generate_blocked_tasks() {
    local has_blocked=false

    for file in $(find_task_files); do
        [ -f "$file" ] || continue

        local status=$(get_field "$file" "status" "planned")
        local blocked_by=$(get_field "$file" "blocked_by" "[]")

        if [ "$status" = "blocked" ] || [ "$blocked_by" != "[]" ]; then
            if [ "$has_blocked" = false ]; then
                echo "### Blocked Tasks"
                echo ""
                echo "| ID | Title | Blocked By | Priority |"
                echo "|----|-------|------------|----------|"
                has_blocked=true
            fi

            local id=$(get_field "$file" "id" "")
            local title=$(get_field "$file" "title" "Untitled")
            local priority=$(get_field "$file" "priority" "P2")
            local priority_emoji_val=$(priority_emoji "$priority")
            local rel_path=$(echo "$file" | sed "s|$TASKS_DIR/||")
            local blocked_by_clean=$(echo "$blocked_by" | tr -d '[]' | tr ',' ' ')

            echo "| [$id]($rel_path) | $title | $blocked_by_clean | $priority_emoji_val $priority |"
        fi
    done

    if [ "$has_blocked" = true ]; then
        echo ""
    fi
}

# Generate recently completed tasks (last 7 days)
generate_recently_completed() {
    local has_recent=false

    for file in $(find_task_files); do
        [ -f "$file" ] || continue

        local status=$(get_field "$file" "status" "planned")
        local completed_date=$(get_field "$file" "completed_date" "null")

        if [ "$status" = "completed" ] && completed_recently "$completed_date" 7; then
            if [ "$has_recent" = false ]; then
                echo "### Recently Completed (Last 7 Days)"
                echo ""
                echo "| ID | Title | Completed | Actual Hours | Domain |"
                echo "|----|-------|-----------|--------------|--------|"
                has_recent=true
            fi

            local id=$(get_field "$file" "id" "")
            local title=$(get_field "$file" "title" "Untitled")
            local actual=$(get_field "$file" "actual_hours" "-")
            local type=$(get_field "$file" "type" "other")
            local rel_path=$(echo "$file" | sed "s|$TASKS_DIR/||")

            echo "| [$id]($rel_path) | $title | $completed_date | $actual | $type |"
        fi
    done

    if [ "$has_recent" = true ]; then
        echo ""
    fi
}

# Generate root README
generate_root_readme() {
    cat > "$ROOT_README" << 'HEADER'
# Shitwiz Development Tasks

This directory contains development tasks for the Shitwiz project, organized by domain with YAML frontmatter for structured metadata.

## Directory Structure

```
docs/tasks/
├── core/           # Backend/API tasks
│   ├── active/
│   └── completed/
├── plugins/        # Plugin development tasks
│   ├── active/
│   └── completed/
├── ui/             # Frontend tasks
│   ├── active/
│   └── completed/
└── other/          # Infrastructure, docs, operations
    ├── active/
    └── completed/
```

## Task Management Commands

**Create a new task:**
```bash
make task
```

**List tasks (filtered):**
```bash
make task-list TYPE=plugin              # All plugin tasks
make task-list STATUS=in_progress       # All in-progress tasks
make task-list TYPE=core PRIORITY=P0,P1 # High-priority core tasks
```

**Complete a task:**
```bash
make task-complete TASK=TASK-007
```

**View statistics:**
```bash
make task-stats
```

**Update READMEs:**
```bash
make task-update
```

## Task Statistics

HEADER

    generate_stats >> "$ROOT_README"

    cat >> "$ROOT_README" << 'SECTION2'

## Task Overview

SECTION2

    generate_active_tasks >> "$ROOT_README"
    generate_blocked_tasks >> "$ROOT_README"
    generate_recently_completed >> "$ROOT_README"

    cat >> "$ROOT_README" << 'FOOTER'

## Task Lifecycle

1. **📋 Planned** - Task defined but not started
2. **🔄 In Progress** - Actively being worked on
3. **🚫 Blocked** - Waiting on dependencies
4. **✅ Completed** - Finished successfully
5. **❌ Cancelled** - Decided not to implement

## Priority Levels

- **🔴 P0** - Blocker (must do immediately)
- **🟠 P1** - High (do soon)
- **🟡 P2** - Medium (can wait)
- **🟢 P3** - Low (nice to have)

---

**Last Updated:** AUTO_DATE (auto-generated by `make task-update`)
FOOTER

    # Replace AUTO_DATE with current date
    local current_date=$(date +%Y-%m-%d)
    sed -i.bak "s/AUTO_DATE/$current_date/" "$ROOT_README" && rm -f "$ROOT_README.bak"
}

# Generate domain README
generate_domain_readme() {
    local domain=$1
    local domain_dir="$TASKS_DIR/$domain"
    local domain_readme="$domain_dir/README.md"

    local domain_title="${domain^}"
    local domain_desc=""

    case "$domain" in
        core) domain_desc="Backend/API development tasks" ;;
        plugin) domain_desc="Plugin development and enhancement tasks" ;;
        ui) domain_desc="Frontend/UI development tasks" ;;
        other) domain_desc="Infrastructure, documentation, and operations tasks" ;;
    esac

    cat > "$domain_readme" << HEADER
# $domain_title Tasks

$domain_desc

## Active Tasks

HEADER

    # Active tasks table
    local active_files=$(find "$domain_dir/active" -name "*.md" 2>/dev/null | sort)
    local active_count=$(echo "$active_files" | grep -c . || true)

    if [ "$active_count" -gt 0 ]; then
        echo "| ID | Title | Status | Priority | Est. Hours | Tags |" >> "$domain_readme"
        echo "|----|-------|--------|----------|------------|------|" >> "$domain_readme"

        for file in $active_files; do
            [ -f "$file" ] || continue

            local id=$(get_field "$file" "id" "")
            local title=$(get_field "$file" "title" "Untitled")
            local status=$(get_field "$file" "status" "planned")
            local priority=$(get_field "$file" "priority" "P2")
            local estimated=$(get_field "$file" "estimated_hours" "-")
            local tags=$(get_field "$file" "tags" "[]" | tr -d '[]' | tr ',' ' ')

            local status_emoji_val=$(status_emoji "$status")
            local priority_emoji_val=$(priority_emoji "$priority")
            local filename=$(basename "$file")

            echo "| [$id](./active/$filename) | $title | $status_emoji_val $status | $priority_emoji_val $priority | $estimated | $tags |" >> "$domain_readme"
        done
    else
        echo "*No active tasks in this domain.*" >> "$domain_readme"
    fi

    echo "" >> "$domain_readme"
    echo "## Completed Tasks" >> "$domain_readme"
    echo "" >> "$domain_readme"

    # Completed tasks table
    local completed_files=$(find "$domain_dir/completed" -name "*.md" 2>/dev/null | sort)
    local completed_count=$(echo "$completed_files" | grep -c . || true)

    if [ "$completed_count" -gt 0 ]; then
        echo "| ID | Title | Completed | Actual Hours | Tags |" >> "$domain_readme"
        echo "|----|-------|-----------|--------------|------|" >> "$domain_readme"

        for file in $completed_files; do
            [ -f "$file" ] || continue

            local id=$(get_field "$file" "id" "")
            local title=$(get_field "$file" "title" "Untitled")
            local completed_date=$(get_field "$file" "completed_date" "-")
            local actual=$(get_field "$file" "actual_hours" "-")
            local tags=$(get_field "$file" "tags" "[]" | tr -d '[]' | tr ',' ' ')
            local filename=$(basename "$file")

            echo "| [$id](./completed/$filename) | $title | $completed_date | $actual | $tags |" >> "$domain_readme"
        done
    else
        echo "*No completed tasks in this domain.*" >> "$domain_readme"
    fi

    # Statistics
    echo "" >> "$domain_readme"
    echo "## Statistics" >> "$domain_readme"
    echo "" >> "$domain_readme"
    echo "- **Active:** $active_count tasks" >> "$domain_readme"
    echo "- **Completed:** $completed_count tasks" >> "$domain_readme"

    # Calculate total estimated and actual hours
    local total_estimated=0
    local total_actual=0

    for file in $active_files $completed_files; do
        [ -f "$file" ] || continue

        local estimated=$(get_field "$file" "estimated_hours" "0")
        local actual=$(get_field "$file" "actual_hours" "0")

        if [ "$estimated" != "null" ] && [ "$estimated" != "0" ]; then
            total_estimated=$(echo "$total_estimated + $estimated" | bc 2>/dev/null || echo "$total_estimated")
        fi

        if [ "$actual" != "null" ] && [ "$actual" != "0" ]; then
            total_actual=$(echo "$total_actual + $actual" | bc 2>/dev/null || echo "$total_actual")
        fi
    done

    echo "- **Total Estimated:** ${total_estimated} hours" >> "$domain_readme"
    echo "- **Total Actual:** ${total_actual} hours" >> "$domain_readme"

    if [ "$completed_count" -gt 0 ] && [ "$total_actual" != "0" ]; then
        local avg_hours=$(echo "scale=1; $total_actual / $completed_count" | bc 2>/dev/null || echo "0")
        echo "- **Average per Task:** ${avg_hours} hours" >> "$domain_readme"
    fi

    echo "" >> "$domain_readme"
    echo "---" >> "$domain_readme"
    echo "" >> "$domain_readme"
    echo "**Last Updated:** $(date +%Y-%m-%d) (auto-generated by \`make task-update\`)" >> "$domain_readme"
}

# Main execution
main() {
    echo -e "${BLUE}[INFO]${NC} Updating task READMEs..."
    echo ""

    # Count total tasks
    local total_count=$(find_task_files | wc -l | xargs)
    echo -e "${BLUE}[INFO]${NC} Found $total_count task file(s)"

    # Generate root README
    echo -e "${BLUE}[INFO]${NC} Generating root README: $ROOT_README"
    generate_root_readme

    # Generate domain READMEs
    for domain in core plugin ui other; do
        echo -e "${BLUE}[INFO]${NC} Generating domain README: $TASKS_DIR/$domain/README.md"
        generate_domain_readme "$domain"
    done

    echo ""
    echo -e "${GREEN}[SUCCESS]${NC} All task READMEs updated successfully!"
    echo -e "${BLUE}[INFO]${NC} Generated files:"
    echo "  - $ROOT_README"
    echo "  - $TASKS_DIR/core/README.md"
    echo "  - $TASKS_DIR/plugin/README.md"
    echo "  - $TASKS_DIR/ui/README.md"
    echo "  - $TASKS_DIR/other/README.md"
}

main "$@"
