/******************************************************************************
 * File Name       : schema.go
 * File Path       : config/schema.go
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
 *   Structured configuration types. Loadable from config.env or JSON. All measurable values are externalized — no hardcoded magic numbers.
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
package config

import (
	"os"
	"strconv"
	"strings"
)

// ==============================
// RUNTIME CONFIG
// ==============================

type RuntimeConfig struct {
	DBHost                          string
	DBPort                          string
	DBUser                          string
	DBPass                          string
	DBName                          string
	LogLevel                        string
	LogOutput                       string
	LogFilePath                     string
	LogCallerFormat                 string
	LogFormat                       string
	HealthCheckIntervalSeconds      int
	RecreateThresholdSeconds        int
	MarketDataRecordIntervalMinutes int
	DBHealthCheckIntervalSeconds    int
	GovernanceUDPPort               int
	SchoolUDPPort                   int
	TradingUDPPort                  int
	GovernanceWebPort               int
	GovernanceAddr                  string   // §66: bind/listen address (localhost in single, 0.0.0.0 in distributed)
	SchoolAddr                      string   // §66: address of school daemon
	TradingAddr                     string   // §66: address of trading daemon
	MaxTasks                        int
	TotalCapital                    float64
	WebOutputDir                    string
	WebRefreshSeconds               int
	WebActionPort                   int
	SingleContainerMode             bool
	PaperTradingOnly                bool
	School                          SchoolConfig
	Trading                         TradingConfig
	TestDaemon                      TestDaemonConfig
}

// SchoolConfig holds all School daemon gene-evolution parameters.
type SchoolConfig struct {
	GAPopulationSize          int
	GATopSurvivors            int
	GAMutationRate            float64
	GACrossoverRate           float64
	GAGenerationsPerCycle     int
	GAGraduateTopN            int
	GARetireBottomN           int
	GACycleIntervalMinutes    int
	ModelGraduationThreshold  float64
	ModelRetirementThreshold  float64
	RemoteAddresses           string  // comma-separated ip:port pairs, empty=local only
	RemoteTimeoutSeconds      int
	RemoteStudentsPerNode     int
	LocalTraining             bool    // enable Go-native training when no remote
	RemoteTrainingProtocol    string  // udp | ssh
	RemoteTrainingSSHKey      string  // path to SSH private key
	RemoteTrainingPython      string  // python3 binary on remote
	RemoteTrainingWorkdir     string  // remote working directory for training
	// §90: 4-tier school system
	PrimarySchoolMaxModels    int    // Primary tier: max single-model count (default 50)
	MiddleSchoolMaxEnsembles  int    // Middle tier: max 3-model ensembles (default 250)
	HighSchoolMaxEnsembles    int    // High tier: max 5-model ensembles (default 150)
	TrainingDataRecords       int    // Records per training cycle (default 300)
	TrainingIntervalMinutes   int    // Interval between training cycles (default 15)
}

// TradingConfig holds all trading/portfolio/MAB parameters.
type TradingConfig struct {
	TradingCycleIntervalSeconds   int
	MaxPaperAgents                int     // Virtual paper-trading agents (always active)
	MaxRealAgents                 int     // Real-capital agents (activated after gate)
	AgentMinCapital               float64
	AgentMaxCapital               float64
	AgentCreationIntervalMinutes  int
	AgentRetirementThresholdScore float64
	MABExplorationRate            float64
	MABUpdateIntervalSeconds      int
	StopLossPercent               float64
	MaxLeverage                   float64
	MinPaperTrades                int     // Min trades before agent eligible for real
	AgentPerformanceThreshold     float64 // Min Sharpe/profit to graduate to real
	ModelConfidenceThreshold      float64 // Min model confidence to graduate to real
}

// TestDaemonConfig holds CI/CD validation parameters.
type TestDaemonConfig struct {
	PollIntervalSeconds  int
	CoverageThreshold     float64
	BuildTimeoutSeconds   int
}

// Defaults returns sensible production defaults.
func Defaults() RuntimeConfig {
	return RuntimeConfig{
		DBHost:                          "127.0.0.1",
		DBPort:                          "5432",
		DBUser:                          "trader",
		DBPass:                          "secret",
		DBName:                          "traderdb",
		LogLevel:                        "INFO",
		LogOutput:                       "both",
		LogFilePath:                     "logs/system.log",
		LogCallerFormat:                 "full",
		LogFormat:                       "text",
		HealthCheckIntervalSeconds:      30,
		RecreateThresholdSeconds:        60,
		MarketDataRecordIntervalMinutes: 15,
		DBHealthCheckIntervalSeconds:    30,
		GovernanceUDPPort:               8081,
		SchoolUDPPort:                   8082,
		TradingUDPPort:                  8083,
		GovernanceWebPort:               8080,
		GovernanceAddr:                  "127.0.0.1",
		SchoolAddr:                      "127.0.0.1",
		TradingAddr:                     "127.0.0.1",
		MaxTasks:                        10,
		TotalCapital:                    10000.00,
		WebOutputDir:                    "web_output",
		WebRefreshSeconds:               10,
		WebActionPort:                   8085,
		SingleContainerMode:             true,
		PaperTradingOnly:                true,
		School: SchoolConfig{
			GAPopulationSize:         50,
			GATopSurvivors:           10,
			GAMutationRate:           0.15,
			GACrossoverRate:          0.60,
			GAGenerationsPerCycle:    3,
			GAGraduateTopN:           3,
			GARetireBottomN:          5,
			GACycleIntervalMinutes:   30,
			ModelGraduationThreshold: 0.60,
			ModelRetirementThreshold: 0.30,
			RemoteAddresses:          "",
			RemoteTimeoutSeconds:     30,
			RemoteStudentsPerNode:    5,
			LocalTraining:            true,
			RemoteTrainingProtocol:   "udp",
			RemoteTrainingSSHKey:     "",
			RemoteTrainingPython:     "python3",
			RemoteTrainingWorkdir:    "/tmp",
			PrimarySchoolMaxModels:   50,
			MiddleSchoolMaxEnsembles: 250,
			HighSchoolMaxEnsembles:   150,
			TrainingDataRecords:      300,
			TrainingIntervalMinutes:  15,
		},
		Trading: TradingConfig{
			TradingCycleIntervalSeconds:   5,
			MaxPaperAgents:                15,
			MaxRealAgents:                 5,
			AgentMinCapital:               100.00,
			AgentMaxCapital:               5000.00,
			AgentCreationIntervalMinutes:  60,
			AgentRetirementThresholdScore: 0.25,
			MABExplorationRate:            0.10,
			MABUpdateIntervalSeconds:      30,
			StopLossPercent:               5.0,
			MaxLeverage:                   2.0,
			MinPaperTrades:                100,
			AgentPerformanceThreshold:     0.40,
			ModelConfidenceThreshold:      0.55,
		},
		TestDaemon: TestDaemonConfig{
			PollIntervalSeconds:  10,
			CoverageThreshold:    70.0,
			BuildTimeoutSeconds:  120,
		},
	}
}

// LoadFromEnv overrides config fields from config.env-style environment variables.
func (c *RuntimeConfig) LoadFromEnv() {
	c.DBHost = envStr("DB_HOST", c.DBHost)
	c.DBPort = envStr("DB_PORT", c.DBPort)
	c.DBUser = envStr("DB_USER", c.DBUser)
	c.DBPass = envStr("DB_PASS", c.DBPass)
	c.DBName = envStr("DB_NAME", c.DBName)
	c.LogLevel = envStr("LOG_LEVEL", c.LogLevel)
	c.LogOutput = envStr("LOG_OUTPUT", c.LogOutput)
	c.LogFilePath = envStr("LOG_FILE_PATH", c.LogFilePath)
	c.LogCallerFormat = envStr("LOG_CALLER_FORMAT", c.LogCallerFormat)
	c.LogFormat = envStr("LOG_FORMAT", c.LogFormat)

	c.HealthCheckIntervalSeconds = envInt("HEALTH_CHECK_INTERVAL_SECONDS", c.HealthCheckIntervalSeconds)
	c.RecreateThresholdSeconds = envInt("RECREATE_THRESHOLD_SECONDS", c.RecreateThresholdSeconds)
	c.MarketDataRecordIntervalMinutes = envInt("MARKET_DATA_RECORD_INTERVAL_MINUTES", c.MarketDataRecordIntervalMinutes)
	c.DBHealthCheckIntervalSeconds = envInt("DB_HEALTH_CHECK_INTERVAL_SECONDS", c.DBHealthCheckIntervalSeconds)
	c.GovernanceUDPPort = envInt("UDP_GOVERNANCE_PORT", c.GovernanceUDPPort)
	c.SchoolUDPPort = envInt("UDP_SCHOOL_PORT", c.SchoolUDPPort)
	c.TradingUDPPort = envInt("UDP_TRADING_PORT", c.TradingUDPPort)
	c.GovernanceWebPort = envInt("GOVERNANCE_WEB_PORT", c.GovernanceWebPort)
	c.GovernanceAddr = envStr("GOVERNANCE_ADDR", c.GovernanceAddr)
	c.SchoolAddr = envStr("SCHOOL_ADDR", c.SchoolAddr)
	c.TradingAddr = envStr("TRADING_ADDR", c.TradingAddr)
	c.MaxTasks = envInt("MAX_TRADING_TASKS", c.MaxTasks)
	c.TotalCapital = envFloat("TOTAL_CAPITAL", c.TotalCapital)
	c.WebOutputDir = envStr("WEB_OUTPUT_DIR", c.WebOutputDir)
	c.WebRefreshSeconds = envInt("WEB_REFRESH_SECONDS", c.WebRefreshSeconds)
	c.WebActionPort = envInt("WEB_ACTION_PORT", c.WebActionPort)
	c.SingleContainerMode = envBool("SINGLE_CONTAINER_MODE", c.SingleContainerMode)
	c.PaperTradingOnly = envBool("PAPER_TRADING_ONLY", c.PaperTradingOnly)

	c.School.GAPopulationSize = envInt("GA_POPULATION_SIZE", c.School.GAPopulationSize)
	c.School.GATopSurvivors = envInt("GA_TOP_SURVIVORS", c.School.GATopSurvivors)
	c.School.GAMutationRate = envFloat("GA_MUTATION_RATE", c.School.GAMutationRate)
	c.School.GACrossoverRate = envFloat("GA_CROSSOVER_RATE", c.School.GACrossoverRate)
	c.School.GAGenerationsPerCycle = envInt("GA_GENERATIONS_PER_CYCLE", c.School.GAGenerationsPerCycle)
	c.School.GAGraduateTopN = envInt("GA_GRADUATE_TOP_N", c.School.GAGraduateTopN)
	c.School.GARetireBottomN = envInt("GA_RETIRE_BOTTOM_N", c.School.GARetireBottomN)
	c.School.GACycleIntervalMinutes = envInt("GA_CYCLE_INTERVAL_MINUTES", c.School.GACycleIntervalMinutes)
	c.School.ModelGraduationThreshold = envFloat("MODEL_GRADUATION_THRESHOLD", c.School.ModelGraduationThreshold)
	c.School.ModelRetirementThreshold = envFloat("MODEL_RETIREMENT_THRESHOLD", c.School.ModelRetirementThreshold)
	c.School.RemoteAddresses = envStr("SCHOOL_REMOTE_ADDRESSES", c.School.RemoteAddresses)
	c.School.RemoteTimeoutSeconds = envInt("SCHOOL_REMOTE_TIMEOUT_SECONDS", c.School.RemoteTimeoutSeconds)
	c.School.RemoteStudentsPerNode = envInt("SCHOOL_REMOTE_STUDENTS_PER_NODE", c.School.RemoteStudentsPerNode)
	c.School.LocalTraining = envBool("LOCAL_TRAINING", c.School.LocalTraining)
	c.School.RemoteTrainingProtocol = envStr("REMOTE_TRAINING_PROTOCOL", c.School.RemoteTrainingProtocol)
	c.School.RemoteTrainingSSHKey = envStr("REMOTE_TRAINING_SSH_KEY", c.School.RemoteTrainingSSHKey)
	c.School.RemoteTrainingPython = envStr("REMOTE_TRAINING_PYTHON", c.School.RemoteTrainingPython)
	c.School.RemoteTrainingWorkdir = envStr("REMOTE_TRAINING_WORKDIR", c.School.RemoteTrainingWorkdir)
	c.School.PrimarySchoolMaxModels = envInt("PRIMARY_SCHOOL_MAX_MODELS", c.School.PrimarySchoolMaxModels)
	c.School.MiddleSchoolMaxEnsembles = envInt("MIDDLE_SCHOOL_MAX_ENSEMBLES", c.School.MiddleSchoolMaxEnsembles)
	c.School.HighSchoolMaxEnsembles = envInt("HIGH_SCHOOL_MAX_ENSEMBLES", c.School.HighSchoolMaxEnsembles)
	c.School.TrainingDataRecords = envInt("TRAINING_DATA_RECORDS", c.School.TrainingDataRecords)
	c.School.TrainingIntervalMinutes = envInt("TRAINING_INTERVAL_MINUTES", c.School.TrainingIntervalMinutes)

	c.Trading.TradingCycleIntervalSeconds = envInt("TRADING_CYCLE_INTERVAL_SECONDS", c.Trading.TradingCycleIntervalSeconds)
	c.Trading.MaxPaperAgents = envInt("MAX_PAPER_AGENTS", c.Trading.MaxPaperAgents)
	c.Trading.MaxRealAgents = envInt("MAX_REAL_AGENTS", c.Trading.MaxRealAgents)
	c.Trading.AgentMinCapital = envFloat("AGENT_MIN_CAPITAL", c.Trading.AgentMinCapital)
	c.Trading.AgentMaxCapital = envFloat("AGENT_MAX_CAPITAL", c.Trading.AgentMaxCapital)
	c.Trading.AgentCreationIntervalMinutes = envInt("AGENT_CREATION_INTERVAL_MINUTES", c.Trading.AgentCreationIntervalMinutes)
	c.Trading.AgentRetirementThresholdScore = envFloat("AGENT_RETIREMENT_THRESHOLD_SCORE", c.Trading.AgentRetirementThresholdScore)
	c.Trading.MABExplorationRate = envFloat("MAB_EXPLORATION_RATE", c.Trading.MABExplorationRate)
	c.Trading.MABUpdateIntervalSeconds = envInt("MAB_UPDATE_INTERVAL_SECONDS", c.Trading.MABUpdateIntervalSeconds)
	c.Trading.StopLossPercent = envFloat("STOP_LOSS_PERCENT", c.Trading.StopLossPercent)
	c.Trading.MaxLeverage = envFloat("MAX_LEVERAGE", c.Trading.MaxLeverage)
	c.Trading.MinPaperTrades = envInt("MIN_PAPER_TRADES", c.Trading.MinPaperTrades)
	c.Trading.AgentPerformanceThreshold = envFloat("AGENT_PERFORMANCE_THRESHOLD", c.Trading.AgentPerformanceThreshold)
	c.Trading.ModelConfidenceThreshold = envFloat("MODEL_CONFIDENCE_THRESHOLD", c.Trading.ModelConfidenceThreshold)

	c.TestDaemon.PollIntervalSeconds = envInt("TEST_POLL_INTERVAL_SECONDS", c.TestDaemon.PollIntervalSeconds)
	c.TestDaemon.CoverageThreshold = envFloat("TEST_COVERAGE_THRESHOLD", c.TestDaemon.CoverageThreshold)
	c.TestDaemon.BuildTimeoutSeconds = envInt("TEST_BUILD_TIMEOUT_SECONDS", c.TestDaemon.BuildTimeoutSeconds)
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		switch strings.ToLower(v) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return fallback
}
