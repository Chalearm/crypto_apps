/******************************************************************************
 * File Name       : risk.go
 * File Path       : models/risk.go
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
 *   Shared data models for risk calculation, trading, and portfolio system.
 *
 * Responsibilities:
 *   - Implement core functionality for models package.
 *
 * Usage :
 *   Directory : models/
 *
 *   Build :
 *     go build ./models
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./models
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/models
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
package models

import "time"

// MarketPrice represents a single market price observation.
type MarketPrice struct {
	Token string
	Price float64
	Time  time.Time
}

// Return represents a calculated return value.
type Return struct {
	Token  string
	Value  float64
	Time   time.Time
}

// Covariance stores covariance between two tokens.
type Covariance struct {
	TokenA string
	TokenB string
	Value  float64
}

// Portfolio represents a trading portfolio.
type Portfolio struct {
	ID      string
	Capital float64
	Value   float64
}

// PortfolioAsset represents a single asset within a portfolio.
type PortfolioAsset struct {
	Token    string
	Weight   float64
	Quantity float64
}
