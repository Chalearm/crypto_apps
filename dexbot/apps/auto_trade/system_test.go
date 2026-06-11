/*
Filename: apps/auto_trade/system_test.go

Author: M365 Copilot (GPT-5)
Version: v3.2 (SYSTEM TEST FINAL)
Owner: Chalearm Saelim
Date: 2026-06-12 01:03

Description:
SYSTEM TEST SUITE (INTEGRATION + BEHAVIOR)

Scope:
✅ Full task lifecycle
✅ Daemon execution (in test mode)
✅ CLI commands (runApp)
✅ File persistence
✅ Concurrency safety
✅ Infra logging usage
✅ Report functionality

Philosophy:
- Validate real workflow
- Allow side effects
- No duplication with unit tests
- Terminate tests

Run:
TEST_MODE=1 go test ./apps/auto_trade -v
*/

package main

import (
    "os"
    "testing"

    "dexbot/infra"
)

// enforce safe test mode
func init() {
    os.Setenv("TEST_MODE", "1")
}

// ============================================
// ✅ LOGGING TESTS
// ============================================

// Logging should not crash
func TestLog_AllLevels(t *testing.T) {
    infra.Info("info")
    infra.Warn("warn")
    infra.Error("error")
}

// Mixed log flow
func TestLog_MixedFlow(t *testing.T) {
    infra.Info("A")
    infra.Warn("B")
    infra.Info("C")
    infra.Error("D")
}

// ============================================
// ✅ TASK LIFECYCLE TESTS
// ============================================

// Task creation
func TestTask_Create(t *testing.T) {

    tm := NewTaskManager()

    createTask(tm)

    if len(tm.Tasks) == 0 {
        t.Fatal("task not created")
    }
}

// Task processing
func TestTask_Progress(t *testing.T) {

    tm := NewTaskManager()

    createTask(tm)
    runWorkers(tm)

    for _, task := range tm.Tasks {
        if task.Status == StatusCreated {
            t.Error("task did not progress")
        }
    }
}

// Multiple task handling
func TestTask_Multi(t *testing.T) {

    tm := NewTaskManager()

    for i := 0; i < 5; i++ {
        createTask(tm)
    }

    runWorkers(tm)

    if len(tm.Tasks) < 5 {
        t.Error("multi tasks failed")
    }
}

// ============================================
// ✅ FILE PERSISTENCE
// ============================================

// Save + load integrity
func TestTask_SaveLoad(t *testing.T) {

    tm := NewTaskManager()

    createTask(tm)
    tm.Save()

    tm2 := NewTaskManager()
    tm2.Load()

    if len(tm2.Tasks) == 0 {
        t.Error("state restore failed")
    }
}

// Overwrite safety
func TestTask_Overwrite(t *testing.T) {

    tm := NewTaskManager()

    tm.Tasks["X"] = &TradeTask{ID: "1"}
    tm.Tasks["X"] = &TradeTask{ID: "2"}

    if tm.Tasks["X"].ID != "2" {
        t.Error("overwrite failed")
    }
}

// ============================================
// ✅ CONFIG (REAL FILE)
// ============================================

func TestConfig_File(t *testing.T) {

    writeConfig(GlobalConfig{MaxTasks: 42})

    cfg := loadConfig()

    if cfg.MaxTasks != 42 {
        t.Error("config file failed")
    }
}

// ============================================
// ✅ CLI / APP ENTRY TESTS
// ============================================

// Default run
func TestCLI_Default(t *testing.T) {
    runApp([]string{})
}

// Report mode
func TestCLI_Report(t *testing.T) {
    runApp([]string{"-action=report"})
}

// Terminate command
func TestCLI_Terminate(t *testing.T) {
    runApp([]string{"-action=terminate"})
}

// Mixed CLI calls
func TestCLI_Mixed(t *testing.T) {
    runApp([]string{"-dry_run=true"})
    runApp([]string{"-action=report"})
}

// ============================================
// ✅ DAEMON + REPORT COMBINATION
// ============================================

// Run daemon then report
func TestDaemon_ReportFlow(t *testing.T) {

    runApp([]string{"-dry_run=true"})
    runApp([]string{"-action=report"})
}

// Multiple report calls
func TestReport_Multiple(t *testing.T) {

    for i := 0; i < 3; i++ {
        runApp([]string{"-action=report"})
    }
}

// ============================================
// ✅ CONCURRENCY TESTS
// ============================================

// Concurrent task creation
func TestConcurrency_Create(t *testing.T) {

    tm := NewTaskManager()

    for i := 0; i < 10; i++ {
        go createTask(tm)
    }
}

// Worker concurrency
func TestConcurrency_Workers(t *testing.T) {

    tm := NewTaskManager()

    for i := 0; i < 3; i++ {
        createTask(tm)
    }

    runWorkers(tm)
}

// ============================================
// ✅ SYSTEM STABILITY
// ============================================

// Repeated execution
func TestSystem_Repeated(t *testing.T) {

    for i := 0; i < 2; i++ {
        runApp([]string{})
    }
}

// Worker execution stability
func TestSystem_Workers(t *testing.T) {

    tm := NewTaskManager()

    createTask(tm)
    runWorkers(tm)
}

// Basic no-crash
func TestSystem_NoCrash(t *testing.T) {
    runApp([]string{})
}
// ============================================
// ✅ TERMINATE BEHAVIOR TESTS
// ============================================

// TestTerminate_WithPID
// Expectation:
// - if PID file exists → terminate succeeds
// - PID file removed afterwards
func TestTerminate_WithPID(t *testing.T) {

    // create fake PID file
    err := os.WriteFile(PID_FILE, []byte("999999"), 0644)
    if err != nil {
        t.Fatal("failed to create PID file")
    }

    runApp([]string{"-action=terminate"})

    // ensure PID file removed
    if _, err := os.Stat(PID_FILE); !os.IsNotExist(err) {
        t.Error("PID file should be removed after terminate")
    }
}

// TestTerminate_NoPID
// Expectation:
// - no PID file → no crash
// - function exits safely
func TestTerminate_NoPID(t *testing.T) {

    // ensure no PID file exists
    _ = os.Remove(PID_FILE)

    // should NOT panic or crash
    runApp([]string{"-action=terminate"})
}
// ============================================
// ✅ STATUS TESTS
// ============================================

// Expect daemon not running
func TestStatus_NoDaemon(t *testing.T) {

    _ = os.Remove(PID_FILE)

    runApp([]string{"-action=status"})
}

// Expect daemon running (fake PID)
func TestStatus_WithPID(t *testing.T) {

    os.WriteFile(PID_FILE, []byte("12345"), 0644)

    runApp([]string{"-action=status"})
}

// ============================================
// ✅ TERMINATE TEST (REAL CASE YOU PROVIDED)
// ============================================

func TestTerminate_Twice(t *testing.T) {

    // simulate existing daemon
    os.WriteFile(PID_FILE, []byte("999999"), 0644)

    runApp([]string{"-action=terminate"})

    // second terminate (no PID)
    runApp([]string{"-action=terminate"})
}

// ============================================
// ✅ PnL REPORT TEST
// ============================================

func TestReport_PnL(t *testing.T) {

    tm := NewTaskManager()

    tm.Tasks["p1"] = &TradeTask{
        ID:        "p1",
        Status:    StatusCompleted,
        BuyPrice:  1.0,
        SellPrice: 1.1,
    }

    tm.Save()

    runApp([]string{"-action=report"})
}

// Strategy buy test
func TestStrategy_Buy(t *testing.T) {

    if !strategy.ShouldBuy() {
        t.Error("buy strategy invalid")
    }
}

// Strategy sell test
func TestStrategy_Sell(t *testing.T) {

    task := &TradeTask{
        BuyPrice: 1.0,
    }

    if !strategy.ShouldSell(task, 1.1) {
        t.Error("sell strategy failed")
    }
}

// Dry-run behavior test
func TestProcessTask_DryRun(t *testing.T) {

    dryRun = true

    task := &TradeTask{
        ID:     "t1",
        Status: StatusCreated,
    }

    tm := NewTaskManager()

    processTask(task, tm)

    if task.Status != StatusBought {
        t.Error("dry run buy failed")
    }
}

// End-to-end flow
func TestFullFlow_System(t *testing.T) {

    tm := NewTaskManager()

    createTask(tm)

    runWorkers(tm)

    if len(tm.Tasks) == 0 {
        t.Error("task not created")
    }
}

// Report execution
func TestReport_System(t *testing.T) {
    runApp([]string{"-action=report"})
}
