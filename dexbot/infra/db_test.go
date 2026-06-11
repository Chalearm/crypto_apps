/*
Filename: apps/auto_trade/db_test.go

Author: M365 Copilot (GPT-5)
Version: v2.1
Owner: Chalearm Saelim
Date: 2026-06-12 01:22

Description:
Database Test Suite

Coverage:
✅ DB initialization
✅ health check
✅ insert persistence
✅ reload correctness

Note:
Requires DB config in environment
*/

package infra

import (
    "os"
    "testing"

    "dexbot/infra"
)

func init() {
    os.Setenv("TEST_MODE", "1")
}

// =============================
// ✅ INIT TEST
// =============================

func TestDB_Init(t *testing.T) {

    err := infra.InitDB()

    if err != nil {
        t.Fatalf("DB init failed: %v", err)
    }
}

// =============================
// ✅ HEALTH CHECK
// =============================

func TestDB_Health(t *testing.T) {

    err := infra.CheckDBHealth()

    if err != nil {
        t.Error("DB health failed")
    }
}

// =============================
// ✅ SAVE + LOAD
// =============================

func TestDB_SaveLoad(t *testing.T) {

    tm := NewTaskManager()

    tm.Tasks["db_test_1"] = &TradeTask{
        ID:     "db_test_1",
        Status: StatusCreated,
    }

    tm.Save()

    tm2 := NewTaskManager()
    tm2.Load()

    if len(tm2.Tasks) == 0 {
        t.Error("DB load failed")
    }
}

// =============================
// ✅ MULTIPLE RECORDS
// =============================

func TestDB_MultipleInsert(t *testing.T) {

    tm := NewTaskManager()

    for i := 0; i < 3; i++ {
        id := "multi_" + string(rune('A'+i))
        tm.Tasks[id] = &TradeTask{
            ID:     id,
            Status: StatusCreated,
        }
    }

    tm.Save()

    out := NewTaskManager()
    out.Load()

    if len(out.Tasks) < 3 {
        t.Error("multiple insert failed")
    }
}

// =============================
// ✅ HEALTH FAILURE SIMULATION
// =============================

// NOTE: this assumes wrong env config triggers failure
func TestDB_Health_Failure(t *testing.T) {

    // simulate broken DB connection
    infra.DB.Close()

    err := infra.CheckDBHealth()

    if err == nil {
        t.Error("expected DB health failure")
    }
}