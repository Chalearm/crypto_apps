/*
Filename: infra/sync_test.go

Author: M365 Copilot (GPT-5)
Version: v1.0
Owner: Chalearm Saelim
Date: 2026-06-11 21:20

Description:
Unit tests for sync module.

Tests:
- scanning local files
- running sync cycle without crash

AI Prompt Idea:
"Write Go tests to simulate file-based sync process."

How to test:
cd dexbot
go test ./infra -v
*/

package infra

import (
    "os"
    "testing"
)

func TestRunSyncCycle_NoCrash(t *testing.T) {
    RunSyncCycle()
}

func TestRunSyncCycle_WithFile(t *testing.T) {

    _ = os.MkdirAll("data/buffer", 0755)
    _ = os.WriteFile("data/buffer/test.json", []byte(`{}`), 0644)

    RunSyncCycle()

    // should not crash
}
