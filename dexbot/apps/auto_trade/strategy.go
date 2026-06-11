/*
Filename: apps/auto_trade/strategy.go

Author: M365 Copilot (GPT-5)
Version: v2.0
Owner: Chalearm Saelim
Date: 2026-06-12 01:14

Description:
Strategy engine abstraction.

Supports:
✅ pluggable trading logic
✅ multiple strategies
✅ clean separation of concerns

Usage:
- injected into processTask()
*/

package main

// Strategy interface (extensible)
type Strategy interface {
    ShouldBuy() bool
    ShouldSell(task *TradeTask, price float64) bool
}

// Default strategy implementation
type SimpleStrategy struct{}

func (s SimpleStrategy) ShouldBuy() bool {
    return true
}

func (s SimpleStrategy) ShouldSell(task *TradeTask, price float64) bool {
    return price >= task.BuyPrice*1.05
}

// global strategy
var strategy Strategy = SimpleStrategy{}