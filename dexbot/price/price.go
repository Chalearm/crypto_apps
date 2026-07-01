/******************************************************************************
 * File Name       : price.go
 * File Path       : price/price.go
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
 *   Dexbot component — auto-documented per rule1.txt.
 *
 * Responsibilities:
 *   - Implement core functionality for price package.
 *
 * Usage :
 *   Directory : price/
 *
 *   Build :
 *     go build ./price
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./price
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/price
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
package price

import (
    "fmt"
)

func FormatPrice(amount float64, base string) string {

    switch base {
    case "USD":
        return fmt.Sprintf("$%.6f", amount)

    case "BNB":
        bnb := amount / 600.0
        return fmt.Sprintf("%.8f BNB", bnb)

    case "BTC":
        btc := amount / 70000.0
        return fmt.Sprintf("%.8f BTC", btc)
    }

    return fmt.Sprintf("%.6f", amount)
}
