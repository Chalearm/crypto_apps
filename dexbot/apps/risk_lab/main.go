/******************************************************************************
 * File Name       : main.go
 * File Path       : apps/risk_lab/main.go
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
 *   Entry point for Risk Lab mini application. ✅ Price table ✅ Returns table ✅ Statistics (Mean / Std / Variance / MDD) ✅ Covariance matrix ✅ Beta table ✅ Option generation (multi-asset) ✅ Portfolio Phase
 *
 * Responsibilities:
 *   - Implement core functionality for apps package.
 *
 * Usage :
 *   Directory : apps/risk_lab/
 *
 *   Build :
 *     go build ./apps/risk_lab
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./apps/risk_lab
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

////////////////////////////////////////////////////////////
// ✅ HELPER
////////////////////////////////////////////////////////////

func printWeights(weights []float64, names []string) {

    for i, n := range names {
        println(n, "weight:", round(weights[i]))
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

    // ✅ 4. SORTED + COVARIANCE
    names := []string{}
    for k := range data {
        names = append(names, k)
    }
    sort.Strings(names)

    matrix := covarianceMatrix(data)

    // ✅ 5. COVARIANCE MATRIX
    printMatrix(matrix, names)

    // ✅ 6. BETA TABLE
    printBetaTable(data, "BTC")

    // ====================================================
    // ✅ OPTIONS SECTION
    // ====================================================

    println("\n=== OPTION GENERATION (DAY 20+) ===")

    options := generateOptions(data)

    println("Total options generated:", len(options))

    // ✅ SHOW SAMPLE PER ASSET
    println("\n=== OPTION SAMPLE (ALL ASSETS) ===")

    countByAsset := map[string]int{}

    for _, o := range options {

        if countByAsset[o.Asset] >= 2 {
            continue
        }

        println(
            "Day", o.Day,
            o.Asset,
            "Call:", round(o.Call),
            "Put:", round(o.Put),
        )

        countByAsset[o.Asset]++
    }
println("\n=== OPTION TIMELINE (DAY 20–35) ===")

for asset, prices := range data {

    decisions := decideOptionTimeline(asset, prices)

    println("\nAsset:", asset)

    for i := 20; i < len(prices); i++ {

        println(
            "Day", i,
            "Price:", prices[i],
            "→", decisions[i],
        )
    }
}
    // ✅ ====================================================
    // ✅ OPTION DECISION ENGINE
    // ====================================================

    println("\n=== OPTION DECISION ===")

    for asset, prices := range data {

        action := decideOptionAction(asset, prices)

        lastPrice := prices[len(prices)-1]

        call := blackScholesCall(lastPrice, lastPrice*1.05, 0.1, 0.01, 0.3)
        put := blackScholesPut(lastPrice, lastPrice*1.05, 0.1, 0.01, 0.3)

        println(
            asset,
            "Action:", action,
            "Call:", round(call),
            "Put:", round(put),
        )
    }

    // ====================================================
    // ✅ PORTFOLIO PHASE 1
    // ====================================================

    println("\n=== PORTFOLIO PHASE 1 (BEFORE OPTIONS) ===")

    w1 := optimizeSharpe(data, names)
    printWeights(w1, names)

    // ====================================================
    // ✅ PORTFOLIO PHASE 2 (HEDGED)
    // ====================================================

    println("\n=== PORTFOLIO PHASE 2 (HEDGED) ===")

    // ✅ clone dataset (future: inject option payoff here)
    hedged := map[string][]float64{}

    for k, v := range data {
        hedged[k] = v
    }

    w2 := optimizeSharpe(hedged, names)
    printWeights(w2, names)

    // ====================================================
    // ✅ OUTPUT HTML DASHBOARD
    // ====================================================

    generateHTML(data)

    println("\n✅ Dashboard generated → open report.html")
}
