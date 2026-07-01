/******************************************************************************
 * File Name       : help.go
 * File Path       : config/help.go
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
 *   Configuration help text generators. Provides Describe() for concise variable listing and DescribeVerbose() for full documentation with examples. Used by -action=help-configuration and -action=help-con
 *
 * Responsibilities:
 *   - Implement core functionality for config package.
 *
 * Usage :
 *   Directory : config/
 *
 *   Build :
 *     go build ./config
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./config
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/config
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
package config

import "strings"

func Describe() string {
	return "═══ Dexbot Configuration Variables (54 keys) ═══\n\n" +
		"── Database ──\n" +
		"  DB_HOST     PostgreSQL hostname (default: 127.0.0.1)\n" +
		"  DB_PORT     PostgreSQL port (default: 5432)\n" +
		"  DB_USER     PostgreSQL user (default: trader)\n" +
		"  DB_PASS     PostgreSQL password (default: secret)\n" +
		"  DB_NAME     PostgreSQL database name (default: traderdb)\n\n" +
		"── Logging ──\n" +
		"  LOG_LEVEL            TRACE|INFO|WARN|ERROR|OFF\n" +
		"  LOG_OUTPUT           terminal|file|both\n" +
		"  LOG_FILE_PATH        Log file path (default: logs/system.log)\n" +
		"  LOG_CALLER_FORMAT    short|full|off\n" +
		"  LOG_FORMAT           text|json\n" +
		"  FN_TRACE             on|off — per-function trace logging\n\n" +
		"── Governance ──\n" +
		"  HEALTH_CHECK_INTERVAL_SECONDS  Health probe interval (default: 30s)\n" +
		"  RECREATE_THRESHOLD_SECONDS     Unhealthy before recreate (default: 60s)\n" +
		"  UDP_GOVERNANCE_PORT  Governance UDP listener (default: 8081)\n" +
		"  UDP_SCHOOL_PORT      School UDP port (default: 8082)\n" +
		"  UDP_TRADING_PORT     Trading UDP port (default: 8083)\n" +
		"  GOVERNANCE_WEB_PORT  Dashboard HTTP port (default: 8080)\n\n" +
		"── School ──\n" +
		"  MARKET_DATA_RECORD_INTERVAL_MINUTES  Market data interval (default: 15m)\n" +
		"  DB_HEALTH_CHECK_INTERVAL_SECONDS     DB check interval (default: 30s)\n" +
		"  GA_POPULATION_SIZE          Models in population (default: 50)\n" +
		"  GA_TOP_SURVIVORS            Selected for reproduction (default: 10)\n" +
		"  GA_MUTATION_RATE            Mutation probability 0-1 (default: 0.15)\n" +
		"  GA_CROSSOVER_RATE           Crossover probability 0-1 (default: 0.60)\n" +
		"  GA_GENERATIONS_PER_CYCLE    Generations per cycle (default: 3)\n" +
		"  GA_GRADUATE_TOP_N           Promoted to graduate (default: 3)\n" +
		"  GA_RETIRE_BOTTOM_N          Retired per cycle (default: 5)\n" +
		"  GA_CYCLE_INTERVAL_MINUTES   Time between GA cycles (default: 30m)\n" +
		"  MODEL_GRADUATION_THRESHOLD  Min fitness to graduate (default: 0.60)\n" +
		"  MODEL_RETIREMENT_THRESHOLD  Max fitness before retire (default: 0.30)\n" +
		"  SCHOOL_REMOTE_ADDRESSES     Comma-separated ip:port for remotes\n" +
		"  SCHOOL_REMOTE_TIMEOUT_SECONDS  Remote UDP timeout (default: 30s)\n" +
		"  SCHOOL_REMOTE_STUDENTS_PER_NODE Models/remote (default: 5)\n\n" +
		"── Trading ──\n" +
		"  MAX_TRADING_TASKS        Max concurrent tasks (default: 10)\n" +
		"  TOTAL_CAPITAL            Total capital USD (default: 10000)\n" +
		"  TRADING_CYCLE_INTERVAL_SECONDS  Main loop (default: 5s)\n" +
		"  MAX_PAPER_AGENTS         Virtual paper agents (default: 15)\n" +
		"  MAX_REAL_AGENTS          Real-capital agents (default: 5)\n" +
		"  AGENT_MIN_CAPITAL        Min capital/agent USD (default: 100)\n" +
		"  AGENT_MAX_CAPITAL        Max capital/agent USD (default: 5000)\n" +
		"  AGENT_CREATION_INTERVAL_MINUTES  Agent creation (default: 60m)\n" +
		"  AGENT_RETIREMENT_THRESHOLD_SCORE Min sharpe to retire (default: 0.25)\n" +
		"  MAB_EXPLORATION_RATE     Bandit exploration 0-1 (default: 0.10)\n" +
		"  MAB_UPDATE_INTERVAL_SECONDS  Evidence update (default: 30s)\n" +
		"  STOP_LOSS_PERCENT        Max loss before exit (default: 5.0%)\n" +
		"  MAX_LEVERAGE             Max leverage ratio (default: 2.0x)\n" +
		"  MIN_PAPER_TRADES         Trades before real eligible (default: 100)\n" +
		"  AGENT_PERFORMANCE_THRESHOLD  Sharpe score for real (default: 0.40)\n" +
		"  MODEL_CONFIDENCE_THRESHOLD   Model prob for real (default: 0.55)\n\n" +
		"── Test Daemon ──\n" +
		"  TEST_POLL_INTERVAL_SECONDS  Poll interval (default: 10s)\n" +
		"  TEST_COVERAGE_THRESHOLD     Min coverage % (default: 70.0)\n" +
		"  TEST_BUILD_TIMEOUT_SECONDS  Build timeout (default: 120s)\n\n" +
		"── Web Dashboard ──\n" +
		"  WEB_OUTPUT_DIR       Static file output dir (default: web_output)\n" +
		"  WEB_REFRESH_SECONDS  Dashboard refresh (default: 10s)\n" +
		"  WEB_ACTION_PORT      TCP action port (default: 8085)\n\n" +
		"── Deployment ──\n" +
		"  SINGLE_CONTAINER_MODE  true|false — all in one container\n" +
		"  PAPER_TRADING_ONLY     true|false — never use real capital\n"
}

func DescribeVerbose() string {
	return strings.Join([]string{
		"",
		"╔══════════════════════════════════════════════════════════════════╗",
		"║        Dexbot Configuration — Full Reference (verbose)           ║",
		"╚══════════════════════════════════════════════════════════════════╝",
		"",
		"┌── DATABASE ──────────────────────────────────────────────────────┐",
		"│ DB_HOST                                                          │",
		"│   Type: string. Default: 127.0.0.1.                              │",
		"│   PostgreSQL hostname. In Docker, use service name 'db'.         │",
		"│   Example: DB_HOST=db                                            │",
		"│                                                                  │",
		"│ DB_PORT   Type: int (1-65535). Default: 5432.                    │",
		"│ DB_USER / DB_PASS / DB_NAME — PostgreSQL credentials.            │",
		"│   Example: DB_USER=trader DB_PASS=secret DB_NAME=traderdb        │",
		"└──────────────────────────────────────────────────────────────────┘",
		"",
		"┌── LOGGING ───────────────────────────────────────────────────────┐",
		"│ LOG_LEVEL                                                         │",
		"│   Values: TRACE, INFO, WARN, ERROR, OFF. Default: INFO.           │",
		"│   TRACE shows all messages including FnTrace function entries.    │",
		"│   OFF suppresses all output.                                      │",
		"│   Example: LOG_LEVEL=TRACE FN_TRACE=on                            │",
		"│                                                                   │",
		"│ LOG_FORMAT                                                        │",
		"│   text — traditional [timestamp][LEVEL] format (default)          │",
		"│   json — structured JSON lines with ts/level/caller/fn/msg        │",
		"│   Example: LOG_FORMAT=json                                        │",
		"│                                                                   │",
		"│ FN_TRACE                                                          │",
		"│   When 'on', every major function logs entry/exit with caller     │",
		"│   name. Requires LOG_LEVEL=TRACE to be visible in output.         │",
		"│   Example: FN_TRACE=on LOG_LEVEL=TRACE                            │",
		"└──────────────────────────────────────────────────────────────────┘",
		"",
		"┌── GOVERNANCE ────────────────────────────────────────────────────┐",
		"│ HEALTH_CHECK_INTERVAL_SECONDS=30                                   │",
		"│   How often governance probes daemons via UDP. Range: 1-3600.     │",
		"│                                                                   │",
		"│ RECREATE_THRESHOLD_SECONDS=60                                     │",
		"│   How long a daemon must be unhealthy before governance           │",
		"│   attempts to recreate it. Range: 1-600.                          │",
		"│                                                                   │",
		"│ UDP_GOVERNANCE_PORT=8081 / SCHOOL_PORT=8082 / TRADING_PORT=8083   │",
		"│   UDP ports for inter-daemon communication. Must match across      │",
		"│   all daemon configurations. In multi-container mode these are    │",
		"│   service ports exposed by docker-compose.                        │",
		"└──────────────────────────────────────────────────────────────────┘",
		"",
		"┌── SCHOOL (Genetic Algorithm + Model Evolution) ───────────────────┐",
		"│ GA_POPULATION_SIZE=50                                              │",
		"│   Total models in the GA population. Larger = more diversity      │",
		"│   but slower cycles. Range: 10-1000.                              │",
		"│                                                                   │",
		"│ GA_TOP_SURVIVORS=10                                                │",
		"│   Top N models selected by tournament for reproduction.           │",
		"│   Range: 1 to GA_POPULATION_SIZE.                                 │",
		"│                                                                   │",
		"│ GA_MUTATION_RATE=0.15                                              │",
		"│   Probability a gene (hyperparameter/weight) is randomly          │",
		"│   perturbed during reproduction. Higher = more exploration.       │",
		"│   Range: 0.0-1.0.                                                 │",
		"│                                                                   │",
		"│ GA_CROSSOVER_RATE=0.60                                             │",
		"│   Probability offspring inherits from both parents vs clone.      │",
		"│   Range: 0.0-1.0. Higher = more diversity.                        │",
		"│                                                                   │",
		"│ GA_GRADUATE_TOP_N=3                                                │",
		"│   Top N models promoted to graduate status per cycle.             │",
		"│   Graduates are sent to Trading daemon via UDP.                   │",
		"│   Range: 1-20.                                                    │",
		"│                                                                   │",
		"│ MODEL_GRADUATION_THRESHOLD=0.60                                    │",
		"│   Minimum composite fitness score (0-1) required to graduate.     │",
		"│   Composite = weighted sum of Sharpe, Sortino, profit, drawdown,  │",
		"│   accuracy, consistency, efficiency.                              │",
		"│                                                                   │",
		"│ SCHOOL_REMOTE_ADDRESSES=                                           │",
		"│   Comma-separated ip:port for remote school sub-daemons.          │",
		"│   Leave empty to train all models locally on main school.         │",
		"│   Example: SCHOOL_REMOTE_ADDRESSES=10.0.1.5:9001,10.0.2.3:9001    │",
		"│                                                                   │",
		"│ SCHOOL_REMOTE_STUDENTS_PER_NODE=5                                  │",
		"│   How many models each remote node trains simultaneously.         │",
		"│   Range: 1-100.                                                   │",
		"└──────────────────────────────────────────────────────────────────┘",
		"",
		"┌── TRADING (Portfolio + MAB + Paper Gate) ─────────────────────────┐",
		"│ MAX_PAPER_AGENTS=15                                                │",
		"│   Virtual paper-trading agents always active. They execute        │",
		"│   simulated trades and accumulate paper PnL without real risk.    │",
		"│   Range: 1-10000.                                                 │",
		"│                                                                   │",
		"│ MAX_REAL_AGENTS=5                                                  │",
		"│   Maximum agents allowed to use real capital after graduating     │",
		"│   through the paper trading gate. Set 0 to disable real capital.  │",
		"│   Range: 0-1000.                                                  │",
		"│                                                                   │",
		"│ MIN_PAPER_TRADES=100                                               │",
		"│   Minimum number of simulated trades an agent must complete       │",
		"│   before it can be considered for real-capital graduation.        │",
		"│   Range: 1-10000.                                                 │",
		"│                                                                   │",
		"│ AGENT_PERFORMANCE_THRESHOLD=0.40                                   │",
		"│   Minimum normalized performance score (0.5 + Sharpe*0.2) needed  │",
		"│   for an agent to graduate from paper to real capital.            │",
		"│   Range: 0.0-1.0.                                                 │",
		"│                                                                   │",
		"│ MODEL_CONFIDENCE_THRESHOLD=0.55                                    │",
		"│   Minimum MAB model probability (Beta mean) needed for the        │",
		"│   assigned model to be considered reliable for real capital.      │",
		"│   Range: 0.0-1.0.                                                 │",
		"│                                                                   │",
		"│ PAPER_TRADING_ONLY=true                                            │",
		"│   When true, real capital is NEVER deployed even if agents pass   │",
		"│   all paper gate thresholds. Use for safe testing.                │",
		"│   Example: PAPER_TRADING_ONLY=false (enable real capital)         │",
		"│                                                                   │",
		"│ TOTAL_CAPITAL=10000.00                                             │",
		"│   Total trading capital pool in USD. Distributed among agents     │",
		"│   by the H-MAB capital allocator.                                 │",
		"│   Range: > 0.0.                                                   │",
		"│                                                                   │",
		"│ STOP_LOSS_PERCENT=5.0                                              │",
		"│   Maximum loss percentage before forced position exit.            │",
		"│   Range: 0.0-100.0. Example: STOP_LOSS_PERCENT=3.0                │",
		"│                                                                   │",
		"│ MAX_LEVERAGE=2.0                                                   │",
		"│   Maximum allowed leverage ratio. 1.0 = no leverage.              │",
		"│   Range: 1.0-100.0.                                               │",
		"└──────────────────────────────────────────────────────────────────┘",
		"",
		"┌── DEPLOYMENT ────────────────────────────────────────────────────┐",
		"│ SINGLE_CONTAINER_MODE=true                                         │",
		"│   true  = all daemons in one container (localhost UDP).           │",
		"│   false = daemons may be on different hosts/IPs.                  │",
		"│   When false, UDP addresses change from 127.0.0.1 to hostnames.  │",
		"│   Example: SINGLE_CONTAINER_MODE=false                            │",
		"└──────────────────────────────────────────────────────────────────┘",
		"",
		"Total: 54 configurable keys in config.env",
		"Location: /workspace/crypto_apps/dexbot/config.env",
		"",
	}, "\n")
}
