/******************************************************************************
 * File Name       : price_test.go
 * File Path       : apps/auto_trade/price_test.go
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
 *   Dexbot component — auto-documented per rule1.txt.
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
 *   [Test Functions] Test suite: TestPriceFallback, TestSimulatePrice
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

import "testing"

/*
Test: fallback works
*/
func TestPriceFallback(t *testing.T) {

    price := GetLatestPrice("UNKNOWN")

    if price <= 0 {
        t.Error("price should fallback > 0")
    }
}

/*
Test: simulate consistency
*/
func TestSimulatePrice(t *testing.T) {

    p := simulatePrice()

    if p == 0 {
        t.Error("simulated price invalid")
    }
}