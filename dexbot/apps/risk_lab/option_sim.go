/*
Filename: apps/risk_lab/option_sim.go
Date: 2026-06-22

Description:
Simulate buying call/put in volatile phase

*/

package main


type OptionResult struct {
    Day   int
    Asset string
    Call  float64
    Put   float64
}

func generateOptions(data map[string][]float64) []OptionResult {

    var results []OptionResult

    for asset, prices := range data {

        for day := 20; day < len(prices); day++ {

            S := prices[day]
            K := S * 1.05

            call := blackScholesCall(S, K, 0.1, 0.01, 0.3)
            put := blackScholesPut(S, K, 0.1, 0.01, 0.3)

            results = append(results, OptionResult{
                Day:   day,
                Asset: asset,
                Call:  call,
                Put:   put,
            })
        }
    }

    return results
}
