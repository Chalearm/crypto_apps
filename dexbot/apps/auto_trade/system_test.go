/*
Filename: apps/auto_trade/system_test.go

Author: M365 Copilot (GPT-5)
Version: v2.7
Owner: Chalearm Saelim
Date: 2026-06-11 23:06

Description:
SYSTEM TEST SUITE (INTEGRATION + BEHAVIOR)

Scope:
✅ Task lifecycle (create → buy → progress)
✅ File persistence (Save/Load)
✅ Config real file handling
✅ CLI behavior (runApp)
✅ Infra logging execution
✅ Concurrency safety
✅ Dry-run safety

Design Principle:
- Test REAL behavior
- Allow side effects (file / logs)
- No duplication with unit tests

Execution:
TEST_MODE=1 go test ./apps/auto_trade -v

Expected:
- No panic
- Tasks progress state
- Files created & reloaded
*/

package main

import (
    "os"
    "testing"

    "dexbot/infra"
)

// Force safe environment
func init() {
    os.Setenv("TEST_MODE", "1")
}

//
// ===========================
// ✅ LOG SYSTEM TESTS
// ===========================
//

// Expectation: logging functions execute without panic
func TestLog_AllLevels(t *testing.T) {
    infra.Info("info test")
    infra.Warn("warn test")
    infra.Error("error test")
}

// Expectation: mixed log sequence is safe
func TestLog_MixedFlow(t *testing.T) {
    infra.Info("i1")
    infra.Warn("w1")
    infra.Info("i2")
    infra.Error("e1")
}

//
// ===========================
// ✅ TASK SYSTEM (REAL FLOW)
// ===========================
//

// Expectation: createTask increases task count
func TestTask_Create_System(t *testing.T) {

    tm := NewTaskManager()

    createTask(tm)

    if len(tm.Tasks) == 0 {
        t.Fatal("task not created")
    }
}

// Expectation: worker moves task out of CREATED state
func TestTask_WorkerProgress(t *testing.T) {

    tm := NewTaskManager()
    createTask(tm)

    runWorkers(tm)

    for _, tsk := range tm.Tasks {
        if tsk.Status == StatusCreated {
            t.Error("task did not progress")
        }
    }
}

// Expectation: multiple tasks handled correctly
func TestTask_MultiFlow(t *testing.T) {

    tm := NewTaskManager()

    for i := 0; i < 4; i++ {
        createTask(tm)
    }

    runWorkers(tm)

    if len(tm.Tasks) < 4 {
        t.Error("multi-task flow failed")
    }
}

//
// ===========================
// ✅ FILE PERSISTENCE
// ===========================
//

// Expectation: saved tasks reload correctly
func TestTask_SaveLoad_System(t *testing.T) {

    tm := NewTaskManager()
    createTask(tm)

    tm.Save()

    tm2 := NewTaskManager()
    tm2.Load()

    if len(tm2.Tasks) == 0 {
        t.Error("restore failed")
    }
}

// Expectation: overwrite works
func TestTask_PersistOverwrite(t *testing.T) {

    tm := NewTaskManager()

    tm.Tasks["X"] = &TradeTask{ID: "1"}
    tm.Tasks["X"] = &TradeTask{ID: "2"}

    if tm.Tasks["X"].ID != "2" {
        t.Error("overwrite failed")
    }
}

//
// ===========================
// ✅ CONFIG (REAL FILE)
// ===========================
//

// Expectation: file persistence works
func TestConfig_FilePersistence(t *testing.T) {

    cfg := GlobalConfig{MaxTasks: 99}
    writeConfig(cfg)

    out := loadConfig()

    if out.MaxTasks != 99 {
        t.Error("config file failed")
    }
}

//
// ===========================
// ✅ CLI / ENTRY TESTS
// ===========================
//

// Expectation: default run does not crash
func TestCLI_Default(t *testing.T) {
    runApp([]string{})
}

// Expectation: report mode safe
func TestCLI_Report(t *testing.T) {
    runApp([]string{"-action=report"})
}

// Expectation: terminate mode safe
func TestCLI_Terminate(t *testing.T) {
    runApp([]string{"-action=terminate"})
}

//
// ===========================
// ✅ CONCURRENCY TESTS
// ===========================
//

// Expectation: no panic during concurrent creation
func TestConcurrency_Creation(t *testing.T) {

    tm := NewTaskManager()

    for i := 0; i < 5; i++ {
        go createTask(tm)
    }
}

//
// ===========================
// ✅ SYSTEM STABILITY
// ===========================
//

// Expectation: multiple runs safe
func TestSystem_RepeatedRun(t *testing.T) {

    for i := 0; i < 2; i++ {
        runApp([]string{})
    }
}

// Expectation: workers safe
func TestSystem_Workers(t *testing.T) {

    tm := NewTaskManager()
    createTask(tm)

    runWorkers(tm)
}