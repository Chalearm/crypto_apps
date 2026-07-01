/******************************************************************************
 * File Name       : mpt.go
 * File Path       : apps/risk_lab/mpt.go
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
 *   Simple Markowitz portfolio optimizer. Computes: ✅ mean returns ✅ covariance matrix ✅ equal-weight baseline ✅ portfolio return ✅ portfolio risk
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

import "math/rand"

// expected returns vector
func expectedReturns(data map[string][]float64, names []string) []float64 {

    out := []float64{}

    for _, n := range names {
        r := returns(data[n])
        out = append(out, mean(r))
    }

    return out
}

// portfolio return
func portfolioReturn(weights []float64, returns []float64) float64 {

    sum := 0.0

    for i := range weights {
        sum += weights[i] * returns[i]
    }

    return sum
}

// portfolio risk
func portfolioRisk(weights []float64, cov [][]float64) float64 {

    sum := 0.0

    for i := range weights {
        for j := range weights {
            sum += weights[i] * weights[j] * cov[i][j]
        }
    }

    return sum
}
// inverse variance weighting (simple MPT)
func optimizeWeights(data map[string][]float64, names []string) []float64 {

    weights := make([]float64, len(names))

    total := 0.0

    for i, n := range names {

        r := returns(data[n])
        v := variance(r)

        // avoid divide by zero
        if v == 0 {
            v = 0.000001
        }

        w := 1.0 / v  // 🔥 lower variance → higher weight

        weights[i] = w
        total += w
    }

    // normalize
    for i := range weights {
        weights[i] /= total
    }

    return weights
}


func optimizeSharpe(data map[string][]float64, names []string) []float64 {

    best := make([]float64, len(names))
    bestSharpe := -1.0

    for k := 0; k < 2000; k++ {

        w := randomWeights(len(names))

        ret := portfolioReturn(w, expectedReturns(data, names))
        risk := portfolioRisk(w, covarianceMatrix(data))

        if risk == 0 {
            continue
        }

        s := ret / risk

        if s > bestSharpe {
            bestSharpe = s
            copy(best, w)
        }
    }

    return best
}

func randomWeights(n int) []float64 {
    w := make([]float64, n)
    sum := 0.0

    for i := range w {
        w[i] = rand.Float64()
        sum += w[i]
    }

    for i := range w {
        w[i] /= sum
    }

    return w
}

