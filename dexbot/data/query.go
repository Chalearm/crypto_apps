/******************************************************************************
 * File Name       : query.go
 * File Path       : data/query.go
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
 *   Simple DB interaction layer: ✅ insert ✅ read ✅ training data
 *
 * Responsibilities:
 *   - Implement core functionality for data package.
 *
 * Usage :
 *   Directory : data/
 *
 *   Build :
 *     go build ./data
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./data
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/data
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
package data

import (
    "database/sql"
)

// insert price
func InsertPrice(db *sql.DB, token string, price float64) error {

    _, err := db.Exec(
        "INSERT INTO market_prices(token, price) VALUES($1,$2)",
        token, price,
    )

    return err
}

// load prices
func LoadPrices(db *sql.DB, token string) ([]float64, error) {

    rows, err := db.Query(
        "SELECT price FROM market_prices WHERE token=$1 ORDER BY id",
        token,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var out []float64

    for rows.Next() {
        var p float64
        rows.Scan(&p)
        out = append(out, p)
    }

    return out, nil
}

// insert return
func InsertReturn(db *sql.DB, token string, value float64) {

    _, _ = db.Exec(
        "INSERT INTO returns(token, return) VALUES($1,$2)",
        token, value,
    )
}
