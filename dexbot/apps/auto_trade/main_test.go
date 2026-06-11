/*
Filename: apps/auto_trade/main_test.go

Author: M365 Copilot (GPT-5)
Version: v2.6 CLEAN
Owner: Chalearm Saelim
Date: 2026-06-11

Description:
UNIT TESTS ONLY

Scope:
✅ Pure functions
✅ Deterministic logic
✅ NO file I/O heavy testing
✅ NO system/daemon behavior

Goal:
Avoid overlap with system_test.go
*/

package main

import (
    "os"
    "testing"
)

func init() {
    os.Setenv("TEST_MODE", "1")
}

// ===============================
// ✅ FLOAT TESTS (UNIT ONLY)
// ===============================

func TestFloat_Positive(t *testing.T) {
    val := floatToBigInt(1, 18)
    if val.Sign() <= 0 {
        t.Error("positive failed")
    }
}

func TestFloat_Zero(t *testing.T) {
    val := floatToBigInt(0, 18)
    if val.Sign() != 0 {
        t.Error("zero failed")
    }
}

func TestFloat_Negative(t *testing.T) {
    val := floatToBigInt(-1, 18)
    if val.Sign() != 0 {
        t.Error("negative failed")
    }
}

func TestFloat_Precision(t *testing.T) {
    val := floatToBigInt(1.234, 18)
    if val.Sign() <= 0 {
        t.Error("precision failed")
    }
}

// ===============================
// ✅ CONFIG (UNIT LIGHT CHECK)
// ===============================

func TestConfig_ParseOnly(t *testing.T) {

    cfg := GlobalConfig{MaxTasks: 10}

    writeConfig(cfg)
    out := loadConfig()

    if out.MaxTasks != 10 {
        t.Error("config parse failed")
    }
}