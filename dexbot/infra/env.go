/******************************************************************************
 * File Name       : env.go
 * File Path       : infra/env.go
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:29 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:29 (UTC+7)
 * Description     : Loads environment configuration files into OS environment.
 ******************************************************************************/
package infra

import (
    "bufio"
    "os"
    "strings"
)

/******************************************************************************
 * Function Name : LoadEnv
 * Purpose       : Loads key-value pairs from a config file into OS environment.
 ******************************************************************************/
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

        // Skip empty lines or comments
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        parts := strings.SplitN(line, "=", 2)
        if len(parts) != 2 {
            continue
        }

        key := strings.TrimSpace(parts[0])
        val := strings.TrimSpace(parts[1])

        // Only set if not already present — allows docker-compose env overrides
        if os.Getenv(key) == "" {
            _ = os.Setenv(key, val)
        }
    }

    Info("config.env loaded")
}