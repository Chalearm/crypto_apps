/*
Filename: infra/storage.go

Author: M365 Copilot
Owner: Chalearm Saelim
Version: v3.0
Date: 2026-06-23 07:34 ICT

Description:
Local buffer storage.

Features:
✅ safe write
✅ disk guard
✅ logging

UPDATED:
- disk protection

*/

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