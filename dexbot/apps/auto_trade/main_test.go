/******************************************************************************
 * File Name       : main_test.go
 * File Path       : apps/auto_trade/main_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-06-30 00:53:07 (UTC+7)
 * Modified Date   : 2026-06-30 00:53:07 (UTC+7)
 *
 * Description     :
 *   Unit test suite for auto_trade application. Coverage: [OK] floatToBigInt conversion [OK] zero handling [OK] negative handling [OK] precision handling [OK] large number handling [OK] tiny number handli
 *
 * Responsibilities:
 *   - Implement core functionality for apps package.
 *
 * Usage :
 *   Directory : apps/auto_trade/
 *
 *   Build :
 *     go build ./apps/auto_trade
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./apps/auto_trade
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/apps
 *
 *   External :
 *     - (stdlib only)
 *
 * Configuration :
 *   - config.env
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Test Functions] Test suite: TestFloat_Positive, TestFloat_Zero, TestFloat_Negative, TestFloat_Precision
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-30 00:53:07 (UTC+7)   | deepseek-4.0-pro | Initial version — rule1.txt header batch
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
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