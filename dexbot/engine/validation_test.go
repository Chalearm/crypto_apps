/******************************************************************************
 * File Name       : validation_test.go
 * File Path       : engine/validation_test.go
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
 *   Unit tests for trade validation logic. Covers: - Boundary conditions (min/max) - Capital limits - Invalid values (negative, zero) AI Prompt Idea: "Write Go unit tests for a trade validation function i
 *
 * Responsibilities:
 *   - Implement core functionality for engine package.
 *
 * Usage :
 *   Directory : engine/
 *
 *   Build :
 *     go build ./engine
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./engine
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/engine
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
 *   [Test Functions] Test suite: TestValidTrade, TestBelowMin, TestAboveMax, TestZeroAmount
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
package engine

import (
    "testing"
)

func getCfg() Config {
    return Config{
        MinUSD:          0.0001,
        MaxUSD:          0.5,
        TotalCapitalUSD: 0.2,
    }
}

// ---------------- TEST CASES ----------------

func TestValidTrade(t *testing.T) {
    if !IsValidTrade(0.0005, 0, getCfg()) {
        t.Error("valid trade should pass")
    }
}

func TestBelowMin(t *testing.T) {
    if IsValidTrade(0.00001, 0, getCfg()) {
        t.Error("below min should fail")
    }
}

func TestAboveMax(t *testing.T) {
    if IsValidTrade(1.0, 0, getCfg()) {
        t.Error("above max should fail")
    }
}

func TestZeroAmount(t *testing.T) {
    if IsValidTrade(0, 0, getCfg()) {
        t.Error("zero should fail")
    }
}

func TestNegativeAmount(t *testing.T) {
    if IsValidTrade(-1, 0, getCfg()) {
        t.Error("negative should fail")
    }
}

func TestExactMin(t *testing.T) {
    if !IsValidTrade(0.0001, 0, getCfg()) {
        t.Error("exact min should pass")
    }
}

func TestExactMax(t *testing.T) {
    if !IsValidTrade(0.001, 0, getCfg()) {
        t.Error("exact max should pass")
    }
}

func TestCapitalLimitFail(t *testing.T) {
    if IsValidTrade(0.1, 0.15, getCfg()) {
        t.Error("should exceed capital")
    }
}

func TestCapitalLimitPass(t *testing.T) {
    if !IsValidTrade(0.05, 0.1, getCfg()) {
        t.Error("should fit capital")
    }
}

func TestCapitalBoundary(t *testing.T) {
    if !IsValidTrade(0.1, 0.1, getCfg()) {
        t.Error("boundary condition failed")
    }
}
