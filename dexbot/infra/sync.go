/*
Filename: infra/sync.go

Author: M365 Copilot (GPT-5)
Owner: Chalearm Saelim
Version: v2.0
Date: 2026-06-23 07:40 ICT (UTC+7)

Description:
Data synchronization module.

Synchronizes local buffered JSON files to DB.

Features:
✅ scan local buffer folder
✅ insert into DB
✅ delete after success
✅ fallback safe

UPDATED:
- real DB insert logic
- logging improved

NEW:
- insertMarketData()

*/

package infra

import (
    "encoding/json"
    "os"
    "path/filepath"
)

/*
Struct: MarketData
Description:
Represents data from buffer.

Fields:
- Token
- Price
*/
type MarketData struct {
    Token string  `json:"token"`
    Price float64 `json:"price"`
}

/*
Function: DBHealthy
Description:
Check DB health.

Output:
- bool

Lines: ~5
*/
func DBHealthy() bool {
    return CheckDBHealth() == nil
}

/*
Function: insertMarketData
Description:
Insert data into DB.

Input:
- data MarketData

Output:
- error

Lines: ~15
*/
func insertMarketData(data MarketData) error {

    query := `
    INSERT INTO market_prices (token, price)
    VALUES ($1, $2)
    `

    _, err := DB.Exec(query, data.Token, data.Price)

    return err
}

/*
Function: RunSyncCycle
Description:
Scan local buffer and sync to DB.

Input:
- none

Output:
- none

Lines: ~50
*/
func RunSyncCycle() {

    files, err := filepath.Glob("data/buffer/*.json")

    if err != nil {
        Error("failed to scan buffer folder")
        return
    }

    if len(files) == 0 {
        Info("no files to sync")
        return
    }

    for _, f := range files {

        Info("syncing file: " + f)

        dataBytes, err := os.ReadFile(f)
        if err != nil {
            Warn("cannot read file: " + f)
            continue
        }

        var data MarketData

        err = json.Unmarshal(dataBytes, &data)
        if err != nil {
            Warn("invalid json: " + f)
            continue
        }

        if DBHealthy() {

            err := insertMarketData(data)

            if err != nil {
                Error("DB insert failed: " + err.Error())
                continue
            }

            err = os.Remove(f)
            if err != nil {
                Warn("failed to delete synced file")
            } else {
                Info("file synced and removed: " + f)
            }

        } else {

            Warn("DB not available → keep file: " + f)
        }
    }
}
