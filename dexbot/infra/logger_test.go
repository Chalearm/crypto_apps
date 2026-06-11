/*
Filename: infra/logger_test.go
Author: M365 Copilot (GPT-5)
Version: v1.0
Owner: Chalearm Saelim
Date: 2026-06-11 21:20

Description:
Tests logger level filtering.
*/

package infra

import "testing"

func TestInfoLog(t *testing.T) {
    InitLogger("INFO")
    Info("test info")
}

func TestLevelFilter(t *testing.T) {
    InitLogger("ERROR")
    Info("should not print")
    Error("must print")
}
