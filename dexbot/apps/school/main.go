/******************************************************************************
 * File Name       : main.go
 * File Path       : apps/school/main.go
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
 *   This daemon is responsible for managing the database, collecting and recording market data, and providing historical data to other daemons. It also monitors the database service and attempts to recove
 *
 * Responsibilities:
 *   - Implement core functionality for apps package.
 *
 * Usage :
 *   Directory : apps/school/
 *
 *   Build :
 *     go build ./apps/school
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./apps/school
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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"dexbot/config"
	"dexbot/governance"
	"dexbot/infra"
	"dexbot/school"
	"dexbot/tokens"

	"math/rand"
	"sort"
)

// ==============================
// CONSTANTS
// ==============================

const (
	CONFIG_FILE        = "config.json"      // Configuration file for the school daemon
	STATE_FILE         = "tasks_state.json" // Placeholder for state (will be replaced by DB for market data)
	UDP_GOVERNANCE_PORT = 8081              // Default UDP port for governance (used by tests)
	UDP_SCHOOL_PORT     = 8082              // Default UDP port for School daemon (used by tests)
	UDP_TRADING_PORT    = 8083              // Default UDP port for Trading daemon (used by tests)
)

// ==============================
// TYPES
// ==============================

// GlobalConfig holds the school daemon's configuration parameters.
type GlobalConfig struct {
	MarketDataRecordIntervalMinutes    int `json:"market_data_record_interval_minutes"`     // Interval for recording market data in minutes
	DatabaseHealthCheckIntervalSeconds int `json:"database_health_check_interval_seconds"` // Interval for checking DB health in seconds
}

// MarketData represents a single record of cryptocurrency market data.
type MarketData struct {
	Timestamp time.Time `json:"timestamp"` // Timestamp of the data record
	Symbol    string    `json:"symbol"`    // Cryptocurrency symbol (e.g., "BNB", "BTC")
	Price     float64   `json:"price"`     // Price of the cryptocurrency
	Volume    float64   `json:"volume"`    // Trading volume of the cryptocurrency
}

// TaskStatus represents the current status of a trading task (for compatibility, to be adapted).
type TaskStatus string

const (
	StatusCreated   TaskStatus = "CREATED"
	StatusBought    TaskStatus = "BOUGHT"
	StatusCompleted TaskStatus = "COMPLETED"
)

// TradeTask represents a single trading operation (for compatibility, to be adapted).
type TradeTask struct {
	ID        string
	Status    TaskStatus
	FromToken string
	ToToken   string
	BuyPrice  float64
	SellPrice float64
	CreatedAt time.Time
}

// TaskManager manages the lifecycle and persistence of trading tasks (for compatibility, to be adapted).
type TaskManager struct {
	mu    sync.Mutex
	Tasks map[string]*TradeTask
}

// NewTaskManager creates and returns a new TaskManager instance.
func NewTaskManager() *TaskManager {
	return &TaskManager{
		Tasks: make(map[string]*TradeTask),
	}
}

// Save persists the current state of trading tasks to a JSON file (placeholder).
func (tm *TaskManager) Save() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	data, _ := json.MarshalIndent(tm, "", "  ")
	_ = os.WriteFile(STATE_FILE, data, 0644)
	infra.Info("school state saved to disk")
}

// Load loads the previous state of trading tasks from a JSON file (placeholder).
func (tm *TaskManager) Load() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	data, err := os.ReadFile(STATE_FILE)
	if err == nil {
		_ = json.Unmarshal(data, tm)
		infra.Info("school state loaded from disk")
	} else {
		infra.Warn("school: no previous state file")
	}
}

// ==============================
// GLOBAL VARIABLES & INITIALIZATION
// ==============================

var ( // Global variables for school daemon
	udpConn          *net.UDPConn             // UDP connection for sending status to Governance
	governanceAddr   *net.UDPAddr             // Address of the Governance daemon
	governanceAddrStr string                  // §66: configurable governance address
	schoolListenAddr  string                  // §66: configurable school listen address
	appCfg           *infra.AppConfig         // Central config from config.env
	runtimeCfg       config.RuntimeConfig     // Full runtime config (Phase 11)
	governancePort   int                      // Resolved governance UDP port
	schoolPort       int                      // Resolved school UDP port
	startTime        = time.Now()             // Daemon start time for uptime calculation

	// GA + Model (Phase 11)
	gaEngine     *school.GAEngine       // Genetic Algorithm engine
	modelPop     *school.ModelPopulation // Full model population
	gaRng        = rand.New(rand.NewSource(time.Now().UnixNano()))
	orch         *school.Orchestrator   // Training orchestrator (Phase 12)
	remoteClient *school.RemoteClient   // Remote school client (Phase 12)

	// Model Registry (§33 — Phase 19)
	modelReg *governance.ModelRegistry // Centralized model lifecycle registry
)

// ==============================
// CLI ENTRY POINT
// ==============================

/*
Function: main
Description:
  Dispatches CLI commands or starts the daemon loop.
  Supports -action=fetchMarket and -action=fetchDB for data inspection,
  otherwise runs as a background daemon.
Input:
  - none (uses os.Args)
Output:
  - none
Lines: ~20
*/
func main() {
	fs := flag.NewFlagSet("school", flag.ContinueOnError)
	action := fs.String("action", "start", "Action: start, fetchMarket, fetchDB")
	_ = fs.Parse(os.Args[1:])

	if *action == "fetchMarket" || *action == "fetchDB" {
		infra.InitLogger()
		loadEnvSmart()
		switch *action {
		case "fetchMarket":
			displayLastMarketData(5)
		case "fetchDB":
			displayLastDBRecords(5)
		}
		return
	}

	// Default: run daemon
	startDaemon()
}

// ==============================

// CLI ACTION HANDLERS
// ==============================

/*
Function: displayLastMarketData
Description:
  Prints the last N market data records (simulated real-market fetch) to stdout.
Input:
  - n int: Number of records to display.
Output:
  - none (prints to stdout)
Lines: ~15
*/
func displayLastMarketData(n int) {
	fmt.Printf("\n=== Last %d Market Data Records (Real Market) ===\n\n", n)
	records := getPastMarketData(n)
	for i, r := range records {
		fmt.Printf("  %d. [%s] %-6s  Price: %12.6f  Volume: %14.2f\n",
			i+1, r.Timestamp.Format("2006-01-02 15:04:05"), r.Symbol, r.Price, r.Volume)
	}
	fmt.Println("\n=== End of Market Data ===")
}

/*
Function: displayLastDBRecords
Description:
  Prints the last N database records (simulated DB fetch) to stdout.
Input:
  - n int: Number of records to display.
Output:
  - none (prints to stdout)
Lines: ~15
*/
func displayLastDBRecords(n int) {
	fmt.Printf("\n=== Last %d Database Records ===\n\n", n)
	records := getPastDatabaseRecords(n)
	for i, r := range records {
		fmt.Printf("  %d. [%s] %-6s  Price: %12.6f  Volume: %14.2f\n",
			i+1, r.Timestamp.Format("2006-01-02 15:04:05"), r.Symbol, r.Price, r.Volume)
	}
	fmt.Println("\n=== End of Database Records ===")
}

// ==============================
// DAEMON STARTUP
// ==============================

/*
Function: startDaemon
Description:
  Initializes the School daemon, starts background loops, and blocks until shutdown.
Input:
  - none
Output:
  - none
Lines: ~30
*/
func startDaemon() {
	infra.Info("School Daemon starting...")

	// Phase 14: restore from checkpoint if available
	restoreSchoolCheckpoint()

	// Load central config
	var err error
	appCfg, err = infra.LoadConfig()
	if err != nil {
		infra.Error("School: failed to load config: " + err.Error())
		// Fallback to hardcoded defaults
		governancePort = 8081
		schoolPort = 8082
	} else {
		governancePort = appCfg.GovernanceUDPPort
		schoolPort = appCfg.SchoolUDPPort
	}

	// §66: Load configurable addresses from env (config.env already loaded)
	if v := os.Getenv("GOVERNANCE_ADDR"); v != "" {
		governanceAddrStr = v
	}
	if governanceAddrStr == "" {
		governanceAddrStr = "127.0.0.1"
	}
	if v := os.Getenv("SCHOOL_ADDR"); v != "" {
		schoolListenAddr = v
	}
	if schoolListenAddr == "" {
		schoolListenAddr = "127.0.0.1"
	}

	initUDP()

	// Load full runtime config
	runtimeCfg = config.Defaults()
	runtimeCfg.LoadFromEnv()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := loadSchoolConfig()

	// Initialize model population + GA engine (Phase 11)
	// §33: Initialize centralized model registry
	modelReg = governance.NewModelRegistry()
	initGAPopulation()

	// Immediately sync models to Governance so the Training dashboard has data
	time.Sleep(2 * time.Second) // let governance UDP listener start first
	sendModelSyncToGovernance()

	go startDatabaseHealthCheckLoop(cfg.DatabaseHealthCheckIntervalSeconds)
	go startMarketDataRecordingLoop(cfg.MarketDataRecordIntervalMinutes)
	go startGALoop(ctx)
	go startUdpListener(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	<-sigChan

	infra.Info("School Daemon shutting down...")
	// Phase 14: checkpoint before shutdown
	saveSchoolCheckpoint()
	if udpConn != nil {
		udpConn.Close()
	}
}

/*
Struct: schoolCheckpoint
Description:
  Serializable school state for checkpoint/restore.

Fields:
  - Version         string : Checkpoint version
  - ModelsCount     int    : Number of models in population
  - GraduatesCount  int    : Number of graduate models
  - Ports           [3]int : gov, school, trading ports

Lines: ~5
*/
type schoolCheckpoint struct {
	Version        string `json:"version"`
	ModelsCount    int    `json:"models_count"`
	GraduatesCount int    `json:"graduates_count"`
	Ports          [2]int `json:"ports"`
}

func saveSchoolCheckpoint() {
	state := schoolCheckpoint{
		Version: "v1.4",
		Ports:   [2]int{governancePort, schoolPort},
	}
	if modelPop != nil {
		state.ModelsCount = modelPop.Count()
		state.GraduatesCount = modelPop.GraduateCount()
	}
	if err := infra.SaveCheckpoint("school", &state); err != nil {
		infra.Error("School: checkpoint save failed: " + err.Error())
	} else {
		infra.Info("School: checkpoint saved")
	}
}

func restoreSchoolCheckpoint() {
	var state schoolCheckpoint
	if err := infra.RestoreCheckpoint("school", &state); err != nil {
		infra.Info("School: no checkpoint to restore (fresh start)")
		return
	}
	infra.Info(fmt.Sprintf("School: checkpoint restored (v=%s models=%d grads=%d)",
		state.Version, state.ModelsCount, state.GraduatesCount))
	if state.Ports[0] > 0 {
		governancePort = state.Ports[0]
	}
	if state.Ports[1] > 0 {
		schoolPort = state.Ports[1]
	}
}

// ==============================
// INITIALIZATION
// ==============================

// ==============================
// GA MODEL EVOLUTION (Phase 11)
// ==============================

/*
Function: initGAPopulation
Description:
  Seeds the model population with mock models across all 8 categories.
  Sizes and thresholds come from config.SchoolConfig.

Lines: ~40
*/
func initGAPopulation() {
	modelPop = school.NewModelPopulation()
	categories := []string{
		school.CategoryOptions, school.CategoryRisk, school.CategoryIntraday,
		school.CategorySwing, school.CategoryLongTerm, school.CategoryVolatility,
		school.CategoryLiquidity, school.CategoryPortfolio,
	}
	architectures := []string{"LSTM", "GRU", "Transformer", "XGBoost", "RandomForest", "CNN"}

	perCat := runtimeCfg.School.GAPopulationSize / len(categories)
	for _, cat := range categories {
		for i := 0; i < perCat; i++ {
			m := &school.ModelMetadata{
				Name:        fmt.Sprintf("%s_%s_%d", archName(cat), arch(architectures, i), i),
				Version:     "v0.1",
				Category:    cat,
				Status:      school.StatusTraining,
				Generation:  0,
				CreatedAt:   time.Now(),
				Architecture: architectures[i%len(architectures)],
				Hyperparameters: map[string]string{
					"lr":     fmt.Sprintf("%.4f", 0.001+gaRng.Float64()*0.1),
					"layers": fmt.Sprintf("%d", 2+gaRng.Intn(4)),
				},
				EnsembleComposition: randomEnsemble(gaRng),
				Fitness: &school.FitnessHistory{
					SharpeRatio:        0.2 + gaRng.Float64()*2.5,
					SortinoRatio:       0.2 + gaRng.Float64()*2.5,
					Profit:             gaRng.Float64()*0.4 - 0.2,
					Drawdown:           gaRng.Float64() * 0.3,
					PredictionAccuracy: 40 + gaRng.Float64()*45,
					Consistency:        gaRng.Float64(),
					CapitalEfficiency:  gaRng.Float64(),
					Timestamp:          time.Now(),
				},
		}
		modelPop.AddModel(m)
		// §33: Register in centralized model registry
		registerModelInRegistry(m)
	}
}
	infra.Info(fmt.Sprintf("GA population seeded: %d models across %d categories",
		modelPop.Count(), len(categories)))

	sc := runtimeCfg.School
	gaEngine = school.NewGA(school.GAConfig{
		PopulationSize:      sc.GAPopulationSize,
		TopSurvivors:        sc.GATopSurvivors,
		MutationRate:        sc.GAMutationRate,
		CrossoverRate:       sc.GACrossoverRate,
		GenerationsPerCycle: sc.GAGenerationsPerCycle,
		GraduateTopN:        sc.GAGraduateTopN,
		RetireBottomN:       sc.GARetireBottomN,
		GraduationThreshold: sc.ModelGraduationThreshold,
		RetirementThreshold: sc.ModelRetirementThreshold,
	}, modelPop, gaRng)

	// Setup remote school client + orchestrator (Phase 12)
	remoteAddrs := parseRemoteAddrs(runtimeCfg.School.RemoteAddresses)
	remoteClient = school.NewRemoteClient(remoteAddrs, runtimeCfg.School.RemoteTimeoutSeconds, runtimeCfg.School.RemoteStudentsPerNode)
	orch = school.NewOrchestrator(gaEngine, remoteClient, modelPop)
	if remoteClient.IsEnabled() {
		infra.Info(fmt.Sprintf("Remote schools configured: %d nodes, %d students/node",
			remoteClient.NodeCount(), runtimeCfg.School.RemoteStudentsPerNode))
	} else {
		infra.Info("No remote schools configured — all training local")
	}
}

/*
Function: startGALoop
Description:
  Periodically runs GA evolution cycles per config.GACycleIntervalMinutes.

Lines: ~20
*/
func startGALoop(ctx context.Context) {
	interval := time.Duration(runtimeCfg.School.GACycleIntervalMinutes) * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	infra.Info(fmt.Sprintf("GA evolution loop started (interval=%v, remote=%v)", interval, remoteClient.IsEnabled()))

	for {
		select {
		case <-ctx.Done():
			infra.Info("GA evolution loop shutting down.")
			return
		case <-ticker.C:
			infra.FnTrace("GA cycle tick")
			w := school.DefaultFitnessWeights()
			summary := orch.RunCycle(w)
			infra.Info(fmt.Sprintf("GA cycle complete: %s", summary))
			// §33: Sync model registry (graduates/retirees)
			syncModelRegistry()
			// Send model registry data to Governance for dashboard display
			sendModelSyncToGovernance()
			// Send graduates to Trading daemon (§17)
			sendGraduates()
		}
	}
}

/*
Function: parseRemoteAddrs
Description:
  Parses SCHOOL_REMOTE_ADDRESSES (comma-separated ip:port) into a string slice.
  Returns empty slice if value is empty.

Lines: ~10
*/
func parseRemoteAddrs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func archName(cat string) string {
	m := map[string]string{
		school.CategoryOptions: "Opt", school.CategoryRisk: "Risk",
		school.CategoryIntraday: "Intra", school.CategorySwing: "Swing",
		school.CategoryLongTerm: "Long", school.CategoryVolatility: "Vol",
		school.CategoryLiquidity: "Liq", school.CategoryPortfolio: "Port",
	}
	if s, ok := m[cat]; ok {
		return s
	}
	return "M"
}

func arch(arches []string, i int) string { return arches[i%len(arches)] }

func randomEnsemble(rng *rand.Rand) map[string]float64 {
	subs := []string{"SVM", "ARIMA", "LSTM", "RL", "BS"}
	n := 2 + rng.Intn(3)
	out := make(map[string]float64)
	sum := 0.0
	for i := 0; i < n; i++ {
		w := rng.Float64()
		out[subs[i]] = w
		sum += w
	}
	for k := range out {
		out[k] /= sum
	}
	return out
}

/*
Function: registerModelInRegistry
Description:
  Registers a school.ModelMetadata in the centralized governance.ModelRegistry.
  Per myreq3.txt §33 — model versioning is independent of software versioning.

Input:
  - m *school.ModelMetadata : Model to register

Output:
  - none

Lines: ~20
*/
func registerModelInRegistry(m *school.ModelMetadata) {
	rec := &governance.ModelRecord{
		ID:           m.Name,
		ModelVersion: fmt.Sprintf("v%d.%d", m.Generation, gaRng.Intn(99)),
		Generation:   m.Generation,
		Category:     m.Category,
		Architecture: m.Architecture,
		Framework:    "Go/GA",
		Status:       governance.ModelStatusExperimental,
		Hyperparameters: m.Hyperparameters,
		FeatureSet:      m.FeatureSet,
		TrainingDataset: "mock_v1",
		CreatedAt:       m.CreatedAt,
	}
	if m.Fitness != nil {
		rec.FitnessScores = append(rec.FitnessScores, governance.FitnessSnapshot{
			Sharpe:     m.Fitness.SharpeRatio,
			Sortino:    m.Fitness.SortinoRatio,
			Profit:     m.Fitness.Profit,
			Drawdown:   m.Fitness.Drawdown,
			Accuracy:   m.Fitness.PredictionAccuracy,
			Consistency: m.Fitness.Consistency,
			Efficiency: m.Fitness.CapitalEfficiency,
			Generation: m.Generation,
			Timestamp:  m.Fitness.Timestamp,
		})
	}
	if m.EnsembleComposition != nil {
		rec.Ensemble = &governance.EnsembleDef{
			Type:          "voting",
			VotingWeights: m.EnsembleComposition,
			Confidence:    0.5,
		}
	}
	modelReg.Register(rec)
}

/*
Function: syncModelRegistry
Description:
  Syncs the ModelRegistry with the current ModelPopulation state.
  Graduates models that have been promoted and retires those abandoned.
  Called after each GA cycle.

Input:
  - none

Output:
  - none

Lines: ~20
*/
func syncModelRegistry() {
	grads := modelPop.Graduates()
	for _, g := range grads {
		if err := modelReg.Graduate(g.Name); err != nil {
			// Model may not be in registry yet — register it
			registerModelInRegistry(g)
			modelReg.Graduate(g.Name)
		}
	}
	// Report counts
	infra.Info(fmt.Sprintf("ModelRegistry: total=%d graduated=%d retired=%d",
		modelReg.Count(), modelReg.CountByStatus(governance.ModelStatusGraduated),
		modelReg.CountByStatus(governance.ModelStatusRetired)))
}

// init initializes the logger and loads environment variables.
func init() {
	infra.InitLogger()
	loadEnvSmart()
}

/*
Function: initUDP
Description:
  Initializes the UDP connection to the Governance daemon using ports from AppConfig.
Input:
  - none
Output:
  - none
Lines: ~15
*/
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
	infra.Info(fmt.Sprintf("School UDP sender initialized, sending to %s:%d", govAddr, governancePort))
}

// ==============================
// UTILITY FUNCTIONS
// ==============================

/*
Function: loadEnvSmart
Description:
  Attempts to load environment variables from `config.env` by checking multiple predefined paths.
  This ensures flexibility in deployment location for the daemon.
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
		"../../../config.env", // For apps/school/main.go
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
Function: loadSchoolConfig
Description:
  Loads the configuration specific to the School daemon from `config.json`.
  If the file is not found, it initializes with default values and writes them to the file.
Input:
  - none
Output:
  - GlobalConfig: The loaded or default configuration.
Lines: ~15
*/
func loadSchoolConfig() GlobalConfig {
	var cfg GlobalConfig
	data, err := os.ReadFile(CONFIG_FILE)
	if err != nil {
		cfg = GlobalConfig{
			MarketDataRecordIntervalMinutes:    15,
			DatabaseHealthCheckIntervalSeconds: 30,
		}
		writeSchoolConfig(cfg)
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

/*
Function: writeSchoolConfig
Description:
  Writes the current School daemon configuration to `config.json`.
Input:
  - cfg GlobalConfig: The configuration to be written.
Output:
  - none
Lines: ~5
*/
func writeSchoolConfig(cfg GlobalConfig) {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(CONFIG_FILE, data, 0644)
}

// ==============================
// UDP LISTENER
// ==============================

/*
Function: startUdpListener
Description:
  Starts a UDP server for the School daemon to listen for incoming commands or queries from
  the Governance daemon. It continuously reads messages and processes them accordingly.
  Specifically, it responds to "governance:probe:health_check" with "school:pong:healthy".
Input:
  - ctx context.Context: Context for graceful shutdown.
Output:
  - none (runs as a goroutine)
Lines: ~55
*/
func startUdpListener(ctx context.Context) {
	listenAddr := schoolListenAddr
	if listenAddr == "" {
		listenAddr = "127.0.0.1"
	}
	addr := net.UDPAddr{
		Port: schoolPort,
		IP:   net.ParseIP(listenAddr),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		infra.Error("School: Failed to start UDP listener: " + err.Error())
		return
	}
	defer conn.Close()
	infra.Info(fmt.Sprintf("School UDP listener started on %s:%d", listenAddr, schoolPort))

	buffer := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			infra.Info("School UDP listener shutting down.")
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, remoteAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				infra.Error("School: UDP read error: " + err.Error())
				continue
			}
			message := string(buffer[:n])
			infra.Info("School: Received UDP message: " + message)

			// Trading feedback reception (§23)
			if strings.HasPrefix(message, "trading:portfolio:") {
				handleTradingFeedback(message)
				continue
			}

			// Handle governance commands
			if strings.HasPrefix(message, "governance:command:") {
				handleSchoolCommand(message, conn, remoteAddr)
				continue
			}

			if strings.HasPrefix(message, "governance:config:reload") {
				infra.Info("School: received config reload command")
				infra.ReloadLoggerConfig()
				continue
			}

			if strings.HasPrefix(message, "governance:probe:health_check") {
				response := "school:pong:healthy"
				_, err := conn.WriteToUDP([]byte(response), remoteAddr)
				if err != nil {
					infra.Error("School: Failed to send health probe response: " + err.Error())
				}
				infra.Info("School: Sent health probe response: " + response)
			}
		}
	}
}

/*
Function: handleSchoolCommand
Description:
  Processes governance commands received via UDP.

Input:
  - msg        string      : Full UDP message
  - conn       *net.UDPConn: UDP connection for response
  - remoteAddr *net.UDPAddr: Sender address

Output:
  - none

Lines: ~30
*/
func handleSchoolCommand(msg string, conn *net.UDPConn, remoteAddr *net.UDPAddr) {
	switch {
	case strings.Contains(msg, "kill"):
		infra.Warn("School: received kill command — exiting immediately")
		conn.WriteToUDP([]byte("school:ack:killed"), remoteAddr)
		os.Exit(1)
	case strings.Contains(msg, "stop"):
		infra.Warn("School: received stop command — shutting down gracefully")
		conn.WriteToUDP([]byte("school:ack:stopping"), remoteAddr)
		os.Exit(0)
	case strings.Contains(msg, "restart"):
		infra.Warn("School: received restart command — restarting")
		conn.WriteToUDP([]byte("school:ack:restarting"), remoteAddr)
		os.Exit(0)
	default:
		infra.Warn("School: unknown command: " + msg)
	}
}

/*
Function: handleTradingFeedback
Description:
  Receives portfolio performance results from Trading daemon (§23).
  Format: "trading:portfolio:modelName,sharpe,profit,drawdown,trades"

Input:
  - msg string: UDP message from Trading

Output:
  - none

Lines: ~20
*/
func handleTradingFeedback(msg string) {
	infra.FnTrace("entering")
	payload := strings.TrimPrefix(msg, "trading:portfolio:")
	parts := strings.Split(payload, ",")
	if len(parts) < 5 {
		infra.Warn("School: malformed trading feedback: " + msg)
		return
	}
	modelName := strings.TrimSpace(parts[0])
	var sharpe, profit, drawdown float64
	var trades int
	fmt.Sscanf(parts[1], "%f", &sharpe)
	fmt.Sscanf(parts[2], "%f", &profit)
	fmt.Sscanf(parts[3], "%f", &drawdown)
	fmt.Sscanf(parts[4], "%d", &trades)

	// Update model fitness with real-world evidence
	m := modelPop.Get(modelName)
	if m == nil {
		infra.Warn("School: trading feedback for unknown model: " + modelName)
		return
	}
	if m.Fitness == nil {
		m.Fitness = &school.FitnessHistory{Timestamp: time.Now()}
	}
	m.Fitness.SharpeRatio = (m.Fitness.SharpeRatio + sharpe) / 2
	m.Fitness.Profit = (m.Fitness.Profit + profit) / 2
	m.Fitness.Drawdown = (m.Fitness.Drawdown + drawdown) / 2
	// Append to timeline
	m.FitnessTimeline = append(m.FitnessTimeline, *m.Fitness)
	if len(m.FitnessTimeline) > 20 {
		m.FitnessTimeline = m.FitnessTimeline[len(m.FitnessTimeline)-20:]
	}
	infra.Info(fmt.Sprintf("School: trading feedback applied to %s: sharpe=%.4f profit=%.4f trades=%d",
		modelName, sharpe, profit, trades))
}

/*
Function: sendGraduates
Description:
  Sends the list of graduate model names to the Trading daemon via UDP.
  Format: "school:graduate:model1,model2,..."

Input:
  - none

Output:
  - none

Lines: ~15
*/
func sendGraduates() {
	if udpConn == nil {
		return
	}
	grads := modelPop.Graduates()
	if len(grads) == 0 {
		return
	}
	names := make([]string, 0, len(grads))
	for _, g := range grads {
		names = append(names, g.Name)
	}
	msg := "school:graduate:" + strings.Join(names, ",")
	udpConn.Write([]byte(msg))
	infra.Info(fmt.Sprintf("School: sent %d graduates to Trading: %s", len(names), msg))
}

/*
Function: sendModelSyncToGovernance
Description:
  Sends the full model registry state to Governance as a JSON payload over UDP.
  Protocol: "model:sync:{json}" where json is a compact model summary list.
  Called after each GA cycle so the Training dashboard page shows real models.

Input: none
Output: none
Lines: ~40
*/
func sendModelSyncToGovernance() {
	if udpConn == nil || modelReg == nil {
		return
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

	// Collect: all graduates + top 15 by fitness (avoids UDP overflow)
	seen := make(map[string]bool)
	var summaries []modelSummary

	addSummary := func(rec *governance.ModelRecord) {
		if rec == nil || seen[rec.ID] {
			return
		}
		seen[rec.ID] = true
		s := modelSummary{
			ID:           rec.ID,
			Version:      rec.ModelVersion,
			Category:     rec.Category,
			Architecture: rec.Architecture,
			Status:       rec.Status,
			Generation:   rec.Generation,
		}
		if fs := rec.LatestFitness(); fs != nil {
			s.Sharpe = fs.Sharpe
			s.Consistency = fs.Consistency
			s.Profit = fs.Profit
		}
		summaries = append(summaries, s)
	}

	// Always send graduates
	for _, id := range modelReg.AllIDs() {
		rec := modelReg.Get(id)
		if rec != nil && (rec.Status == governance.ModelStatusGraduated || rec.Status == governance.ModelStatusActive) {
			addSummary(rec)
		}
	}

	// Fill remaining slots with top-fitness models (up to 20 total)
	type scored struct {
		id  string
		val float64
	}
	var ranked []scored
	for _, id := range modelReg.AllIDs() {
		if seen[id] {
			continue
		}
		rec := modelReg.Get(id)
		if rec == nil {
			continue
		}
		val := 0.0
		if fs := rec.LatestFitness(); fs != nil {
			val = fs.Sharpe
		}
		ranked = append(ranked, scored{id, val})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].val > ranked[j].val })

	maxTotal := 20
	for _, r := range ranked {
		if len(summaries) >= maxTotal {
			break
		}
		addSummary(modelReg.Get(r.id))
	}

	payload, err := json.Marshal(map[string]interface{}{"models": summaries})
	if err != nil {
		infra.Error("School: failed to marshal model sync: " + err.Error())
		return
	}

	msg := "model:sync:" + string(payload)
	_, err = udpConn.Write([]byte(msg))
	if err != nil {
		infra.Error("School: failed to send model sync: " + err.Error())
	} else {
		infra.Info(fmt.Sprintf("School: sent %d models to Governance (graduates=%d, top=%d)",
			len(summaries), modelReg.CountByStatus(governance.ModelStatusGraduated), len(summaries)))
	}
}

/*
Function: sendStatusToGovernance
Description:
  Sends a structured status message from the School daemon to the Governance daemon via UDP.
  The message format is "daemon_type:status:message", where daemon_type can be "school" or "database".
Input:
  - daemonType string: The type of daemon/service reporting status (e.g., "school", "database").
  - status string: The health status (e.g., "healthy", "unhealthy", "critical").
  - message string: A descriptive message about the status.
Output:
  - none
Lines: ~15
*/
func sendStatusToGovernance(daemonType, status, message string) {
	if udpConn == nil {
		infra.Error("School: UDP connection not initialized.")
		return
	}
	fullMessage := fmt.Sprintf("%s:%s:%s", daemonType, status, message)
	_, err := udpConn.Write([]byte(fullMessage))
	if err != nil {
		infra.Error("School: Failed to send UDP status to governance: " + err.Error())
	}
}

/*
Function: collectResourceMetrics
Description:
  Mock resource metrics collection. Returns CPU percent, memory MB, and storage MB.
  In production, this would read /proc or use a system library.

Input:
  - none

Output:
  - cpu     float64 : CPU utilization percent (0-100)
  - mem     float64 : Memory usage in MB
  - storage float64 : Storage usage in MB

Lines: ~8
*/
func collectResourceMetrics() (float64, float64, float64) {
	return 5.0 + float64(time.Now().UnixNano()%500)/100.0, // CPU 5-10%
		128.0 + float64(time.Now().UnixNano()%128),          // Memory 128-256MB
		1024.0                                                // Storage ~1GB
}

/*
Function: sendHeartbeat
Description:
  Sends a full heartbeat message (8-field format) to the Governance daemon.

Input:
  - none

Output:
  - none

Lines: ~15
*/
func sendHeartbeat() {
	if udpConn == nil {
		return
	}
	cpu, mem, storage := collectResourceMetrics()
	info := &governance.DaemonInfo{
		Name:           "school",
		Version:        "v1.4",
		Status:         "healthy",
		CPUPercent:     cpu,
		MemoryMB:       mem,
		StorageMB:      storage,
		ActiveTasks:    0,
		Uptime:         time.Since(startTime),
		Message:        "School daemon running",
		LastHeartbeat:  time.Now(),
	}
	raw := governance.FormatHeartbeat(info)
	udpConn.Write([]byte(raw))
}

// ==============================
// DATABASE HEALTH CHECK
// ==============================

/*
Function: startDatabaseHealthCheckLoop
Description:
  Periodically checks the health of the database service. If the database is unhealthy,
  it attempts to recompose it using Docker Compose. It reports the database status
  to the Governance daemon.
Input:
  - intervalSeconds int: The interval in seconds between health checks.
Output:
  - none (runs as a goroutine)
Lines: ~45
*/
func startDatabaseHealthCheckLoop(intervalSeconds int) {
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		dbHealthy := infra.CheckDBHealth() == nil
		if dbHealthy {
			infra.Info("Database is healthy.")
			sendStatusToGovernance("database", "healthy", "Database is healthy.")
			sendStatusToGovernance("school", "healthy", "School daemon is running and DB is healthy.")
			sendHeartbeat()
		} else {
			infra.Warn("Database is NOT healthy. Attempting to recompose via Docker Compose...")
			sendStatusToGovernance("database", "unhealthy", "Database is unhealthy. Attempting recompose.")
			sendStatusToGovernance("school", "unhealthy", "School daemon is running but DB is unhealthy, attempting recompose.")
			recomposeDatabase()
			if infra.CheckDBHealth() == nil {
				infra.Info("Database is now healthy after recompose.")
				sendStatusToGovernance("database", "healthy", "Database is healthy after recompose.")
				sendStatusToGovernance("school", "healthy", "School daemon is running and DB is healthy after recompose.")
			} else {
				infra.Error("Database still unhealthy after recompose. Falling back to local training files.")
				sendStatusToGovernance("database", "critical", "Database critical, using local files.")
				sendStatusToGovernance("school", "critical", "School daemon critical, DB unavailable, using local files.")
			}
		}
	}
}

/*
Function: recomposeDatabase
Description:
  Executes a `docker-compose up -d db` command to restart or recreate the database service.
  It assumes the `docker-compose.yml` file is located in the project root (`dexbot/`).
Input:
  - none
Output:
  - none
Lines: ~15
*/
var recomposeDatabase = func() {
	infra.Info("Attempting to recompose database via Docker Compose...")
	cmd := exec.Command("docker-compose", "-f", "../../../docker-compose.yml", "up", "-d", "db")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		infra.Error("Failed to run docker-compose: " + err.Error())
	}
}

// ==============================
// MARKET DATA RECORDING
// ==============================

/*
Function: startMarketDataRecordingLoop
Description:
  Starts a periodic loop that records market data at a configurable interval.
Input:
  - intervalMinutes int: The interval in minutes between market data recordings.
Output:
  - none (runs as a goroutine)
Lines: ~15
*/
func startMarketDataRecordingLoop(intervalMinutes int) {
	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		infra.Info("Recording market data...")
		recordMarketData()
		sendStatusToGovernance("school", "healthy", "School daemon recorded market data.")
		sendHeartbeat()
	}
}

/*
Function: recordMarketData
Description:
  Fetches (simulated) and saves market data for a predefined list of tokens (BNB, BTC)
  and tokens from `dexbot/tokens/tokens.go`. In a real scenario, this would interact
  with external market APIs and persist data to the database.
Input:
  - none
Output:
  - none
Lines: ~30
*/
func recordMarketData() {
	targetTokens := []string{"BNB", "BTC"}

	for sym, _ := range tokens.Tokens {
		targetTokens = append(targetTokens, sym)
	}

	for _, symbol := range targetTokens {
		price := simulatePrice()
		volume := 1000000.0 + float64(time.Now().UnixNano()%1000000)

		marketData := MarketData{
			Timestamp: time.Now(),
			Symbol:    symbol,
			Price:     price,
			Volume:    volume,
		}

		infra.Info(fmt.Sprintf("Recorded market data for %s: Price=%.6f, Volume=%.2f", symbol, marketData.Price, marketData.Volume))
	}
}

// ==============================
// DATA FETCHING (MOCK)
// ==============================

/*
Function: simulatePrice
Description:
  Generates a simulated cryptocurrency price for demonstration and testing purposes.
Input:
  - none
Output:
  - float64: A simulated price value.
Lines: ~5
*/
var simulatePrice = func() float64 {
	return 1.0 + float64(time.Now().UnixNano()%100)/10000
}

/*
Function: getPastMarketData
Description:
  Fetches the last N market data records from a real market source (simulated for now).
  In a full implementation, this would interact with external APIs.
Input:
  - n int: The number of past market data records to retrieve.
Output:
  - []MarketData: A slice of `MarketData` containing the requested records.
Lines: ~15
*/
func getPastMarketData(n int) []MarketData {
	infra.Info(fmt.Sprintf("Fetching last %d real market data records... (simulated)", n))
	data := []MarketData{}
	for i := 0; i < n; i++ {
		data = append(data, MarketData{
			Timestamp: time.Now().Add(-time.Duration(i) * time.Hour),
			Symbol:    "BNB",
			Price:     300.0 + float64(i),
			Volume:    100000.0 + float64(i*1000),
		})
	}
	return data
}

/*
Function: getPastDatabaseRecords
Description:
  Fetches the last N market data records from the database (simulated for now).
  In a full implementation, this would query the actual database.
Input:
  - n int: The number of past database records to retrieve.
Output:
  - []MarketData: A slice of `MarketData` containing the requested records from the DB.
Lines: ~15
*/
func getPastDatabaseRecords(n int) []MarketData {
	infra.Info(fmt.Sprintf("Fetching last %d database records... (simulated)", n))
	data := []MarketData{}
	for i := 0; i < n; i++ {
		data = append(data, MarketData{
			Timestamp: time.Now().Add(-time.Duration(i) * time.Hour),
			Symbol:    "BTC",
			Price:     60000.0 + float64(i*100),
			Volume:    50000.0 + float64(i*500),
		})
	}
	return data
}
