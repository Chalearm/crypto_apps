#!/bin/bash
###############################################################################
# File Name       : run_test.sh
# File Path       : regulator/run_test.sh
#
# Author          : Gemini 3.1 Pro
# Owner           : Chalearm Saelim
# Reviewer        : Chalearm Saelim
#
# Version         : 1.2.0
# Status          : Development
# Created Date    : 2026-07-26 12:00:00 (UTC+7)
# Modified Date   : 2026-07-26 13:50:00 (UTC+7)
#
# Description     :
#   Test runner script executing linux grep checks, Go regulator suite,
#   and Go unit test execution across target repositories.
#
# Responsibilities:
#   - Parse target private key snippet from parameter or config.env.
#   - Execute grep audit excluding configuration environment files.
#   - Run pre-compiled ./regulator binary if present, else fallback to go run.
#   - Run pre-compiled ./regulator.test binary if present, else fallback to go test.
#
# Usage :
#   Directory : regulator/
#
#   Run :
#     ./run_test.sh [search_snippet]
#
# Dependencies :
#   Internal :
#     - main.go
#     - key_scanner.go
#     - comment_checker.go
#     - file_header_checker.go
#
#   External :
#     - grep, go
#
# Updated Parts :
#   [Script Execution]
#     - Step 3: Added pre-compiled ./regulator.test execution check before falling back to go test.
#
# New Parts :
#   [Script Execution]
#     - Test binary fallback detection
#
# Change History :
#   -------------------------------------------------------------------------
#   Version | Date Time (UTC+7)        | Author          | Description
#   -------------------------------------------------------------------------
#   1.0.0   | 2026-07-26 12:00:00      | Gemini 3.1 Pro  | Initial script
#   1.1.0   | 2026-07-26 13:45:00      | Gemini 3.1 Pro  | Added binary fallback
#   1.2.0   | 2026-07-26 13:50:00      | Gemini 3.1 Pro  | Added test binary fallback
#   -------------------------------------------------------------------------
#
# TODO :
#   - Add colorized output toggles.
#
# Notes :
#   - Complies with rule1.txt coding standard.
###############################################################################
### linux basic checking command
#   grep -rn "xxxx" ../
#

CONFIG_FILE="../dexbot/config.env"

if [ -n "$1" ]; then
    SEARCH_SNIPPET="$1"
elif [ -f "$CONFIG_FILE" ]; then
    SEARCH_SNIPPET=$(grep -E '^(PRIVATE_KEY|TARGET_KEY)=' "$CONFIG_FILE" | cut -d '=' -f2- | tr -d '"' | tr -d "'")
fi

if [ -z "$SEARCH_SNIPPET" ]; then
    echo "🚨 ERROR: No key provided!"
    exit 1
fi

SCAN_TARGET="../"

echo "=========================================="
echo "Searching for snippet/key: '${SEARCH_SNIPPET}'"
echo "=========================================="

echo -e "\n--- 1. Linux Grep Check (Excluding config files) ---"
grep -rn --exclude="*.env" --exclude="*.json" "${SEARCH_SNIPPET}" ../
UNAUTHORIZED_GREP_STATUS=$?

echo -e "\n--- 2. Go Regulator Suite ---"
if [ -x "./regulator" ]; then
    echo "🚀 Running pre-compiled binary: ./regulator"
    ./regulator "${SCAN_TARGET}" "${SEARCH_SNIPPET}"
    GO_RUN_STATUS=$?
else
    echo "⚡ Pre-compiled binary not found. Falling back to: go run ."
    go run . "${SCAN_TARGET}" "${SEARCH_SNIPPET}"
    GO_RUN_STATUS=$?
fi

echo -e "\n--- 3. Go Test Execution ---"
if [ -x "./regulator.test" ]; then
    echo "🚀 Running pre-compiled test binary: ./regulator.test"
    TARGET_KEY="${SEARCH_SNIPPET}" SCAN_DIR="${SCAN_TARGET}" ./regulator.test -test.v
    GO_TEST_STATUS=$?
else
    echo "⚡ Pre-compiled test binary not found. Falling back to: go test"
    TARGET_KEY="${SEARCH_SNIPPET}" SCAN_DIR="${SCAN_TARGET}" go test -v .
    GO_TEST_STATUS=$?
fi

if [ $UNAUTHORIZED_GREP_STATUS -eq 0 ] || [ $GO_RUN_STATUS -ne 0 ] || [ $GO_TEST_STATUS -ne 0 ]; then
    echo -e "\n❌ AUDIT FAILED: Private key exposure, invalid comments, or missing headers detected!"
    exit 1
else
    echo -e "\n✅ ALL AUDITS PASSED!"
    exit 0
fi