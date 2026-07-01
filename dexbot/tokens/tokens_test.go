/******************************************************************************
 * File Name       : tokens_test.go
 * File Path       : tokens/tokens_test.go
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
 *   Tests for token registry. AI Prompt Idea: "Create Go unit tests for validating a token map with blockchain addresses." How to test: go test ./tokens -v
 *
 * Responsibilities:
 *   - Implement core functionality for tokens package.
 *
 * Usage :
 *   Directory : tokens/
 *
 *   Build :
 *     go build ./tokens
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./tokens
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/tokens
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
 *   [Test Functions] Test suite: TestUSDCExists, TestInvalidToken, TestTokenCount, TestAddressNotZero
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
package tokens

import "testing"

func TestUSDCExists(t *testing.T) {
    if _, ok := Tokens["USDC"]; !ok {
        t.Error("USDC missing")
    }
}

func TestInvalidToken(t *testing.T) {
    if _, ok := Tokens["FAKE"]; ok {
        t.Error("fake token exists")
    }
}

func TestTokenCount(t *testing.T) {
    if len(Tokens) < 5 {
        t.Error("not enough tokens")
    }
}

func TestAddressNotZero(t *testing.T) {
    if Tokens["USDC"].Hex() == "0x0000000000000000000000000000000000000000" {
        t.Error("invalid address")
    }
}

func TestSymbolsNonEmpty(t *testing.T) {
    for k := range Tokens {
        if k == "" {
            t.Error("empty symbol")
        }
    }
}

