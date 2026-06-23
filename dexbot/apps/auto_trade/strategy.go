/*
Filename: apps/auto_trade/strategy.go

Author: M365 Copilot (GPT-5)
Owner: Chalearm Saelim
Version: v3.0
Date: 2026-06-23 07:00 ICT (UTC+7)

Description:
Strategy engine abstraction.

Supports:
✅ pluggable strategies
✅ decoupled decision logic
✅ future ML integration

Usage:
- used in processTask()

UPDATED:
- integrated logging compatibility

*/

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
