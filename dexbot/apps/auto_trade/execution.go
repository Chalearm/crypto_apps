/******************************************************************************
 * File Name       : execution.go
 * File Path       : apps/auto_trade/execution.go
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
 *   Execution engine for trading. UPDATED: ✅ removed missing formatFloat ✅ added helper ✅ integrated logging NEW: - ExecuteTrade - simulatePnL - formatFloat
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