/******************************************************************************
 * File Name       : disk.go
 * File Path       : infra/disk.go
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
 *   Disk monitoring module. ✅ detect disk usage ✅ prevent overflow ✅ fallback safe UPDATED: - real disk usage calculation NEW: - GetFreeDiskPercent implemented
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
    "syscall"
)

/*
Function: GetFreeDiskPercent
Description:
Returns available disk percentage.

Input:
- none

Output:
- float64 (0–100)

Lines: ~20
*/
func GetFreeDiskPercent() float64 {

    var stat syscall.Statfs_t

    err := syscall.Statfs(".", &stat)
    if err != nil {
        Error("disk stat error")
        return 0
    }

    total := stat.Blocks * uint64(stat.Bsize)
    free := stat.Bavail * uint64(stat.Bsize)

    if total == 0 {
        return 0
    }

    return (float64(free) / float64(total)) * 100
}