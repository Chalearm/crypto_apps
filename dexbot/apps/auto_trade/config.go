/*
Filename: apps/auto_trade/config.go

Author: M365 Copilot (GPT-5)
Owner: Chalearm Saelim
Version: v1.0
Date: 2026-06-23 06:41 ICT

Description:
Global runtime configuration for auto_trade system.

Supports:
✅ fake trading mode
✅ options enable switch
✅ gas simulation

Usage:
    auto-loaded in daemon

NEW:
- Config struct

*/

package main

/*
Function: Config struct
Description:
System configuration.

Fields:
- FakeTrading: enable fake execution
- EnableOptions: enable options trading
- GasPerTrade: simulated gas fee

Lines: ~10
*/
type Config struct {
    FakeTrading   bool
    EnableOptions bool
    GasPerTrade   float64
}