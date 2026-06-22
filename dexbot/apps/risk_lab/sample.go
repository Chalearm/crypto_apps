/*
Filename: apps/risk_lab/sample.go

Author: M365 Copilot (GPT-5)
Version: v4.0 (Multi-Asset + Options Scenario Dataset)
Owner: Chalearm Saelim
Date: 2026-06-22 07:08 ICT (UTC+7)

Description:
This file defines the synthetic market dataset used for the risk lab system.

The dataset is intentionally designed to simulate:
✅ multi-asset portfolio (BTC, ETH, BNB, UNI, ADA)
✅ regime shift behavior (normal → volatile)
✅ conditions that justify option usage (call/put)
✅ portfolio rebalancing after risk events

--------------------------------------------------
📊 DATA STRUCTURE
--------------------------------------------------

Each asset contains 35 time steps:

Phase 1 (Day 1–20):
- relatively stable market
- moderate upward trend
- low-to-medium volatility

Phase 2 (Day 21–35):
- volatility spike (event regime)
- price swings (sharp up/down)
- drawdowns (especially ADA, UNI)

--------------------------------------------------
🧠 SYSTEM PURPOSE
--------------------------------------------------

This dataset supports the following system behaviors:

1. Risk Analysis
   - compute returns, variance, covariance
   - calculate max drawdown (MDD)
   - identify high-risk assets

2. Portfolio Allocation
   - Phase 1: optimize Sharpe ratio (pre-event)
   - Phase 2: re-optimize after volatility

3. Options Strategy Simulation
   - from Day 20 onward
   - generate CALL and PUT options for each asset
   - used for hedging and speculative decisions

--------------------------------------------------
📉 OPTIONS LOGIC (CONCEPT)
--------------------------------------------------

Based on asset behavior:

- High MDD (e.g., ADA):
    → BUY PUT (hedging downside risk)

- High volatility (e.g., UNI):
    → BUY CALL + PUT (straddle)
    → profit from large movement in either direction

- Stable assets (e.g., BTC):
    → SELL CALL or no option (income strategy)

--------------------------------------------------
📈 EXPECTED SYSTEM OUTPUT
--------------------------------------------------

The system will display:

✅ Terminal:
    - price table
    - returns table
    - statistics (mean, std, variance, MDD)
    - covariance matrix
    - beta table
    - portfolio weights (Phase 1 & Phase 2)
    - sample option outputs

✅ HTML Dashboard (report.html):
    - price chart
    - returns + statistics tables
    - covariance heatmap
    - option table (multi-asset)
    - portfolio phase comparison
    - pie charts (before vs after)

--------------------------------------------------
⚠️ NOTE
--------------------------------------------------

Currently:
- option values are generated (Black-Scholes)
- but NOT yet injected into portfolio returns

Future enhancement:
→ integrate option payoff into returns
→ true hedged portfolio simulation

--------------------------------------------------
RUN INSTRUCTIONS
--------------------------------------------------

Run application:
    cd dexbot/apps/risk_lab
    go run .

Open dashboard:
    open report.html   (macOS)

Run tests:
    go test ./apps/risk_lab -v

--------------------------------------------------
*/

package main

func sampleData() map[string][]float64 {

    return map[string][]float64{

        "BTC": {
            100,102,104,106,108,110,112,114,116,118,
            120,122,124,126,128,130,132,134,136,138,
            // event
            140,135,138,142,145,148,150,147,152,155,
            158,160,162,165,170,
        },

        "ETH": {
            70,72,74,75,77,79,80,82,84,85,
            87,89,90,92,94,96,98,100,102,104,
            // event
            106,101,105,108,110,112,115,113,117,120,
            123,125,128,130,135,
        },

        "BNB": {
            50,52,51,55,53,57,60,58,61,63,
            65,62,66,68,70,69,72,74,76,78,
            // event
            80,75,82,78,85,83,88,90,87,92,
            95,93,97,100,105,
        },

        "UNI": {
            20,21,23,22,24,26,25,27,29,28,
            30,32,31,33,35,34,36,38,37,39,
            // event (volatile)
            42,35,45,38,48,40,50,42,55,45,
            60,48,65,50,70,
        },

        "ADA": {
            10,11,12,11,10,12,11,13,12,14,
            13,15,14,16,15,17,16,18,17,19,
            // event (crash cycles)
            18,12,20,11,22,10,25,9,28,8,
            30,7,32,6,35,
        },
    }
}
