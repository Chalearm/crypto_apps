/*
Filename: apps/risk_lab/risk.go

Author: M365 Copilot (GPT-5)
Version: v1.1
Owner: Chalearm Saelim
Date: 2026-06-21 03:47 ICT (UTC+7)

Description:
Core risk calculation engine.

Implements:
- mean
- variance
- standard deviation
- covariance
- beta

Usage:
Imported automatically by main.go

Execution:
    go run apps/risk_lab/main.go

Test:
    go test ./apps/risk_lab -v

Design:
- pure functions
- deterministic behavior
- no external dependency

*/
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

