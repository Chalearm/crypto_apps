/******************************************************************************
 * File Name       : strategy.go
 * File Path       : apps/auto_trade/strategy.go
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
 *   Strategy engine abstraction. Supports: ✅ pluggable strategies ✅ decoupled decision logic ✅ future ML integration - used in processTask() UPDATED: - integrated logging compatibility
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
 *   [Types] Struct definitions in this file
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

import "dexbot/infra"

// =====================================================
// ✅ INTERFACE
// =====================================================

/*
Function: Strategy (interface)
Description:
Defines trading decision behavior.

Methods:
- ShouldBuy()
- ShouldSell()

Lines: ~10
*/
type Strategy interface {
    ShouldBuy() bool
    ShouldSell(task *TradeTask, price float64) bool
}

// =====================================================
// ✅ DEFAULT IMPLEMENTATION
// =====================================================

/*
Function: SimpleStrategy.ShouldBuy
Description:
Always returns true (baseline entry strategy)

Output:
- bool

Lines: ~5
*/
func (s SimpleStrategy) ShouldBuy() bool {

    infra.Info("Strategy → ShouldBuy = TRUE")

    return true
}

/*
Function: SimpleStrategy.ShouldSell
Description:
Sell when profit threshold reached.

Input:
- task *TradeTask
- price float64

Output:
- bool

Lines: ~10
*/
func (s SimpleStrategy) ShouldSell(task *TradeTask, price float64) bool {

    if task.BuyPrice == 0 {
        return false
    }

    if price >= task.BuyPrice*1.05 {
        infra.Info("Strategy → SELL condition met")
        return true
    }

    return false
}

/*
Struct: SimpleStrategy
Description:
Baseline strategy.

Lines: ~5
*/
type SimpleStrategy struct{}

// ✅ global strategy instance
var strategy Strategy = SimpleStrategy{}
