/*
Filename: apps/risk_lab/options.go
Date: 2026-06-22


Description:
Option pricing and simulation logic using Black-Scholes model.

Used in Phase 2 (Day >= 20):

- CALL option → profit when price increases
- PUT option  → profit when price decreases

Purpose:
✅ hedging high-risk assets (via PUT)
✅ capturing volatility (via CALL+PUT)
✅ extending portfolio beyond spot trading

Future:
- integrate payoff into portfolio returns
- add Greeks (delta, gamma)


Usage:
    used in option phase (day >=20)

*/

package main

import "math"

func normCDF(x float64) float64 {
    return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

func blackScholesCall(S, K, T, r, sigma float64) float64 {

    d1 := (math.Log(S/K) + (r+sigma*sigma/2)*T) / (sigma * math.Sqrt(T))
    d2 := d1 - sigma*math.Sqrt(T)

    return S*normCDF(d1) - K*math.Exp(-r*T)*normCDF(d2)
}

func blackScholesPut(S, K, T, r, sigma float64) float64 {

    d1 := (math.Log(S/K) + (r+sigma*sigma/2)*T) / (sigma * math.Sqrt(T))
    d2 := d1 - sigma*math.Sqrt(T)

    return K*math.Exp(-r*T)*normCDF(-d2) - S*normCDF(-d1)
}


/*
Option payoff approximation

- simulate profit/loss of holding option for each day
*/

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
func deltaCall(S, K, T, r, sigma float64) float64 {

    d1 := (math.Log(S/K) + (r+sigma*sigma/2)*T) / (sigma * math.Sqrt(T))

    return normCDF(d1)
}
func gamma(S, K, T, r, sigma float64) float64 {

    d1 := (math.Log(S/K) + (r+sigma*sigma/2)*T) / (sigma * math.Sqrt(T))

    return math.Exp(-d1*d1/2) / (S * sigma * math.Sqrt(2*math.Pi*T))
}
func deltaPut(S, K, T, r, sigma float64) float64 {

    return deltaCall(S, K, T, r, sigma) - 1
}
