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
 * Created Date    : 2026-07-01 19:25:38 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:38 (UTC+7)
 *
 * Description     :
 *   Strategy engine abstraction. Supports: ✅ pluggable strategies ✅ decoupled decision logic ✅ future ML integration - used in processTask() UPDATED: - integrated logging compatibility
 *
 * Responsibilities:
 *   - - Implement core functionality for apps package.
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
 *   1.0.0   | 2026-07-01 19:25:38 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
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

/******************************************************************************
 * Function Name : ShouldBuy
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

  *
  * Function Name : ShouldBuy
  * Purpose :
  *   Performs its designated operation.
  * Inputs :
  *   None (see function signature)
  * Return :
  *   Type        : varies
  *   Description : Result of computation.
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  * Error Cases :
  *   - None
  * Number Of Lines :
  *   10
func (s SimpleStrategy) ShouldBuy() bool {

    infra.Info("Strategy → ShouldBuy = TRUE")

    return true
}

/******************************************************************************
 * Function Name : ShouldSell
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

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
