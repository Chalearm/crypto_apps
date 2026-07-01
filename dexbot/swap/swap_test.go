/******************************************************************************
 * File Name       : swap_test.go
 * File Path       : swap/swap_test.go
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
 *   Tests for swap module configuration and visual precision parsers. AI Prompt Idea: "Create tests to validate router address, token decimals, and decimal layout parser metrics in a DEX swap engine." How
 *
 * Responsibilities:
 *   - Implement core functionality for swap package.
 *
 * Usage :
 *   Directory : swap/
 *
 *   Build :
 *     go build ./swap
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./swap
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/swap
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
 *   [Test Functions] Test suite: TestRouterNotEmpty, TestDecimalsExist, TestDecimalsPositive, TestCommonTokens
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