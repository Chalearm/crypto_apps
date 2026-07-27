/******************************************************************************
 * File Name       : sync.go
 * File Path       : infra/sync.go
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:32 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:32 (UTC+7)
 * Description     : Data synchronization module for buffered local files to DB.
 ******************************************************************************/
package infra

import (
    "encoding/json"
    "os"
    "path/filepath"
)

type MarketData struct {
    Token string  `json:"token"`
    Price float64 `json:"price"`
}

/******************************************************************************
 * Function Name : DBHealthy
 * Purpose       : Helper to verify if database connectivity is available.
 ******************************************************************************/
func DBHealthy() bool {
    return CheckDBHealth() == nil
}

/******************************************************************************
 * Function Name : insertMarketData
 * Purpose       : Inserts buffered MarketData struct into market_prices DB table.
 ******************************************************************************/
func insertMarketData(data MarketData) error {
    query := `
    INSERT INTO market_prices (symbol, price, volume, high_24h, low_24h, base_asset, quote_asset, chain_id, source)
    VALUES ($1, $2, 0, 0, 0, $1, 'USD', '56', 'buffer_sync')
    `
    _, err := DB.Exec(query, data.Token, data.Price)
    return err
}

/******************************************************************************
 * Function Name : RunSyncCycle
 * Purpose       : Scans local buffer files, syncs valid JSON to DB, and deletes synced files.
 ******************************************************************************/
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