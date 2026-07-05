/******************************************************************************
 * File Name       : help.go
 * File Path       : apps/governance/help.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:43 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:43 (UTC+7)
 *
 * Description     :
 *   Dexbot component — auto-documented per rule1.txt.
 *
 * Responsibilities:
 *   - - Implement core functionality for apps package.
 *
 * Usage :
 *   Directory : apps/governance/
 *
 *   Build :
 *     go build ./apps/governance
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./apps/governance
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/apps
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
 *   1.0.0   | 2026-07-01 19:25:43 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package main

import (
	"dexbot/config"
	"strings"
)

func handleHelpCommand(args map[string]string) (string, error) {
	lines := []string{
		"",
		"DEXBOT PLATFORM — Self-Managing Quantitative Trading Ecosystem",
		"",
		"OVERVIEW: 4 daemons (Governance, School, Trading, Test).",
		"  Governance — operational control, health monitoring, CLI, dashboard.",
		"  School     — model evolution via Genetic Algorithms, market data.",
		"  Trading    — portfolio management via H-MAB + Thompson Sampling.",
		"  Test       — CI/CD pipeline, dependency analysis, automated testing.",
		"",
		"QUICK START (from dexbot/ directory):",
		"  go run ./apps/governance                    # Start governance",
		"  go run ./apps/school                        # Start school",
		"  go run ./apps/trading                       # Start trading",
		"  go run ./testdaemon                         # Start test daemon",
		"",
		"CLI ACTIONS (governance -action=<name>):",
		"  status              Show all daemon health as JSON",
		"  help                This help text",
		"  help-configuration       List all 54 config variables",
		"  help-configuration-vvv   Full config docs with examples",
		"  reload-log          Reload logger configuration at runtime",
		"  reload-config       Reload config.env + propagate to daemons",
		"  restart -daemon=X   Restart daemon (school/trading)",
		"  stop -daemon=X      Stop daemon",
		"  start -daemon=X     Start daemon",
		"  shutdown            Gracefully stop all daemons",
		"  balance             Show real BSC wallet balance (all tokens)",
		"  addToken -name=X -address=0x...   Add token to registry",
		"  addChain -name=X -id=N -url=URL   Add chain config",
		"",
		"CLI ACTIONS (school):",
		"  start               Run as daemon (default)",
		"  fetchMarket         Show last 5 real market data records",
		"  fetchDB             Show last 5 database records",
		"",
		"CLI ACTIONS (trading):",
		"  start               Run as daemon (default)",
		"",
		"CLI ACTIONS (testdaemon):",
		"  start               Run as daemon (poll loop)",
		"  history             Show stored test run history",
		"",
		"WEB DASHBOARD (http://localhost:8080):",
		"  /           Governance — daemon status tabs",
		"  /trading    Trading — balance + portfolio blocks",
		"  /school     School — 4-tier training + DB browser",
		"  /api/daemons  JSON daemon status",
		"",
		"INTERACTION METHODS:",
		"  1. CLI: go run ./apps/governance -action=status",
		"  2. CLI: go run ./apps/governance -action=balance",
		"  3. Web: http://localhost:8080/",
		"  4. API: curl http://localhost:8080/api/daemons",
		"  5. API: curl -X POST http://localhost:8080/api/daemon/school/restart",
		"",
		"CONFIGURATION: All settings in config.env (67 variables).",
		"  Run -action=help-configuration to see all variables.",
		"  Run -action=help-configuration-vvv for full docs with examples.",
		"",
	}
	return strings.Join(lines, "\n"), nil
}

func handleHelpConfigCommand(args map[string]string) (string, error) {
	return config.Describe(), nil
}

func handleHelpConfigVVVCommand(args map[string]string) (string, error) {
	return config.DescribeVerbose(), nil
}
