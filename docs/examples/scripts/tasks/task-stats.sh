#!/usr/bin/env bash
# Task Management - Display task statistics
# Aggregates statistics from all tasks by domain, status, priority, and time tracking
# Requires: bash 4+ for associative arrays

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
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

# Generate statistics
generate_stats() {
    echo ""
    echo -e "${CYAN}╔══════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║           SHITWIZ TASK STATISTICS                    ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════╝${NC}"
    echo ""

    # Initialize counters
    declare -A domain_count status_count priority_count
    declare -A domain_estimated domain_actual
    local total_tasks=0
    local total_estimated=0
    local total_actual=0
    local blocked_tasks=()

    # Collect statistics (excluding archive and reports)
    for file in $(find "$TASKS_DIR" -name "TASK-*.md" -o \( -name "*.md" ! -name "README.md" ! -name "template.md" ! -path "*/.*" ! -path "*/archive/*" ! -path "*/reports/*" \) 2>/dev/null | sort); do
        [ -f "$file" ] || continue

        # Skip if no frontmatter
        if ! grep -q "^---$" "$file" 2>/dev/null; then
            continue
        fi

        total_tasks=$((total_tasks + 1))

        # Extract fields
        local type=$(get_field "$file" "type" "other")
        local status=$(get_field "$file" "status" "planned")
        local priority=$(get_field "$file" "priority" "P2")
        local estimated=$(get_field "$file" "estimated_hours" "0")
        local actual=$(get_field "$file" "actual_hours" "0")
        local blocked_by=$(get_field "$file" "blocked_by" "[]")
        local id=$(get_field "$file" "id" "")
        local title=$(get_field "$file" "title" "Untitled")

        # Count by domain
        domain_count[$type]=$((${domain_count[$type]:-0} + 1))

        # Count by status
        status_count[$status]=$((${status_count[$status]:-0} + 1))

        # Count by priority
        priority_count[$priority]=$((${priority_count[$priority]:-0} + 1))

        # Sum hours by domain
        if [ "$estimated" != "null" ] && [ "$estimated" != "0" ]; then
            total_estimated=$(echo "$total_estimated + $estimated" | bc 2>/dev/null || echo "$total_estimated")
            domain_estimated[$type]=$(echo "${domain_estimated[$type]:-0} + $estimated" | bc 2>/dev/null || echo "${domain_estimated[$type]:-0}")
        fi

        if [ "$actual" != "null" ] && [ "$actual" != "0" ]; then
            total_actual=$(echo "$total_actual + $actual" | bc 2>/dev/null || echo "$total_actual")
            domain_actual[$type]=$(echo "${domain_actual[$type]:-0} + $actual" | bc 2>/dev/null || echo "${domain_actual[$type]:-0}")
        fi

        # Track blocked tasks
        if [ "$status" = "blocked" ] || [ "$blocked_by" != "[]" ]; then
            blocked_tasks+=("$id|$title|$blocked_by")
        fi
    done

    # Display Overall Statistics
    echo -e "${BLUE}━━━ Overall Statistics ━━━${NC}"
    echo ""
    echo "  Total Tasks: $total_tasks"
    echo ""

    # By Domain
    echo -e "${BLUE}━━━ By Domain ━━━${NC}"
    echo ""
    printf "  %-15s %10s %15s %15s\n" "Domain" "Tasks" "Est. Hours" "Actual Hours"
    printf "  %-15s %10s %15s %15s\n" "---------------" "----------" "---------------" "---------------"

    for domain in core plugin ui other; do
        local count=${domain_count[$domain]:-0}
        local est=${domain_estimated[$domain]:-0}
        local act=${domain_actual[$domain]:-0}
        printf "  %-15s %10s %15s %15s\n" "$domain" "$count" "$est" "$act"
    done
    echo ""

    # By Status
    echo -e "${BLUE}━━━ By Status ━━━${NC}"
    echo ""
    printf "  %-20s %10s\n" "Status" "Count"
    printf "  %-20s %10s\n" "--------------------" "----------"

    for status in planned in_progress blocked completed cancelled; do
        local count=${status_count[$status]:-0}
        local emoji=$(status_emoji "$status")
        printf "  %-20s %10s\n" "$emoji $status" "$count"
    done
    echo ""

    # By Priority
    echo -e "${BLUE}━━━ By Priority ━━━${NC}"
    echo ""
    printf "  %-20s %10s\n" "Priority" "Count"
    printf "  %-20s %10s\n" "--------------------" "----------"

    for priority in P0 P1 P2 P3; do
        local count=${priority_count[$priority]:-0}
        local emoji=$(priority_emoji "$priority")
        printf "  %-20s %10s\n" "$emoji $priority" "$count"
    done
    echo ""

    # Time Tracking
    echo -e "${BLUE}━━━ Time Tracking ━━━${NC}"
    echo ""
    echo "  Estimated Hours:  $total_estimated"
    echo "  Actual Hours:     $total_actual"

    if [ "$total_actual" != "0" ] && [ "$total_estimated" != "0" ]; then
        local variance=$(echo "$total_actual - $total_estimated" | bc 2>/dev/null || echo "0")
        local variance_pct=$(echo "scale=1; ($variance / $total_estimated) * 100" | bc 2>/dev/null || echo "0")

        echo "  Variance:         $variance hours (${variance_pct}%)"

        if [ "$variance_pct" != "0" ]; then
            if (( $(echo "$variance_pct > 0" | bc -l) )); then
                echo -e "  ${YELLOW}→ Tasks taking longer than estimated${NC}"
            else
                echo -e "  ${GREEN}→ Tasks completing faster than estimated${NC}"
            fi
        fi
    fi

    local completed_count=${status_count[completed]:-0}
    if [ "$completed_count" -gt 0 ] && [ "$total_actual" != "0" ]; then
        local avg_hours=$(echo "scale=1; $total_actual / $completed_count" | bc 2>/dev/null || echo "0")
        echo "  Avg per Task:     $avg_hours hours"
    fi

    echo ""

    # Blocked Tasks
    if [ ${#blocked_tasks[@]} -gt 0 ]; then
        echo -e "${BLUE}━━━ Blocked Tasks ━━━${NC}"
        echo ""
        printf "  %-12s %-35s %-20s\n" "ID" "Title" "Blocked By"
        printf "  %-12s %-35s %-20s\n" "------------" "-----------------------------------" "--------------------"

        for task_info in "${blocked_tasks[@]}"; do
            IFS='|' read -r id title blocked_by <<< "$task_info"

            # Truncate title if too long
            if [ ${#title} -gt 33 ]; then
                title="${title:0:30}..."
            fi

            # Clean blocked_by
            blocked_by_clean=$(echo "$blocked_by" | tr -d '[]' | tr ',' ' ')

            printf "  %-12s %-35s %-20s\n" "$id" "$title" "$blocked_by_clean"
        done
        echo ""
    fi

    # Completion Rate
    if [ "$total_tasks" -gt 0 ]; then
        echo -e "${BLUE}━━━ Progress ━━━${NC}"
        echo ""

        local completion_rate=$(echo "scale=1; (${status_count[completed]:-0} / $total_tasks) * 100" | bc 2>/dev/null || echo "0")
        local in_progress_rate=$(echo "scale=1; (${status_count[in_progress]:-0} / $total_tasks) * 100" | bc 2>/dev/null || echo "0")

        echo "  Completion Rate:   ${completion_rate}% (${status_count[completed]:-0}/$total_tasks tasks)"
        echo "  In Progress:       ${in_progress_rate}% (${status_count[in_progress]:-0}/$total_tasks tasks)"

        # Progress bar
        local completed=${status_count[completed]:-0}
        local bar_length=40
        local filled=$(echo "scale=0; ($completed * $bar_length) / $total_tasks" | bc 2>/dev/null || echo "0")
        local empty=$((bar_length - filled))

        printf "  Progress: ["
        printf "%${filled}s" | tr ' ' '█'
        printf "%${empty}s" | tr ' ' '░'
        printf "]\n"

        echo ""
    fi

    echo -e "${CYAN}╚══════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# Main execution
main() {
    generate_stats
}

main "$@"
