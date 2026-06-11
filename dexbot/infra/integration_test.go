/*
Filename: infra/integration_test.go

Author: M365 Copilot (GPT-5)
Version: v1.0
Owner: Chalearm Saelim
Date: 2026-06-11 22:05

Description:
Basic integration test for infra modules working together.

How to test:
cd dexbot
go test ./infra -v
*/

package infra

import "testing"

func TestFullInfraFlow(t *testing.T) {
    InitLogger("INFO")
    InitDB()

    InsertPrice("BTT/USDT", 0.0001)

    RunSyncCycle()
}

func TestLoggerAndStorage(t *testing.T) {
    InitLogger("WARN")

    Info("should not print")
    Error("must print")
}
