/******************************************************************************
 * File Name       : validation.go
 * File Path       : engine/validation.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:23 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:23 (UTC+7)
 *
 * Description     :
 *   Trade validation logic, migrated from deleted dexbot/config package during Phase 1 deduplication. Provides IsValidTrade for amount validation. Import "dexbot/engine" and call engine.IsValidTrade(amoun
 *
 * Responsibilities:
 *   - - Implement core functionality for engine package.
 *
 * Usage :
 *   Directory : engine/
 *
 *   Build :
 *     go build ./engine
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./engine
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/engine
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
 *   1.0.0   | 2026-07-01 19:25:23 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package engine

// Config holds trade validation parameters.
type Config struct {
	MinUSD          float64 // Minimum trade amount in USD
	MaxUSD          float64 // Maximum trade amount in USD
	TotalCapitalUSD float64 // Total available capital in USD
}

/*
Function: IsValidTrade
Description:
  Validates that a proposed trade amount is positive, within min/max bounds,
  and does not exceed remaining capital.

Input:
  - amount float64: Proposed trade amount in USD (range: >0).
  - used   float64: Already-committed capital in USD (range: >=0).
  - cfg    Config: Bounds and total capital (see Config struct).

Output:
  - bool: true if trade is valid, false otherwise.

Lines: ~20
*/
/******************************************************************************
 * Function Name : IsValidTrade
 *
 * Purpose :
 *   Validate whether a trade meets all risk management constraints.
 *
 * Inputs :
 *   trade   Trade
 *     Type        : Trade struct
 *     Description : Trade to validate.
 *
 * Return :
 *   Type        : bool
 *   Description : True if the trade passes all validation rules.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None (returns false for invalid trades).
 *
 * Number Of Lines :
 *   20
 ******************************************************************************/
  *
  * Function Name : IsValidTrade
  * Purpose :
  *   Performs its designated operation.
  * Inputs :
  *   None (see function signature)
  * Return :
  *   Type        : varies
  *   Description : Result of computation.
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  * Error Cases :
  *   - None
  * Number Of Lines :
  *   10
func IsValidTrade(amount float64, used float64, cfg Config) bool {
	if amount <= 0 {
		return false
	}
	if amount < cfg.MinUSD {
		return false
	}
	if amount > cfg.MaxUSD {
		return false
	}
	if used+amount > cfg.TotalCapitalUSD {
		return false
	}
	return true
}
