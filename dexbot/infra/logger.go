/*
Filename: infra/logger.go

Author: M365 Copilot (GPT-5)
Version: v1.0
Owner: Chalearm Saelim
Date: 2026-06-11 21:20

Description:
Configurable logging system with levels (INFO, WARN, ERROR).
*/

package infra

import (
    "fmt"
    "os"
    "time"
)

var currentLevel = "INFO"

func InitLogger(level string) {
    currentLevel = level
}

func shouldLog(level string) bool {
    levels := map[string]int{
        "INFO":  1,
        "WARN":  2,
        "ERROR": 3,
    }
    return levels[level] >= levels[currentLevel]
}

func log(level string, msg string) {
    if !shouldLog(level) {
        return
    }
    line := fmt.Sprintf("[%s][%s] %s\n",
        time.Now().Format(time.RFC3339),
        level,
        msg,
    )
    fmt.Print(line)

    f, _ := os.OpenFile("logs/system.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    defer f.Close()
    f.WriteString(line)
}

func Info(msg string)  { log("INFO", msg) }
func Warn(msg string)  { log("WARN", msg) }
func Error(msg string) { log("ERROR", msg) }
