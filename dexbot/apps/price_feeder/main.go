/******************************************************************************
 * File Name       : main.go
 * File Path       : apps/price_feeder/main.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:35 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:35 (UTC+7)
 *
 * Description     :
 *   Live price feeder service. ✅ generate price every second ✅ insert into DB ✅ production-safe logging Run: go run ./apps/price_feeder UPDATED: - initial version
 *
 * Responsibilities:
 *   - - Implement core functionality for apps package.
 *
 * Usage :
 *   Directory : apps/price_feeder/
 *
 *   Build :
 *     go build ./apps/price_feeder
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./apps/price_feeder
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
 *   1.0.0   | 2026-07-01 19:25:35 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
/*
Filename: apps/price_feeder/main.go

Author: M365 Copilot (GPT-5)
Owner: Chalearm Saelim
Version: v1.0
Date: 2026-06-23 08:35 ICT (UTC+7)

Description:
Live price feeder service.

Features:
✅ generate price every second
✅ insert into DB
✅ production-safe logging

Usage:

Run:
go run ./apps/price_feeder

UPDATED:
- initial version

NEW:
- feeder main loop
*/

package main

import (
    "fmt"
    "math/rand"
    "time"

    "dexbot/infra"
)

/******************************************************************************
 * Function Name : generatePrice
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

  *
  * Function Name : generatePrice
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
func generatePrice() float64 {
    base := 1.0
    noise := rand.Float64() * 0.05
    return base + noise
}

/******************************************************************************
 * Function Name : insertPrice
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

func insertPrice(token string, price float64) {

    if infra.DB == nil {
        infra.Warn("DB not ready → skip insert")
        return
    }

    query := `
    INSERT INTO market_prices (token, price)
    VALUES ($1, $2)
    `

    _, err := infra.DB.Exec(query, token, price)

    if err != nil {
        infra.Error("insert failed: " + err.Error())
        return
    }

    infra.Info(fmt.Sprintf("price inserted: %s = %.4f", token, price))
}

/******************************************************************************
 * Function Name : main
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

func main() {

    infra.InitLogger()
    infra.LoadEnv("../../config.env")

    err := infra.InitDB()
    if err != nil {
        infra.Error("DB init failed")
        return
    }

    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for range ticker.C {

        price := generatePrice()

        insertPrice("BTT", price)
    }
}
