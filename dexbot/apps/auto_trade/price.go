/******************************************************************************
 * File Name       : price.go
 * File Path       : apps/auto_trade/price.go
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
 *   Fetch latest price from DB. Fallback: simulate if DB unavailable NEW: - GetLatestPrice
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
 *   [Functions] All exported functions in this file
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
    "dexbot/infra"
)

/*
Function: GetLatestPrice
Description:
Fetch latest price from DB.

Input:
- token string

Output:
- float64 price

Lines: ~25
*/
func GetLatestPrice(token string) float64 {

    if infra.DB == nil {
        infra.Warn("DB not available → using simulated price")
        return simulatePrice()
    }

    var price float64

    query := `
    SELECT price FROM market_prices
    WHERE token=$1
    ORDER BY ts DESC LIMIT 1
    `

    err := infra.DB.QueryRow(query, token).Scan(&price)

    if err != nil {
        infra.Warn("No DB price → fallback simulation")
        return simulatePrice()
    }

    infra.Info("DB price fetched: " + formatFloat(price))

    return price
}