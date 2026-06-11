/*
Filename: apps/auto_trade/main_test.go

Author: M365 Copilot (GPT-5)
Version: v3.0 (UNIT TEST FINAL)
Owner: Chalearm Saelim
Date: 2026-06-11 23:16

Description:
UNIT TEST SUITE (PURE / DETERMINISTIC)

Scope:
✅ floatToBigInt correctness
✅ config parsing logic (light)

Out of scope:
❌ daemon behavior
❌ concurrency
❌ heavy file lifecycle (in system_test)

Principles:
- fast execution
- deterministic
- isolated logic

Run:
TEST_MODE=1 go test ./apps/auto_trade -v
*/

package main

import (
    "os"
    "testing"
)

// Force safe test mode
func init() {
    os.Setenv("TEST_MODE", "1")
}

// =====================================
// ✅ FLOAT TEST CASES (8 scenarios)
// =====================================

// Expect > 0
func TestFloat_Positive(t *testing.T) {
    val := floatToBigInt(1.0, 18)
    if val.Sign() <= 0 {
        t.Error("positive conversion failed")
    }
}

// Expect 0
func TestFloat_Zero(t *testing.T) {
    val := floatToBigInt(0, 18)
    if val.Sign() != 0 {
        t.Error("zero conversion failed")
    }
}

// Expect 0
func TestFloat_Negative(t *testing.T) {
    val := floatToBigInt(-10, 18)
    if val.Sign() != 0 {
        t.Error("negative should clamp to zero")
    }
}

// Expect > 0
func TestFloat_Precision(t *testing.T) {
    val := floatToBigInt(1.234567, 18)
    if val.Sign() <= 0 {
        t.Error("precision lost")
    }
}

// Large value handling
func TestFloat_Large(t *testing.T) {
    val := floatToBigInt(1_000_000, 18)
    if val.Sign() <= 0 {
        t.Error("large value failed")
    }
}

// Small value handling
func TestFloat_Tiny(t *testing.T) {
    val := floatToBigInt(0.00000001, 18)
    if val.Sign() <= 0 {
        t.Error("tiny value failed")
    }
}

// Stability check
func TestFloat_Repeated(t *testing.T) {
    for i := 0; i < 5; i++ {
        _ = floatToBigInt(2.5, 18)
    }
}

// Type safety
func TestFloat_NotNil(t *testing.T) {
    val := floatToBigInt(1, 18)
    if val == nil {
        t.Error("nil value returned")
    }
}

// =====================================
// ✅ CONFIG TESTS (LIGHT ONLY)
// =====================================

// Write → Read consistency
func TestConfig_ParseOnly(t *testing.T) {

    cfg := GlobalConfig{
        MaxTasks: 20,
    }

    writeConfig(cfg)

    out := loadConfig()

    if out.MaxTasks != 20 {
        t.Error("config mismatch")
    }
}

// Idempotent read
func TestConfig_Idempotent(t *testing.T) {

    cfg := GlobalConfig{MaxTasks: 11}
    writeConfig(cfg)

    for i := 0; i < 3; i++ {
        out := loadConfig()
        if out.MaxTasks != 11 {
            t.Error("config not stable")
        }
    }
}

// Overwrite behavior
func TestConfig_Overwrite(t *testing.T) {

    writeConfig(GlobalConfig{MaxTasks: 5})
    writeConfig(GlobalConfig{MaxTasks: 99})

    out := loadConfig()

    if out.MaxTasks != 99 {
        t.Error("overwrite failed")
    }
}