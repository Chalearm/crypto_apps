/******************************************************************************
 * File Name       : main.go
 * File Path       : apps/governance/main.go
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
 *   Central control unit for the Dexbot system. Monitors daemon health via UDP heartbeats, manages daemon lifecycle (restarting if unhealthy), provides CLI command interface, and hosts a web dashboard. Us
 *
 * Responsibilities:
 *   - Implement core functionality for apps package.
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
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dexbot/governance"
	"dexbot/infra"
	"dexbot/webui"
)

// ==============================
// CONSTANTS
// ==============================

const (
	PID_FILE             = "governance.pid"
	HEALTH_CHECK_TIMEOUT = 500 * time.Millisecond
	UDP_GOVERNANCE_PORT  = 8081
	UDP_SCHOOL_PORT      = 8082
	UDP_TRADING_PORT     = 8083
	GOVERNANCE_WEB_PORT  = 8080
)

// ==============================
// GLOBAL VARIABLES
// ==============================

var (
	registry   *governance.Registry         // shared daemon registry (Phase 8)
	commander  *governance.DefaultCommander // CLI command dispatch (Phase 8)

	schoolUdpConn  *net.UDPConn
	tradingUdpConn *net.UDPConn

	recreateDaemonFunc func(string, string)
	sendUdpProbeFunc   func(*net.UDPConn, string, time.Duration) (string, error)

	governancePort int
	schoolPort     int
	tradingPort    int
	webPort        int
	governanceAddr string  // §66: configurable listen address (127.0.0.1 or 0.0.0.0)
	schoolAddrStr  string  // §66: configurable school daemon address
	tradingAddrStr string  // §66: configurable trading daemon address

	recreateThreshold = 1 * time.Minute

	publisher   *infra.Publisher     // file-based dashboard output (Phase 18)
	modelReg    *governance.ModelRegistry // centralized model registry (§33)
	tokenReg    *infra.TokenRegistry      // §83: dynamic token registry for balance panel
)

// ==============================
// CLI ENTRY POINT
// ==============================

/*
Function: main
Description:
  Dispatches CLI commands via Commander or starts the daemon.

Input:
  - none (uses os.Args)

Output:
  - none

Lines: ~25
*/
func main() {
	fs := flag.NewFlagSet("governance", flag.ContinueOnError)
	action := fs.String("action", "start", "Action: start, status, help, help-configuration, help-configuration-vvv, reload-log, reload-config, restart, stop, shutdown")
	daemon := fs.String("daemon", "", "Target daemon for restart/stop/start")
	_ = fs.Parse(os.Args[1:])

	// Build args for commander
	args := map[string]string{}
	if *daemon != "" {
		args["daemon"] = *daemon
	}

	if *action != "start" {
		infra.InitLogger()
		loadEnvSmart()
		initCommander()
		result, err := commander.Dispatch(*action, args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(result)
		return
	}

	startDaemon()
}

/*
Function: initCommander
Description:
  Registers all CLI command handlers with the commander.

Input:
  - none

Output:
  - none

Lines: ~15
*/
func initCommander() {
	commander = governance.NewCommander()
	commander.Register(governance.ActionStatus, handleStatusCommand)
	commander.Register(governance.ActionReloadLog, handleReloadLogCommand)
	commander.Register(governance.ActionReloadConfig, handleReloadConfigCommand)
	commander.Register(governance.ActionRestart, handleRestartCommand)
	commander.Register(governance.ActionStop, handleStopCommand)
	commander.Register(governance.ActionStart, handleStartCommand)
	commander.Register(governance.ActionShutdown, handleShutdownCommand)
	commander.Register(governance.ActionHelp, handleHelpCommand)
	commander.Register(governance.ActionHelpConfig, handleHelpConfigCommand)
	commander.Register(governance.ActionHelpConfigVVV, handleHelpConfigVVVCommand)
}

/*
Function: webuiActionCallback
Description:
  Callback from webui Server when POST /api/daemon/{name}/{action} is called.
  Executes the real daemon lifecycle action.

Input:
  - name   string : Daemon name
  - action string : restart, stop, start

Output:
  - none

Lines: ~20
*/
func webuiActionCallback(name, action string) {
	infra.Info(fmt.Sprintf("WebUI action: %s → %s", name, action))
	switch action {
	case "restart":
		if name == "school" && schoolUdpConn != nil {
			schoolUdpConn.Write([]byte("governance:command:restart"))
		}
		if name == "trading" && tradingUdpConn != nil {
			tradingUdpConn.Write([]byte("governance:command:restart"))
		}
		recreateDaemonFunc(name, fmt.Sprintf("/workspace/crypto_apps/dexbot/apps/%s/main.go", name))
	case "stop":
		if name == "school" && schoolUdpConn != nil {
			schoolUdpConn.Write([]byte("governance:command:stop"))
		}
		if name == "trading" && tradingUdpConn != nil {
			tradingUdpConn.Write([]byte("governance:command:stop"))
		}
	case "start":
		recreateDaemonFunc(name, fmt.Sprintf("/workspace/crypto_apps/dexbot/apps/%s/main.go", name))
	case "kill":
		// §86: Force-kill a daemon for self-test. Governance marks it as
		// "killing", sends kill signal, then the health loop detects the
		// dead daemon and recreates it with "building" → "recovering" → "healthy".
		if info := registry.GetStatus(name); info != nil {
			info.PostStatus("killing")
			registry.Register(info)
		}
		if name == "school" && schoolUdpConn != nil {
			schoolUdpConn.Write([]byte("governance:command:kill"))
		}
		if name == "trading" && tradingUdpConn != nil {
			tradingUdpConn.Write([]byte("governance:command:kill"))
		}
		infra.Info(fmt.Sprintf("Kill command sent to %s", name))
	}
}

// ==============================
// ACTION CALLBACK (Phase 18 — called by TCP action listener)
// ==============================

/*
Function: webuiActionCallback
Description:
  Executes daemon lifecycle actions. Called by the TCP action listener
  when the middle server forwards a POST request, or by CLI commands.
  Prints all daemon info from the registry as JSON.

Input:
  - args map[string]string: unused

Output:
  - string: JSON status output
  - error: nil

Lines: ~20
*/
func handleStatusCommand(args map[string]string) (string, error) {
	if registry == nil {
		// CLI mode: create a temporary registry
		registry = governance.NewRegistry()
	}
	names := registry.List()
	type statusOut struct {
		Daemons []*governance.DaemonInfo `json:"daemons"`
		Count   int                       `json:"count"`
	}
	out := statusOut{Count: len(names)}
	for _, n := range names {
		out.Daemons = append(out.Daemons, registry.GetStatus(n))
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

/*
Function: handleReloadLogCommand
Description:
  Reloads logger configuration at runtime.

Input:
  - args map[string]string: unused

Output:
  - string: confirmation message
  - error: nil

Lines: ~8
*/
func handleReloadLogCommand(args map[string]string) (string, error) {
	infra.ReloadLoggerConfig()
	return "Governance: logger configuration reloaded.", nil
}

/*
Function: handleReloadConfigCommand
Description:
  Reloads config.env and propagates to daemons via UDP.

Input:
  - args map[string]string: unused

Output:
  - string: confirmation message
  - error: nil

Lines: ~15
*/
func handleReloadConfigCommand(args map[string]string) (string, error) {
	loadEnvSmart()
	// Propagate to daemons via UDP
	if schoolUdpConn != nil {
		schoolUdpConn.Write([]byte("governance:config:reload"))
	}
	if tradingUdpConn != nil {
		tradingUdpConn.Write([]byte("governance:config:reload"))
	}
	return "Governance: configuration reloaded and propagated.", nil
}

/*
Function: handleRestartCommand
Description:
  Sends a restart signal to a specific daemon via UDP.

Input:
  - args map[string]string: must contain "daemon"

Output:
  - string: restart result
  - error: non-nil if daemon not specified or unknown

Lines: ~20
*/
func handleRestartCommand(args map[string]string) (string, error) {
	name, ok := args["daemon"]
	if !ok || name == "" {
		return "", fmt.Errorf("restart requires -daemon=<name>")
	}
	switch name {
	case "school":
		if schoolUdpConn != nil {
			schoolUdpConn.Write([]byte("governance:command:restart"))
		}
		recreateDaemonFunc("school", "/workspace/crypto_apps/dexbot/apps/school/main.go")
	case "trading":
		if tradingUdpConn != nil {
			tradingUdpConn.Write([]byte("governance:command:restart"))
		}
		recreateDaemonFunc("trading", "/workspace/crypto_apps/dexbot/apps/trading/main.go")
	default:
		return "", fmt.Errorf("unknown daemon: %s (valid: school, trading)", name)
	}
	return fmt.Sprintf("Restart signal sent to %s daemon.", name), nil
}

/*
Function: handleStopCommand
Description:
  Sends a stop signal to a specific daemon via UDP and updates registry.

Input:
  - args map[string]string: must contain "daemon"

Output:
  - string: stop result
  - error: non-nil if daemon not specified or unknown

Lines: ~25
*/
func handleStopCommand(args map[string]string) (string, error) {
	name, ok := args["daemon"]
	if !ok || name == "" {
		return "", fmt.Errorf("stop requires -daemon=<name>")
	}
	var conn *net.UDPConn
	switch name {
	case "school":
		conn = schoolUdpConn
	case "trading":
		conn = tradingUdpConn
	default:
		return "", fmt.Errorf("unknown daemon: %s (valid: school, trading)", name)
	}
	if conn != nil {
		conn.Write([]byte("governance:command:stop"))
	}
	if registry != nil {
		info := registry.GetStatus(name)
		if info != nil {
			info.Status = "stopping"
			registry.Register(info)
		}
	}
	return fmt.Sprintf("Stop signal sent to %s daemon.", name), nil
}

/*
Function: handleStartCommand
Description:
  Starts a specific daemon by spawning its process.

Input:
  - args map[string]string: must contain "daemon"

Output:
  - string: start result
  - error: non-nil if daemon not specified or unknown

Lines: ~25
*/
func handleStartCommand(args map[string]string) (string, error) {
	name, ok := args["daemon"]
	if !ok || name == "" {
		return "", fmt.Errorf("start requires -daemon=<name>")
	}
	switch name {
	case "school":
		recreateDaemonFunc("school", "/workspace/crypto_apps/dexbot/apps/school/main.go")
	case "trading":
		recreateDaemonFunc("trading", "/workspace/crypto_apps/dexbot/apps/trading/main.go")
	default:
		return "", fmt.Errorf("unknown daemon: %s (valid: school, trading)", name)
	}
	return fmt.Sprintf("Start signal sent to %s daemon.", name), nil
}

/*
Function: handleShutdownCommand
Description:
  Gracefully shuts down all daemons by sending stop signals,
  then terminates the governance daemon itself.

Input:
  - args map[string]string: unused

Output:
  - string: shutdown result
  - error: nil

Lines: ~20
*/
func handleShutdownCommand(args map[string]string) (string, error) {
	if schoolUdpConn != nil {
		schoolUdpConn.Write([]byte("governance:command:stop"))
	}
	if tradingUdpConn != nil {
		tradingUdpConn.Write([]byte("governance:command:stop"))
	}
	if registry != nil {
		for _, name := range []string{"school", "trading"} {
			info := registry.GetStatus(name)
			if info != nil {
				info.Status = "stopping"
				registry.Register(info)
			}
		}
	}
	return "Shutdown signals sent to all daemons. Governance daemon will now exit.", nil
}

// ==============================
// TEST DAEMON REPORT HANDLERS (Phase 24)
// ==============================

/*
Function: handleTestDaemonPass
Description:
  Called when testdaemon reports all tests passed and lists affected daemons.
  Parses the heartbeat message for daemon names and recreates them.

Input:
  - msg string: Heartbeat message from testdaemon

Output:
  - none

Lines: ~20
*/
func handleTestDaemonPass(msg string) {
	infra.FnTrace("entering")
	infra.Info(fmt.Sprintf("TestDaemon reports ALL PASS: %s", msg))

	// Parse daemons=... from the heartbeat message
	if strings.Contains(msg, "daemons=") {
		parts := strings.Split(msg, "daemons=")
		if len(parts) >= 2 {
			daemonList := strings.TrimSpace(parts[len(parts)-1])
			// Extract just the daemons=word before any space
			if idx := strings.Index(daemonList, " "); idx > 0 {
				daemonList = daemonList[:idx]
			}
			if daemonList != "" && daemonList != "none" {
				for _, name := range strings.Split(daemonList, ",") {
					name = strings.TrimSpace(name)
					if name == "" || name == "none" {
						continue
					}
					infra.Info(fmt.Sprintf("TestDaemon: recreating %s (affected by changes)", name))
					recreateDaemonFunc(name, fmt.Sprintf("/workspace/crypto_apps/dexbot/apps/%s/main.go", name))
				}
			}
		}
	}
}

/*
Function: handleTestDaemonLegacyReport
Description:
  Handles legacy-format testdaemon reports via UDP.

Input:
  - msg string: Legacy format "testdaemon:pass|fail:..."

Output:
  - none

Lines: ~15
*/
func handleTestDaemonLegacyReport(msg string) {
	infra.FnTrace("entering")
	parts := strings.SplitN(msg, ":", 4)
	if len(parts) < 3 {
		return
	}
	status := parts[1]
	infra.Info(fmt.Sprintf("TestDaemon legacy report: status=%s msg=%s", status, msg))
	registry.Register(&governance.DaemonInfo{
		Name: "testdaemon", Version: "legacy",
		Status:  status,
		Message: msg,
	})
}

/*
Function: handleModelSync
Description:
  Handles "model:sync:{json}" messages from the School daemon.
  Parses the JSON payload and populates governance's central ModelRegistry
  so the Training dashboard page displays real model data (§33).

Input:
  - jsonPayload string: JSON containing {"models":[{...},...]}

Output: none

Lines: ~50
*/
func handleModelSync(jsonPayload string) {
	if modelReg == nil {
		modelReg = governance.NewModelRegistry()
	}

	type modelSummary struct {
		ID           string  `json:"id"`
		Version      string  `json:"version"`
		Category     string  `json:"category"`
		Architecture string  `json:"architecture"`
		Status       string  `json:"status"`
		Sharpe       float64 `json:"sharpe"`
		Consistency  float64 `json:"consistency"`
		Profit       float64 `json:"profit"`
		Generation   int     `json:"generation"`
	}

	var wrapper struct {
		Models []modelSummary `json:"models"`
	}
	if err := json.Unmarshal([]byte(jsonPayload), &wrapper); err != nil {
		infra.Error("Governance: failed to parse model sync JSON: " + err.Error())
		return
	}

	added := 0
	updated := 0
	for _, s := range wrapper.Models {
		rec := modelReg.Get(s.ID)
		if rec != nil {
			// Update existing
			rec.ModelVersion = s.Version
			rec.Category = s.Category
			rec.Architecture = s.Architecture
			rec.Generation = s.Generation
			if s.Status == governance.ModelStatusGraduated || s.Status == governance.ModelStatusActive {
				modelReg.Graduate(s.ID)
			} else if s.Status == governance.ModelStatusRetired {
				modelReg.Retire(s.ID)
			}
			if s.Sharpe != 0 || s.Consistency != 0 {
				modelReg.RecordFitness(s.ID, governance.FitnessSnapshot{
					Sharpe:      s.Sharpe,
					Consistency: s.Consistency,
					Profit:      s.Profit,
					Generation:  s.Generation,
				})
			}
			updated++
		} else {
			// Register new
			modelReg.Register(&governance.ModelRecord{
				ID:           s.ID,
				ModelVersion: s.Version,
				Category:     s.Category,
				Architecture: s.Architecture,
				Status:       s.Status,
				Generation:   s.Generation,
			})
			if s.Sharpe != 0 || s.Consistency != 0 {
				modelReg.RecordFitness(s.ID, governance.FitnessSnapshot{
					Sharpe:      s.Sharpe,
					Consistency: s.Consistency,
					Profit:      s.Profit,
					Generation:  s.Generation,
				})
			}
			added++
		}
	}
	infra.Info(fmt.Sprintf("ModelRegistry sync: +%d new, ~%d updated, total=%d",
		added, updated, modelReg.Count()))
}

/*
Function: startDaemon
Description:
  Initializes and runs the Governance daemon with registry, UDP listener,
  health check loop, and web server.

Input:
  - none

Output:
  - none

Lines: ~35
*/
func startDaemon() {
	infra.Info("Governance Daemon starting...")

	// Phase 14: restore from checkpoint if available
	restoreGovernanceCheckpoint()

	// Initialize registry
	registry = governance.NewRegistry()

	// Initialize centralized model registry (§33)
	modelReg = governance.NewModelRegistry()

	// §83: Initialize dynamic token registry for balance panel
	tokenReg = infra.NewTokenRegistry()

	// §87: Ensure database is initialized for table browser queries
	_ = infra.InitDB()

	// Register self
	registry.Register(&governance.DaemonInfo{
		Name:    "governance",
		Version: "v2.0",
		Status:  "healthy",
		Message: "Governance daemon started",
	})

	// Load central config
	cfg, err := infra.LoadConfig()
	if err != nil {
		infra.Error("Governance: failed to load config: " + err.Error())
		governancePort = UDP_GOVERNANCE_PORT
		schoolPort = UDP_SCHOOL_PORT
		tradingPort = UDP_TRADING_PORT
		webPort = GOVERNANCE_WEB_PORT
	} else {
		governancePort = cfg.GovernanceUDPPort
		schoolPort = cfg.SchoolUDPPort
		tradingPort = cfg.TradingUDPPort
		webPort = cfg.GovernanceWebPort
	}

	// Load config from env (config.env already loaded by init())
	// In single-container mode: all addresses are 127.0.0.1 (default)
	// In distributed mode (§66): governance listens on 0.0.0.0, peers use container hostnames
	if v := os.Getenv("GOVERNANCE_ADDR"); v != "" {
		governanceAddr = v
	}
	if governanceAddr == "" {
		governanceAddr = "127.0.0.1"
	}
	if v := os.Getenv("SCHOOL_ADDR"); v != "" {
		schoolAddrStr = v
	}
	if schoolAddrStr == "" {
		schoolAddrStr = "127.0.0.1"
	}
	if v := os.Getenv("TRADING_ADDR"); v != "" {
		tradingAddrStr = v
	}
	if tradingAddrStr == "" {
		tradingAddrStr = "127.0.0.1"
	}

	initUDP()
	initCommander()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startUdpListener(ctx)
	go startHealthCheckLoop(ctx)
	go startPublisher(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	<-sigChan

	infra.Info("Governance Daemon shutting down...")
	// Phase 14: checkpoint before shutdown
	saveGovernanceCheckpoint()
	if schoolUdpConn != nil {
		schoolUdpConn.Close()
	}
	if tradingUdpConn != nil {
		tradingUdpConn.Close()
	}
}

/*
Function: governanceCheckpoint
Description:
  Serializable snapshot of governance state for checkpoint/restore.

Fields:
  - Version     string : Checkpoint version
  - Ports       [4]int : gov, school, trading, web ports
  - Threshold   int    : Recreate threshold seconds
  - DaemonNames []string : Known daemon names

Lines: ~5
*/
type governanceCheckpoint struct {
	Version     string   `json:"version"`
	Ports       [4]int   `json:"ports"`
	Threshold   int      `json:"threshold_seconds"`
	DaemonNames []string `json:"daemon_names"`
}

/*
Function: saveGovernanceCheckpoint
Description:
  Saves governance runtime state to a checkpoint file.

Input:
  - none

Output:
  - none

Lines: ~15
*/
func saveGovernanceCheckpoint() {
	state := governanceCheckpoint{
		Version: "v2.0",
		Ports:   [4]int{governancePort, schoolPort, tradingPort, webPort},
		Threshold: int(recreateThreshold.Seconds()),
	}
	if registry != nil {
		state.DaemonNames = registry.List()
	}
	if err := infra.SaveCheckpoint("governance", &state); err != nil {
		infra.Error("Governance: checkpoint save failed: " + err.Error())
	} else {
		infra.Info("Governance: checkpoint saved")
	}
}

/*
Function: restoreGovernanceCheckpoint
Description:
  Restores governance state from a checkpoint file if it exists.

Input:
  - none

Output:
  - none

Lines: ~15
*/
func restoreGovernanceCheckpoint() {
	var state governanceCheckpoint
	if err := infra.RestoreCheckpoint("governance", &state); err != nil {
		infra.Info("Governance: no checkpoint to restore (fresh start)")
		return
	}
	infra.Info(fmt.Sprintf("Governance: checkpoint restored (v=%s, ports=%v, daemons=%v)",
		state.Version, state.Ports, state.DaemonNames))
	if state.Ports[0] > 0 {
		governancePort = state.Ports[0]
	}
	if state.Ports[1] > 0 {
		schoolPort = state.Ports[1]
	}
	if state.Ports[2] > 0 {
		tradingPort = state.Ports[2]
	}
	if state.Ports[3] > 0 {
		webPort = state.Ports[3]
	}
	if state.Threshold > 0 {
		recreateThreshold = time.Duration(state.Threshold) * time.Second
	}
}

// ==============================
// INITIALIZATION
// ==============================

func init() {
	infra.InitLogger()
	loadEnvSmart()
	recreateDaemonFunc = recreateDaemonReal
	sendUdpProbeFunc = sendUdpProbeReal
}

/*
Function: initUDP
Description:
  Initializes UDP sender connections to School and Trading daemons.

Input:
  - none

Output:
  - none

Lines: ~20
*/
func initUDP() {
	var err error
	schoolAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", schoolAddrStr, schoolPort))
	if err != nil {
		infra.Error("Failed to resolve School UDP address: " + err.Error())
	} else {
		schoolUdpConn, err = net.DialUDP("udp", nil, schoolAddr)
		if err != nil {
			infra.Error("Failed to dial School UDP: " + err.Error())
		}
	}

	tradingAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", tradingAddrStr, tradingPort))
	if err != nil {
		infra.Error("Failed to resolve Trading UDP address: " + err.Error())
	} else {
		tradingUdpConn, err = net.DialUDP("udp", nil, tradingAddr)
		if err != nil {
			infra.Error("Failed to dial Trading UDP: " + err.Error())
		}
	}
}

// ==============================
// UTILITY FUNCTIONS
// ==============================

/*
Function: loadEnvSmart
Description:
  Attempts to load config.env from multiple paths.

Input:
  - none

Output:
  - none

Lines: ~15
*/
func loadEnvSmart() {
	paths := []string{
		"config.env",
		"../config.env",
		"../../config.env",
		"../../../config.env",
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

// ==============================
// UDP LISTENER
// ==============================

/*
Function: startUdpListener
Description:
  Listens for incoming UDP messages from daemons. Supports:
  - Full heartbeat: "name:ver:status:cpu:mem:storage:tasks:uptime:msg" (8+ fields)
  - Legacy format: "daemon_type:status:message" (3 fields, fallback)

Input:
  - ctx context.Context: Context for graceful shutdown.

Output:
  - none (runs as a goroutine)

Lines: ~65
*/
func startUdpListener(ctx context.Context) {
	listenAddr := governanceAddr
	if listenAddr == "" {
		listenAddr = "127.0.0.1"
	}
	addr := net.UDPAddr{
		Port: governancePort,
		IP:   net.ParseIP(listenAddr),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		infra.Error("Failed to start UDP listener: " + err.Error())
		return
	}
	defer conn.Close()
	infra.Info(fmt.Sprintf("Governance UDP listener started on %s:%d", listenAddr, governancePort))

	buffer := make([]byte, 16384)
	for {
		select {
		case <-ctx.Done():
			infra.Info("UDP listener shutting down.")
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, _, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				infra.Error("UDP read error: " + err.Error())
				continue
			}
			message := string(buffer[:n])

			// Check for model:sync protocol BEFORE heartbeat parsing
			// (JSON payloads contain colons that confuse the heartbeat parser)
			if strings.HasPrefix(message, "model:sync:") {
				jsonPayload := strings.TrimPrefix(message, "model:sync:")
				handleModelSync(jsonPayload)
				continue
			}

			// Try full heartbeat format first (8+ fields)
			if info, err := governance.ParseHeartbeat(message); err == nil {
				registry.Register(info)
				infra.Info(fmt.Sprintf("Heartbeat received: %s status=%s cpu=%.1f%% mem=%.0fMB",
					info.Name, info.Status, info.CPUPercent, info.MemoryMB))
				// Phase 24: Handle testdaemon results → recreate affected daemons
				if info.Name == "testdaemon" && info.Status == "pass" && info.ActiveTasks > 0 {
					handleTestDaemonPass(info.Message)
				}
				continue
			}

			// Fallback: legacy 3-field format "name:status:message"
			parts := strings.SplitN(message, ":", 3)
			if len(parts) < 3 {
				if strings.Contains(message, "pong:healthy") {
					infra.Info("UDP: received health pong: " + message)
					continue
				}
				// Phase 24: legacy testdaemon message
				if strings.HasPrefix(message, "testdaemon:pass:") || strings.HasPrefix(message, "testdaemon:fail:") {
					handleTestDaemonLegacyReport(message)
					continue
				}
				infra.Warn("Received malformed UDP message: " + message)
				continue
			}

			daemonType := parts[0]
			statusStr := parts[1]
			statusMessage := parts[2]

			// Handle model:sync protocol from School daemon (§33)
			if daemonType == "model" && statusStr == "sync" {
				handleModelSync(statusMessage)
				continue
			}

			isHealthy := statusStr == "healthy"

			info := &governance.DaemonInfo{
				Name:    daemonType,
				Version: "legacy",
				Status:  statusStr,
				Message: statusMessage,
			}
			registry.Register(info)

			if isHealthy {
				infra.Info(fmt.Sprintf("Legacy status: %s healthy — %s", daemonType, statusMessage))
			} else {
				infra.Warn(fmt.Sprintf("Legacy status: %s %s — %s", daemonType, statusStr, statusMessage))
			}
		}
	}
}

// ==============================
// UDP PROBE & HEALTH CHECK
// ==============================

/*
Function: sendUdpProbeReal
Description:
  Sends a UDP probe and waits for a response.

Input:
  - conn    *net.UDPConn  : UDP connection
  - message string        : Probe message
  - timeout time.Duration : Max wait time

Output:
  - string: Response, or "" on timeout
  - error: Non-nil if send/receive fails (excluding timeout)

Lines: ~25
*/
func sendUdpProbeReal(conn *net.UDPConn, message string, timeout time.Duration) (string, error) {
	if conn == nil {
		return "", fmt.Errorf("UDP connection is nil")
	}
	_, err := conn.Write([]byte(message))
	if err != nil {
		return "", fmt.Errorf("failed to send UDP probe: %w", err)
	}
	buffer := make([]byte, 16384)
	conn.SetReadDeadline(time.Now().Add(timeout))
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return "", nil
		}
		return "", fmt.Errorf("failed to receive UDP probe response: %w", err)
	}
	return string(buffer[:n]), nil
}

/*
Function: startHealthCheckLoop
Description:
  Periodically probes School and Trading daemons. Updates registry
  and triggers recreation if unhealthy past threshold.

Input:
  - ctx context.Context: Context for graceful shutdown.

Output:
  - none (runs as a goroutine)

Lines: ~60
*/
func startHealthCheckLoop(ctx context.Context) {
	healthCheckIntervalStr := os.Getenv("HEALTH_CHECK_INTERVAL_SECONDS")
	healthCheckInterval, err := strconv.Atoi(healthCheckIntervalStr)
	if err != nil || healthCheckInterval <= 0 {
		healthCheckInterval = 30
	}
	ticker := time.NewTicker(time.Duration(healthCheckInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			infra.Info("Health check loop shutting down.")
			return
		case <-ticker.C:
			infra.Info("Performing active health checks...")

			// Check School daemon
			schoolResp, err := sendUdpProbeFunc(schoolUdpConn, "governance:probe:health_check", HEALTH_CHECK_TIMEOUT)
			schoolInfo := getOrCreateInfo("school")
			if err != nil || !strings.Contains(schoolResp, "school:pong:healthy") {
				if err != nil {
					infra.Error("School probe error: " + err.Error())
				}
				// §86: Lifecycle transitions — killing → building → recovering → healthy
				if schoolInfo.Status == "killing" {
					schoolInfo.Status = "building"
					schoolInfo.Message = "Daemon killed — recreating..."
				} else if schoolInfo.Status == "building" || schoolInfo.Status == "recovering" {
					schoolInfo.Status = "recovering"
					schoolInfo.Message = "Daemon recovering — waiting for healthy response."
				} else {
					schoolInfo.Status = "unhealthy"
					schoolInfo.Message = "School daemon unresponsive or unhealthy."
				}
				infra.Warn("School daemon status: " + schoolInfo.Status)
				if schoolInfo.Status == "building" || (time.Since(schoolInfo.LastHeartbeat) > recreateThreshold && schoolInfo.Status != "killing") {
					recreateDaemonFunc("school", "/workspace/crypto_apps/dexbot/apps/school/main.go")
					schoolInfo.RecordRestart()
					schoolInfo.Status = "recovering"
					schoolInfo.Message = "Recreating school daemon..."
				}
			} else {
				schoolInfo.Status = "healthy"
				schoolInfo.Message = "School daemon is healthy."
			}
			registry.Register(schoolInfo)

			// Check Trading daemon
			tradingResp, err := sendUdpProbeFunc(tradingUdpConn, "governance:probe:health_check", HEALTH_CHECK_TIMEOUT)
			tradingInfo := getOrCreateInfo("trading")
			if err != nil || !strings.Contains(tradingResp, "trading:pong:healthy") {
				if err != nil {
					infra.Error("Trading probe error: " + err.Error())
				}
				if tradingInfo.Status == "killing" {
					tradingInfo.Status = "building"
					tradingInfo.Message = "Daemon killed — recreating..."
				} else if tradingInfo.Status == "building" || tradingInfo.Status == "recovering" {
					tradingInfo.Status = "recovering"
					tradingInfo.Message = "Daemon recovering — waiting for healthy response."
				} else {
					tradingInfo.Status = "unhealthy"
					tradingInfo.Message = "Trading daemon unresponsive or unhealthy."
				}
				infra.Warn("Trading daemon status: " + tradingInfo.Status)
				if tradingInfo.Status == "building" || (time.Since(tradingInfo.LastHeartbeat) > recreateThreshold && tradingInfo.Status != "killing") {
					recreateDaemonFunc("trading", "/workspace/crypto_apps/dexbot/apps/trading/main.go")
					tradingInfo.RecordRestart()
					tradingInfo.Status = "recovering"
					tradingInfo.Message = "Recreating trading daemon..."
				}
			} else {
				tradingInfo.Status = "healthy"
				tradingInfo.Message = "Trading daemon is healthy."
			}
			registry.Register(tradingInfo)
		}
	}
}

/*
Function: getOrCreateInfo
Description:
  Retrieves a DaemonInfo from the registry or creates a default entry.

Input:
  - name string: Daemon name

Output:
  - *governance.DaemonInfo: Existing or new entry

Lines: ~12
*/
func getOrCreateInfo(name string) *governance.DaemonInfo {
	info := registry.GetStatus(name)
	if info == nil {
		info = &governance.DaemonInfo{
			Name:    name,
			Version: "unknown",
			Status:  "unknown",
		}
	}
	return info
}

/*
Function: recreateDaemonReal
Description:
  Attempts to restart a specified daemon. Skips if recently healthy.

Input:
  - daemonName string: Logical name (school, trading)
  - daemonPath string: Absolute path to main.go

Output:
  - none

Lines: ~40
*/
func recreateDaemonReal(daemonName string, daemonPath string) {
	infra.Warn(fmt.Sprintf("Attempting to recreate %s daemon...", daemonName))

	info := registry.GetStatus(daemonName)
	if info != nil && info.IsHealthy() && time.Since(info.LastHeartbeat) < 1*time.Minute {
		infra.Info(fmt.Sprintf("Skipping recreation for %s, it recently reported healthy.", daemonName))
		return
	}

	cmd := exec.Command("/usr/local/go/bin/go", "run", daemonPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	err := cmd.Start()
	if err != nil {
		infra.Error(fmt.Sprintf("Failed to start %s daemon: %v", daemonName, err))
		return
	}
	infra.Info(fmt.Sprintf("%s daemon started with PID %d", daemonName, cmd.Process.Pid))

	if info == nil {
		info = &governance.DaemonInfo{Name: daemonName, Version: "restarted"}
	}
	info.Status = "starting"
	info.Message = fmt.Sprintf("Attempted recreation, new PID %d", cmd.Process.Pid)
	info.RecordRestart()
	registry.Register(info)
}

// ==============================
// DASHBOARD PUBLISHER (Phase 18 — §68-70)
// ==============================

/*
Function: startPublisher
Description:
  Periodically generates HTML + JSON dashboard files to WEB_OUTPUT_DIR
  and listens on a TCP port for action commands forwarded by the
  middle server (serve.py/nginx/apache). Governance does NOT run
  an HTTP server — the middle server handles HTTP.

  File refresh interval: WEB_REFRESH_SECONDS (default 10s)
  TCP action port: WEB_ACTION_PORT (default 8085)

Input:
  - ctx context.Context : For graceful shutdown

Output:
  - none (runs as goroutine)

Lines: ~40
*/
func startPublisher(ctx context.Context) {
	outputDir := os.Getenv("WEB_OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "web_output"
	}
	publisher = infra.NewPublisher(outputDir)

	refreshStr := os.Getenv("WEB_REFRESH_SECONDS")
	refreshSec := 10
	if n, err := strconv.Atoi(refreshStr); err == nil && n > 0 {
		refreshSec = n
	}

	// Start TCP action listener for the middle server
	actionPortStr := os.Getenv("WEB_ACTION_PORT")
	actionPort := 8085
	if n, err := strconv.Atoi(actionPortStr); err == nil && n > 0 {
		actionPort = n
	}
	go startActionListener(ctx, actionPort)

	// Create renderer (not as HTTP server — just for rendering)
	renderer := webui.NewRenderer(registry)
	renderer.SetPorts(governancePort, schoolPort, tradingPort, webPort)
	if modelReg != nil {
		renderer.SetModelRegistry(modelReg)
	}

	// Initialize account manager for balance display (§79-81)
	acctMgr := infra.NewAccountManager()
	_ = acctMgr

	ticker := time.NewTicker(time.Duration(refreshSec) * time.Second)
	defer ticker.Stop()

	infra.Info(fmt.Sprintf("Dashboard publisher started (dir=%s, refresh=%ds, actionPort=%d)",
		outputDir, refreshSec, actionPort))

	// Do one immediate refresh
	refreshDashboard(renderer)

	for {
		select {
		case <-ctx.Done():
			infra.Info("Dashboard publisher shutting down.")
			return
		case <-ticker.C:
			refreshDashboard(renderer)
		}
	}
}

/*
Function: refreshDashboard
Description:
  Generates all dashboard HTML pages + JSON API files into the output directory.
  Called periodically by the publisher loop.

Input:
  - renderer *webui.Renderer

Output:
  - none

Lines: ~25
*/
func refreshDashboard(renderer *webui.Renderer) {
	infra.FnTrace("refreshing dashboard files")

	// Pull latest model data from the registry before rendering
	renderer.RefreshModels()

	// §79-81: Generate balance summary from dynamic token registry
	am := infra.NewAccountManager()
	balance := &infra.BalanceSummary{
		AccountName:   am.FullKey(),
		AccountMasked: am.MaskedKey(),
		BTCPrice:      infra.BTCPriceMock,
	}
	// Use tokens from the dynamic registry if available
	if tokenReg != nil {
		balance.Assets = tokenReg.AsBalanceAssets()
	} else {
		raw := infra.GetBalanceSummary(am)
		if raw != nil {
			balance.Assets = raw.Assets
		}
	}
	totalUSD := 0.0
	for i := range balance.Assets {
		balance.Assets[i].USDValue = balance.Assets[i].Amount * balance.Assets[i].USDPrice
		totalUSD += balance.Assets[i].USDValue
	}
	balance.TotalUSD = totalUSD
	balance.TotalBTC = totalUSD / infra.BTCPriceMock
	balance.IsPaperTrade = false
	renderer.SetBalance(balance)

	// HTML pages
	pages := []struct{ name, content string }{
		{"index", renderToBytes(renderer.Operations)},
		{"training", renderToBytes(renderer.SchoolDashboard)},
		{"portfolio", renderToBytes(renderer.Portfolio)},
		{"predict", renderToBytes(renderer.PredictionComparison)},
	}
	for _, p := range pages {
		if err := publisher.WriteHTML(p.name, p.content); err != nil {
			infra.Error("Failed to write " + p.name + ".html: " + err.Error())
		}
	}

	// JSON API
	names := registry.List()
	type daemonList struct {
		Daemons []*governance.DaemonInfo `json:"daemons"`
	}
	dl := daemonList{}
	for _, n := range names {
		dl.Daemons = append(dl.Daemons, registry.GetStatus(n))
	}
	if err := publisher.WriteJSON("api/daemons", dl); err != nil {
		infra.Error("Failed to write api/daemons.json: " + err.Error())
	}

	// §79: Write balance summary for web display
	if balance != nil {
		if err := publisher.WriteJSON("api/balance", balance); err != nil {
			infra.Error("Failed to write api/balance.json: " + err.Error())
		}
	}

	// §83: Sync token registry to dashboard
	if tokenReg != nil {
		tokens := tokenReg.ListTokens()
		if err := publisher.WriteJSON("api/tokens", map[string]interface{}{"tokens": tokens}); err != nil {
			infra.Error("Failed to write api/tokens.json: " + err.Error())
		}
	}

	// §87: Database table list + per-table row data
	tables := infra.ListTables()
	if tables != nil {
		if err := publisher.WriteJSON("api/database_tables", map[string]interface{}{"tables": tables}); err != nil {
			infra.Error("Failed to write api/database_tables.json: " + err.Error())
		}
		// Build per-table data (limit 5, newest first)
		dbData := make(map[string]interface{})
		for _, tbl := range tables {
			cols, rows := infra.QueryTableRows(tbl, 5, "newest")
			if cols != nil {
				dbData[tbl] = map[string]interface{}{
					"columns": cols,
					"rows":    rows,
				}
			}
		}
		if len(dbData) > 0 {
			if err := publisher.WriteJSON("api/database", dbData); err != nil {
				infra.Error("Failed to write api/database.json: " + err.Error())
			}
		}
	}

	publisher.MarkRefreshed()
	infra.FnTrace(fmt.Sprintf("dashboard refreshed: %d pages, %d daemons", len(pages), len(names)))
}

/*
Function: startActionListener
Description:
  Listens on a TCP port for action commands forwarded by the middle server
  (serve.py, nginx, or apache). Protocol is simple text:
    "restart school"   → webuiActionCallback("school", "restart")
    "stop trading"     → webuiActionCallback("trading", "stop")
    "start school"     → webuiActionCallback("school", "start")

Input:
  - ctx  context.Context : Graceful shutdown
  - port int             : Listen port

Output:
  - none (runs as goroutine)

Lines: ~35
*/
func startActionListener(ctx context.Context, port int) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		infra.Error("Action listener failed to bind " + addr + ": " + err.Error())
		return
	}
	defer ln.Close()
	infra.Info(fmt.Sprintf("Action listener started on %s (for middle server)", addr))

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Set accept deadline so we can check ctx cancellation periodically
		if tcpLn, ok := ln.(*net.TCPListener); ok {
			tcpLn.SetDeadline(time.Now().Add(1 * time.Second))
		}
		conn, err := ln.Accept()
		if err != nil {
			// Timeout is expected — just loop to check ctx
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			continue
		}
		go handleActionConn(conn)
	}
}

func handleActionConn(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 256)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	msg := strings.TrimSpace(string(buf[:n]))
	parts := strings.Fields(msg)
	if len(parts) >= 2 {
		action, name := parts[0], parts[1]
		webuiActionCallback(name, action)
		conn.Write([]byte("OK " + action + " " + name + "\n"))
	} else {
		conn.Write([]byte("ERROR invalid format\n"))
	}
}

// renderToBytes calls a renderer method and captures its output to a string
func renderToBytes(fn func(http.ResponseWriter)) string {
	var buf bytes.Buffer
	// Create a minimal response writer that captures the buffer
	fn(&bufferWriter{&buf})
	return buf.String()
}

// bufferWriter implements http.ResponseWriter by writing to a bytes.Buffer
type bufferWriter struct {
	buf *bytes.Buffer
}

func (bw *bufferWriter) Header() http.Header           { return http.Header{} }
func (bw *bufferWriter) Write(data []byte) (int, error) { return bw.buf.Write(data) }
func (bw *bufferWriter) WriteHeader(statusCode int)     {}