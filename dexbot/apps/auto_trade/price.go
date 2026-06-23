/*
Filename: apps/auto_trade/price.go
Author: M365 Copilot
Version: v1.0
Date: 2026-06-23

Description:
Fetch latest price from DB.

Fallback: simulate if DB unavailable

NEW:
- GetLatestPrice

*/

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