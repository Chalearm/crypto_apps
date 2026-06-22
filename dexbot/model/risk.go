/*
Filename: models/risk.go

Description:
Shared data models for:
✅ risk calculation
✅ trading
✅ portfolio system
*/

package models

import "time"

type MarketPrice struct {
    Token string
    Price float64
    Time  time.Time
}

type Return struct {
    Token  string
    Value  float64
    Time   time.Time
}

type Covariance struct {
    TokenA string
    TokenB string
    Value  float64
}

type Portfolio struct {
    ID      string
    Capital float64
    Value   float64
}

type PortfolioAsset struct {
    Token    string
    Weight   float64
    Quantity float64
}