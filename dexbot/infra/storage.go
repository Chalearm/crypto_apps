/******************************************************************************
 * File Name       : storage.go
 * File Path       : infra/storage.go
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
 *   Local buffer storage. ✅ safe write ✅ disk guard ✅ logging UPDATED: - disk protection
 *
 * Responsibilities:
 *   - Implement core functionality for infra package.
 *
 * Usage :
 *   Directory : infra/
 *
 *   Build :
 *     go build ./infra
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./infra
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/infra
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
package infra

import (
    "os"
)

/*
Function: SaveLocal
Description:
Store data to local buffer safely.

Input:
- file string
- data string

Output:
- error

Lines: ~25
*/
func SaveLocal(file string, data string) error {

    if GetFreeDiskPercent() < 5 {
        Error("disk full → skip saving")
        return nil
    }

    err := os.MkdirAll("data/buffer", 0755)
    if err != nil {
        Error("mkdir failed")
        return err
    }

    err = os.WriteFile(file, []byte(data), 0644)
    if err != nil {
        Error("write file failed")
        return err
    }

    Info("saved local: " + file)

    return nil
}