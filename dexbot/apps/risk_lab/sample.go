/*
Filename: apps/risk_lab/sample.go

Author: M365 Copilot (GPT-5)
Version: v2.0
Owner: Chalearm Saelim
Date: 2026-06-21 05:26 ICT (UTC+7)

Description:
Extended dataset:
✅ 5 assets
✅ 17 days
✅ realistic variations

Assets:
BTC, BNB, UNI, ETH, ADA
*/

package main

func sampleData() map[string][]float64 {

    return map[string][]float64{

        // ✅ stable upward trend (LOW RISK)
        "BTC": {
            100, 102, 104, 106, 108, 110, 112, 113, 115,
            117, 118, 120, 122, 124, 125, 127, 128,
        },

        // ✅ medium volatility
        "BNB": {
            50, 52, 51, 55, 53, 57, 60, 58, 61,
            63, 65, 62, 66, 68, 70, 69, 72,
        },

        // ✅ HIGH volatility (big swings)
        "UNI": {
            20, 25, 18, 28, 22, 30, 26, 35, 29,
            40, 33, 45, 36, 50, 42, 55, 48,
        },

        // ✅ moderate steady growth
        "ETH": {
            70, 71, 73, 75, 76, 78, 80, 82, 83,
            85, 87, 89, 90, 92, 94, 96, 98,
        },

        // ✅ risky drawdowns (big drops)
        "ADA": {
            10, 12, 9, 13, 8, 14, 7, 15, 6,
            16, 5, 17, 6, 18, 7, 19, 8,
        },
    }
}