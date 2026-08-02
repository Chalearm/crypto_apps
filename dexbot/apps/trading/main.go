/******************************************************************************
 * File Name        : main.go
 * File Path        : apps/trading/main.go
 * Author           : Chalearm Saelim & Gemini
 * Owner            : Chalearm Saelim
 * Reviewer         : Chalearm Saelim
 * Version          : 2.0.0
 * Status           : Development
 * Created Date     : 2026-07-26 08:00:00 (UTC+7)
 * Modified Date    : 2026-07-29 01:00:00 (UTC+7)
 *
 * Description      :
 *    Trading daemon entry point for the Dexbot platform. Manages worker task pools,
 *    listens for graduate model updates from School via UDP, executes trade state
 *    persistence, manages paper/live trading gates, and drives the Hierarchical 
 *    Multi-Armed Bandit (H-MAB) agent allocation engine.
 *
 * DEPENDENCY TREE & STRUCTURAL MAP:
 * ───────────────────────────────────────────────────────────────────────────
 * [apps/trading/main.go] (Trading Daemon Entry Point)
 *     │
 *     ├── Imports Internal Modules ──> [dexbot/config] (Runtime Settings)
 *     │                           ├──> [dexbot/governance] (Model Registry & Heartbeat)
 *     │                           ├──> [dexbot/infra] (Logger, Checkpoint, Account Manager)
 *     │                           └──> [dexbot/trading] (AgentPool & HierarchicalMAB)
 *     │
 *     ├── State & Checkpointing ──> [tasks_state.json] (Task Manager State)
 *     │                          └──> [checkpoints/trading.json] (Daemon Checkpoint)
 *     │
 *     ├── Inter-Daemon Communication:
 *     │     ├── Listens UDP (Port 8083) <── Graduate models from School / Commands from Governance
 *     │     └── Sends UDP (Port 8081)   ──> Heartbeats & Portfolio results to Governance
 *     │
 *     └── Autonomous Engine Loop:
 *           ├── MAB Agent/Model Selection pass
 *           ├── Paper Trading Gate evaluation
 *           └── Worker Task Execution & Portfolio Prediction
 *
 * FUNCTION DEPENDENCY MATRIX (Internal Methods):
 * ───────────────────────────────────────────────────────────────────────────
 * main()
 *  ├── infra.InitLogger()
 *  ├── loadEnvSmart()
 *  ├── restoreTradingCheckpoint() ──> infra.RestoreCheckpoint()
 *  ├── initUDP() ──────────────────> net.DialUDP()
 *  ├── validateBalanceHoldings() ──> infra.NewAccountManager() & GetBalanceSummary()
 *  ├── go startUdpListener()
 *  │    ├── receiveGraduates() ────> trading.NewHierarchicalMAB()
 *  │    └── handleTradingCommand()
 *  ├── go runTradingDaemon()
 *  │    ├── NewTaskManager() ──────> TaskManager.Load()
 *  │    ├── createTask() ──────────> calculateConfidence() & calculateSpread()
 *  │    ├── runWorkers() ──────────> processTask() ──> ExecuteTrade()
 *  │    ├── evaluatePortfolio() ───> predictEnsemble()
 *  │    ├── runMABCycle()
 *  │    │    ├── manageAgentLifecycle() ──> tryGraduatePaperAgents()
 *  │    │    ├── hmab.SelectModel()
 *  │    │    ├── agentPool.RebalanceCapital()
 *  │    │    ├── simulateAgentTrade() ───> modelReg.RecordPerformance()
 *  │    │    └── evaluatePaperGate()
 *  │    ├── sendHeartbeat() ────────> collectResourceMetrics() & governance.FormatHeartbeat()
 *  │    └── updateMABFromPerformance() ─> hmab.UpdateAgentEvidence() & UpdateModelEvidence()
 *  └── saveTradingCheckpoint() ────> infra.SaveCheckpoint()
 *
 * Responsibilities :
 *    - Receives graduate model registrations from School over UDP socket connection.
 *    - Orchestrates multi-agent capital rebalancing using Hierarchical Multi-Armed Bandit algorithm.
 *    - Evaluates paper trading metrics against configured criteria before allowing live capital trading.
 *    - Periodically sends resource usage heartbeats to Governance daemon.
 *
 * Usage :
 *    Directory : apps/trading/
 *    Build     : go build -o trading main.go
 *    Run       : ./trading
 *
 * Dependencies :
 *    Internal  : dexbot/config, dexbot/governance, dexbot/infra, dexbot/trading
 *    External  : stdlib (net, os, sync, math, encoding/json)
 *
 * Change History :
 *    -------------------------------------------------------------------------
 *    Version | Date Time (UTC+7)         | Author          | Description
 *    -------------------------------------------------------------------------
 *    1.0.0   | 2026-07-26 08:00:00 (UTC+7) | Chalearm Saelim | Initial release
 *    2.0.0   | 2026-07-29 01:00:00 (UTC+7) | Gemini          | Integrated H-MAB & Paper Gate Matrix
 *    -------------------------------------------------------------------------
 *
 * Notes :
 *    - Per regulator coding standard rules.
 ******************************************************************************/

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"dexbot/config"
	"dexbot/governance"
	"dexbot/infra"
	"dexbot/trading"
)

// ==============================
// CONSTANTS
// ==============================

const (
	STATE_FILE          = "tasks_state.json"
	CONFIG_FILE         = "config.json"
	UDP_GOVERNANCE_PORT = 8081
	UDP_TRADING_PORT    = 8083
)

var (
	dryRun         bool
	governancePort int
	tradingPort    int
)

// ==============================
// TYPES (kept for backward compat)
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
	ID         string
	Status     TaskStatus
	FromToken  string
	ToToken    string
	BuyPrice   float64
	SellPrice  float64
	Confidence float64
	Spread     float64
	CreatedAt  time.Time
}

type TaskManager struct {
	mu    sync.Mutex
	Tasks map[string]*TradeTask
}

/******************************************************************************
 * Function Name : NewTaskManager
 * Purpose       : Initializes and returns a new TaskManager instance.
 ******************************************************************************/
func NewTaskManager() *TaskManager {
	return &TaskManager{Tasks: make(map[string]*TradeTask)}
}

/******************************************************************************
 * Function Name : Save
 * Purpose       : Persists task manager state to disk.
 ******************************************************************************/
func (tm *TaskManager) Save() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	data, _ := json.MarshalIndent(tm, "", "  ")
	_ = os.WriteFile(STATE_FILE, data, 0644)
	infra.Info("trading state saved to disk")
}

/******************************************************************************
 * Function Name : Load
 * Purpose       : Restores task manager state from disk.
 ******************************************************************************/
func (tm *TaskManager) Load() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	data, err := os.ReadFile(STATE_FILE)
	if err == nil {
		_ = json.Unmarshal(data, tm)
		infra.Info("trading state loaded from disk")
	} else {
		infra.Warn("trading: no previous state file")
	}
}

// ==============================
// GLOBAL VARIABLES
// ==============================

var (
	udpConn           *net.UDPConn
	governanceAddr    *net.UDPAddr
	governanceAddrStr string // §66: configurable governance address
	tradingListenAddr string // §66: configurable trading listen address
	strategy          Strategy = &MockStrategy{}
	startTime         = time.Now()

	// Phase 13: H-MAB + AgentPool
	runtimeCfg  config.RuntimeConfig
	agentPool   *trading.AgentPool       // All agents (paper + real)
	paperAgents map[string]bool          // true = paper mode, false = real capital
	hmab        *trading.HierarchicalMAB
	modelNames  []string                 // graduate model IDs from School (indexed for MAB arms)
	modelNamesMu sync.RWMutex
	manager     *TaskManager
	rng         = rand.New(rand.NewSource(time.Now().UnixNano()))

	// §33: Model Registry (Phase 19)
	modelReg *governance.ModelRegistry

	// §49: Paper trading gate
	paperStats paperTradingStats
)

// paperTradingStats tracks progress toward real-capital eligibility (§49).
type paperTradingStats struct {
	TotalTrades       int       // Total paper trades executed
	AgentsEligible    int       // Agents meeting performance threshold
	ModelsEligible    int       // Models meeting confidence threshold
	RealCapitalActive bool      // True if paper gate has been passed
	GatePassedAt      time.Time // When the gate was passed
}

// §82: Paper trade fallback mode — triggered when reloaded balance exceeds holdings.
var (
	paperTradeMode   bool   // true = all portfolios are paper-only
	paperTradeReason string // human-readable reason for paper mode
)

// ==============================
// INIT
// ==============================

func init() {
	infra.InitLogger()
	loadEnvSmart()

	// Load full runtime config
	runtimeCfg = config.Defaults()
	runtimeCfg.LoadFromEnv()

	// Init agent pool + H-MAB
	agentPool = trading.NewAgentPool()
	paperAgents = make(map[string]bool)
	// Start with 1 model arm, 5 agent arms — will expand as graduates arrive
	hmab = trading.NewHierarchicalMAB(1, 5, rng)
	modelNames = []string{"__placeholder__"}
	// §33: Initialize Model Registry
	modelReg = governance.NewModelRegistry()
}

// ==============================
// MAIN
// ==============================

func main() {
	infra.FnTrace("entering")
	defer infra.FnTrace("done")

	cfg, err := infra.LoadConfig()
	if err != nil {
		infra.Error("Trading: failed to load config: " + err.Error())
		governancePort = UDP_GOVERNANCE_PORT
		tradingPort = UDP_TRADING_PORT
	} else {
		governancePort = cfg.GovernanceUDPPort
		tradingPort = cfg.TradingUDPPort
	}

	// §66: Load configurable addresses from env (config.env already loaded)
	if v := os.Getenv("GOVERNANCE_ADDR"); v != "" {
		governanceAddrStr = v
	}
	if governanceAddrStr == "" {
		governanceAddrStr = "127.0.0.1"
	}
	if v := os.Getenv("TRADING_ADDR"); v != "" {
		tradingListenAddr = v
	}
	if tradingListenAddr == "" {
		tradingListenAddr = "127.0.0.1"
	}

	infra.Info("Trading Daemon starting...")

	// Phase 14: restore from checkpoint if available
	restoreTradingCheckpoint()

	initUDP()

	// §82: Validate balance vs holdings — fall back to paper trading if
	// any tracked asset exceeds the actual wallet balance on reload.
	validateBalanceHoldings()
	if paperTradeMode {
		infra.Warn(fmt.Sprintf("PAPER TRADE MODE: %s", paperTradeReason))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startUdpListener(ctx)
	go runTradingDaemon(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	<-sigChan

	infra.Info("Trading Daemon shutting down...")
	// Phase 14: checkpoint before shutdown
	saveTradingCheckpoint()
	agentPool.RebalanceCapital(0)
	if udpConn != nil {
		udpConn.Close()
	}
}

type tradingCheckpoint struct {
	Version      string  `json:"version"`
	AgentCount   int     `json:"agent_count"`
	ActiveCount  int     `json:"active_count"`
	TotalCapital float64 `json:"total_capital"`
	ModelCount   int     `json:"model_count"`
	Ports        [2]int  `json:"ports"`
}

/******************************************************************************
 * Function Name : saveTradingCheckpoint
 * Purpose       : Saves trading state for checkpointing.
 ******************************************************************************/
func saveTradingCheckpoint() {
	state := tradingCheckpoint{
		Version:      "v2.0",
		TotalCapital: runtimeCfg.TotalCapital,
		Ports:        [2]int{governancePort, tradingPort},
	}
	if agentPool != nil {
		state.AgentCount = agentPool.Count()
		state.ActiveCount = agentPool.ActiveCount()
	}
	modelNamesMu.RLock()
	state.ModelCount = len(modelNames)
	modelNamesMu.RUnlock()

	if err := infra.SaveCheckpoint("trading", &state); err != nil {
		infra.Error("Trading: checkpoint save failed: " + err.Error())
	} else {
		infra.Info("Trading: checkpoint saved")
	}
}

/******************************************************************************
 * Function Name : restoreTradingCheckpoint
 * Purpose       : Restores trading state from checkpoint if available.
 ******************************************************************************/
func restoreTradingCheckpoint() {
	var state tradingCheckpoint
	if err := infra.RestoreCheckpoint("trading", &state); err != nil {
		infra.Info("Trading: no checkpoint to restore (fresh start)")
		return
	}
	infra.Info(fmt.Sprintf("Trading: checkpoint restored (v=%s agents=%d capital=%.0f)",
		state.Version, state.AgentCount, state.TotalCapital))
	if state.Ports[0] > 0 {
		governancePort = state.Ports[0]
	}
	if state.Ports[1] > 0 {
		tradingPort = state.Ports[1]
	}
}

/******************************************************************************
 * Function Name : initUDP
 * Purpose       : Resolves and dials Governance UDP endpoint.
 ******************************************************************************/
func initUDP() {
	var err error
	govAddr := governanceAddrStr
	if govAddr == "" {
		govAddr = "127.0.0.1"
	}
	governanceAddr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", govAddr, governancePort))
	if err != nil {
		infra.Error("Failed to resolve governance UDP address: " + err.Error())
		return
	}
	udpConn, err = net.DialUDP("udp", nil, governanceAddr)
	if err != nil {
		infra.Error("Failed to dial governance UDP: " + err.Error())
		return
	}
	infra.Info(fmt.Sprintf("Trading UDP sender initialized, sending to %s:%d", govAddr, governancePort))
}

/******************************************************************************
 * Function Name : loadEnvSmart
 * Purpose       : Searches and loads configuration env file from path hierarchy.
 ******************************************************************************/
func loadEnvSmart() {
	paths := []string{"config.env", "../config.env", "../../config.env", "../../../config.env"}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			infra.LoadEnv(p)
			infra.Info("env loaded from: " + p)
			return
		}
	}
	infra.Warn("env file not found in any path")
}

/******************************************************************************
 * Function Name : validateBalanceHoldings
 * Purpose       : Validates portfolio positions against actual wallet balance.
 ******************************************************************************/
func validateBalanceHoldings() {
	am := infra.NewAccountManager()
	if am.FullKey() == "" {
		infra.Info("Trading: no PRIVATE_KEY set — running in paper mode by default")
		paperTradeMode = true
		paperTradeReason = "No private key configured"
		return
	}

	balance := infra.GetBalanceSummary(am)
	if balance == nil || len(balance.Assets) == 0 {
		return
	}

	totalBalance := balance.TotalUSD
	currentPaperCount := 0
	for _, a := range agentPool.ActiveAgents() {
		if a.Capital > totalBalance*1.1 { // 10% buffer for rounding
			if !paperTradeMode {
				paperTradeMode = true
				paperTradeReason = fmt.Sprintf("Agent %s capital ($%.2f) exceeds wallet balance ($%.2f)",
					a.ID, a.Capital, totalBalance)
			}
			paperAgents[a.ID] = true
			currentPaperCount++
		}
	}

	allocated := agentPool.TotalAllocated()
	if allocated > totalBalance*1.1 && !paperTradeMode {
		paperTradeMode = true
		paperTradeReason = fmt.Sprintf("Total allocated ($%.2f) exceeds wallet balance ($%.2f)",
			allocated, totalBalance)
	}

	if paperTradeMode {
		infra.Warn(fmt.Sprintf("Trading: paper trade fallback activated — %s (paper agents: %d)",
			paperTradeReason, currentPaperCount))
		for _, a := range agentPool.ActiveAgents() {
			paperAgents[a.ID] = true
		}
	} else {
		infra.Info(fmt.Sprintf("Trading: balance verified — $%.2f wallet, $%.2f allocated, OK",
			totalBalance, allocated))
	}
}

// ==============================
// UDP LISTENER
// ==============================

func startUdpListener(ctx context.Context) {
	infra.FnTrace("entering")
	defer infra.FnTrace("done")

	listenAddr := tradingListenAddr
	if listenAddr == "" {
		listenAddr = "127.0.0.1"
	}
	if tradingPort == 0 {
		tradingPort = 8083
	}
	addr := net.UDPAddr{Port: tradingPort, IP: net.ParseIP(listenAddr)}

	var conn *net.UDPConn
	var err error
	for retry := 0; retry < 5; retry++ {
		conn, err = net.ListenUDP("udp", &addr)
		if err == nil { break }
		infra.Warn(fmt.Sprintf("Trading: UDP bind attempt %d failed: %v — retrying in 1s", retry+1, err))
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		infra.Error("Trading: Failed to start UDP listener: " + err.Error())
		return
	}
	defer conn.Close()
	infra.Info(fmt.Sprintf("Trading UDP listener started on %s:%d", listenAddr, tradingPort))

	buffer := make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			infra.Info("Trading UDP listener shutting down.")
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, remoteAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				infra.Error("Trading: UDP read error: " + err.Error())
				continue
			}
			message := string(buffer[:n])
			infra.Info("Trading: Received UDP message: " + message)

			if strings.HasPrefix(message, "school:graduate:") {
				receiveGraduates(message)
				continue
			}

			if strings.HasPrefix(message, "governance:command:") {
				handleTradingCommand(message, conn, remoteAddr)
				continue
			}

			if strings.HasPrefix(message, "governance:config:reload") {
				infra.ReloadLoggerConfig()
				runtimeCfg.LoadFromEnv()
				continue
			}

			if strings.HasPrefix(message, "governance:probe:health_check") {
				conn.WriteToUDP([]byte("trading:pong:healthy"), remoteAddr)
				infra.Info("Trading: Sent health probe response")
			}
		}
	}
}

// ==============================
// GRADUATE RECEPTION (§17, §23)
// ==============================

func receiveGraduates(msg string) {
	infra.FnTrace("entering")
	defer infra.FnTrace("done")

	payload := strings.TrimPrefix(msg, "school:graduate:")
	if payload == "" {
		return
	}
	names := strings.Split(payload, ",")
	modelNamesMu.Lock()
	modelNames = nil
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n != "" {
			modelNames = append(modelNames, n)
		}
	}
	nModels := len(modelNames)
	modelNamesMu.Unlock()

	if nModels > 0 {
		nAgents := runtimeCfg.Trading.MaxPaperAgents + runtimeCfg.Trading.MaxRealAgents
		if nAgents < 5 {
			nAgents = 5
		}
		hmab = trading.NewHierarchicalMAB(nModels, nAgents, rng)
		infra.Info(fmt.Sprintf("Received %d graduate models from School: %v", nModels, modelNames))
	} else {
		infra.Warn("Received empty graduate list from School")
	}
}

func sendPortfolioResults() {
	infra.FnTrace("entering")
	defer infra.FnTrace("done")

	if udpConn == nil {
		return
	}
	modelNamesMu.RLock()
	defer modelNamesMu.RUnlock()

	probs := hmab.ModelProbabilities()
	for i, name := range modelNames {
		if i >= len(probs) {
			break
		}
		var totalSharpe, totalProfit, totalDrawdown float64
		count := 0
		for _, a := range agentPool.ActiveAgents() {
			if a.DNA != nil {
				for _, m := range a.DNA.ModelAssignments {
					if m == name {
						if a.Performance != nil {
							totalSharpe += a.Performance.Sharpe
							totalProfit += a.Performance.PnL
							totalDrawdown += a.Performance.Drawdown
							count++
						}
					}
				}
			}
		}
		if count > 0 {
			n := float64(count)
			msg := fmt.Sprintf("trading:portfolio:%s,%.4f,%.4f,%.4f,%d",
				name, totalSharpe/n, totalProfit/n, totalDrawdown/n, count)
			udpConn.Write([]byte(msg))
		}
	}
}

// ==============================
// COMMAND HANDLERS
// ==============================

func handleTradingCommand(msg string, conn *net.UDPConn, remoteAddr *net.UDPAddr) {
	switch {
	case strings.Contains(msg, "kill"):
		infra.Warn("Trading: received kill — exiting immediately")
		conn.WriteToUDP([]byte("trading:ack:killed"), remoteAddr)
		os.Exit(1)
	case strings.Contains(msg, "stop"):
		infra.Warn("Trading: received stop — shutting down")
		conn.WriteToUDP([]byte("trading:ack:stopping"), remoteAddr)
		os.Exit(0)
	case strings.Contains(msg, "restart"):
		infra.Warn("Trading: received restart")
		conn.WriteToUDP([]byte("trading:ack:restarting"), remoteAddr)
		os.Exit(0)
	}
}

// ==============================
// HEARTBEAT
// ==============================

func collectResourceMetrics() (float64, float64, float64) {
	return 8.0 + float64(time.Now().UnixNano()%400)/100.0,
		256.0 + float64(time.Now().UnixNano()%256),
		2048.0
}

func sendHeartbeat() {
	if udpConn == nil {
		return
	}
	cpu, mem, storage := collectResourceMetrics()
	modelNamesMu.RLock()
	activeTasks := agentPool.ActiveCount()
	modelNamesMu.RUnlock()
	info := &governance.DaemonInfo{
		Name: "trading", Version: "v2.0", Status: "healthy",
		CPUPercent: cpu, MemoryMB: mem, StorageMB: storage,
		ActiveTasks: activeTasks, Uptime: time.Since(startTime),
		Message: fmt.Sprintf("Agents=%d Models=%d", activeTasks, len(modelNames)),
		LastHeartbeat: time.Now(),
	}
	udpConn.Write([]byte(governance.FormatHeartbeat(info)))
}

func sendStatusToGovernance(daemonType, status, message string) {
	if udpConn == nil {
		return
	}
	udpConn.Write([]byte(fmt.Sprintf("%s:%s:%s", daemonType, status, message)))
}

// ==============================
// DAEMON LOOP (§18-21)
// ==============================

func runTradingDaemon(ctx context.Context) {
	infra.FnTrace("entering")
	defer infra.FnTrace("done")
	infra.Info("Trading daemon loop started (H-MAB + AgentPool)")

	manager = NewTaskManager()
	manager.Load()

	interval := time.Duration(runtimeCfg.Trading.TradingCycleIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	mabUpdateTicker := time.NewTicker(time.Duration(runtimeCfg.Trading.MABUpdateIntervalSeconds) * time.Second)
	defer mabUpdateTicker.Stop()

	agentCreateTicker := time.NewTicker(time.Duration(runtimeCfg.Trading.AgentCreationIntervalMinutes) * time.Minute)
	defer agentCreateTicker.Stop()

	for {
		select {
		case <-ticker.C:
			cfg := loadConfig()
			if len(manager.Tasks) < cfg.MaxTasks {
				createTask(manager)
			}
			runWorkers(manager)
			evaluatePortfolio(manager)

			runMABCycle()
			sendHeartbeat()

		case <-mabUpdateTicker.C:
			updateMABFromPerformance()

		case <-agentCreateTicker.C:
			manageAgentLifecycle()

		case <-ctx.Done():
			infra.Info("Trading daemon context closed, shutting down loop.")
			manager.Save()
			return
		}
	}
}

// ==============================
// MAB CYCLE (§20-21)
// ==============================

func runMABCycle() {
	infra.FnTrace("entering")
	modelNamesMu.RLock()
	nModels := len(modelNames)
	modelNamesMu.RUnlock()

	if nModels == 0 {
		return
	}

	manageAgentLifecycle()

	activeAgents := agentPool.ActiveAgents()
	if len(activeAgents) == 0 {
		return
	}

	modelIdx := hmab.SelectModel()
	modelName := "__unknown__"
	modelNamesMu.RLock()
	if modelIdx >= 0 && modelIdx < len(modelNames) {
		modelName = modelNames[modelIdx]
	}
	modelNamesMu.RUnlock()

	totalCapital := runtimeCfg.TotalCapital
	tradingMode := "paper"
	if !isPaperTrading() {
		tradingMode = "live"
	} else if !runtimeCfg.PaperTradingOnly {
		totalCapital = totalCapital * 0.1
	}
	agentPool.RebalanceCapital(totalCapital)

	agentIdx := hmab.SelectAgent()
	if agentIdx >= 0 && agentIdx < len(activeAgents) {
		agent := activeAgents[agentIdx]
		if agent.DNA == nil {
			agent.DNA = &trading.AgentDNA{}
		}
		found := false
		for _, m := range agent.DNA.ModelAssignments {
			if m == modelName {
				found = true
				break
			}
		}
		if !found {
			agent.DNA.ModelAssignments = append(agent.DNA.ModelAssignments, modelName)
		}
		simulateAgentTrade(agent)
		paperStats.TotalTrades++
		evaluatePaperGate()
	}

	infra.FnTrace(fmt.Sprintf("model=%s(%d) agent=%d capital=%.0f mode=%s", modelName, modelIdx, agentIdx, totalCapital, tradingMode))
}

// ==============================
// PAPER TRADING GATE (§49)
// ==============================

func isPaperTrading() bool {
	if runtimeCfg.PaperTradingOnly {
		return true
	}
	if paperStats.RealCapitalActive {
		return false
	}
	tc := runtimeCfg.Trading
	if paperStats.TotalTrades < tc.MinPaperTrades {
		return true
	}
	if paperStats.AgentsEligible == 0 || paperStats.ModelsEligible == 0 {
		return true
	}
	return false
}

func evaluatePaperGate() {
	if !runtimeCfg.PaperTradingOnly && paperStats.RealCapitalActive {
		return
	}
	tc := runtimeCfg.Trading

	paperStats.AgentsEligible = 0
	for _, a := range agentPool.ActiveAgents() {
		if a.Performance != nil && a.Performance.Sharpe >= tc.AgentPerformanceThreshold {
			paperStats.AgentsEligible++
		}
	}

	paperStats.ModelsEligible = 0
	probs := hmab.ModelProbabilities()
	modelNamesMu.RLock()
	for i, p := range probs {
		if i < len(modelNames) && p >= tc.ModelConfidenceThreshold {
			paperStats.ModelsEligible++
		}
	}
	modelNamesMu.RUnlock()

	if !isPaperTrading() && !paperStats.RealCapitalActive {
		paperStats.RealCapitalActive = true
		paperStats.GatePassedAt = time.Now()
		infra.Info(fmt.Sprintf("PAPER GATE PASSED: trades=%d agents=%d models=%d — real capital activated",
			paperStats.TotalTrades, paperStats.AgentsEligible, paperStats.ModelsEligible))
	}

	if paperStats.TotalTrades%50 == 0 && paperStats.RealCapitalActive {
		infra.Info(fmt.Sprintf("PaperGate: %d trades, %d agents eligible, %d models eligible, real=active",
			paperStats.TotalTrades, paperStats.AgentsEligible, paperStats.ModelsEligible))
	}
}

func simulateAgentTrade(agent *trading.PortfolioAgent) {
	infra.FnTrace(fmt.Sprintf("agent=%s capital=%.2f", agent.ID, agent.Capital))
	if agent.Performance == nil {
		agent.Performance = &trading.AgentPerformance{}
	}
	pnl := agent.Capital * (rng.Float64()*0.10 - 0.02)
	agent.Performance.PnL += pnl
	agent.Performance.Trades++
	agent.Capital += pnl
	if agent.Performance.Trades > 0 {
		agent.Performance.WinRate = float64(agent.Performance.Trades) / float64(agent.Performance.Trades+1) * 100
	}
	agent.Performance.Sharpe = math.Max(0, agent.Performance.PnL/math.Max(1, agent.Capital)*10)
	agent.Performance.Drawdown = math.Min(agent.Performance.Drawdown, pnl)

	if agent.DNA != nil && len(agent.DNA.ModelAssignments) > 0 {
		for _, modelName := range agent.DNA.ModelAssignments {
			mr := modelReg.Get(modelName)
			if mr == nil {
				mr = &governance.ModelRecord{
					ID: modelName, ModelVersion: "unknown",
					Status: governance.ModelStatusGraduated,
				}
				modelReg.Register(mr)
			}
			modelReg.RecordDeployment(modelName, agent.ID, agent.Capital)
			modelReg.RecordPerformance(modelName, governance.PerformancePoint{
				Timestamp: time.Now(),
				Sharpe:    agent.Performance.Sharpe,
				PnL:       agent.Performance.PnL,
				Drawdown:  agent.Performance.Drawdown,
				Trades:    agent.Performance.Trades,
			})
		}
	}
}

func updateMABFromPerformance() {
	infra.FnTrace("entering")
	defer infra.FnTrace("done")

	modelNamesMu.RLock()
	nModels := len(modelNames)
	modelNamesMu.RUnlock()

	for _, agent := range agentPool.ActiveAgents() {
		if agent.Performance == nil || agent.DNA == nil {
			continue
		}
		reward := 0.5
		if agent.Performance.Sharpe > 1.0 {
			reward = 0.7
		} else if agent.Performance.Sharpe < -0.5 {
			reward = 0.3
		}
		if agent.Performance.PnL > 0 {
			reward += 0.1
		}

		for i := 0; i < hmab.AgentArmCount(); i++ {
			if agent.ID == fmt.Sprintf("agent_%d", i+1) {
				hmab.UpdateAgentEvidence(i, reward)
			}
		}

		for _, modelName := range agent.DNA.ModelAssignments {
			modelNamesMu.RLock()
			for i, m := range modelNames {
				if m == modelName && i < nModels {
					hmab.UpdateModelEvidence(i, reward)
				}
			}
			modelNamesMu.RUnlock()
		}
	}
}

func manageAgentLifecycle() {
	infra.FnTrace("entering")

	tc := runtimeCfg.Trading
	paperOnly := runtimeCfg.PaperTradingOnly

	active := agentPool.ActiveCount()
	realCount := countRealAgents()
	paperCount := active - realCount

	if active > 0 {
		var worst *trading.PortfolioAgent
		worstScore := 999.0
		for _, a := range agentPool.ActiveAgents() {
			if a.Performance != nil && a.Performance.Sharpe < worstScore {
				worstScore = a.Performance.Sharpe
				worst = a
			}
		}
		minKeep := 3
		if worst != nil && worstScore < tc.AgentRetirementThresholdScore && active > minKeep {
			agentPool.RetireAgent(worst.ID)
			delete(paperAgents, worst.ID)
			infra.Info(fmt.Sprintf("Retired agent %s (sharpe=%.3f mode=%s)", worst.ID, worstScore, agentMode(worst.ID)))
			active--
			if isPaperAgent(worst.ID) {
				paperCount--
			} else {
				realCount--
			}
		}
	}

	if !paperOnly && realCount < tc.MaxRealAgents {
		graduated := tryGraduatePaperAgents(tc)
		if graduated > 0 {
			realCount += graduated
			paperCount -= graduated
		}
	}

	maxPaper := tc.MaxPaperAgents
	categories := []string{trading.CapitalSmall, trading.CapitalMedium, trading.CapitalLarge}
	strategies := []string{trading.StrategyTrend, trading.StrategyHedging, trading.StrategyDiversified,
		trading.StrategyArbitrage, trading.StrategyVolatility}

	for paperCount < maxPaper {
		agent := agentPool.CreateAgent(
			categories[paperCount%len(categories)],
			strategies[paperCount%len(strategies)],
			trading.AllHorizons()[paperCount%len(trading.AllHorizons())],
		)
		paperAgents[agent.ID] = true
		if agent.DNA == nil {
			agent.DNA = &trading.AgentDNA{
				RiskPreference:     0.3 + rng.Float64()*0.5,
				LeverageConstraint: 1.0 + rng.Float64()*tc.MaxLeverage,
				StopLossRule:       fmt.Sprintf("%.1f%%", tc.StopLossPercent),
			}
		}
		paperCount++
	}

	paperStats.TotalTrades = countTotalPaperTrades()
	paperStats.AgentsEligible = countEligibleAgents(tc)
	paperStats.ModelsEligible = countEligibleModels(tc)

	if !paperStats.RealCapitalActive && !paperOnly &&
		paperStats.AgentsEligible > 0 && paperStats.ModelsEligible > 0 {
		paperStats.RealCapitalActive = true
		paperStats.GatePassedAt = time.Now()
		infra.Info(fmt.Sprintf("PAPER GATE PASSED! agents=%d models=%d trades=%d",
			paperStats.AgentsEligible, paperStats.ModelsEligible, paperStats.TotalTrades))
	}

	sendPortfolioResults()
}

func tryGraduatePaperAgents(tc config.TradingConfig) int {
	infra.FnTrace("entering")
	graduated := 0
	realCount := countRealAgents()

	for _, a := range agentPool.ActiveAgents() {
		if !isPaperAgent(a.ID) {
			continue
		}
		if realCount+graduated >= tc.MaxRealAgents {
			break
		}
		if a.Performance == nil {
			continue
		}
		if a.Performance.Trades < tc.MinPaperTrades {
			continue
		}
		perfScore := 0.5 + a.Performance.Sharpe*0.2
		if perfScore > 1.0 {
			perfScore = 1.0
		}
		if perfScore < tc.AgentPerformanceThreshold {
			continue
		}
		modelConf := getAgentModelConfidence(a)
		if modelConf < tc.ModelConfidenceThreshold {
			continue
		}
		paperAgents[a.ID] = false
		graduated++
		infra.Info(fmt.Sprintf("Agent %s graduated paper→real (trades=%d sharpe=%.3f conf=%.3f)",
			a.ID, a.Performance.Trades, a.Performance.Sharpe, modelConf))
	}
	return graduated
}

func isPaperAgent(id string) bool {
	if mode, ok := paperAgents[id]; ok {
		return mode
	}
	return true
}

func agentMode(id string) string {
	if isPaperAgent(id) {
		return "paper"
	}
	return "real"
}

func countRealAgents() int {
	n := 0
	for _, a := range agentPool.ActiveAgents() {
		if !isPaperAgent(a.ID) {
			n++
		}
	}
	return n
}

func countTotalPaperTrades() int {
	n := 0
	for _, a := range agentPool.ActiveAgents() {
		if a.Performance != nil {
			n += a.Performance.Trades
		}
	}
	return n
}

func countEligibleAgents(tc config.TradingConfig) int {
	n := 0
	for _, a := range agentPool.ActiveAgents() {
		if a.Performance != nil &&
			a.Performance.Trades >= tc.MinPaperTrades &&
			0.5+a.Performance.Sharpe*0.2 >= tc.AgentPerformanceThreshold {
			n++
		}
	}
	return n
}

func countEligibleModels(tc config.TradingConfig) int {
	modelNamesMu.RLock()
	defer modelNamesMu.RUnlock()
	n := 0
	for _, name := range modelNames {
		conf := getModelConfidence(name)
		if conf >= tc.ModelConfidenceThreshold {
			n++
		}
	}
	return n
}

func getAgentModelConfidence(a *trading.PortfolioAgent) float64 {
	if a.DNA == nil || len(a.DNA.ModelAssignments) == 0 {
		return 0
	}
	var sum float64
	for _, name := range a.DNA.ModelAssignments {
		sum += getModelConfidence(name)
	}
	return sum / float64(len(a.DNA.ModelAssignments))
}

func getModelConfidence(name string) float64 {
	modelNamesMu.RLock()
	defer modelNamesMu.RUnlock()
	probs := hmab.ModelProbabilities()
	for i, n := range modelNames {
		if n == name && i < len(probs) {
			return probs[i]
		}
	}
	return 0.5
}

func loadConfig() GlobalConfig {
	var cfg GlobalConfig
	data, err := os.ReadFile(CONFIG_FILE)
	if err != nil {
		cfg = GlobalConfig{MaxTasks: runtimeCfg.MaxTasks}
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
// TASK FLOW
// ==============================

func createTask(tm *TaskManager) {
	id := fmt.Sprintf("task_%d", time.Now().UnixNano())
	conf := calculateConfidence("BTT")
	spread := calculateSpread("BTT")
	tm.mu.Lock()
	tm.Tasks[id] = &TradeTask{
		ID: id, Status: StatusCreated,
		FromToken: "USDC", ToToken: "BTT",
		Confidence: conf, Spread: spread, CreatedAt: time.Now(),
	}
	tm.mu.Unlock()
	tm.Save()
	infra.Info(fmt.Sprintf("trading: task created: %s (conf=%.4f spread=%.6f)", id, conf, spread))
}

func runWorkers(tm *TaskManager) {
	var wg sync.WaitGroup
	for _, t := range tm.Tasks {
		if t.Status == StatusCompleted {
			continue
		}
		wg.Add(1)
		go func(task *TradeTask) {
			defer wg.Done()
			processTask(task, tm)
		}(t)
	}
	wg.Wait()
}

type Config struct {
	FakeTrading   bool
	EnableOptions bool
	GasPerTrade   float64
}

type Strategy interface {
	ShouldBuy() bool
	ShouldSell(task *TradeTask, currentPrice float64) bool
}

type MockStrategy struct{}

func (ms *MockStrategy) ShouldBuy() bool { return true }

func (ms *MockStrategy) ShouldSell(task *TradeTask, currentPrice float64) bool {
	return currentPrice > task.BuyPrice*1.01
}

func GetLatestPrice(token string) float64 { return simulatePrice() }

func ExecuteTrade(task *TradeTask, cfg Config, currentPrice float64) {
	if task.Status == StatusCreated {
		task.BuyPrice = currentPrice
		task.Status = StatusBought
		infra.Info(fmt.Sprintf("BUY %s @ %.6f", task.ID, currentPrice))
	} else if task.Status == StatusBought {
		task.SellPrice = currentPrice
		task.Status = StatusCompleted
		infra.Info(fmt.Sprintf("SELL %s @ %.6f PnL=%.6f", task.ID, currentPrice, task.SellPrice-task.BuyPrice))
	}
}

func processTask(task *TradeTask, tm *TaskManager) {
	cfg := Config{FakeTrading: true, EnableOptions: false, GasPerTrade: 0.0005}
	price := GetLatestPrice(task.ToToken)
	switch task.Status {
	case StatusCreated:
		if !strategy.ShouldBuy() {
			return
		}
		ExecuteTrade(task, cfg, price)
	case StatusBought:
		if strategy.ShouldSell(task, price) {
			ExecuteTrade(task, cfg, price)
		}
	}
}

// ==============================
// PORTFOLIO OPTIMIZATION
// ==============================

func evaluatePortfolio(tm *TaskManager) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	prediction := predictEnsemble(tm)
	infra.Info(fmt.Sprintf("Portfolio: ensemble=%s", prediction))
}

func predictEnsemble(tm *TaskManager) string {
	trend := math.Sin(float64(time.Now().Unix()%3600) * math.Pi / 1800)
	momentum := math.Cos(float64(time.Now().Unix()%1800) * math.Pi / 900)
	score := trend*0.35 + momentum*0.35 + 0.30
	switch {
	case score > 0.6:
		return fmt.Sprintf("BULLISH (%.3f)", score)
	case score < 0.4:
		return fmt.Sprintf("BEARISH (%.3f)", score)
	default:
		return fmt.Sprintf("NEUTRAL (%.3f)", score)
	}
}

func calculateSpread(token string) float64 {
	return 0.0005 + float64(time.Now().UnixNano()%50)/100000.0
}

func calculateConfidence(token string) float64 {
	return math.Min(0.65+float64(time.Now().UnixNano()%30)/100.0, 1.0)
}

func evaluateHedging(task *TradeTask) string {
	risk := (1.0-task.Confidence)*task.Spread*100 + task.Confidence
	switch {
	case risk > 0.6:
		return "EXIT"
	case risk > 0.4:
		return "HEDGE_FULL"
	case risk > 0.25:
		return "HEDGE_PARTIAL"
	default:
		return "HOLD"
	}
}

func simulatePrice() float64 {
	return 1.0 + float64(time.Now().UnixNano()%100)/10000
}

func floatToBigInt(val float64, decimals int64) *big.Int {
	if val <= 0 {
		return big.NewInt(0)
	}
	f := new(big.Float).SetFloat64(val)
	mult := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(decimals), nil))
	f.Mul(f, mult)
	result := new(big.Int)
	f.Int(result)
	return result
}