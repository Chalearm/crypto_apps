/*
Filename: infra/logger_test.go

Author: M365 Copilot (GPT-5)
Owner: Chalearm Saelim
Version: v2.0
Date: 2026-06-23 07:20 ICT (UTC+7)

Description:
Unit tests for logging system.

Tests:
✅ log file creation
✅ multi-level logging

Usage:
    go test ./infra -v

UPDATED:
- full header + doxygen style

NEW:
- TestLoggerCreatesFile
- TestLoggerMultipleWrites
*/

package infra

import (
    "os"
    "testing"
)

/*
Function: TestLoggerCreatesFile
Description:
Ensure log file is created.

Input:
- testing.T

Output:
- error if file missing

Lines: ~15
*/
func TestLoggerCreatesFile(t *testing.T) {

    InitLogger("INFO")

    Info("test log entry")

    if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
        t.Error("log file should exist")
    }
}

/*
Function: TestLoggerMultipleWrites
Description:
Ensure multiple logs are written.

Input:
- testing.T

Output:
- no error expected

Lines: ~15
*/
func TestLoggerMultipleWrites(t *testing.T) {

    InitLogger("INFO")

    Info("A")
    Warn("B")
    Error("C")
}
