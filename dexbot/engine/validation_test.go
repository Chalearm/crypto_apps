/*
Filename: engine/validation_test.go

Author: M365 Copilot (GPT-5)
Version: v1.0
Owner: Chalearm Saelim
Date: 2026-06-10

Description:
Unit tests for trade validation logic.

Covers:
- Boundary conditions (min/max)
- Capital limits
- Invalid values (negative, zero)

AI Prompt Idea:
"Write Go unit tests for a trade validation function including edge cases and boundary conditions."

How to test:
go test ./engine -v
*/

package engine

import (
    "dexbot/config"
    "testing"
)

func getCfg() config.Config {
    return config.Config{
        MinUSD:          0.0001,
        MaxUSD:          0.001,
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
