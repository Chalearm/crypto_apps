/*
Filename: infra/logger.go

Author: M365 Copilot (GPT-5)
Owner: Chalearm Saelim
Version: v3.0
Date: 2026-06-23 07:03 ICT (UTC+7)

Description:
Central logging system with file + console output.

Features:
✅ INFO / WARN / ERROR levels
✅ auto create logs directory
✅ print log file location at startup
✅ safe for daemon & test mode

Usage:
    infra.InitLogger("INFO")

UPDATED:
- auto-create logs directory
- improved error handling

NEW:
- ensureLogDir()

*/

package infra

import (
    "fmt"
    "os"
    "path/filepath"
    "time"
)

var currentLevel = "INFO"
var logFilePath = "logs/system.log"

/*
Function: InitLogger
Description:
Initialize logger and ensure log directory exists.

Input:
- level string (INFO | WARN | ERROR)

Output:
- none

Lines: ~15
*/
func InitLogger(level string) {

    currentLevel = level

    ensureLogDir()

    fmt.Println("✅ Logger initialized →", logFilePath)
}

/*
Function: ensureLogDir
Description:
Create logs directory if not exists.

Input:
- none

Output:
- none

Lines: ~10
*/
func ensureLogDir() {

    dir := filepath.Dir(logFilePath)

    if _, err := os.Stat(dir); os.IsNotExist(err) {
        _ = os.MkdirAll(dir, 0755)
    }
}

/*
Function: writeLog
Description:
Write log to console and file.

Input:
- level string
- msg string

Output:
- none

Lines: ~25
*/
func writeLog(level string, msg string) {

    if !shouldLog(level) {
        return
    }

    line := fmt.Sprintf("[%s][%s] %s\n",
        time.Now().Format(time.RFC3339),
        level,
        msg,
    )

    fmt.Print(line)

    f, err := os.OpenFile(logFilePath,
        os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

    if err != nil {
        fmt.Println("⚠️ log file error:", err)
        return
    }

    defer f.Close()
    _, _ = f.WriteString(line)
}

func shouldLog(level string) bool {
    levels := map[string]int{
        "INFO":  1,
        "WARN":  2,
        "ERROR": 3,
    }
    return levels[level] >= levels[currentLevel]
}

func Info(msg string)  { writeLog("INFO", msg) }
func Warn(msg string)  { writeLog("WARN", msg) }
func Error(msg string) { writeLog("ERROR", msg) }