/******************************************************************************
 * File Name       : storage.go
 * File Path       : infra/storage.go
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:31 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:31 (UTC+7)
 * Description     : Local buffer storage with disk protection guards.
 ******************************************************************************/
package infra

import (
    "os"
)

/******************************************************************************
 * Function Name : SaveLocal
 * Purpose       : Writes buffered data to a local file if free disk space >= 5%.
 ******************************************************************************/
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