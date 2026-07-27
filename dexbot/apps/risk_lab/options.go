/******************************************************************************
 * File Name       : options.go
 * File Path       : apps/risk_lab/options.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:40 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:40 (UTC+7)
 *
 * Description     :
 *   Option pricing and simulation logic using Black-Scholes model. Used in Phase 2 (Day >= 20): - CALL option → profit when price increases - PUT option  → profit when price decreases Purpose: ✅ hedging h
 *
 * Responsibilities:
 *   - - Implement core functionality for apps package.
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
 *   1.0.0   | 2026-07-01 19:25:40 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
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
/******************************************************************************
 * Function Name : normCDF
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func normCDF(x float64) float64 {
    return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}
/******************************************************************************
 * Function Name : blackScholesCall
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func blackScholesCall(S, K, T, r, sigma float64) float64 {

    d1 := (math.Log(S/K) + (r+sigma*sigma/2)*T) / (sigma * math.Sqrt(T))
    d2 := d1 - sigma*math.Sqrt(T)

    return S*normCDF(d1) - K*math.Exp(-r*T)*normCDF(d2)
}
/******************************************************************************
 * Function Name : blackScholesPut
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func blackScholesPut(S, K, T, r, sigma float64) float64 {

    d1 := (math.Log(S/K) + (r+sigma*sigma/2)*T) / (sigma * math.Sqrt(T))
    d2 := d1 - sigma*math.Sqrt(T)

    return K*math.Exp(-r*T)*normCDF(-d2) - S*normCDF(-d1)
}


/******************************************************************************
 * Function Name : optionPayoff
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func optionPayoff(priceToday, pricePrev float64) float64 {

    change := (priceToday - pricePrev) / pricePrev

    // ✅ simple logic:

    if change < -0.1 {
        // big drop → PUT wins
        return -change * 0.5
    }

    if change > 0.1 {
        // big upward → CALL wins
        return change * 0.5
    }

    // no major move → premium loss
    return -0.01
}
/******************************************************************************
 * Function Name : deltaCall
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func deltaCall(S, K, T, r, sigma float64) float64 {

    d1 := (math.Log(S/K) + (r+sigma*sigma/2)*T) / (sigma * math.Sqrt(T))

    return normCDF(d1)
}
/******************************************************************************
 * Function Name : gamma
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func gamma(S, K, T, r, sigma float64) float64 {

    d1 := (math.Log(S/K) + (r+sigma*sigma/2)*T) / (sigma * math.Sqrt(T))

    return math.Exp(-d1*d1/2) / (S * sigma * math.Sqrt(2*math.Pi*T))
}
/******************************************************************************
 * Function Name : deltaPut
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func deltaPut(S, K, T, r, sigma float64) float64 {

    return deltaCall(S, K, T, r, sigma) - 1
}
