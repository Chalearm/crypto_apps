/*
Filename: infra/disk.go

Author: M365 Copilot (GPT-5)
Owner: Chalearm Saelim
Version: v2.0
Date: 2026-06-23 07:34 ICT (UTC+7)

Description:
Disk monitoring module.

Features:
✅ detect disk usage
✅ prevent overflow
✅ fallback safe

UPDATED:
- real disk usage calculation

NEW:
- GetFreeDiskPercent implemented

Usage:
infra.GetFreeDiskPercent()
*/

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