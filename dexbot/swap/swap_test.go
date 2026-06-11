/*
Filename: swap/swap_test.go

Author: Gemini
Version: v1.1
Owner: Chalearm Saelim
Date: 2026-06-11

Description:
Tests for swap module configuration and visual precision parsers.

AI Prompt Idea:
"Create tests to validate router address, token decimals, and decimal layout parser metrics in a DEX swap engine."

How to test:
go test ./swap -v
*/

package swap

import "testing"

func TestRouterNotEmpty(t *testing.T) {
    if ROUTER == "" {
        t.Error("router empty")
    }
}

func TestDecimalsExist(t *testing.T) {
    if Decimals["USDC"] == 0 {
        t.Error("missing decimals")
    }
}

func TestDecimalsPositive(t *testing.T) {
    for _, d := range Decimals {
        if d <= 0 {
            t.Error("invalid decimals")
        }
    }
}

func TestCommonTokens(t *testing.T) {
    if _, ok := Decimals["BTT"]; !ok {
        t.Error("missing BTT")
    }
}

func TestRouterLength(t *testing.T) {
    if len(ROUTER) < 10 {
        t.Error("router too short")
    }
}

// NEW: Test Case verifying precision layout alignment functionality 
func TestFormatWithSpacedDecimals(t *testing.T) {
    inputVal := 12345.678901234567
    expected := "12,345.678 901 234 567" // Truncated/rounded safely to 12 decimals with spaces
    
    result := formatWithSpacedDecimals(inputVal)
    if result != expected {
        t.Errorf("Format validation mismatched.\nExpected: %s\nGot:      %s", expected, result)
    }
}

// NEW: Test Case verifying spatial formatting rules for very small fractions
func TestFormatWithSpacedDecimalsSmallFraction(t *testing.T) {
    inputVal := 0.000398501311
    expected := "0.000 398 501 311"
    
    result := formatWithSpacedDecimals(inputVal)
    if result != expected {
        t.Errorf("Format validation mismatched for fractions.\nExpected: %s\nGot:      %s", expected, result)
    }
}