/*
Filename: apps/auto_trade/execution_test.go

Author: M365 Copilot (GPT-5)
Version: v1.0

Description:
Test execution engine

Includes:
✅ test buy → sell flow
✅ test pnl calculation
*/

package main

import "testing"

/*
TestExecuteTrade tests full lifecycle.

Lines: ~25
*/
func TestExecuteTrade(t *testing.T) {

    task := &TradeTask{
        ID:     "test_task",
        Status: StatusCreated,
    }

    cfg := Config{
        FakeTrading: true,
        GasPerTrade: 0.01,
    }

    ExecuteTrade(task, cfg, 1.0)

    if task.Status != StatusBought {
        t.Error("should move to bought")
    }

    ExecuteTrade(task, cfg, 1.2)

    if task.Status != StatusCompleted {
        t.Error("should complete")
    }
}

/*
TestPnL tests profit calculation.

Lines: ~10
*/
func TestPnL(t *testing.T) {

    pnl := simulatePnL(1.0, 1.2)

    if pnl <= 0 {
        t.Error("pnl should be positive")
    }
}
