/******************************************************************************
 * File Name       : option_sim.go
 * File Path       : apps/risk_lab/option_sim.go
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
 *   Simulate buying call/put in volatile phase
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
 *   [Types] Struct definitions in this file
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


type OptionResult struct {
    Day   int
    Asset string
    Call  float64
    Put   float64
}
/*
Decide whether to BUY CALL / BUY PUT / STRADDLE
*/

func decideOptionAction(asset string, prices []float64) string {

    r := returns(prices)
    v := variance(r)
    mdd := maxDrawdown(prices)

    // ✅ HIGH RISK → hedge
    if mdd > 0.4 {
        return "BUY PUT (HEDGE)"
    }

    // ✅ HIGH VOLATILITY
    if v > 0.02 {
        return "STRADDLE (CALL+PUT)"
    }

    // ✅ LOW RISK
    return "SELL CALL"
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
/*
Decision timeline for each asset per day
*/

func decideOptionTimeline(asset string, prices []float64) []string {

    decisions := []string{"HOLD"} // day 0

    for i := 1; i < len(prices); i++ {

        change := (prices[i] - prices[i-1]) / prices[i-1]

        decision := "HOLD"

        // ✅ BIG DROP → hedge
        if change < -0.15 {
            decision = "BUY PUT (HEDGE CRASH)"
        }

        // ✅ BIG RISE → bullish
        if change > 0.15 {
            decision = "BUY CALL (BULLISH)"
        }

        // ✅ EXTREME VOL
        if change > 0.25 || change < -0.25 {
            decision = "STRADDLE (VOL PLAY)"
        }

        // ✅ LOW MOVEMENT
        if change < 0.02 && change > -0.02 {
            decision = "SELL CALL (INCOME)"
        }

        decisions = append(decisions, decision)
    }

    return decisions
}

