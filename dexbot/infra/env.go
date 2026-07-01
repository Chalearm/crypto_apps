/******************************************************************************
 * File Name       : env.go
 * File Path       : infra/env.go
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
 *   ✅ load config.env into OS environment ✅ fallback safe infra.LoadEnv("config.env") NEW: - LoadEnv
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

        // Only set if not already present — allows docker-compose env overrides
        if os.Getenv(key) == "" {
            _ = os.Setenv(key, val)
        }
    }

    Info("config.env loaded")
}