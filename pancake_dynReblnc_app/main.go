/*
Filename: main.go
Version: v5.0 (Split-Route Skim & Auto-Unwrap)
Description: Object-oriented multi-pair rebalancing daemon. Tracks independent 
dual-token matrix pools. Implements 2-step skim routing to secure profit and 
auto-unwrap WBNB to native BNB for perpetual gas self-funding.
*/

package main

import (
        "context"
        "encoding/json"
        "flag"
        "fmt"
        "log"
        "math"
        "math/big"
        "math/rand"
        "os"
        "os/exec"
        "path/filepath"
        "strconv"
        "strings"
        "sync" // <-- ADDED
        "time"

        "github.com/ethereum/go-ethereum/accounts/abi"
        "github.com/ethereum/go-ethereum/accounts/abi/bind"
        "github.com/ethereum/go-ethereum/common"
        "github.com/ethereum/go-ethereum"
        "github.com/ethereum/go-ethereum/core/types"
        "github.com/ethereum/go-ethereum/crypto"
        "github.com/ethereum/go-ethereum/ethclient"
)
/// =======================
/// CONSTANTS & CORE ADDRS
/// =======================
const (
        LOG_FILE = "performance.log"
        PID_FILE = "daemon.pid"
)

var (
        USDT_ADDRESS = common.HexToAddress("0x55d398326f99059fF775485246999027B3197955")
        WBNB_ADDRESS = common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c")
        ROUTER_ADDR  = common.HexToAddress("0x10ED43C718714eb63d5aA57B78B54704E256024E")

        // Token Definitions
        WBTC_ADDRESS = common.HexToAddress("0x7130d2A12B9BCbFAe4f2634d864A1EE1Ce3Ead9c")
        WUNI_ADDRESS = common.HexToAddress("0xBf5140A22578168FD562DCcF235E5D43A02ce9B1")
        WETH_ADDRESS = common.HexToAddress("0x2170Ed0880ac9A755fd29B2688956BD959F933F8")
        ATOM_ADDRESS = common.HexToAddress("0x0eb3a705fc54725037cc9e008bdede697f62f335")
        // Global Registry
        GlobalRegistry = &EngineRegistry{
                Engines: make([]*RebalancePairEngine, 0),
        }
)
type EngineRegistry struct {
        sync.RWMutex
        Engines []*RebalancePairEngine
}
/// =======================
/// PAIR REBALANCE ENGINE (CLASS)

// RebalanceMetrics holds the 9 core tracking variables for any given timeframe.
type RebalanceMetrics struct {
	TotalEarnings float64 `json:"total_earnings"`
	EarningsAtoB  float64 `json:"earnings_a_to_b"`
	EarningsBtoA  float64 `json:"earnings_b_to_a"`

	TotalFees     float64 `json:"total_fees"`
	FeesAtoB      float64 `json:"fees_a_to_b"`
	FeesBtoA      float64 `json:"fees_b_to_a"`

	TotalSwitches int     `json:"total_switches"`
	SwitchesAtoB  int     `json:"switches_a_to_b"`
	SwitchesBtoA  int     `json:"switches_b_to_a"`
}

/// =======================
type PairConfig struct {
        // ... existing fields ...
        StartMode string `json:"start_mode"` // "START-AUTO" or "START-MANUAL"
        PairName        string         `json:"pair_name"`
        TokenAName      string         `json:"token_a_name"`
        TokenBName      string         `json:"token_b_name"`
        TokenAAddress   common.Address `json:"token_a_address"`
        TokenBAddress   common.Address `json:"token_b_address"`
        InitValueInUSD  float64        `json:"init_value_in_usd"`
        MinMins         int            `json:"min_mins"`
        MaxMins         int            `json:"max_mins"`
        TargetGrowthPct float64        `json:"target_growth_pct"`
        MinAlphaPct     float64        `json:"min_alpha_pct"` // Now used as Max Gas % of Earnings
        // Add this line so the algorithm can access it:
        ThresholdPct     float64 `json:"threshold_pct"`
        CreatedDate      string        `json:"created_date"`
}

type PairState struct {
        Status           string    `json:"status"` // "MONITORING", "FAILED", or "" (pending)
        FailedReason     string    `json:"failed_reason,omitempty"`
        AllocatedQtyA    float64   `json:"allocated_qty_a"` // Virtual tracking for shared pool
        AllocatedQtyB    float64   `json:"allocated_qty_b"` // Virtual tracking for shared pool
        InitialValueUSD  float64   `json:"initial_value_usd"`
        CurrentTotalUSD  float64   `json:"current_total_usd"`
        TotalFeesPaidBNB float64   `json:"total_fees_paid_bnb"`
        LastRebalanceTotalBNB float64 `json:"last_rebalance_total_bnb"` // Add this if you want to track profit between actions
        LastRebalanceBNBPriceUSD float64 `json:"last_rebalance_bnb_price_usd"`
        LastPriceA       float64   `json:"last_price_a"`       
        LastPriceB       float64   `json:"last_price_b"`       
        LastUpdated      time.Time `json:"last_updated"`
        NextCheckTime    time.Time `json:"next_check_time"`

        // Multi-Timeframe Execution Metrics
        AllTime             RebalanceMetrics `json:"all_time"`
        Today               RebalanceMetrics `json:"today"`
        Yesterday           RebalanceMetrics `json:"yesterday"`
}

type RebalancePairEngine struct {
        mu         sync.Mutex // Add this
        Config     PairConfig
        State      PairState
        ConfigFile string `json:"-"`
        StateFile  string `json:"-"`
}

func (engine *RebalancePairEngine) LoadState() {
        if _, err := os.Stat(engine.StateFile); os.IsNotExist(err) {
                return // State file doesn't exist yet; relies on config defaults
        }
        data, err := os.ReadFile(engine.StateFile)
        if err != nil {
                logToFile(fmt.Sprintf("❌ [SYSTEM ERROR] Failed to read state file %s: %v", engine.StateFile, err))
                return
        }
        _ = json.Unmarshal(data, &engine.State)
}

func (engine *RebalancePairEngine) SaveState() {
        engine.mu.Lock()
        defer engine.mu.Unlock()
        
        engine.State.LastUpdated = time.Now()
        data, _ := json.MarshalIndent(engine.State, "", "  ")
        _ = os.WriteFile(engine.StateFile, data, 0644)
}

func (engine *RebalancePairEngine) FailState(reason string) {
        engine.State.Status = "FAILED"
        engine.State.FailedReason = reason
        engine.SaveState()
        logToFile(fmt.Sprintf("🚫 [CONFIG VALIDATION][%s] Engine disabled. Reason: %s", engine.Config.PairName, reason))
}

// Replace the existing UpdateMetrics signature and internal block
func (engine *RebalancePairEngine) UpdateMetrics(earningsBNB float64, feesBNB float64, isSellingA bool) {
        engine.mu.Lock()
        defer engine.mu.Unlock() // Ensure absolute isolation during mutation
        
        now := time.Now()
        
        if !engine.State.LastUpdated.IsZero() {
                y1, m1, d1 := engine.State.LastUpdated.Date()
                y2, m2, d2 := now.Date()
                
                if y1 != y2 || m1 != m2 || d1 != d2 {
                        engine.State.Yesterday = engine.State.Today
                        engine.State.Today = RebalanceMetrics{} 
                }
        }

        updateBlock := func(m *RebalanceMetrics) {
                m.TotalEarnings += earningsBNB // Now accumulates BNB
                m.TotalFees += feesBNB
                m.TotalSwitches += 1
                if isSellingA {
                        m.EarningsAtoB += earningsBNB
                        m.FeesAtoB += feesBNB
                        m.SwitchesAtoB += 1
                } else {
                        m.EarningsBtoA += earningsBNB
                        m.FeesBtoA += feesBNB
                        m.SwitchesBtoA += 1
                }
        }

        updateBlock(&engine.State.AllTime)
        updateBlock(&engine.State.Today)
        
        engine.State.TotalFeesPaidBNB += feesBNB
        // Update this state tracking field to reflect BNB if it exists in PairState
        engine.State.LastRebalanceTotalBNB = earningsBNB 
}

// ====================================================================
// CROSS-ENGINE ACCOUNTING: Safely calculating total liabilities
// ====================================================================

func (r *EngineRegistry) GetTotalAllocated(tokenAddr common.Address) *big.Float {
        r.RLock()
        defer r.RUnlock()
        
        total := big.NewFloat(0)
        for _, engine := range r.Engines {
                engine.mu.Lock() // Must lock individual engine state during aggregate lookups
                if engine.Config.TokenAAddress == tokenAddr {
                        total = new(big.Float).Add(total, big.NewFloat(engine.State.AllocatedQtyA))
                }
                if engine.Config.TokenBAddress == tokenAddr {
                        total = new(big.Float).Add(total, big.NewFloat(engine.State.AllocatedQtyB))
                }
                engine.mu.Unlock()
        }
        return total
}

func (engine *RebalancePairEngine) RunStage3(client *ethclient.Client, auth *bind.TransactOpts, wallet common.Address) {
        logToFile(fmt.Sprintf("🚀 [%s ENGINE ACTIVATED] Evaluating pair divergence metrics...", engine.Config.PairName))

        bnbPrice := getLiveBNBPrice(client)
        priceA := getLiveTokenPriceInBNB(client, engine.Config.TokenAAddress) * bnbPrice
        priceB := getLiveTokenPriceInBNB(client, engine.Config.TokenBAddress) * bnbPrice

        // VIRTUAL POOL TRACKING (Replacing getERC20Balance)
        valueA := engine.State.AllocatedQtyA * priceA
        valueB := engine.State.AllocatedQtyB * priceB
        currentTotalUSD := valueA + valueB

        // =========================================================
        // PHYSICAL VS VIRTUAL SANITY CHECK LOGGING
        // =========================================================
        physicalBalA := getERC20Balance(client, engine.Config.TokenAAddress, wallet)
        physicalBalB := getERC20Balance(client, engine.Config.TokenBAddress, wallet)

        logToFile(fmt.Sprintf("   • %s (Token A): %.6f (Virtual State) | %.6f (Physical Wallet)", engine.Config.TokenAName, engine.State.AllocatedQtyA, physicalBalA))
        logToFile(fmt.Sprintf("   • %s (Token B): %.6f (Virtual State) | %.6f (Physical Wallet)", engine.Config.TokenBName, engine.State.AllocatedQtyB, physicalBalB))

        engine.State.CurrentTotalUSD = currentTotalUSD

        pctChangeA := 0.0
        if engine.State.LastPriceA > 0 {
                pctChangeA = ((priceA - engine.State.LastPriceA) / engine.State.LastPriceA) * 100
        }
        pctChangeB := 0.0
        if engine.State.LastPriceB > 0 {
                pctChangeB = ((priceB - engine.State.LastPriceB) / engine.State.LastPriceB) * 100
        }

        // 1. Run the safety check using the virtual pool total
        err := engine.evaluatePoolIntegrity(physicalBalA, physicalBalB, GlobalRegistry.GetEngines())
        if err != nil {
            errMsg := fmt.Sprintf("🛑 [%s] Stage 3 aborted: %v", engine.Config.PairName, err)
            engine.FailState(errMsg)
            logToFile(errMsg)
            return // Exit the loop entirely for this pair. No buys, no swaps.
        }

        // 2. Only proceed to log and calculate divergence if integrity passes
        logToFile(fmt.Sprintf("   • Initial Baseline Value : $%f USD", engine.State.InitialValueUSD))
        logToFile(fmt.Sprintf("   • Current Combined Value : $%f USD", currentTotalUSD))

        // =========================================================
        // CORE ALGORITHMIC SKIM FORMULA & GAS ESTIMATION
        // =========================================================
        divergenceUSD := math.Abs(valueA - valueB)
        sellValueUSD := divergenceUSD / 2.0

        // Set a reasonable timeout context for RPC node inquiries
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        suggestedGasPrice, err := client.SuggestGasPrice(ctx)
        if err != nil {
                suggestedGasPrice = big.NewInt(500000000)
        }

        compiledSwapDataPayload := []byte{0x7f, 0xf3, 0x6a, 0xb5}
        msg := ethereum.CallMsg{
                From:     wallet,
                To:       &ROUTER_ADDR,
                GasPrice: suggestedGasPrice,
                Value:    big.NewInt(1000000000),
                Data:     compiledSwapDataPayload,
        }

        strategyEvaluation := "Live Chain RPC Estimate"
        liveGasUnits, err := client.EstimateGas(ctx, msg)
        if err != nil {
                liveGasUnits = 105000
                strategyEvaluation = "Static Fallback Baseline"
        }

        gasLimitUnits := big.NewInt(int64(liveGasUnits))
        totalGasWei := new(big.Int).Mul(gasLimitUnits, suggestedGasPrice)

        base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
        liveGasBNBFloat, _ := new(big.Float).Quo(new(big.Float).SetInt(totalGasWei), base18).Float64()
        liveGasPriceGwei, _ := new(big.Float).Quo(new(big.Float).SetInt(suggestedGasPrice), big.NewFloat(1e9)).Float64()

        liveTxFeeUSD := liveGasBNBFloat * bnbPrice
        totalRoundTripFeeUSD := liveTxFeeUSD * 2.0 // 2x Gas

        earningValueUSD := (divergenceUSD - totalRoundTripFeeUSD) * (engine.Config.TargetGrowthPct / 100.0)
        buyValueUSD := sellValueUSD - totalRoundTripFeeUSD - earningValueUSD

        decision := "HOLD"
        reason := ""
        isSafeToTrade := false

        maxAcceptableGasUSD := earningValueUSD * (engine.Config.MinAlphaPct / 100.0)

        if totalRoundTripFeeUSD > maxAcceptableGasUSD {
                decision = "HOLD 🛑 [MIN ALPHA GAS GUARD]"
                reason = fmt.Sprintf("Gas (%s) exceeds the %.2f%% limit of the total earning value (%s max allowance).",
                        formatUSD(totalRoundTripFeeUSD),
                        engine.Config.MinAlphaPct,
                        formatUSD(maxAcceptableGasUSD))
        } else if earningValueUSD > totalRoundTripFeeUSD && buyValueUSD > 0 {
                decision = "EXECUTE 🚀 [SPLIT-ROUTE SKIM]"
                reason = fmt.Sprintf("Calculated Earning (%s) > Total Gas (%s). Target Buy is positive.", formatUSD(earningValueUSD), formatUSD(totalRoundTripFeeUSD))
                isSafeToTrade = true
        } else if divergenceUSD <= totalRoundTripFeeUSD {
                decision = "HOLD 🛑 [GAS FRICTION GUARD]"
                reason = fmt.Sprintf("Divergence %s cannot support round-trip transaction friction of %s.", formatUSD(divergenceUSD), formatUSD(totalRoundTripFeeUSD))
        } else {
                decision = "HOLD 🛑 [INSUFFICIENT ROUTE YIELD]"
                reason = fmt.Sprintf("Projected earning (%s) does not securely cover total gas (%s).", formatUSD(earningValueUSD), formatUSD(totalRoundTripFeeUSD))
        }

        logToFile(fmt.Sprintf("==================== %s ====================", engine.Config.PairName))
        logToFile(fmt.Sprintf("📊 Token A Bal: %.6f Qty × %s Price = %s [Trend: %+.2f%% vs last run]", engine.State.AllocatedQtyA, formatUSD(priceA), formatUSD(valueA), pctChangeA))
        logToFile(fmt.Sprintf("📊 Token B Bal: %.6f Qty × %s Price = %s [Trend: %+.2f%% vs last run]", engine.State.AllocatedQtyB, formatUSD(priceB), formatUSD(valueB), pctChangeB))

        logToFile(fmt.Sprintf("🛠️ [DEBUG] Live Network Gas Estimation:"))
        logToFile(fmt.Sprintf("   • Gas Price              : %.4f Gwei", liveGasPriceGwei))
        logToFile(fmt.Sprintf("   • Estimated Gas Units   : %d units (%s)", liveGasUnits, strategyEvaluation))
        logToFile(fmt.Sprintf("   • 1-Way Gas Fee         : %.8f BNB (~%s USD)", liveGasBNBFloat, formatUSD(liveTxFeeUSD)))

        logToFile(fmt.Sprintf("🔮 [DEBUG] Algorithmic Swap Formula Breakdown:"))
        logToFile(fmt.Sprintf("   • Divergence (Diff)      : %s", formatUSD(divergenceUSD)))
        logToFile(fmt.Sprintf("   • Total Gas (2-Way)      : %s", formatUSD(totalRoundTripFeeUSD)))
        logToFile(fmt.Sprintf("   • Earning Value          : %s (Target: %.3f%%)", formatUSD(earningValueUSD), engine.Config.TargetGrowthPct))
        logToFile(fmt.Sprintf("   • Target Sell Execution  : Sell %s (Diff / 2)", formatUSD(sellValueUSD)))
        logToFile(fmt.Sprintf("   • Target Buy Allocation  : Buy %s (Sell - 2xGas - Earning)", formatUSD(buyValueUSD)))
        logToFile(fmt.Sprintf("   • Total Skim Secured      : %s (Gas + Earning stays in WBNB)", formatUSD(totalRoundTripFeeUSD + earningValueUSD)))

        logToFile(fmt.Sprintf("💰 Initial Baseline Value : $%s USD", formatWithSpacedDecimals(engine.State.InitialValueUSD)))
        logToFile(fmt.Sprintf("📈 Current Combined Value : $%s USD", formatWithSpacedDecimals(currentTotalUSD)))
        logToFile(fmt.Sprintf("⚖️ JUDGMENT   : %s", decision))
        logToFile(fmt.Sprintf("   ↳ REASON   : %s", reason))
        logToFile("=======================================================")

        engine.State.LastPriceA = priceA
        engine.State.LastPriceB = priceB

        // =========================================================
        // SPLIT-ROUTE EXECUTION PIPELINE
        // =========================================================
        if isSafeToTrade {
                logToFile(fmt.Sprintf("[SIGNAL][%s][🔥] SKIM REBALANCE VALIDATED: Executing split-route correction...", engine.Config.PairName))

                var sellToken, buyToken common.Address
                var sellPrice float64
                var isSellingA bool

                if valueA > valueB {
                        sellToken = engine.Config.TokenAAddress
                        buyToken = engine.Config.TokenBAddress
                        sellPrice = priceA
                        isSellingA = true
                } else {
                        sellToken = engine.Config.TokenBAddress
                        buyToken = engine.Config.TokenAAddress
                        sellPrice = priceB
                        isSellingA = false
                }

                amtInTokens := sellValueUSD / sellPrice
                fee1 := swapTokenForToken(client, auth, sellToken, WBNB_ADDRESS, amtInTokens)

                wbnbPriceUSD := bnbPrice
                amtWBNBToSpend := buyValueUSD / wbnbPriceUSD
                fee2 := swapTokenForToken(client, auth, WBNB_ADDRESS, buyToken, amtWBNBToSpend)

                totalFeeCost := fee1 + fee2

                // VIRTUAL LEDGER UPDATE: Update the allocated quantities immediately after executing
                if isSellingA {
                        engine.State.AllocatedQtyA -= amtInTokens
                        engine.State.AllocatedQtyB += (amtWBNBToSpend * wbnbPriceUSD) / priceB
                } else {
                        engine.State.AllocatedQtyB -= amtInTokens
                        engine.State.AllocatedQtyA += (amtWBNBToSpend * wbnbPriceUSD) / priceA
                }

                unwrapWBNB(client, auth)

                // -------------------------------------------------------------
                // ACCUMULATE EARNINGS & MULTI-TIMEFRAME METRICS (IN BNB)
                // -------------------------------------------------------------
                // Convert USD earnings to BNB using the current WBNB price
                earningValueBNB := 0.0
                if wbnbPriceUSD > 0 {
                        earningValueBNB = earningValueUSD / wbnbPriceUSD
                }
                // Record the point-in-time BNB price
                engine.State.LastRebalanceBNBPriceUSD = bnbPrice
                engine.UpdateMetrics(earningValueBNB, totalFeeCost, isSellingA)
                
                // Assuming you have a formatBNB or similar formatter (or use %.6f)
                logToFile(fmt.Sprintf("💰 [EARNINGS][%s] Captured %.6f BNB (%s USD) in profit from this cycle. All-Time Accrued: %.8f BNB", 
                        engine.Config.PairName, earningValueBNB,formatUSD(earningValueUSD), engine.State.AllTime.TotalEarnings))
                
                logToFile(fmt.Sprintf("✅ [SUCCESS][%s] Split-route rebalance finalized. Virtual ledgers updated.", engine.Config.PairName))
        }

        // Thread-safe decoupled localized pseudo-random instance initialization
        localRand := rand.New(rand.NewSource(time.Now().UnixNano()))
        delta := engine.Config.MaxMins - engine.Config.MinMins + 1
        var minutes int
        if delta > 0 {
                minutes = localRand.Intn(delta) + engine.Config.MinMins
        } else {
                minutes = engine.Config.MinMins
        }

        nextInterval := time.Duration(minutes) * time.Minute
        engine.State.NextCheckTime = time.Now().Add(nextInterval)
        engine.SaveState()

        logToFile(fmt.Sprintf("[%s] Next monitoring window locked for: %s", engine.Config.PairName, engine.State.NextCheckTime.Format("2006-01-02 15:04:05")))
}

// =========================================================
// DYNAMIC CONFIGURATION & VALIDATION
// =========================================================

// 2. Update the watcher signature and its internal call
func StartDynamicConfigWatcher(registry *EngineRegistry, client *ethclient.Client, auth *bind.TransactOpts, walletAddress common.Address) {
        ticker := time.NewTicker(1 * time.Minute)
        go func() {
                for range ticker.C {
                        files, err := filepath.Glob(filepath.Join(getWorkingDir(), "config_*.json"))
                        if err != nil { continue }
                        
                        for _, file := range files {
                                if !isConfigTracked(registry, file) {
                                        logToFile(fmt.Sprintf("📂 [SYSTEM INFO] New dynamic configuration detected: %s", filepath.Base(file)))
                                        processDynamicConfig(registry, file, client, auth, walletAddress)
                                } else {
                                        reevaluateTrackedConfig(registry, file, client)
                                }
                        }
                }
        }()
}


func (r *EngineRegistry) Add(engine *RebalancePairEngine) {
        r.Lock()
        defer r.Unlock()
        r.Engines = append(r.Engines, engine)
}

func (r *EngineRegistry) GetEngines() []*RebalancePairEngine {
        r.RLock()
        defer r.RUnlock()
        // Return a copy of the slice to prevent locking the main loop during execution
        enginesCopy := make([]*RebalancePairEngine, len(r.Engines))
        copy(enginesCopy, r.Engines)
        return enginesCopy
}

func processDynamicConfig(registry *EngineRegistry, configPath string, client *ethclient.Client, auth *bind.TransactOpts, walletAddress common.Address) {
        data, err := os.ReadFile(configPath)
        if err != nil {
                logToFile(fmt.Sprintf("❌ [SYSTEM ERROR] Could not read config file %s", configPath))
                return 
        }

        var config PairConfig
        if err := json.Unmarshal(data, &config); err != nil {
                logToFile(fmt.Sprintf("❌ [SYSTEM ERROR] Malformed JSON in %s", configPath))
                return 
        }

        if config.PairName == "" {
                config.PairName = fmt.Sprintf("%svs%s_%s", config.TokenAName, config.TokenBName, generateHashID())
                logToFile(fmt.Sprintf("⚠️ [SYSTEM WARN] Config missing PairName. Auto-generated: %s", config.PairName))
        }

        engine := &RebalancePairEngine{
                Config:     config,
                ConfigFile: configPath,
                StateFile:  filepath.Join(getWorkingDir(), fmt.Sprintf("state_%s.json", config.PairName)),
        }

        engine.LoadState()
        
        // Safely add to registry
        registry.Add(engine)
        
        validateAndFundEngine(registry, engine, client, auth, walletAddress)
}

func reevaluateTrackedConfig(registry *EngineRegistry, configPath string, client *ethclient.Client) {
        engines := registry.GetEngines()
        for _, engine := range engines {
                if engine.ConfigFile == configPath {
                        oldStatus := engine.State.Status
                        engine.LoadState()
                        
                        if oldStatus == "FAILED" && (engine.State.Status == "" || engine.State.Status == "PENDING") {
                                logToFile(fmt.Sprintf("🔄 [SYSTEM INFO][%s] User cleared FAILED state. Re-evaluating configuration...", engine.Config.PairName))
                                
                                data, _ := os.ReadFile(engine.ConfigFile)
                                _ = json.Unmarshal(data, &engine.Config)
                                
                                validateAndInitializeEngine(engine, client)
                        }
                        return
                }
        }
}

func validateAndInitializeEngine(engine *RebalancePairEngine, client *ethclient.Client) {
        logToFile(fmt.Sprintf("🔍 [DEBUG][%s] Validating engine parameters...", engine.Config.PairName))

        // 1. Validation Checks
        if engine.State.Status == "FAILED" {
                logToFile(fmt.Sprintf("⏭️ [SYSTEM INFO][%s] Engine in FAILED state. Skipping. (Reason: %s)", engine.Config.PairName, engine.State.FailedReason))
                return
        }
        
        emptyAddr := common.Address{}
        if engine.Config.TokenAAddress == emptyAddr || engine.Config.TokenBAddress == emptyAddr {
                engine.FailState("Invalid token address mapping. Address cannot be 0x00...00.")
                return
        }

        if engine.Config.InitValueInUSD <= 0 {
                engine.FailState(fmt.Sprintf("Invalid InitValueInUSD: %.2f. Must be > 0.", engine.Config.InitValueInUSD))
                return
        }

        if engine.Config.TargetGrowthPct <= 0 || engine.Config.MinAlphaPct <= 0 {
                engine.FailState("TargetGrowthPct and MinAlphaPct must be strictly positive numerical thresholds.")
                return
        }

        // 2. Initial Setup (If valid and not yet monitoring)
        if engine.State.Status != "MONITORING" {
                logToFile(fmt.Sprintf("⚙️ [SYSTEM INFO][%s] Validation passed. Bootstrapping shared-pool virtual allocations...", engine.Config.PairName))
                
                bnbPrice := getLiveBNBPrice(client)
                priceA := getLiveTokenPriceInBNB(client, engine.Config.TokenAAddress) * bnbPrice
                priceB := getLiveTokenPriceInBNB(client, engine.Config.TokenBAddress) * bnbPrice
                
                if priceA <= 0 || priceB <= 0 {
                        engine.FailState(fmt.Sprintf("Failed to fetch live oracle spot prices. (PriceA: $%.4f, PriceB: $%.4f)", priceA, priceB))
                        return
                }

                targetHalfUSD := engine.Config.InitValueInUSD / 2.0
                
                // Set virtual allocations independent of absolute wallet balance
                engine.State.AllocatedQtyA = targetHalfUSD / priceA
                engine.State.AllocatedQtyB = targetHalfUSD / priceB
                engine.State.InitialValueUSD = engine.Config.InitValueInUSD
                
                engine.State.Status = "MONITORING"
                engine.State.FailedReason = ""
                // ADD THIS LINE HERE: Record the exact time the bot started watching this pair
                
                
                // --- NEW CODE: Stamp the config file ---
                if engine.Config.CreatedDate == "" {
                        // Use a standard readable timestamp format
                        engine.Config.CreatedDate = time.Now().Format("2006-01-02 15:04:05")
                        
                        // Construct the config filename dynamically based on your pair name
                        configFilename := fmt.Sprintf("config_%s.json", engine.Config.PairName)
                        
                        err := saveConfigToFile(engine.Config, configFilename)
                        if err != nil {
                                logToFile(fmt.Sprintf("⚠️ [%s] Failed to write created_date to config: %v", engine.Config.PairName, err))
                        } else {
                                logToFile(fmt.Sprintf("📅 [%s] Stamped created_date (%s) to %s", engine.Config.PairName, engine.Config.CreatedDate, configFilename))
                        }
                }
                engine.State.NextCheckTime = time.Now()
                engine.SaveState()

                logToFile(fmt.Sprintf("✅ [SUCCESS][%s] Engine active.", engine.Config.PairName))
                logToFile(fmt.Sprintf("   ↳ Locked Virtual Allocation %s: %.6f Qty (~$%s)", engine.Config.TokenAName, engine.State.AllocatedQtyA, formatUSD(targetHalfUSD)))
                logToFile(fmt.Sprintf("   ↳ Locked Virtual Allocation %s: %.6f Qty (~$%s)", engine.Config.TokenBName, engine.State.AllocatedQtyB, formatUSD(targetHalfUSD)))
        }
}

func validateAndFundEngine(registry *EngineRegistry, engine *RebalancePairEngine, client *ethclient.Client, auth *bind.TransactOpts, wallet common.Address) error {
        logToFile(fmt.Sprintf("🔍 [%s] Checking master pool liquidity...", engine.Config.PairName))

        // 1. Fetch live prices ONLY for USD valuation
        bnbPrice := getLiveBNBPrice(client)
        priceA := getLiveTokenPriceInBNB(client, engine.Config.TokenAAddress) * bnbPrice
        priceB := getLiveTokenPriceInBNB(client, engine.Config.TokenBAddress) * bnbPrice

        // 2. Fetch ACTUAL physical wallet balances
        physicalBalA := getERC20Balance(client, engine.Config.TokenAAddress, wallet)
        physicalBalB := getERC20Balance(client, engine.Config.TokenBAddress, wallet)

        // 3. STATE RECOVERY: If state already exists, restore it and move on
        if engine.State.AllocatedQtyA > 0 || engine.State.AllocatedQtyB > 0 {
                logToFile(fmt.Sprintf("ℹ️ [%s] Restoring existing virtual state (A: %.6f, B: %.6f)", 
                        engine.Config.PairName, engine.State.AllocatedQtyA, engine.State.AllocatedQtyB))
                
                if engine.State.NextCheckTime.IsZero() {
                        engine.State.NextCheckTime = time.Now()
                        engine.SaveState()
                }
                
                return nil
        }

        // 4. FRESH BOOT BOOTSTRAP: State file is empty/0. 
        logToFile(fmt.Sprintf("🌱 [%s] Fresh state detected. Evaluating physical wallet balances...", engine.Config.PairName))
        
        targetInitialUSD := engine.Config.InitValueInUSD 
        halfAllocationUSD := targetInitialUSD / 2.0

        // Derive target quantities based on current asset pricing
        targetQtyA := halfAllocationUSD / priceA
        targetQtyB := halfAllocationUSD / priceB

        // Calculate missing amounts
        missingA := 0.0
        if physicalBalA < targetQtyA {
                missingA = targetQtyA - physicalBalA
        }
        
        missingB := 0.0
        if physicalBalB < targetQtyB {
                missingB = targetQtyB - physicalBalB
        }

        // 5. Shortfall Detection & Routing
        if missingA > 0 || missingB > 0 {
                if engine.Config.StartMode == "START-AUTO" {
                        logToFile(fmt.Sprintf("⚠️ [%s] Insufficient tokens. START-AUTO mode engaged. Buying missing amounts (A: %.6f, B: %.6f)...", 
                                engine.Config.PairName, missingA, missingB))
                        
                        // Call the auto-funder logic
                        err := attemptPoolFunding(engine, client, auth, missingA, missingB, bnbPrice)
                        if err != nil {
                                errMsg := fmt.Sprintf("Auto-Funding Failed: %v", err)
                                engine.FailState(errMsg)
                                return fmt.Errorf(errMsg)
                        }
                        
                        logToFile(fmt.Sprintf("✅ [%s] Auto-funding complete.", engine.Config.PairName))
                        
                        // Refresh balances after successful purchase
                        physicalBalA = getERC20Balance(client, engine.Config.TokenAAddress, wallet)
                        physicalBalB = getERC20Balance(client, engine.Config.TokenBAddress, wallet)

                } else {
                        // START-MANUAL (or default)
                        errMsg := fmt.Sprintf("the physical wallet actually has not enough runway to cover this target allocation. Required: (A: %.6f, B: %.6f), Has: (A: %.6f, B: %.6f)", 
                                targetQtyA, targetQtyB, physicalBalA, physicalBalB)
                        engine.FailState(errMsg)                
                        return fmt.Errorf("Funding Failed (MANUAL MODE): %s", errMsg)
                }
        }

        
        // Lock the quantities into the virtual state, taking exactly 99.90% to leave a safety margin
        engine.State.AllocatedQtyA = physicalBalA * 0.999
        engine.State.AllocatedQtyB = physicalBalB * 0.999
        
        // Calculate the EXACT USD value of these adjusted quantities
        // (priceA and priceB are the USD prices calculated at the top of the function)
        actualValA := engine.State.AllocatedQtyA * priceA
        actualValB := engine.State.AllocatedQtyB * priceB
        adjustedTotalUSD := actualValA + actualValB
        
        // Establish the baseline valuations using the precise starting math
        engine.State.InitialValueUSD = adjustedTotalUSD
        engine.State.CurrentTotalUSD = adjustedTotalUSD
        
        // Set timestamps for baseline and next execution      
        engine.State.NextCheckTime = time.Now()        
        
        logToFile(fmt.Sprintf("✅ [%s] Virtual state locked at 99.90%%. Actual Baseline Value: $%.2f USD", 
                engine.Config.PairName, adjustedTotalUSD))
        
        // Force the engine into MONITORING status so Stage 3 picks it up
        // Your existing code locking the state...
        engine.State.Status = "MONITORING"
        
        logToFile(fmt.Sprintf("🚀 [%s] Status set to MONITORING. Handing off to daemon loop.", engine.Config.PairName))

        // --- NEW CODE: Stamp the config file ---
        if engine.Config.CreatedDate == "" {
                // Use a standard readable timestamp format
                engine.Config.CreatedDate = time.Now().Format("2006-01-02 15:04:05")
                
                // Construct the config filename dynamically based on your pair name
                configFilename := fmt.Sprintf("config_%s.json", engine.Config.PairName)
                
                err := saveConfigToFile(engine.Config, configFilename)
                if err != nil {
                        logToFile(fmt.Sprintf("⚠️ [%s] Failed to write created_date to config: %v", engine.Config.PairName, err))
                } else {
                        logToFile(fmt.Sprintf("📅 [%s] Stamped created_date (%s) to %s", engine.Config.PairName, engine.Config.CreatedDate, configFilename))
                }
        }
        engine.SaveState()
        
        logToFile(fmt.Sprintf("🚀 [%s] Status set to MONITORING. Handing off to daemon loop.", engine.Config.PairName))
        return nil
}
func saveConfigToFile(config PairConfig, filename string) error {
        file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
        if err != nil {
                return err
        }
        defer file.Close()
        
        encoder := json.NewEncoder(file)
        encoder.SetIndent("", "  ") // Keeps your JSON looking neat and readable
        return encoder.Encode(config)
}
func attemptPoolFunding(engine *RebalancePairEngine, client *ethclient.Client, auth *bind.TransactOpts, missingA, missingB, bnbPrice float64) error {
        logToFile(fmt.Sprintf("⚙️ [%s] Auto-funding initiated. Target Buys - A: %.6f, B: %.6f", 
                engine.Config.PairName, missingA, missingB))

        // 1. Estimate total BNB cost based on current DEX prices
        priceA_in_BNB := getLiveTokenPriceInBNB(client, engine.Config.TokenAAddress)
        priceB_in_BNB := getLiveTokenPriceInBNB(client, engine.Config.TokenBAddress)
        
        estimatedBnbCostA := missingA * priceA_in_BNB
        estimatedBnbCostB := missingB * priceB_in_BNB
        totalEstimatedBnb := estimatedBnbCostA + estimatedBnbCostB
        
        // Add 5% buffer strictly for slippage and gas fees
        requiredBnbWithBuffer := totalEstimatedBnb * 1.05 

        // 2. Verify BNB Balance
        bnbBalanceFloat := getBNBBalanceFloat(client, auth.From)
        if bnbBalanceFloat < requiredBnbWithBuffer {
                return fmt.Errorf("insufficient BNB for auto-funding. Need %.4f BNB (inc. buffer), have %.4f BNB", requiredBnbWithBuffer, bnbBalanceFloat)
        }

        // 3. Execute Swaps via your Router Integration
        if missingA > 0 {
                logToFile(fmt.Sprintf("🔄 [%s] Swapping approx %.4f BNB for %.6f Token A...", 
                        engine.Config.PairName, estimatedBnbCostA, missingA))
                        
                err := executeSwapBNBForTokens(client, auth, engine.Config.TokenAAddress, missingA, estimatedBnbCostA)
                if err != nil {
                        return fmt.Errorf("failed to buy Token A: %v", err)
                }
        }

        if missingB > 0 {
                logToFile(fmt.Sprintf("🔄 [%s] Swapping approx %.4f BNB for %.6f Token B...", 
                        engine.Config.PairName, estimatedBnbCostB, missingB))
                        
                err := executeSwapBNBForTokens(client, auth, engine.Config.TokenBAddress, missingB, estimatedBnbCostB)
                if err != nil {
                        return fmt.Errorf("failed to buy Token B: %v", err)
                }
        }

        return nil
}

func executeSwapBNBForTokens(client *ethclient.Client, auth *bind.TransactOpts, targetToken common.Address, amountOut float64, amountInMax float64) error {
        if amountOut <= 0 || amountInMax <= 0 {
                return fmt.Errorf("invalid swap amounts: out=%f, maxIn=%f", amountOut, amountInMax)
        }

        base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
        
        // 1. Calculate the exact amount of tokens we want back
        outAmtFloat := new(big.Float).Mul(big.NewFloat(amountOut), base18)
        amountOutBig := new(big.Int)
        outAmtFloat.Int(amountOutBig)

        // 2. Add a 5% Slippage Buffer to the Maximum BNB spend. 
        // PancakeSwap will automatically refund whatever BNB is not used.
        safeMaxInFloat := amountInMax * 1.05 
        
        inAmtMaxFloat := new(big.Float).Mul(big.NewFloat(safeMaxInFloat), base18)
        amountInMaxBig := new(big.Int)
        inAmtMaxFloat.Int(amountInMaxBig)

        nonce, err := client.PendingNonceAt(context.Background(), auth.From)
        if err != nil {
                return err
        }
        auth.Nonce = big.NewInt(int64(nonce))
        
        // Set the BNB value we are sending along with the transaction
        auth.Value = amountInMaxBig
        gasPrice, err := client.SuggestGasPrice(context.Background())
        if err != nil {
                return err
        }
        auth.GasPrice = gasPrice

        path := []common.Address{WBNB_ADDRESS, targetToken}
        routerABI, _ := abi.JSON(strings.NewReader(PANCAKE_ROUTER_ABI))
        contract := bind.NewBoundContract(ROUTER_ADDR, routerABI, client, client, client)
        
        // 3. Call the Exact Out endpoint
        deadline := big.NewInt(time.Now().Add(5 * time.Minute).Unix())
        tx, err := contract.Transact(auth, "swapETHForExactTokens", amountOutBig, path, auth.From, deadline)
        
        // 4. CRITICAL: Reset auth.Value immediately so subsequent transactions don't accidentally send BNB
        auth.Value = big.NewInt(0)
        
        if err != nil {
                return fmt.Errorf("contract transact failed: %v", err)
        }

        _, err = bind.WaitMined(context.Background(), client, tx)
        if err != nil {
                return fmt.Errorf("transaction failed during mining: %v", err)
        }

        return nil
}

func getNativeBNBBalance(client *ethclient.Client, account common.Address) float64 {
        balance, err := client.BalanceAt(context.Background(), account, nil)
        if err != nil {
                return 0.0
        }
        base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
        balFloat, _ := new(big.Float).Quo(new(big.Float).SetInt(balance), base18).Float64()
        return balFloat
}
 

func getLockedPoolAllocation(registry *EngineRegistry, tokenAddress common.Address) float64 {
        var lockedQty float64
        engines := registry.GetEngines()
        for _, engine := range engines {
                if engine.State.Status != "MONITORING" {
                        continue
                }
                if engine.Config.TokenAAddress == tokenAddress {
                        lockedQty += engine.State.AllocatedQtyA
                }
                if engine.Config.TokenBAddress == tokenAddress {
                        lockedQty += engine.State.AllocatedQtyB
                }
        }
        return lockedQty
}

func generateHashID() string {
        const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
        b := make([]byte, 8)
        for i := range b {
                b[i] = charset[rand.Intn(len(charset))]
        }
        return string(b)
}


func isConfigTracked(registry *EngineRegistry, configPath string) bool {
        engines := registry.GetEngines()
        for _, engine := range engines {
                if engine.ConfigFile == configPath {
                        return true
                }
        }
        return false
}
func generateFallbackName() string {
        const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
        b := make([]byte, 8)
        for i := range b {
                b[i] = charset[rand.Intn(len(charset))]
        }
        return fmt.Sprintf("TokenAvsTokenB_%s", string(b))
}
/// =======================
/// SYSTEM EXECUTION CONTEXT
/// =======================
func main() {
        actionFlag := flag.String("action", "start", "Execution panel entry command [start | kill | run]")
        flag.Parse()
        handleDaemonLifecycle(*actionFlag)

        pk := loadPrivateKey()
        client, _ := ethclient.Dial("https://bsc-dataseed.binance.org/")
        auth, walletAddress := getWallet(client, pk)
        rand.Seed(time.Now().UnixNano())

        // Start the directory watcher to load configs dynamically
        StartDynamicConfigWatcher(GlobalRegistry, client, auth, walletAddress)

        logToFile("📡 Dynamic Multi-Object Rebalancer Core Online. (Shared-Pool Engine Active)")

        // Track when we last executed our system solvency audit
        var lastAuditTime time.Time
        auditInterval := 60 * time.Second

        // 4. Main Event Loop
        for {
                // --- INTEGRITY SAFETY GATE ---
                // Executes a system-wide solvency audit every 60 seconds
                if time.Since(lastAuditTime) >= auditInterval {
                        auditSystemSolvency(GlobalRegistry, client, walletAddress)
                        lastAuditTime = time.Now()
                }

                engines := GlobalRegistry.GetEngines()
                for _, pair := range engines {
                        // Safe to read Status because we filter for the baseline "MONITORING" token
                        if pair.State.Status != "MONITORING" {
                                continue
                        }

                        if time.Now().Before(pair.State.NextCheckTime) {
                                continue
                        }

                        // RunStage3 will internally trigger evaluatePoolIntegrity using GetTotalAllocated
                        pair.RunStage3(client, auth, walletAddress)
                }
                
                time.Sleep(5 * time.Second)
        }
}
func getBNBBalanceFloat(client *ethclient.Client, account common.Address) float64 {
        balanceWei, err := client.BalanceAt(context.Background(), account, nil)
        if err != nil {
                logToFile(fmt.Sprintf("Failed to fetch BNB balance: %v", err))
                return 0.0
        }

        // Convert Wei to standard BNB format (divide by 10^18)
        base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
        balanceFloat := new(big.Float).SetInt(balanceWei)
        bnbBal, _ := new(big.Float).Quo(balanceFloat, base18).Float64()
        
        return bnbBal
}
// auditSystemSolvency inspects virtual vs physical pools for every distinct active asset.
func auditSystemSolvency(registry *EngineRegistry, client *ethclient.Client, walletAddr common.Address) {
        trackedTokens := make(map[common.Address]bool)
        
        registry.RLock()
        for _, engine := range registry.Engines {
                trackedTokens[engine.Config.TokenAAddress] = true
                trackedTokens[engine.Config.TokenBAddress] = true
        }
        registry.RUnlock()

        for token := range trackedTokens {
                physicalBal := getERC20Balance(client, token, walletAddr)

                // Convert *big.Float to float64 for comparison
                totalAllocatedBig := registry.GetTotalAllocated(token)
                totalAllocated, _ := totalAllocatedBig.Float64()

                if physicalBal < totalAllocated {
                        drift := totalAllocated - physicalBal
                        logToFile(fmt.Sprintf(
                                "🚨 [SOLVENCY DEFICIT ALERT] Asset %s underfunded! Physical: %.4f, Allocated Liability: %.4f (Deficit: %.4f)",
                                token.Hex(), physicalBal, totalAllocated, drift,
                        ))
                }
        }
}
/// =======================
/// DAEMON LOGISTICS & WRAPPERS
/// =======================
/*
func initializeDefaults() {
        // Run this before starting the watcher to ensure base pairs exist
        files, _ := filepath.Glob(filepath.Join(getWorkingDir(), "config_*.json"))
        if len(files) == 0 {
                writeDefaultConfig("WBTC_WUNI_agent_1", WBTC_ADDRESS, WUNI_ADDRESS, 24.028, 8.1945, 97.1826, 15, 4)
                writeDefaultConfig("WETH_ATOM_agent_2", WETH_ADDRESS, ATOM_ADDRESS, 161.56, 7.8945, 98.3827, 14, 2)
        }
}
*/
func writeDefaultConfig(pairName string, tokenA, tokenB common.Address, initValue, growthPct, minAlpha float64, maxMin, minMin int) {
        // Basic parser to split pair names (e.g., "WBTC_WUNI")
        parts := strings.Split(pairName, "_")
        nameA, nameB := "TokenA", "TokenB"
        if len(parts) == 2 {
                nameA, nameB = parts[0], parts[1]
        }

        config := PairConfig{
                PairName:        pairName,
                TokenAName:      nameA,
                TokenBName:      nameB,
                TokenAAddress:   tokenA,
                TokenBAddress:   tokenB,
                InitValueInUSD:  initValue,
                MinMins:         minMin,
                MaxMins:         maxMin,
                TargetGrowthPct: growthPct,
                MinAlphaPct:     minAlpha,
        }
        data, err := json.MarshalIndent(config, "", "  ")
        if err == nil {
                _ = os.WriteFile(filepath.Join(getWorkingDir(), fmt.Sprintf("config_%s.json", pairName)), data, 0644)
        }
}

func getWorkingDir() string {
        executable, err := os.Executable()
        if err != nil { return "." }
        return filepath.Dir(executable)
}

func logToFile(message string) {
        logPath := filepath.Join(getWorkingDir(), LOG_FILE)
        f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
        if err != nil { return }
        defer f.Close()
        logFormat := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), message)
        fmt.Print(logFormat)
        f.WriteString(logFormat)
}

func handleDaemonLifecycle(action string) {
        pidPath := filepath.Join(getWorkingDir(), PID_FILE)
        switch action {
        case "start":
                if _, err := os.Stat(pidPath); err == nil {
                        log.Fatalf("[CRITICAL] Daemon is already active or PID lock tracking file exists.")
                }
                executable, _ := os.Executable()
                cmd := exec.Command(executable, "-action=run")
                cmd.Dir = getWorkingDir()
                _ = cmd.Start()
                _ = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
                fmt.Printf("🚀 Pair Matrix Engine Daemon spawned. PID: %d\n", cmd.Process.Pid)
                os.Exit(0)
        case "kill":
                data, err := os.ReadFile(pidPath)
                if err != nil {
                        fmt.Printf("❌ No PID file found: %v\n", err)
                        os.Exit(1)
                }
                pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
                process, err := os.FindProcess(pid)
                if err == nil {
                        _ = process.Kill()
                        fmt.Printf("🛑 Pair Matrix Engine Daemon PID %d terminated.\n", pid)
                }
                _ = os.Remove(pidPath)
                os.Exit(0)
        case "run":
        default:
                log.Fatalf("Unknown entry instruction parameter flag.")
        }
}

func formatWithSpacedDecimals(val float64) string {
        rawStr := fmt.Sprintf("%.4f", val)
        parts := strings.Split(rawStr, ".")
        intPart, decPart := parts[0], parts[1]
        var intResult []string
        for i, c := range intPart {
                if i > 0 && (len(intPart)-i)%3 == 0 {
                        intResult = append(intResult, ",")
                }
                intResult = append(intResult, string(c))
        }
        return strings.Join(intResult, "") + "." + decPart
}

func loadPrivateKey() string {
        dir := getWorkingDir()
        data, err := os.ReadFile(filepath.Join(dir, "config.env"))
        if err != nil { log.Fatal("Missing config.env") }
        for _, line := range strings.Split(string(data), "\n") {
                line = strings.TrimSpace(line)
                if strings.HasPrefix(line, "PRIVATE_KEY=") {
                        return strings.TrimPrefix(line, "PRIVATE_KEY=")
                }
        }
        return ""
}

func getLiveBNBPrice(client *ethclient.Client) float64 {
        amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
        return executeAmountsOutCall(client, amountIn, []common.Address{WBNB_ADDRESS, USDT_ADDRESS})
}

func getLiveTokenPriceInBNB(client *ethclient.Client, tokenAddress common.Address) float64 {
        // Query the price for 0.01 tokens instead of 1 full token to bypass slippage penalties
        base18 := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
        amountIn := new(big.Int).Div(base18, big.NewInt(100)) 
        
        path := []common.Address{tokenAddress, WBNB_ADDRESS}
        microPrice := executeAmountsOutCall(client, amountIn, path)
        
        // Scale the result back up to represent the price of 1 full token
        return microPrice * 100 
}


func executeAmountsOutCall(client *ethclient.Client, amountIn *big.Int, path []common.Address) float64 {
        routerABI, _ := abi.JSON(strings.NewReader(PANCAKE_ROUTER_ABI))
        contract := bind.NewBoundContract(ROUTER_ADDR, routerABI, client, client, client)
        var result []interface{}
        
        err := contract.Call(nil, &result, "getAmountsOut", amountIn, path)
        if err != nil || len(result) == 0 || result[0] == nil { return 0.0 }
        
        amounts, ok := result[0].([]*big.Int)
        if !ok || len(amounts) == 0 { return 0.0 }
        
        base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
        price, _ := new(big.Float).Quo(new(big.Float).SetInt(amounts[len(amounts)-1]), base18).Float64()
        return price
}

func formatUSD(val float64) string {
        if val > 0 && val < 0.01 {
                return fmt.Sprintf("$%.8f", val) 
        }
        return fmt.Sprintf("$%.6f", val)
}

func getERC20Balance(client *ethclient.Client, token common.Address, owner common.Address) float64 {
        erc20ABI, _ := abi.JSON(strings.NewReader(ERC20_ABI))
        contract := bind.NewBoundContract(token, erc20ABI, client, client, client)
        var result []interface{}
        _ = contract.Call(nil, &result, "balanceOf", owner)
        if len(result) == 0 { return 0.0 }
        base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
        cleanBal, _ := new(big.Float).Quo(new(big.Float).SetInt(result[0].(*big.Int)), base18).Float64()
        return cleanBal
}

func swapBNBForToken(client *ethclient.Client, auth *bind.TransactOpts, targetToken common.Address, amountBNB float64) float64 {
        if amountBNB <= 0 { return 0.0 }
        base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
        bigAmt := new(big.Float).Mul(big.NewFloat(amountBNB), base18)
        amountIn := new(big.Int)
        bigAmt.Int(amountIn)

        nonce, _ := client.PendingNonceAt(context.Background(), auth.From)
        auth.Nonce = big.NewInt(int64(nonce))
        
        auth.Value = amountIn
        gasPrice, _ := client.SuggestGasPrice(context.Background())
        auth.GasPrice = gasPrice

        path := []common.Address{WBNB_ADDRESS, targetToken}
        routerABI, _ := abi.JSON(strings.NewReader(PANCAKE_ROUTER_ABI))
        contract := bind.NewBoundContract(ROUTER_ADDR, routerABI, client, client, client)
        
        tx, err := contract.Transact(auth, "swapExactETHForTokens", big.NewInt(0), path, auth.From, big.NewInt(time.Now().Add(5*time.Minute).Unix()))
        if err != nil { return 0.0 }

        receipt, err := bind.WaitMined(context.Background(), client, tx)
        if err != nil { return 0.0 }

        gasUsed := new(big.Float).SetInt64(int64(receipt.GasUsed))
        effectiveGasPrice := new(big.Float).SetInt(tx.GasPrice())
        totalGasWei := new(big.Float).Mul(gasUsed, effectiveGasPrice)
        feeBNB, _ := new(big.Float).Quo(totalGasWei, base18).Float64()

        return feeBNB
}

func ensureRouterAllowance(client *ethclient.Client, auth *bind.TransactOpts, token common.Address, owner common.Address) {
        erc20ABI, _ := abi.JSON(strings.NewReader(ERC20_ABI))
        contract := bind.NewBoundContract(token, erc20ABI, client, client, client)

        var result []interface{}
        err := contract.Call(nil, &result, "allowance", owner, ROUTER_ADDR)
        if err != nil || len(result) == 0 { return }

        currentAllowance := result[0].(*big.Int)
        
        if currentAllowance.Cmp(big.NewInt(1000000000000000000)) < 0 {
                logToFile(fmt.Sprintf("🔒 Missing Router allowance for token %s. Executing infinite approve...", token.Hex()))
                
                nonce, _ := client.PendingNonceAt(context.Background(), auth.From)
                auth.Nonce = big.NewInt(int64(nonce))
                gasPrice, _ := client.SuggestGasPrice(context.Background())
                auth.GasPrice = gasPrice
                auth.Value = big.NewInt(0)

                maxApproval := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1))
                
                tx, err := contract.Transact(auth, "approve", ROUTER_ADDR, maxApproval)
                if err != nil {
                        logToFile(fmt.Sprintf("❌ Approval failed to broadcast: %v", err))
                        return
                }

                logToFile(fmt.Sprintf("⏳ Waiting for approval confirmation... Tx: %s", tx.Hash().Hex()))
                _, _ = bind.WaitMined(context.Background(), client, tx)
        }
}

func (engine *RebalancePairEngine) evaluatePoolIntegrity(physicalBalA, physicalBalB float64, allActiveEngines []*RebalancePairEngine) error {
        threshold := engine.Config.ThresholdPct

        // 1. Calculate how much of Token A and B are locked by OTHER engines
        var lockedByOthersA float64
        var lockedByOthersB float64

        // Step 1: Use GlobalRegistry to fetch absolute liabilities safely
        totalAllocA_Big := GlobalRegistry.GetTotalAllocated(engine.Config.TokenAAddress)
        totalAllocB_Big := GlobalRegistry.GetTotalAllocated(engine.Config.TokenBAddress)

        // Convert the *big.Float values to standard float64 for math operations
        totalAllocatedA, _ := totalAllocA_Big.Float64()
        totalAllocatedB, _ := totalAllocB_Big.Float64()

        // Step 2: Deduct this engine's own slice to calculate what other pairs are utilizing
        engine.mu.Lock()
        lockedByOthersA = totalAllocatedA - engine.State.AllocatedQtyA
        lockedByOthersB = totalAllocatedB - engine.State.AllocatedQtyB
        engine.mu.Unlock()
        
        // 2. Derive what is ACTUALLY available specifically for this engine instance
        availPhysicalA := physicalBalA - lockedByOthersA
        availPhysicalB := physicalBalB - lockedByOthersB

        // 3. Perform integrity checks against the AVAILABLE pool headroom, not the raw wallet pool
        // --- TOKEN A CHECK ---
        if availPhysicalA < engine.State.AllocatedQtyA {
                requestedA := engine.State.AllocatedQtyA
                // If it goes negative due to over-allocation by others, cap it cleanly at 0 for calculation
                if availPhysicalA < 0 {
                        availPhysicalA = 0
                }
                shortfallA := requestedA - availPhysicalA
                shortfallRatioA := (shortfallA / requestedA) * 100.0

                if shortfallRatioA > threshold {
                        // Create the message string once so it's identical for both calls
                        failMsg := fmt.Sprintf("Token A integrity violation: Shared asset over-allocated by concurrent engines. Shortfall: %.3f%% (Req: %.4f, Net Avail: %.4f) Threshold: %.1f%%", 
                                shortfallRatioA, requestedA, availPhysicalA, threshold)
                        
                        engine.FailState(failMsg)     
                        return fmt.Errorf("%s", failMsg)
                }

                adjustedA := availPhysicalA * 0.999
                logToFile(fmt.Sprintf("⚠️ [%s] Token A net headroom mismatch within limits (%.3f%%). Adjusting state to %.6f", 
                        engine.Config.PairName, shortfallRatioA, adjustedA))
                
                engine.mu.Lock()
                engine.State.AllocatedQtyA = adjustedA
                engine.mu.Unlock()
        }

        // --- TOKEN B CHECK ---
        if availPhysicalB < engine.State.AllocatedQtyB {
                requestedB := engine.State.AllocatedQtyB
                if availPhysicalB < 0 {
                        availPhysicalB = 0
                }
                shortfallB := requestedB - availPhysicalB
                shortfallRatioB := (shortfallB / requestedB) * 100.0

                if shortfallRatioB > threshold {
                        return fmt.Errorf("Token B integrity violation: Shared asset over-allocated by concurrent engines. Shortfall: %.3f%% (Req: %.4f, Net Avail: %.4f) Threshold: %.1f%%", 
                                shortfallRatioB, requestedB, availPhysicalB, threshold)
                }

                adjustedB := availPhysicalB * 0.999
                logToFile(fmt.Sprintf("⚠️ [%s] Token B mismatch within limits (%.3f%%). Adjusting...", engine.Config.PairName, shortfallRatioB))
                
                engine.mu.Lock()
                engine.State.AllocatedQtyB = adjustedB
                engine.mu.Unlock()
        }

        return nil
}
func swapTokenForToken(client *ethclient.Client, auth *bind.TransactOpts, tokenIn common.Address, tokenOut common.Address, amountIn float64) float64 {
        logToFile(fmt.Sprintf("🔄 [EXEC DEBUG] Initiating Token Swap sequence... Volume: %.6f", amountIn))
        ensureRouterAllowance(client, auth, tokenIn, auth.From)
        
        base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
        scaledAmt := new(big.Float).Mul(big.NewFloat(amountIn), base18)
        amountInWei := new(big.Int)
        scaledAmt.Int(amountInWei)

        if amountInWei.Cmp(big.NewInt(0)) <= 0 {
                logToFile("❌ [EXEC ERROR] Swap aborted: calculated transaction volume rounds down to zero wei.")
                return 0
        }

        nonce, _ := client.PendingNonceAt(context.Background(), auth.From)
        auth.Nonce = big.NewInt(int64(nonce))
        gasPrice, _ := client.SuggestGasPrice(context.Background())
        auth.GasPrice = gasPrice
        auth.Value = big.NewInt(0)

        path := []common.Address{tokenIn, tokenOut}
        deadline := big.NewInt(time.Now().Unix() + 1200)

        routerABI, _ := abi.JSON(strings.NewReader(PANCAKE_ROUTER_ABI))
        contract := bind.NewBoundContract(ROUTER_ADDR, routerABI, client, client, client)

        tx, err := contract.Transact(auth, "swapExactTokensForTokens", amountInWei, big.NewInt(0), path, auth.From, deadline)
        if err != nil {
                logToFile(fmt.Sprintf("❌ [EXEC ERROR] Execution failed on router transaction pipeline: %v", err))
                return 0
        }

        logToFile(fmt.Sprintf("🚀 [EXEC] Tx broadcasted safely! Hash: %s", tx.Hash().Hex()))

        receipt, err := bind.WaitMined(context.Background(), client, tx)
        if err != nil {
                logToFile(fmt.Sprintf("⚠️ [EXEC WARN] Failed to fetch execution receipt: %v", err))
                return 0
        }

        gasUsed := new(big.Float).SetInt64(int64(receipt.GasUsed))
        effectiveGasPrice := new(big.Float).SetInt(tx.GasPrice())
        totalGasWei := new(big.Float).Mul(gasUsed, effectiveGasPrice)
        gasCostBNB, _ := new(big.Float).Quo(totalGasWei, base18).Float64()

        logToFile(fmt.Sprintf("✅ [EXEC SUCCESS] Block confirmed. Fee Paid: %.8f BNB", gasCostBNB))
        return gasCostBNB
}
func executeSwapETHForExactTokens(
        client *ethclient.Client,
        auth *bind.TransactOpts,
        router common.Address,
        wbnb common.Address,
        token common.Address,
        amountOut float64,
        maxBnb float64,
        tokenDecimals int,
) (*types.Transaction, error) {
        parsedABI, err := abi.JSON(strings.NewReader(routerABI))
        if err != nil {
                return nil, err
        }

        // 1. Convert exact target token amount to uint256 based on its decimals
        outWeiFloat := new(big.Float).Mul(big.NewFloat(amountOut), new(big.Float).SetFloat64(math.Pow(10, float64(tokenDecimals))))
        finalAmountOut := new(big.Int)
        outWeiFloat.Int(finalAmountOut)

        // 2. Convert max BNB allowed to uint256 (18 decimals)
        maxBnbWeiFloat := new(big.Float).Mul(big.NewFloat(maxBnb), big.NewFloat(1e18))
        finalMaxBnb := new(big.Int)
        maxBnbWeiFloat.Int(finalMaxBnb)

        path := []common.Address{wbnb, token}
        deadline := big.NewInt(time.Now().Add(5 * time.Minute).Unix())

        // 3. Clone TransactOpts to inject the exact payable Value for this specific swap
        swapAuth := &bind.TransactOpts{
                From:   auth.From,
                Signer: auth.Signer,
                Value:  finalMaxBnb, // The router requires the transaction value to cover the max spend
        }

        // 4. Bind and Transact
        contract := bind.NewBoundContract(router, parsedABI, client, client, client)
        return contract.Transact(swapAuth, "swapETHForExactTokens", finalAmountOut, path, auth.From, deadline)
}
func unwrapWBNB(client *ethclient.Client, auth *bind.TransactOpts) {
        wbnbBalance := getERC20Balance(client, WBNB_ADDRESS, auth.From)
        
        // Threshold check to avoid unwrapping microscopic dust and wasting gas
        if wbnbBalance > 0.002 {
                logToFile(fmt.Sprintf("🔋 [GAS GUARD] Detected %.6f WBNB in wallet. Unwrapping to native BNB for gas refueling...", wbnbBalance))
                
                base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
                bigAmt := new(big.Float).Mul(big.NewFloat(wbnbBalance), base18)
                amountInWei := new(big.Int)
                bigAmt.Int(amountInWei)

                nonce, _ := client.PendingNonceAt(context.Background(), auth.From)
                auth.Nonce = big.NewInt(int64(nonce))
                gasPrice, _ := client.SuggestGasPrice(context.Background())
                auth.GasPrice = gasPrice
                auth.Value = big.NewInt(0)

                wbnbABI, _ := abi.JSON(strings.NewReader(WBNB_ABI))
                contract := bind.NewBoundContract(WBNB_ADDRESS, wbnbABI, client, client, client)
                
                tx, err := contract.Transact(auth, "withdraw", amountInWei)
                if err != nil {
                        logToFile(fmt.Sprintf("❌ [GAS GUARD ERROR] Failed to unwrap WBNB: %v", err))
                        return
                }
                
                logToFile(fmt.Sprintf("🚀 [GAS GUARD] Unwrap broadcasted! Hash: %s", tx.Hash().Hex()))
                _, err = bind.WaitMined(context.Background(), client, tx)
                if err != nil {
                        logToFile(fmt.Sprintf("⚠️ [GAS GUARD WARN] Failed to fetch unwrap receipt: %v", err))
                } else {
                        logToFile("✅ [GAS GUARD SUCCESS] Successfully converted WBNB to native BNB. Tank refueled.")
                }
        } else {
                logToFile(fmt.Sprintf("🔋 [GAS GUARD INFO] WBNB balance (%.6f) is below standard unwrap threshold. Leaving as is.", wbnbBalance))
        }
}

func getWallet(client *ethclient.Client, pk string) (*bind.TransactOpts, common.Address) {
        privateKey, _ := crypto.HexToECDSA(pk)
        auth, _ := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(56))
        return auth, auth.From
}

const WBNB_ABI = `[
        {
                "constant": true,
                "inputs": [{"name": "account", "type": "address"}],
                "name": "balanceOf",
                "outputs": [{"name": "", "type": "uint256"}],
                "payable": false,
                "stateMutability": "view",
                "type": "function"
        },
        {
                "constant": false,
                "inputs": [
                        {"name": "wad", "type": "uint256"}
                ],
                "name": "withdraw",
                "outputs": [],
                "payable": false,
                "stateMutability": "nonpayable",
                "type": "function"
        }
]`
// Standard V2 Router ABI strictly for swapETHForExactTokens
const routerABI = `[{"inputs":[{"internalType":"uint256","name":"amountOut","type":"uint256"},{"internalType":"address[]","name":"path","type":"address[]"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"deadline","type":"uint256"}],"name":"swapETHForExactTokens","outputs":[{"internalType":"uint256[]","name":"amounts","type":"uint256[]"}],"stateMutability":"payable","type":"function"}]`
const PANCAKE_ROUTER_ABI = `[
        {
                "inputs": [
                        {"internalType": "uint256", "name": "amountOutMin", "type": "uint256"},
                        {"internalType": "address[]", "name": "path", "type": "address[]"},
                        {"internalType": "address", "name": "to", "type": "address"},
                        {"internalType": "uint256", "name": "deadline", "type": "uint256"}
                ],
                "name": "swapExactETHForTokens",
                "outputs": [
                        {"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}
                ],
                "stateMutability": "payable",
                "type": "function"
        },
        {
                "inputs": [
                        {"internalType": "uint256", "name": "amountIn", "type": "uint256"},
                        {"internalType": "uint256", "name": "amountOutMin", "type": "uint256"},
                        {"internalType": "address[]", "name": "path", "type": "address[]"},
                        {"internalType": "address", "name": "to", "type": "address"},
                        {"internalType": "uint256", "name": "deadline", "type": "uint256"}
                ],
                "name": "swapExactTokensForTokens",
                "outputs": [
                        {"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}
                ],
                "stateMutability": "nonpayable",
                "type": "function"
        },
        {
                "inputs": [
                        {"internalType": "uint256", "name": "amountIn", "type": "uint256"},
                        {"internalType": "address[]", "name": "path", "type": "address[]"}
                ],
                "name": "getAmountsOut",
                "outputs": [
                        {"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [
                        {"internalType": "uint256", "name": "amountOut", "type": "uint256"},
                        {"internalType": "address[]", "name": "path", "type": "address[]"},
                        {"internalType": "address", "name": "to", "type": "address"},
                        {"internalType": "uint256", "name": "deadline", "type": "uint256"}
                ],
                "name": "swapETHForExactTokens",
                "outputs": [
                        {"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}
                ],
                "stateMutability": "payable",
                "type": "function"
        }
]`
const ERC20_ABI = `[
        {"name":"balanceOf","type":"function","inputs":[{"name":"account","type":"address"}],"outputs":[{"name":"balance","type":"uint256"}],"stateMutability":"view"},
        {"name":"allowance","type":"function","inputs":[{"name":"owner","type":"address"},{"name":"spender","type":"address"}],"outputs":[{"name":"","type":"uint256"}],"stateMutability":"view"},
        {"name":"approve","type":"function","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}],"stateMutability":"nonpayable"}
]`
