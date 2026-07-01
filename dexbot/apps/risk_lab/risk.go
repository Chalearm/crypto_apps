/******************************************************************************
 * File Name       : risk.go
 * File Path       : apps/risk_lab/risk.go
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
 *   Core risk calculation engine. Implements: - mean - variance - standard deviation - covariance - beta Imported automatically by main.go Execution: go run apps/risk_lab/main.go
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

import "math"

func mean(data []float64) float64 {
    sum := 0.0
    for _, v := range data {
        sum += v
    }
    return sum / float64(len(data))
}

func variance(data []float64) float64 {
    m := mean(data)

    sum := 0.0
    for _, v := range data {
        diff := v - m
        sum += diff * diff
    }

    return sum / float64(len(data))
}

func stddev(data []float64) float64 {
    return math.Sqrt(variance(data))
}

func covariance(a, b []float64) float64 {
    ma := mean(a)
    mb := mean(b)

    sum := 0.0

    for i := range a {
        sum += (a[i] - ma) * (b[i] - mb)
    }

    return sum / float64(len(a))
}
// build covariance matrix
func covarianceMatrix(data map[string][]float64) [][]float64 {

    keys := []string{}

    for k := range data {
        keys = append(keys, k)
    }

    n := len(keys)
    matrix := make([][]float64, n)

    for i := 0; i < n; i++ {

        matrix[i] = make([]float64, n)

        ri := returns(data[keys[i]])

        for j := 0; j < n; j++ {

            rj := returns(data[keys[j]])

            matrix[i][j] = covariance(ri, rj)
        }
    }

    return matrix
}
func beta(assetPrices, marketPrices []float64) float64 {

    rA := returns(assetPrices)
    rM := returns(marketPrices)

    return covariance(rA, rM) / variance(rM)
}
/*
calculate percentage returns

R_t = (P_t - P_t-1) / P_t-1

Output length = N-1
*/
func returns(data []float64) []float64 {

    if len(data) < 2 {
        return []float64{}
    }

    out := make([]float64, 0, len(data)-1)

    for i := 1; i < len(data); i++ {

        r := (data[i] - data[i-1]) / data[i-1]
        out = append(out, r)
    }

    return out
}
// simplified HRP → equal cluster split
func hrpWeights(n int) []float64 {

    // placeholder: equal weighting
    return riskParityWeights(n)
}
// Maximum Drawdown
func maxDrawdown(prices []float64) float64 {

    maxPeak := prices[0]
    maxDD := 0.0

    for _, p := range prices {

        if p > maxPeak {
            maxPeak = p
        }

        dd := (maxPeak - p) / maxPeak

        if dd > maxDD {
            maxDD = dd
        }
    }

    return maxDD
}

// equal risk contribution (simplified)
func riskParityWeights(n int) []float64 {

    w := make([]float64, n)

    for i := range w {
        w[i] = 1.0 / float64(n)
    }

    return w
}
/*
Return with option payoff (hedged return)
*/

func returnsWithOptions(data []float64) []float64 {

    out := []float64{}

    for i := 1; i < len(data); i++ {

        r := (data[i] - data[i-1]) / data[i-1]

        opt := optionPayoff(data[i], data[i-1])

        // ✅ combine
        out = append(out, r+opt)
    }

    return out
}
