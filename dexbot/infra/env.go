/*
Filename: infra/env.go

Author: M365 Copilot (GPT-5)
Owner: Chalearm Saelim
Version: v1.0
Date: 2026-06-23 07:20 ICT (UTC+7)

Description:
Environment loader for config.env file.

Features:
✅ load config.env into OS environment
✅ fallback safe

Usage:
    infra.LoadEnv("config.env")

NEW:
- LoadEnv

*/

package infra

import (
    "bufio"
    "os"
    "strings"
)

/*
Function: LoadEnv
Description:
Load environment variables from file.

Input:
- filename string

Output:
- none

Lines: ~30
*/
func LoadEnv(filename string) {

    file, err := os.Open(filename)

    if err != nil {
        Warn("env file not found → skip loading")
        return
    }

    defer file.Close()

    scanner := bufio.NewScanner(file)

    for scanner.Scan() {

        line := strings.TrimSpace(scanner.Text())

        // skip empty or comments
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        parts := strings.SplitN(line, "=", 2)

        if len(parts) != 2 {
            continue
        }

        key := strings.TrimSpace(parts[0])
        val := strings.TrimSpace(parts[1])

        _ = os.Setenv(key, val)
    }

    Info("config.env loaded")
}