#!/usr/bin/env bash
# Task Management - Mark task as complete and move to completed directory
# Updates frontmatter (status, completed_date, actual_hours) and moves file

set -e

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
TASK_ID=""

while [[ $# -gt 0 ]]; do
    case $1 in
        TASK=*)
            TASK_ID="${1#*=}"
            shift
            ;;
        *)
            TASK_ID="$1"
            shift
            ;;
    esac
done

if [ -z "$TASK_ID" ]; then
    echo -e "${RED}[ERROR]${NC} Task ID is required"
    echo "Usage: $0 TASK=TASK-NNN"
    echo "   or: $0 TASK-NNN"
    exit 1
fi

# Prompt for input
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

# Find task file by ID
find_task_file() {
    local task_id=$1

    # Search in all domain active directories
    for domain in core plugin ui other; do
        local file="$TASKS_DIR/$domain/active/$task_id"*.md
        if ls $file 2>/dev/null | head -1 | grep -q .; then
            ls $file 2>/dev/null | head -1
            return 0
        fi
    done

    # Also check root active directory for old-format tasks
    local file="$TASKS_DIR/active/$task_id"*.md
    if ls $file 2>/dev/null | head -1 | grep -q .; then
        ls $file 2>/dev/null | head -1
        return 0
    fi

    # Check root directory (very old format)
    file="$TASKS_DIR/$task_id"*.md
    if ls $file 2>/dev/null | head -1 | grep -q .; then
        ls $file 2>/dev/null | head -1
        return 0
    fi

    return 1
}

# Main execution
main() {
    echo -e "${BLUE}=== Complete Task: $TASK_ID ===${NC}"
    echo ""

    # Find the task file
    TASK_FILE=$(find_task_file "$TASK_ID")

    if [ -z "$TASK_FILE" ]; then
        echo -e "${RED}[ERROR]${NC} Task file not found: $TASK_ID"
        echo ""
        echo "Available tasks:"
        find "$TASKS_DIR" -name "TASK-*.md" | while read file; do
            id=$($YQ eval --front-matter=extract ".id" "$file" 2>/dev/null || echo "")
            if [ -n "$id" ]; then
                echo "  - $id: $(basename "$file")"
            fi
        done
        exit 1
    fi

    echo -e "${GREEN}[INFO]${NC} Found task: $(basename "$TASK_FILE")"
    echo ""

    # Extract current metadata
    TITLE=$($YQ eval --front-matter=extract ".title" "$TASK_FILE" 2>/dev/null || echo "Untitled")
    TYPE=$($YQ eval --front-matter=extract ".type" "$TASK_FILE" 2>/dev/null || echo "other")
    ESTIMATED=$($YQ eval --front-matter=extract ".estimated_hours" "$TASK_FILE" 2>/dev/null || echo "0")

    echo "Task: $TITLE"
    echo "Type: $TYPE"
    echo "Estimated: $ESTIMATED hours"
    echo ""

    # Prompt for actual hours
    ACTUAL_HOURS=$(prompt "Actual hours spent" "$ESTIMATED")

    if ! [[ "$ACTUAL_HOURS" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
        echo -e "${RED}[ERROR]${NC} Actual hours must be a number"
        exit 1
    fi

    # Get current date
    COMPLETED_DATE=$(date +%Y-%m-%d)

    # Confirm
    echo ""
    echo -e "${YELLOW}[CONFIRM]${NC} Mark task as complete?"
    echo "  - Actual hours: $ACTUAL_HOURS"
    echo "  - Completed date: $COMPLETED_DATE"
    echo "  - Will move to: $TASKS_DIR/$TYPE/completed/"
    echo ""

    CONFIRM=$(prompt "Proceed? (yes/no)" "yes")

    if [ "$CONFIRM" != "yes" ]; then
        echo "Cancelled."
        exit 0
    fi

    echo ""
    echo -e "${BLUE}[INFO]${NC} Updating task frontmatter..."

    # Update frontmatter using yq
    $YQ eval --front-matter=extract -i ".status = \"completed\"" "$TASK_FILE"
    $YQ eval --front-matter=extract -i ".completed_date = \"$COMPLETED_DATE\"" "$TASK_FILE"
    $YQ eval --front-matter=extract -i ".actual_hours = $ACTUAL_HOURS" "$TASK_FILE"

    # Determine target directory
    TARGET_DIR="$TASKS_DIR/$TYPE/completed"
    mkdir -p "$TARGET_DIR"

    FILENAME=$(basename "$TASK_FILE")
    TARGET_FILE="$TARGET_DIR/$FILENAME"

    # Move file
    echo -e "${BLUE}[INFO]${NC} Moving to completed directory..."

    if [ -f "$TARGET_FILE" ]; then
        echo -e "${YELLOW}[WARN]${NC} Target file already exists: $TARGET_FILE"
        OVERWRITE=$(prompt "Overwrite? (yes/no)" "no")
        if [ "$OVERWRITE" != "yes" ]; then
            echo "Cancelled."
            exit 1
        fi
        rm "$TARGET_FILE"
    fi

    mv "$TASK_FILE" "$TARGET_FILE"

    echo ""
    echo -e "${GREEN}[SUCCESS]${NC} Task marked as complete!"
    echo ""
    echo -e "${BLUE}[INFO]${NC} Summary:"
    echo "  - Task ID: $TASK_ID"
    echo "  - Title: $TITLE"
    echo "  - Actual hours: $ACTUAL_HOURS"
    echo "  - Completed: $COMPLETED_DATE"
    echo "  - Location: $TARGET_FILE"
    echo ""
    echo -e "${YELLOW}[NEXT]${NC} Suggested next steps:"
    echo "  1. Update READMEs: make task-update"
    echo "  2. Commit changes: git add . && git commit -m 'Complete task: $TITLE'"
}

main "$@"
