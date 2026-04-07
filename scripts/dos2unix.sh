#!/bin/bash

# =============================================================
# Batch dos2unix conversion script (WSL)
# Place this script at: script/dos2unix.sh
# It always operates relative to the project root (parent of script/).
#
# Usage: ./script/dos2unix.sh <extension> [directory]
#   extension: md, go, ts, js, etc. (without dot)
#   directory: optional, defaults to project root
# Examples:
#   ./script/dos2unix.sh md
#   ./script/dos2unix.sh go src
# =============================================================

set -euo pipefail

# ---------------------- Resolve project root ----------------------
# Get the directory where this script lives, then go up one level
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ---------------------- Color definitions ----------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# ---------------------- Argument validation ----------------------
if [[ $# -lt 1 ]]; then
    echo -e "${RED}Error: please specify a file extension${NC}"
    echo ""
    echo "Usage: $0 <extension> [directory]"
    echo "Examples: $0 md"
    echo "          $0 go src"
    exit 1
fi

EXT="$1"

# If a second argument is given, resolve it relative to project root;
# otherwise default to project root itself.
if [[ -n "${2:-}" ]]; then
    TARGET_DIR="$PROJECT_ROOT/$2"
else
    TARGET_DIR="$PROJECT_ROOT"
fi

# Strip leading dot if user typed ".md" instead of "md"
EXT="${EXT#.}"

if [[ ! -d "$TARGET_DIR" ]]; then
    echo -e "${RED}Error: directory '${TARGET_DIR}' does not exist${NC}"
    exit 1
fi

# ---------------------- Check if dos2unix is installed ----------------------
if ! command -v dos2unix &>/dev/null; then
    echo -e "${YELLOW}dos2unix not found, attempting to install...${NC}"
    sudo apt-get update && sudo apt-get install -y dos2unix
    if [[ $? -ne 0 ]]; then
        echo -e "${RED}Failed to install dos2unix. Please install it manually and retry.${NC}"
        exit 1
    fi
fi

# ---------------------- Find matching files ----------------------
echo ""
echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN} Batch dos2unix Conversion${NC}"
echo -e "${CYAN} Extension:    *.${EXT}${NC}"
echo -e "${CYAN} Project root: ${PROJECT_ROOT}${NC}"
echo -e "${CYAN} Scan target:  ${TARGET_DIR}${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""

# Collect files into an array, sorted alphabetically
# Display paths relative to project root for readability
mapfile -t FILES < <(find "$TARGET_DIR" -type f -name "*.${EXT}" | sort)

TOTAL=${#FILES[@]}

if [[ $TOTAL -eq 0 ]]; then
    echo -e "${YELLOW}No *.${EXT} files found${NC}"
    exit 0
fi

echo -e "Found ${GREEN}${TOTAL}${NC} *.${EXT} file(s)"
echo ""

# ---------------------- Convert one by one ----------------------
CONVERTED=0
SKIPPED=0
FAILED=0
CONVERTED_LIST=()
SKIPPED_LIST=()
FAILED_LIST=()

for filepath in "${FILES[@]}"; do
    # Show path relative to project root for cleaner output
    relative_path="${filepath#$PROJECT_ROOT/}"

    # Detect whether the file contains CRLF line endings before conversion
    if file "$filepath" | grep -q "CRLF"; then
        HAS_CRLF=true
    else
        HAS_CRLF=false
    fi

    # Run dos2unix and capture output (dos2unix writes to stderr)
    OUTPUT=$(dos2unix "$filepath" 2>&1)
    EXIT_CODE=$?

    if [[ $EXIT_CODE -ne 0 ]]; then
        echo -e "  ${RED}✗${NC} ${relative_path}"
        echo -e "    ${RED}Failed: ${OUTPUT}${NC}"
        FAILED=$((FAILED + 1))
        FAILED_LIST+=("$relative_path")
    elif [[ "$HAS_CRLF" == true ]]; then
        echo -e "  ${GREEN}✔${NC} ${relative_path}  ${GREEN}<- Converted (CRLF -> LF)${NC}"
        CONVERTED=$((CONVERTED + 1))
        CONVERTED_LIST+=("$relative_path")
    else
        echo -e "  ${YELLOW}─${NC} ${relative_path}  ${YELLOW}(Already LF, no change)${NC}"
        SKIPPED=$((SKIPPED + 1))
        SKIPPED_LIST+=("$relative_path")
    fi
done

# ---------------------- Summary report ----------------------
echo ""
echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN} Summary${NC}"
echo -e "${CYAN}========================================${NC}"
echo -e "  Total files scanned:   ${TOTAL}"
echo -e "  ${GREEN}Converted (CRLF->LF):   ${CONVERTED}${NC}"
echo -e "  ${YELLOW}Skipped (already LF):    ${SKIPPED}${NC}"
echo -e "  ${RED}Failed:                  ${FAILED}${NC}"
echo ""

if [[ ${#CONVERTED_LIST[@]} -gt 0 ]]; then
    echo -e "${GREEN}--- Converted files ---${NC}"
    for f in "${CONVERTED_LIST[@]}"; do
        echo "  $f"
    done
    echo ""
fi

if [[ ${#FAILED_LIST[@]} -gt 0 ]]; then
    echo -e "${RED}--- Failed files ---${NC}"
    for f in "${FAILED_LIST[@]}"; do
        echo "  $f"
    done
    echo ""
fi

if [[ $FAILED -gt 0 ]]; then
    exit 1
fi

echo -e "${GREEN}Done!${NC}"
