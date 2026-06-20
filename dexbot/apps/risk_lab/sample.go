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

        "BTC": {100, 102, 101, 105, 110, 108, 111, 115, 120, 125, 128, 130, 135, 140, 138, 142, 145},
        "BNB": {50, 51, 52, 54, 55, 57, 60, 62, 64, 65, 67, 69, 70, 72, 74, 75, 77},
        "UNI": {20, 21, 19, 22, 23, 24, 25, 26, 27, 28, 29, 30, 32, 31, 33, 34, 35},
        "ETH": {70, 69, 72, 75, 77, 78, 80, 82, 85, 90, 92, 95, 97, 100, 102, 105, 108},
        "ADA": {10, 10.5, 10.2, 11, 11.5, 12, 12.5, 13, 13.5, 14, 14.2, 14.8, 15, 15.5, 16, 16.3, 17},
    }
}