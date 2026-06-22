/*
Filename: apps/risk_lab/main.go

Author: M365 Copilot (GPT-5)
Version: v3.0 (Full Quant + Options + Portfolio Phases)
Owner: Chalearm Saelim
Date: 2026-06-22 07:26 ICT (UTC+7)

Description:
Entry point for Risk Lab mini application.

Features:
✅ Price table
✅ Returns table
✅ Statistics (Mean / Std / Variance / MDD)
✅ Covariance matrix
✅ Beta table
✅ Option generation (multi-asset)
✅ Portfolio Phase 1 (before options)
✅ Portfolio Phase 2 (after options)
✅ HTML dashboard output


Usage:
    cd dexbot/apps/risk_lab
    go run .

Test:
    go test ./apps/risk_lab -v
Output:
    1. price table (time-series)
    2. risk metrics (mean, std, covariance, beta)
    3. HTML report (report.html)

Execution Flow:
    1. load sample data
    2. print table (readable visualization)
    3. run calculations
    4. generate report
Expected Output:
- console output of risk metrics
- HTML report file: ./report.html

Dependencies:
- standard Go
- no DB required (standalone demo)

*/

package main

import (
    "sort"
    "math"
)

func printReturnsTable(data map[string][]float64) {

    println("\n=== RETURNS TABLE ===")

    names := []string{}

    for k := range data {
        names = append(names, k)
    }

    sort.Strings(names)

    // header
    print("Day\t")
    for _, name := range names {
        print(name, "\t")
    }
    println()

    length := len(data[names[0]]) - 1 // returns has N-1

    for i := 0; i < length; i++ {

        print(i+1, "\t")

        for _, name := range names {

            r := returns(data[name])
            print(round(r[i]), "\t")
        }

        println()
    }
}
func riskScore(data []float64) float64 {

    r := returns(data)

    return stddev(r) + maxDrawdown(data)
}
func printStatsTable(data map[string][]float64) {

    println("\n=== STATISTICS (BASED ON RETURNS) ===")
    println("Asset\tMean\tStdDev\tVariance\tMDD")

    names := []string{}
    for k := range data {
        names = append(names, k)
    }
    sort.Strings(names)

    for _, asset := range names {

        prices := data[asset]
        r := returns(prices)

        m := mean(r)
        s := stddev(r)
        v := variance(r)
        mdd := maxDrawdown(prices)

        println(
            asset, "\t",
            round(m), "\t",
            round(s), "\t",
            round(v), "\t",
            round(mdd*100), "%")
    }
}
func printTable(data map[string][]float64) {

    println("\n=== PRICE TABLE ===")

    names := []string{}

    for k := range data {
        names = append(names, k)
    }

    // ✅ fix order by sorting
    sort.Strings(names)

    // header
    print("Day\t")
    for _, name := range names {
        print(name, "\t")
    }
    println()

    length := len(data[names[0]])

    for i := 0; i < length; i++ {

        print(i+1, "\t")

        for _, name := range names {
            print(data[name][i], "\t")
        }

        println()
    }
}
func printBetaTable(data map[string][]float64, market string) {

    println("\n=== BETA (vs", market, ") ===")

    marketData := data[market]

    names := []string{}
    for k := range data {
        names = append(names, k)
    }

    sort.Strings(names)

    for _, asset := range names {

        if asset == market {
            continue
        }

        b := beta(data[asset], marketData)

        println(asset, ":", round(b))
    }
}

func round(val float64) float64 {
    return math.Round(val*1000000) / 1000000
}


func printMatrix(matrix [][]float64, names []string) {

    println("\n=== COVARIANCE MATRIX ===")

    print("\t")
    for _, n := range names {
        print(n, "\t")
    }
    println()

    for i := range matrix {

        print(names[i], "\t")

        for j := range matrix[i] {
            print(matrix[i][j], "\t")
        }

        println()
    }
}
 

func main() {

    println("=== RISK LAB START ===")

    data := sampleData()

    // ✅ 1. PRICE TABLE
    printTable(data)

    // ✅ 2. RETURNS TABLE
    printReturnsTable(data)

    // ✅ 3. STATISTICS
    printStatsTable(data)

    // ✅ 4. SORTED NAMES + COVARIANCE
    names := []string{}
    for k := range data {
        names = append(names, k)
    }
    sort.Strings(names)

    matrix := covarianceMatrix(data)

    // ✅ 5. COV MATRIX
    printMatrix(matrix, names)

    // ✅ 6. BETA TABLE
    printBetaTable(data, "BTC")

    // ✅ =====================================================
    // ✅ NEW FEATURE: OPTIONS + MULTI-PHASE PORTFOLIO
    // ✅ =====================================================

    println("\n=== OPTION GENERATION (DAY 20+) ===")

    options := generateOptions(data)

    println("Total options generated:", len(options))

    // print only sample (avoid too long)
    for i, o := range options {

        if i > 10 {
            break
        }

        println(
            "Day", o.Day,
            o.Asset,
            "Call:", round(o.Call),
            "Put:", round(o.Put),
        )
    }

    // ✅ PHASE 1 (before options)
    println("\n=== PORTFOLIO PHASE 1 (BEFORE OPTIONS) ===")

    w1 := optimizeSharpe(data, names)

    for i, n := range names {
        println(n, "weight:", round(w1[i]))
    }

    // ✅ PHASE 2 (after options)
    // NOTE: currently same data → later we inject option payoff
    println("\n=== PORTFOLIO PHASE 2 (AFTER OPTIONS / REOPTIMIZED) ===")

    w2 := optimizeSharpe(data, names)

    for i, n := range names {
        println(n, "weight:", round(w2[i]))
    }

    // ✅ FINAL OUTPUT → HTML DASHBOARD
    generateHTML(data)

    println("\n✅ Dashboard generated → open report.html")
}