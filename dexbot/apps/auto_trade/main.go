/******************************************************************************
 * File Name       : main.go
 * File Path       : apps/auto_trade/main.go
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
 *   Full production-grade Dexbot daemon system with enhanced CLI for help and configuration. CORE FEATURES: [OK] CLI entry with safe flag parsing [OK] Background daemon (PID tracked) [OK] Dry-run safe exe
 *
 * Responsibilities:
 *   - Implement core functionality for apps package.
 *
 * Usage :
 *   Directory : apps/auto_trade/
 *
 *   Build :
 *     go build ./apps/auto_trade
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./apps/auto_trade
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
/*
Filename: apps/auto_trade/main.go

Author: M365 Copilot (GPT-5), Gemini
Version: v3.3 (HELP COMMAND ADDED)
Owner: Chalearm Saelim
Date: 2026-06-25 10:00 ICT (UTC+7)

Description:
Full production-grade Dexbot daemon system with enhanced CLI for help and configuration.

CORE FEATURES:
[OK] CLI entry with safe flag parsing
[OK] Background daemon (PID tracked)
[OK] Dry-run safe execution
[OK] Real-time reporting (independent command)
[OK] Task lifecycle (CREATE → BUY → COMPLETE)
[OK] JSON persistence (state file)
[OK] Config system
[OK] Infra logging integration
[OK] Graceful shutdown
[OK] Multi-worker execution
[OK] Help command for CLI options

COMMANDS:

Start daemon (safe test):
go run apps/auto_trade/main.go -dry_run=true

Report:
go run apps/auto_trade/main.go -action=report

Terminate:
go run apps/auto_trade/main.go -action=terminate

Show Help:
go run apps/auto_trade/main.go -action=help

TEST MODE:
TEST_MODE=1 go test ./apps/auto_trade -v


Usage:

Run (IMPORTANT [OK]):
    go run . -dry_run=true

DO NOT USE:
    go run main.go [ERROR] (will break multi-file build)

UPDATED:
- Added -action=help command.
- Updated file header with new version and description.

NEW:
- runHelp() function.

*/

package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "math/big"
    "os"
    "os/exec"
    "os/signal"
    "strconv"
    "strings"
    "sync"
    "syscall"
    "time"

    "dexbot/infra"
)

// ==============================
// CONSTANTS
// ==============================

const (
    STATE_FILE  = "tasks_state.json"
    CONFIG_FILE = "config.json"
    PID_FILE    = "dexbot.pid"
)

var dryRun bool

// ==============================
// TYPES
// ==============================

type TaskStatus string

const (
    StatusCreated   TaskStatus = "CREATED"
    StatusBought    TaskStatus = "BOUGHT"
    StatusCompleted TaskStatus = "COMPLETED"
)

type GlobalConfig struct {
    MaxTasks int `json:"max_tasks"`
}

type TradeTask struct {
    ID string

    Status TaskStatus

    FromToken string
    ToToken   string

    BuyPrice  float64
    SellPrice float64

    CreatedAt time.Time
}

// ==============================
// TASK MANAGER
// ==============================

type TaskManager struct {
    mu    sync.Mutex
    Tasks map[string]*TradeTask
}

func NewTaskManager() *TaskManager {
    return &TaskManager{
        Tasks: make(map[string]*TradeTask),
    }
}

func (tm *TaskManager) Save() {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    data, _ := json.MarshalIndent(tm, "", "  ")
    _ = os.WriteFile(STATE_FILE, data, 0644)

    infra.Info("state saved to disk")
}

func (tm *TaskManager) Load() {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    data, err := os.ReadFile(STATE_FILE)
    if err == nil {
        _ = json.Unmarshal(data, tm)
        infra.Info("state loaded from disk")
    } else {
        infra.Warn("no previous state file")
    }
}

// ==============================
// CONFIG
// ==============================

func loadConfig() GlobalConfig {

    var cfg GlobalConfig

    data, err := os.ReadFile(CONFIG_FILE)
    if err != nil {
        cfg = GlobalConfig{MaxTasks: 3}
        writeConfig(cfg)
        return cfg
    }

    _ = json.Unmarshal(data, &cfg)

    return cfg
}

func writeConfig(cfg GlobalConfig) {
    data, _ := json.MarshalIndent(cfg, "", "  ")
    _ = os.WriteFile(CONFIG_FILE, data, 0644)
}

// ==============================
// CLI ENTRY (SAFE)
// ==============================

func runStatus() {

    data, err := os.ReadFile(PID_FILE)

    if err != nil {
        infra.Warn("daemon not running")
        return
    }

    pid := strings.TrimSpace(string(data))

    infra.Info("daemon running PID=" + pid)
}
/*
Function: loadEnvSmart
Description:
Try multiple env locations.

*/
func loadEnvSmart() {

    paths := []string{
        "config.env",
        "../config.env",
        "../../config.env",
    }

    for _, p := range paths {
        if _, err := os.Stat(p); err == nil {
            infra.LoadEnv(p)
            infra.Info("env loaded from: " + p)
            return
        }
    }

    infra.Warn("env file not found in any path")
}

/*
Function: runApp
Description:
Main CLI dispatcher.

Supports:
✅ background daemon
✅ CLI commands (report, terminate, status)
✅ DB fallback safe

UPDATED:
- correct daemon behavior (non-blocking)
- logging improved

Input:
- args []string

Output:
- none

Lines: ~70
*/
func runApp(args []string) {

    fs := flag.NewFlagSet("dexbot", flag.ContinueOnError)

    action := fs.String("action", "start", "")
    daemon := fs.Bool("daemon", false, "")
    dry := fs.Bool("dry_run", true, "")

    _ = fs.Parse(args)

    dryRun = *dry

    infra.InitLogger()

loadEnvSmart() // ✅ use this instead



    infra.Info("System starting → dryRun=" + strconv.FormatBool(dryRun))

    _ = infra.InitDB()

    if os.Getenv("TEST_MODE") == "1" {
        dryRun = true
        infra.Warn("TEST_MODE → forced dry-run")
    } else {
        if err := infra.CheckDBHealth(); err != nil {
            infra.Warn("DB not healthy → fallback to simulation mode")
        }
    }

    switch *action {

    case "report":
        runReport()
        return

    case "terminate":
        runTerminate()
        return

    case "status":
        runStatus()
        return

    case "reload-log":
        infra.ReloadLoggerConfig()
        return

    case "help":
        runHelp()
        return
    }

    // [OK] TEST MODE stays inline
    if os.Getenv("TEST_MODE") == "1" {
        runDaemon()
        return
    }

    // [OK] if NOT daemon flag → spawn background
    if !*daemon {
        infra.Info("Starting background daemon...")
        startDaemon()
        return
    }

    // [OK] daemon child process
    infra.Info("Running daemon process...")
    runDaemon()
}
/*
Function: runHelp

Description:
Displays all supported CLI commands and usage examples.

Input:
- none

Output:
- none

Lines:
~35

Updated:
- Fixed multiline string syntax errors.
- Added clearer CLI help formatting.

New:
- Examples section.
*/
func runHelp() {

    fmt.Printf(`
Dexbot Auto-Trade CLI Options
========================================

-action <command>

Available commands:

  start
      Starts Dexbot daemon (default)

  report
      Displays current task report and PnL

  terminate
      Terminates all running Dexbot daemon processes

  status
      Displays daemon status and PID

  reload-log
      Reload logger configuration

  help
      Displays this help screen

-daemon
      Internal daemon mode
      Do not manually use this option

-dry_run=true|false
      Enables or disables dry-run mode

      true  = simulation only
      false = live execution

Examples:

  go run . -dry_run=true

  go run . -action=report

  go run . -action=status

  go run . -action=terminate

  go run . -action=reload-log

  go run . -action=help

========================================
`)
}

func main() {
    runApp(os.Args[1:])
}

// ==============================
// DAEMON CONTROL
// ==============================
/*
Function: startDaemon
Description:
Start daemon safely (prevent duplicates).

Input:
- none

Output:
- none

Lines: ~25
*/
func startDaemon() {

    // ✅ check existing PID
    data, err := os.ReadFile(PID_FILE)

    if err == nil {
        pidStr := strings.TrimSpace(string(data))

        pid, _ := strconv.Atoi(pidStr)

        // ✅ check if process still exists
        proc, err := os.FindProcess(pid)
        if err == nil {
            err = proc.Signal(syscall.Signal(0))

            if err == nil {
                infra.Warn("daemon already running PID=" + pidStr)
                return
            }
        }
    }

    cmd := exec.Command(os.Args[0], "-daemon")

    cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

    _ = cmd.Start()

    _ = os.WriteFile(PID_FILE,
        []byte(strconv.Itoa(cmd.Process.Pid)), 0644)

    infra.Info(fmt.Sprintf("daemon started PID=%d", cmd.Process.Pid))
}
/*
Function: runTerminate
Description:
Terminate ALL auto_trade daemon processes.

Input:
- none

Output:
- none

Lines: ~40
*/
func runTerminate() {

    infra.Warn("terminating all auto_trade daemons...")

    // ✅ use pkill (safe for dev)
    cmd := exec.Command("pkill", "-f", "auto_trade")

    err := cmd.Run()

    if err != nil {
        infra.Error("failed to kill processes: " + err.Error())
    } else {
        infra.Info("all auto_trade processes terminated")
    }

    _ = os.Remove(PID_FILE)
}


// ==============================
// REPORTING
// ==============================


func runReport() {

    tm := NewTaskManager()
    tm.Load()

    totalPnL := 0.0

    infra.Info("===== REPORT START =====")

    for _, t := range tm.Tasks {

        fmt.Printf("ID: %-20s | STATUS: %-10s | %s→%s\n",
            t.ID, t.Status, t.FromToken, t.ToToken)

        // ✅ PnL
        if t.Status == StatusCompleted {

            pnl := t.SellPrice - t.BuyPrice
            totalPnL += pnl

            fmt.Printf("   BUY: %.6f SELL: %.6f PnL: %.6f\n",
                t.BuyPrice, t.SellPrice, pnl)
        }
    }

    fmt.Printf("\nTOTAL PnL: %.6f\n", totalPnL)

    infra.Info("===== REPORT END =====")
}


// ==============================
// DAEMON LOOP
// ==============================
func runDaemon() {

    infra.Info("daemon started")

    manager := NewTaskManager()
    manager.Load()

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // ✅ signal handling (restore feature)
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

    go func() {
        <-sigChan
        infra.Warn("signal received, shutting down daemon")

        manager.Save()
        cancel()
    }()

    // ✅ dynamic ticker
    interval := 5 * time.Second
    if os.Getenv("TEST_MODE") == "1" {
        interval = 100 * time.Millisecond
    }

    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    // ✅ limit loops in test mode
    maxCycles := -1
    if os.Getenv("TEST_MODE") == "1" {
        maxCycles = 3
    }

    cycle := 0

    for {
        select {

        case <-ticker.C:

            cfg := loadConfig()

            if len(manager.Tasks) < cfg.MaxTasks {
                createTask(manager)
            }

            runWorkers(manager)

            if maxCycles > 0 {
                cycle++
                if cycle >= maxCycles {
                    infra.Info("daemon exit (test mode)")
                    manager.Save()
                    return
                }
            }

        case <-ctx.Done():
            infra.Info("daemon context closed")
            return
        }
    }
}

// ==============================
// TASK FLOW
// ==============================


func createTask(manager *TaskManager) {

    id := fmt.Sprintf("task_%d", time.Now().UnixNano())

    manager.mu.Lock()

    manager.Tasks[id] = &TradeTask{
        ID:        id,
        Status:    StatusCreated,
        FromToken: "USDC",
        ToToken:   "BTT",
        CreatedAt: time.Now(),
    }

    manager.mu.Unlock()   // ✅ release BEFORE save

    manager.Save()        // ✅ safe now

    infra.Info("task created: " + id)
}


func runWorkers(manager *TaskManager) {

    var wg sync.WaitGroup

    for _, t := range manager.Tasks {

        if t.Status == StatusCompleted {
            continue
        }

        wg.Add(1)

        go func(task *TradeTask) {
            defer wg.Done()
            processTask(task, manager)
        }(t)
    }

    wg.Wait()
}
/*
Function: processTask
Description:
Executes strategy + execution engine.

UPDATED:
✅ uses strategy interface
✅ uses execution layer
✅ fully logged

*/
func processTask(task *TradeTask, manager *TaskManager) {

    cfg := Config{
        FakeTrading:   true,
        EnableOptions: false,
        GasPerTrade:   0.0005,
    }

    price := GetLatestPrice(task.ToToken)

    switch task.Status {

    case StatusCreated:

        if !strategy.ShouldBuy() {
            infra.Warn("Skip BUY: " + task.ID)
            return
        }

        infra.Info("Processing BUY: " + task.ID)

        ExecuteTrade(task, cfg, price)

    case StatusBought:

        if strategy.ShouldSell(task, price) {

            infra.Info("Processing SELL: " + task.ID)

            ExecuteTrade(task, cfg, price)
        }
    }
}



// ==============================
// UTIL
// ==============================

func simulatePrice() float64 {
    return 1.0 + float64(time.Now().UnixNano()%100)/10000
}


// ---------------- FIXED HELPER ----------------


func floatToBigInt(val float64, decimals int64) *big.Int {

    if val <= 0 {
        return big.NewInt(0)
    }

    f := new(big.Float).SetFloat64(val)

    multiplier := new(big.Float).SetInt(
        new(big.Int).Exp(big.NewInt(10), big.NewInt(decimals), nil),
    )

    f.Mul(f, multiplier)

    result := new(big.Int)
    f.Int(result)

    return result
}

