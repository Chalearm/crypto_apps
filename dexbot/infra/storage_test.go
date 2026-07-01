/******************************************************************************
 * File Name       : storage_test.go
 * File Path       : infra/storage_test.go
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
 *   Test sync functionality
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
 *   [Test Functions] Test suite: TestSyncLocalFile, TestSyncInvalidJSON
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
    "testing"
)

/*
Test: create local file and sync

*/
func TestSyncLocalFile(t *testing.T) {

    file := "data/buffer/test.json"

    content := `{"token":"BTT","price":1.2}`

    _ = SaveLocal(file, content)

    RunSyncCycle()

    // file should either exist or be removed (no crash)
}

/*
Test: invalid json

*/
func TestSyncInvalidJSON(t *testing.T) {

    file := "data/buffer/bad.json"

    _ = os.WriteFile(file, []byte("invalid"), 0644)

    RunSyncCycle()
}