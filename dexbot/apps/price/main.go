/******************************************************************************
 * File Name       : main.go
 * File Path       : apps/price/main.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:39 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:39 (UTC+7)
 *
 * Description     :
 *   Dexbot component — auto-documented per rule1.txt.
 *
 * Responsibilities:
 *   - - Implement core functionality for apps package.
 *
 * Usage :
 *   Directory : apps/price/
 *
 *   Build :
 *     go build ./apps/price
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./apps/price
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
 *   1.0.0   | 2026-07-01 19:25:39 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package main

import (
    "fmt"
    "dexbot/price"
)

/******************************************************************************
 * Function Name : main
 *
 * Purpose :
 *   Entry point for the application.
 *
 * Inputs :
 *   None (reads os.Args or stdlib flags)
 *
 * Return :
 *   None (exits with code 0 on success)
 *
 * Complexity :
 *   Time  : O(N)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   15
 ******************************************************************************/
func main() {

    shibUSD := 0.000025

    usd := price.FormatPrice(shibUSD, "USD")
    bnb := price.FormatPrice(shibUSD, "BNB")

    fmt.Println("SHIBA PRICE:")
    fmt.Printf("%s (%s)\n", usd, bnb)
}
