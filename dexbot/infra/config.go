/******************************************************************************
 * File Name       : config.go
 * File Path       : infra/config.go
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
 *   Provides utilities for loading and accessing application configuration from environment variables. Import and use `infra.LoadConfig()` to get the application configuration. Updated Part: - Initial cre
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
 *   [Types] Struct definitions in this file
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
    "fmt"
    "os"
    "strconv"
)

// AppConfig holds the application-wide configuration extracted from environment variables.
// Purpose: Stores configuration values like UDP ports for inter-daemon communication.
// Input: None
// Output: AppConfig struct instance
// Lines: ~15
type AppConfig struct {
    GovernanceUDPPort int
    SchoolUDPPort     int
    TradingUDPPort    int
    GovernanceWebPort int
}

// LoadConfig reads configuration from environment variables and returns an AppConfig struct.
// Purpose: Centralized function to load all necessary configuration for the daemons.
// Input:
//   None
// Output:
//   *AppConfig: A pointer to the loaded configuration, or nil if an error occurs.
//   error: An error if any configuration value cannot be parsed.
// Lines: ~30
func LoadConfig() (*AppConfig, error) {
    cfg := &AppConfig{}

    var err error

    // Load Governance UDP Port
    cfg.GovernanceUDPPort, err = strconv.Atoi(os.Getenv("UDP_GOVERNANCE_PORT"))
    if err != nil {
        return nil, fmt.Errorf("invalid UDP_GOVERNANCE_PORT: %w", err)
    }

    // Load School UDP Port
    cfg.SchoolUDPPort, err = strconv.Atoi(os.Getenv("UDP_SCHOOL_PORT"))
    if err != nil {
        return nil, fmt.Errorf("invalid UDP_SCHOOL_PORT: %w", err)
    }

    // Load Trading UDP Port
    cfg.TradingUDPPort, err = strconv.Atoi(os.Getenv("UDP_TRADING_PORT"))
    if err != nil {
        return nil, fmt.Errorf("invalid UDP_TRADING_PORT: %w", err)
    }

    // Load Governance Web Port
    cfg.GovernanceWebPort, err = strconv.Atoi(os.Getenv("GOVERNANCE_WEB_PORT"))
    if err != nil {
        return nil, fmt.Errorf("invalid GOVERNANCE_WEB_PORT: %w", err)
    }

    return cfg, nil
}
