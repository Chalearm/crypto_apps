#!/bin/bash

APP_DIR="$(pwd)"

SEARCH_DIRS=(
    "${APP_DIR}/logs"
    "${APP_DIR}/old_logs"
)

# ==============================================================================
# Arguments
# ==============================================================================
#
# First argument:
#   Menu option, such as 1, 2, 3, 4, 6, 7, 8, 9, or 10.
#
# Second argument for search options 1-4:
#   Optional group/model filter, such as G1, G2, or G10-M12.
#
# Arguments for option 10:
#   All arguments after option 10 are treated as a command.
# ==============================================================================

CHOICE="${1:-}"
GROUP_FILTER="${2:-}"

# Convert group filter to uppercase.
# For example: g2 becomes G2.
GROUP_FILTER="${GROUP_FILTER^^}"

# ==============================================================================
# Common validation functions
# ==============================================================================

validate_search_directories() {
    local directory
    local valid_count=0

    for directory in "${SEARCH_DIRS[@]}"; do
        if [[ -d "${directory}" ]]; then
            ((valid_count++))
        else
            echo "WARNING: Search directory does not exist: ${directory}" >&2
        fi
    done

    if ((valid_count == 0)); then
        echo "ERROR: None of the configured search directories exist." >&2
        return 1
    fi

    return 0
}

validate_group_filter() {
    local group_filter="$1"

    # An empty filter means search all groups and models.
    if [[ -z "${group_filter}" ]]; then
        return 0
    fi

    # Accepted formats:
    #   G2
    #   G2-M4
    #   G10-M12
    if [[ ! "${group_filter}" =~ ^G[0-9]+(-M[0-9]+)?$ ]]; then
        echo "ERROR: Invalid group/model filter '${group_filter}'." >&2
        echo "Accepted formats:" >&2
        echo "  G2       Search all models belonging to G2" >&2
        echo "  G2-M4    Search only model G2-M4" >&2
        echo "  G10-M12  Search only model G10-M12" >&2
        return 1
    fi

    return 0
}

# ==============================================================================
# Common log-search function
# ==============================================================================

search_logs() {
    local search_term="$1"
    local description="$2"
    local group_filter="${3:-}"

    local directory
    local search_pattern
    local grep_status
    local overall_status=1
    local found_any_match=0
    local search_error=0

    validate_search_directories || return 2
    validate_group_filter "${group_filter}" || return 2

    if [[ -z "${group_filter}" ]]; then
        # No filter. Show every matching group and model.
        search_pattern="${search_term}"

    elif [[ "${group_filter}" =~ ^G[0-9]+-M[0-9]+$ ]]; then
        # Exact group and model filter.
        #
        # G2-M4 matches:
        #   [FOLD SUCCESS] G2-M4
        #
        # G2-M4 does not match:
        #   G2-M40
        #   G2-M41
        #   G1-M4
        search_pattern="${search_term}.*${group_filter}([^0-9]|$)"

    else
        # Group-only filter.
        #
        # G2 matches:
        #   G2-M1
        #   G2-M4
        #   G2-M10
        #
        # G2 does not match:
        #   G1-M4
        #   G20-M4
        search_pattern="${search_term}.*${group_filter}-M[0-9]+([^0-9]|$)"
    fi

    echo "========================================================================"
    echo "${description}"
    echo "Primary search term: ${search_term}"

    if [[ -z "${group_filter}" ]]; then
        echo "Filter: ALL groups and models"
    elif [[ "${group_filter}" =~ ^G[0-9]+-M[0-9]+$ ]]; then
        echo "Exact model filter: ${group_filter}"
        echo "Only records for ${group_filter} will be displayed."
    else
        echo "Group filter: ${group_filter}"
        echo "All models belonging to ${group_filter} will be displayed."
    fi

    echo "Search directories:"

    for directory in "${SEARCH_DIRS[@]}"; do
        echo "  - ${directory}"
    done

    echo "========================================================================"

    for directory in "${SEARCH_DIRS[@]}"; do
        if [[ ! -d "${directory}" ]]; then
            continue
        fi

        echo
        echo "Searching in: ${directory}"
        echo "------------------------------------------------------------------------"

        grep \
            --recursive \
            --line-number \
            --ignore-case \
            --extended-regexp \
            --exclude-dir="venv" \
            -- "${search_pattern}" "${directory}"

        grep_status=$?

        case "${grep_status}" in
            0)
                found_any_match=1
                overall_status=0
                ;;

            1)
                echo "No matching records found in ${directory}"
                ;;

            *)
                echo "ERROR: Search failed in ${directory}" >&2
                echo "grep exit status: ${grep_status}" >&2
                search_error=1
                overall_status="${grep_status}"
                ;;
        esac
    done

    echo
    echo "========================================================================"

    if ((search_error != 0)); then
        echo "Search completed with one or more errors."
    elif ((found_any_match != 0)); then
        echo "Search completed. Matching records were found."
    else
        echo "Search completed. No matching records were found."
    fi

    echo "========================================================================"

    return "${overall_status}"
}

# ==============================================================================
# Search-option functions
# ==============================================================================

find_fold_success() {
    search_logs \
        "FOLD SUCCESS" \
        "Option 1: Searching for successful FOLD operations" \
        "${GROUP_FILTER}"
}

find_lazy_prune() {
    search_logs \
        "LAZY PRUNE" \
        "Option 2: Searching for LAZY PRUNE operations" \
        "${GROUP_FILTER}"
}

find_evaluation_complete() {
    search_logs \
        "EVALUATION COMPLETE" \
        "Option 3: Searching for completed evaluations" \
        "${GROUP_FILTER}"
}

find_model_complete() {
    search_logs \
        "MODEL COMPLETE" \
        "Option 4: Searching for completed model evaluations" \
        "${GROUP_FILTER}"
}

# ==============================================================================
# Model execution function
# ==============================================================================

run_model_option() {
    local model_option="$1"
    local description="$2"
    local command_status

    echo "========================================================================"
    echo "${description}"
    echo "Command: ./man* ${model_option}"
    echo "Application directory: ${APP_DIR}"
    echo "========================================================================"

    cd "${APP_DIR}" || {
        echo "ERROR: Could not enter application directory:" >&2
        echo "       ${APP_DIR}" >&2
        return 1
    }

    ./man* "${model_option}"
    command_status=$?

    echo
    echo "========================================================================"
    echo "Command exit status: ${command_status}"
    echo
    echo "Current system memory usage:"
    free -h
    echo "========================================================================"

    return "${command_status}"
}

# ==============================================================================
# Custom command function for option 10
# ==============================================================================

run_custom_command() {
    local custom_command
    local command_status

    # Remove option 10 from the argument list.
    shift

    if (($# == 0)); then
        echo "ERROR: Option 10 requires a command." >&2
        echo "Examples:" >&2
        echo "  $0 10 ls -lath" >&2
        echo "  $0 10 './man* 7'" >&2
        return 1
    fi

    echo "========================================================================"
    echo "Option 10: Running a custom command"
    echo "Application directory: ${APP_DIR}"

    cd "${APP_DIR}" || {
        echo "ERROR: Could not enter application directory:" >&2
        echo "       ${APP_DIR}" >&2
        return 1
    }

    if (($# == 1)); then
        # One quoted argument is treated as a shell command.
        #
        # Example:
        #   ./mn.sh 10 './man* 7'
        #
        # Bash expands ./man* and passes 7 as an argument.
        custom_command="$1"

        echo "Execution mode: Bash command string"
        echo "Command: ${custom_command}"
        echo "========================================================================"

        bash -lc "${custom_command}"
        command_status=$?
    else
        # Multiple arguments are executed directly without eval.
        #
        # Example:
        #   ./mn.sh 10 ls -lath
        printf -v custom_command '%q ' "$@"

        echo "Execution mode: Direct argument execution"
        echo "Command: ${custom_command}"
        echo "========================================================================"

        "$@"
        command_status=$?
    fi

    echo
    echo "========================================================================"
    echo "Custom command exit status: ${command_status}"
    echo "========================================================================"

    return "${command_status}"
}

# ==============================================================================
# Application-directory validation
# ==============================================================================

if [[ ! -d "${APP_DIR}" ]]; then
    echo "ERROR: Application directory does not exist:" >&2
    echo "       ${APP_DIR}" >&2
    exit 1
fi

# ==============================================================================
# Read option
# ==============================================================================

# If no option was supplied on the command line, ask interactively.
if [[ $# -eq 0 ]]; then
    echo
    read -rp "Enter execution option (0, 1, 2, 3, 4, 6, 7, 8, 9, or 10): " CHOICE
    CHOICE="${CHOICE//[[:space:]]/}"
fi

# ==============================================================================
# Option menu
# ==============================================================================

echo
echo "Select execution option:"
echo "========================================================================"
echo "  1) Find FOLD SUCCESS"
echo "     Searches logs/ and old_logs/ for successful FOLD operations."
echo
echo "  2) Find LAZY PRUNE"
echo "     Searches logs/ and old_logs/ for LAZY PRUNE operations."
echo
echo "  3) Find EVALUATION COMPLETE"
echo "     Searches logs/ and old_logs/ for completed evaluations."
echo
echo "  4) Find MODEL COMPLETE"
echo "     Searches logs/ and old_logs/ for completed model evaluations."
echo
echo "  6) Run model option 6"
echo "     Executes ./man* 6 and displays system memory usage."
echo
echo "  7) Run model option 7"
echo "     Executes ./man* 7 and displays system memory usage."
echo
echo "  8) Run force-save option 8"
echo "     Executes ./man* 8 and displays system memory usage."
echo
echo "  9) Run force-print option 9"
echo "     Executes ./man* 9 and displays system memory usage."
echo
echo " 10) Run a custom command in the application directory"
echo "     Example: $0 10 ls -lath"
echo
echo "  0) Exit"
echo "     Exits without running an operation."
echo "========================================================================"
echo "Optional group or exact-model filter for search options 1-4:"
echo
echo "  $0 1          Search FOLD SUCCESS for all groups and models"
echo "  $0 1 G2       Search FOLD SUCCESS for all G2 models"
echo "  $0 1 G2-M4    Search FOLD SUCCESS for G2-M4 only"
echo "  $0 2 G1       Search LAZY PRUNE for all G1 models"
echo "  $0 2 G1-M8    Search LAZY PRUNE for G1-M8 only"
echo "  $0 3 G2-M4    Search EVALUATION COMPLETE for G2-M4 only"
echo "  $0 4 G10-M12  Search MODEL COMPLETE for G10-M12 only"
echo "========================================================================"

# ==============================================================================
# Read optional group/model filter for search options
# ==============================================================================

if [[ "${CHOICE}" =~ ^[1-4]$ ]]; then
    if [[ $# -lt 2 ]]; then
        read -rp "Enter group/model filter, such as G2 or G2-M4, or press Enter for all: " GROUP_FILTER
        GROUP_FILTER="${GROUP_FILTER^^}"
    fi

    if [[ -z "${GROUP_FILTER}" ]]; then
        echo "Selected filter: ALL groups and models"
    elif [[ "${GROUP_FILTER}" =~ ^G[0-9]+-M[0-9]+$ ]]; then
        echo "Selected exact model filter: ${GROUP_FILTER}"
    else
        echo "Selected group filter: ${GROUP_FILTER}"
    fi

    echo
fi

# ==============================================================================
# Execute selected option
# ==============================================================================

case "${CHOICE}" in
    1)
        find_fold_success
        ;;

    2)
        find_lazy_prune
        ;;

    3)
        find_evaluation_complete
        ;;

    4)
        find_model_complete
        ;;

    6)
        run_model_option \
            6 \
            "Option 6: Running model option 6"
        ;;

    7)
        run_model_option \
            7 \
            "Option 7: Running model option 7"
        ;;

    8)
        run_model_option \
            8 \
            "Option 8: Running force-save option 8"
        ;;

    9)
        run_model_option \
            9 \
            "Option 9: Running force-print option 9"
        ;;

    10)
        run_custom_command "$@"
        ;;

    0)
        echo "Exit selected. No operation was executed."
        exit 0
        ;;

    *)
        echo "ERROR: Invalid option '${CHOICE}'." >&2
        echo "Valid options are 0, 1, 2, 3, 4, 6, 7, 8, 9, and 10." >&2
        exit 1
        ;;
esac

exit $?