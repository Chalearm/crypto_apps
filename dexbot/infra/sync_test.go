/******************************************************************************
 * File Name       : sync_test.go
 * File Path       : infra/sync_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:32 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:32 (UTC+7)
 *
 * Description     :
 *   Sync tests. UPDATED: - fixed import cycle
 *
 * Responsibilities:
 *   - - Implement core functionality for infra package.
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
 *   1.0.0   | 2026-07-01 19:25:32 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package infra_test

import (
    "os"
    "testing"

    "dexbot/infra"
)

/*
Function: TestSync_File
*/
func TestSync_File(t *testing.T) {

    os.MkdirAll("data/buffer", 0755)

    infra.SaveLocal("data/buffer/test.json",
        `{"token":"BTT","price":1.1}`)

    infra.RunSyncCycle()
}

/*
Function: TestSync_NoCrash
*/
func TestSync_NoCrash(t *testing.T) {
    infra.RunSyncCycle()
}