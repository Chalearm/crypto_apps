/*
Filename: apps/auto_trade/execution.go

Author: M365 Copilot (GPT-5)
Owner: Chalearm Saelim
Version: v2.0
Date: 2026-06-23 06:41 ICT

Description:
Execution engine for trading.

UPDATED:
✅ removed missing formatFloat
✅ added helper
✅ integrated logging

NEW:
- ExecuteTrade
- simulatePnL
- formatFloat

*/

package main

import (
    "dexbot/infra"
    "fmt"
)

/*
Function: formatFloat
Description:
Format float for logging.

Input:
- val float64

Output:
- string formatted number

Lines: ~5
*/
func formatFloat(val float64) string {
    return fmt.Sprintf("%.6f", val)
}

/*
Function: ExecuteTrade
Description:
Execute buy/sell logic.

Input:
- task *TradeTask
- cfg Config
- price float64

Output:
- none (mutates task)

Lines: ~40
*/
func ExecuteTrade(task *TradeTask, cfg Config, price float64) {

    if task.Status == StatusCreated {

        infra.Info("EXEC BUY → " + task.ID)

        task.BuyPrice = price + cfg.GasPerTrade
        task.Status = StatusBought

        infra.Info("BUY PRICE: " + formatFloat(task.BuyPrice))
        return
    }

    if task.Status == StatusBought {

        infra.Info("EXEC SELL → " + task.ID)

        task.SellPrice = price - cfg.GasPerTrade
        task.Status = StatusCompleted

        pnl := simulatePnL(task.BuyPrice, task.SellPrice)

        infra.Info("SELL PRICE: " + formatFloat(task.SellPrice))
        infra.Info("PNL: " + formatFloat(pnl))
    }
}

/*
Function: simulatePnL
Description:
Calculate profit/loss.

Input:
- buy float64
- sell float64

Output:
- float64 pnl

Lines: ~5
*/
func simulatePnL(buy, sell float64) float64 {
    return sell - buy
}