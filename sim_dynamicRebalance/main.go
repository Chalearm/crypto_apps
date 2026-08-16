/******************************************************************************
 * File Name        : main.go
 * File Path        : dynamicRebalance/main.go
 * Author           : Chalearm Saelim & Gemini
 * Version          : 1.2.0
 * Status           : Production / Functional Development
 * Description      : Automated smart portfolio rebalancing array mapping 
 *                    mathematical drift offsets with cumulative fee tracking.
 ******************************************************************************/
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"time"
)

const (
	CheckpointFile = "state_checkpoint.json"
	GasFeeRate     = 0.04 // 4% total gas fee drag
	EarningRate    = 0.07 // 7% earning cost drag
)

// PortfolioState preserves token layout allocations and running metrics
type PortfolioState struct {
	Day                 int       `json:"day"`
	UniQuantity         float64   `json:"uni_quantity"`
	BtcQuantity         float64   `json:"btc_quantity"`
	UniPrice            float64   `json:"uni_price"`
	BtcPrice            float64   `json:"btc_price"`
	LastRatio           float64   `json:"last_ratio"`
	TotalTradeProfitUSD float64   `json:"total_trade_profit_usd"`
	AccumulatedFees     float64   `json:"accumulated_fees"`
	FeesPaidCount       int       `json:"fees_paid_count"`
	NextActionRequired  string    `json:"next_action_required"` // "SELL_UNI" or "BUY_UNI"
	LastUniPriceUSD     float64   `json:"last_uni_price_usd"`   // Reference price checkpoint
	LastUpdatedAt       time.Time `json:"last_updated_at"`
}

func main() {
	// 1. Initialize Logging subsystem from infra patch
	InitLogger()
	SetDaemonID("REBALANCE-DAEMON")
	NewCorrelationID()

	Info("🚀 Launching Automated Rebalancing Engine Matrix...")

	// 2. Initialize or Recover State Engine
	state := loadOrCreateState()

	// Keep track of yesterday's prices to compute the daily % change
	var lastUniPrice, lastBtcPrice float64

	// 4. Execution Loop Simulation / Matrix Processing
	for state.Day <= 5 {
		fmt.Printf("\n--- [PROCESSING PHASE: DAY %d] ---\n", state.Day)
		
		// Capture yesterday's price before generating the new random walk step
		lastUniPrice = state.UniPrice
		lastBtcPrice = state.BtcPrice

		// Fetch or simulate current dynamic prices
		state.UniPrice = simulateRandomWalk(state.UniPrice, 2.0, 8.0, 0.05)
		state.BtcPrice = simulateRandomWalk(state.BtcPrice, 50000.0, 80000.0, 0.03)

		uniPrice := state.UniPrice
		btcPrice := state.BtcPrice

		// Calculate percentage price changes from yesterday
		var uniPct, btcPct float64
		var uniPctChangeStr, btcPctChangeStr string
		
		if state.Day == 1 || lastUniPrice == 0 || lastBtcPrice == 0 {
			uniPctChangeStr = "0.0000000%"
			btcPctChangeStr = "0.0000000%"
		} else {
			uniPct = ((uniPrice - lastUniPrice) / lastUniPrice) * 100.0
			btcPct = ((btcPrice - lastBtcPrice) / lastBtcPrice) * 100.0
			
			uniPctChangeStr = fmt.Sprintf("%+s%%", formatWithSpacedDecimals(uniPct))
			btcPctChangeStr = fmt.Sprintf("%+s%%", formatWithSpacedDecimals(btcPct))
		}

		// SAFETY GUARD 1: Prevent crash state if quantities are broken on boot
		if state.UniQuantity <= 0 || state.BtcQuantity <= 0 {
			Warn(fmt.Sprintf("⚠️ Day %d stopped: Detected negative asset quantity (UNI: %.6f, BTC: %.6f).", state.Day, state.UniQuantity, state.BtcQuantity))
			state.Day++
			saveState(state)
			time.Sleep(1 * time.Second)
			continue
		}

		uniValueUSD := state.UniQuantity * uniPrice 
		btcValueUSD := state.BtcQuantity * btcPrice
		totalPortfolioUSD := uniValueUSD + btcValueUSD
		currentRatio := uniValueUSD / btcValueUSD

		// 1. New Percentage Drift Metric Core Formula
		pctDiff := uniPct - btcPct // Total divergence speed difference
		absPctDiff := math.Abs(pctDiff)

		fmt.Printf("🔍 Ratio & Percentage Equation Day %d:\n", state.Day)
		fmt.Printf("   UNI Vector -> Q: %s * P: $%.2f (%s) = Value: $%s\n", 
			formatWithSpacedDecimals(state.UniQuantity), uniPrice, uniPctChangeStr, formatWithSpacedDecimals(uniValueUSD))
		fmt.Printf("   BTC Vector -> Q: %s * P: $%.2f (%s) = Value: $%s\n", 
			formatWithSpacedDecimals(state.BtcQuantity), btcPrice, btcPctChangeStr, formatWithSpacedDecimals(btcValueUSD))
		fmt.Printf("   Formula    -> R(%d) = (Q_uni * P_uni) / (Q_btc * P_btc) = %.4f\n", state.Day, currentRatio)
		
		fmt.Printf("📊 Percentage Drift Analysis:\n")
		fmt.Printf("   UNI Change: %+s%% | BTC Change: %+s%%\n", formatWithSpacedDecimals(uniPct), formatWithSpacedDecimals(btcPct))
		fmt.Printf("   Divergence Delta -> |UNI%% - BTC%%| = |%s%%| = %s%%\n", formatWithSpacedDecimals(pctDiff), formatWithSpacedDecimals(absPctDiff))

		// 2. Convert Half-Drift value to raw USD sizing first
		halfDiffPct := absPctDiff / 2.0
		var rawShiftValueUSD float64
		if pctDiff > 0 {
			rawShiftValueUSD = (halfDiffPct / 100.0) * uniValueUSD
		} else {
			rawShiftValueUSD = (halfDiffPct / 100.0) * btcValueUSD
		}

		// Calculate routing math in native BNB denominations
		bnbPriceUSD := 580.00 
		rawShiftInBNB := rawShiftValueUSD / bnbPriceUSD

		// Real Protocol Fees from UI (Double it because of the 2-hop route: UNI->BNB->WBTC)
		pancakeSwapFeePerHopBNB := 0.00000000428398
		totalTradingFeeBNB := pancakeSwapFeePerHopBNB * 2
		networkGasFeeBNB := 0.0001 * 2 

		// Total structural Gas & Fee drags
		gasDragBNB := totalTradingFeeBNB + networkGasFeeBNB
		gasDragUSD := gasDragBNB * bnbPriceUSD

		// Calculate your 7% profit slice
		earningCostDragBNB := (rawShiftInBNB - gasDragBNB) * EarningRate
		earningCostDragUSD := earningCostDragBNB * bnbPriceUSD

		// Deduct costs to locate your pure buyback target value
		buyValueBNB := rawShiftInBNB - gasDragBNB - earningCostDragBNB
		buyValueUSD := buyValueBNB * bnbPriceUSD

		// Total required overhead that the trade must overcome
		minRequiredValueUSD := gasDragUSD + earningCostDragUSD

		// 3. STRICT DOUBLE GUARD: 
		// Guard A: Raw trade value must clear overall costs.
		// Guard B: The 7% Earning Value must explicitly be greater than the Gas/Fee Drag.
		if rawShiftValueUSD <= minRequiredValueUSD || buyValueUSD <= 0 || earningCostDragUSD <= gasDragUSD {
			var reason string
			if earningCostDragUSD <= gasDragUSD && rawShiftValueUSD > 0 {
				reason = fmt.Sprintf("Earning value ($%s) failed to beat Gas fee ($%s)", 
					formatWithSpacedDecimals(earningCostDragUSD), formatWithSpacedDecimals(gasDragUSD))
			} else {
				reason = "Total trade value does not clear minimal costs"
			}

			Debug(fmt.Sprintf("No rebalancing required for Day %d. 🛑 [GUARD TRIGGERED]: %s. Skipping.", state.Day, reason))
			
			state.LastRatio = currentRatio
			state.Day++
			state.LastUpdatedAt = time.Now()
			saveState(state)
			time.Sleep(50 * time.Millisecond)
			continue
		}

		// Print trace logs once the threshold is cleared successfully
		fmt.Printf("💸 2-Hop Routing Optimization via BNB Chain (Threshold Passed):\n")
		fmt.Printf("   Divergence Half Diff     = %s%%\n", formatWithSpacedDecimals(halfDiffPct))
		fmt.Printf("   Raw Target Shift Value   = %s BNB ($%s)\n", formatWithSpacedDecimals(rawShiftInBNB), formatWithSpacedDecimals(rawShiftValueUSD))
		fmt.Printf("   (-) Real Routing Gas Drag= %s BNB ($%s) [2x Hops Swaps]\n", formatWithSpacedDecimals(gasDragBNB), formatWithSpacedDecimals(gasDragUSD))
		fmt.Printf("   (-) Earning Value (7%%)  = %s BNB ($%s) [✨ WINS AGAINST GAS]\n", formatWithSpacedDecimals(earningCostDragBNB), formatWithSpacedDecimals(earningCostDragUSD))
		fmt.Printf("   (=) Net Target Buy back  = %s BNB ($%s) [Net Profit Win]\n", formatWithSpacedDecimals(buyValueBNB), formatWithSpacedDecimals(buyValueUSD))
		// 4. Sequence Lock Execution Steps
		if buyValueUSD > 0 {
			Info(fmt.Sprintf("⚠️ Strategy Drift Breach Validated on Day %d! Running sequence checks...", state.Day))
			
			if pctDiff > 0 {
				fmt.Printf("🔄 [DIRECTION ALERT]: UNI outperformed BTC by %s%% -> Strategy proposes Selling UNI, Buying BTC\n", formatWithSpacedDecimals(absPctDiff))

				if state.NextActionRequired != "SELL_UNI" {
					Warn(fmt.Sprintf("🔒 Lock Blocked: Strategy requires a 'BUY_UNI' operation next. Skipping Sell action on Day %d.", state.Day))
				} else {
					tokensToSell := rawShiftValueUSD / uniPrice
					
					if state.UniQuantity - tokensToSell <= 0 {
						Warn("❌ Rebalance aborted: Action would push UNI quantity into negative space.")
					} else {
						state.AccumulatedFees += gasDragUSD
						state.FeesPaidCount++
						state.TotalTradeProfitUSD += earningCostDragUSD

						state.UniQuantity -= tokensToSell
						state.BtcQuantity += buyValueUSD / btcPrice
						
						state.LastUniPriceUSD = uniPrice
						state.NextActionRequired = "BUY_UNI"
						
						Info(fmt.Sprintf("🔄 Action Executed: Sold %s UNI. Next step locked to BUY_UNI below $%.4f", formatWithSpacedDecimals(tokensToSell), state.LastUniPriceUSD))
					}
				}
			} else {
				fmt.Printf("🔄 [DIRECTION ALERT]: BTC outperformed UNI by %s%% -> Strategy proposes Selling BTC, Buying UNI\n", formatWithSpacedDecimals(absPctDiff))

				if state.NextActionRequired != "BUY_UNI" {
					Warn(fmt.Sprintf("🔒 Lock Blocked: Strategy requires a 'SELL_UNI' operation next. Skipping Buy action on Day %d.", state.Day))
				} else if uniPrice >= state.LastUniPriceUSD {
					Warn(fmt.Sprintf("🛑 Profit Guard: Current UNI Price ($%.4f) >= Last Sell Price ($%.4f). Buyback skipped.", uniPrice, state.LastUniPriceUSD))
				} else {
					tokensToSell := rawShiftValueUSD / btcPrice

					if state.BtcQuantity - tokensToSell <= 0 {
						Warn("❌ Rebalance aborted: Action would push BTC quantity into negative space.")
					} else {
						state.AccumulatedFees += gasDragUSD
						state.FeesPaidCount++
						state.TotalTradeProfitUSD += earningCostDragUSD

						state.BtcQuantity -= tokensToSell
						state.UniQuantity += buyValueUSD / uniPrice
						
						state.NextActionRequired = "SELL_UNI"
						Info(fmt.Sprintf("🔄 Action Executed: Bought back UNI at a profit ($%.4f cheaper).", state.LastUniPriceUSD - uniPrice))
					}
				}
			}
		}

		// Update persistent states safely for the next processing block
		state.LastRatio = currentRatio
		state.Day++
		state.LastUpdatedAt = time.Now()

		saveState(state)
		
		fmt.Printf("📊 Net Metrics Status: [UNI: $%.2f | BTC: $%.2f] | Inventory USD Value=$%s | UNI_Qty=%s | BTC_Qty=%s\n", 
			uniPrice, btcPrice, formatWithSpacedDecimals(totalPortfolioUSD), formatWithSpacedDecimals(state.UniQuantity), formatWithSpacedDecimals(state.BtcQuantity),
		)
		fmt.Printf("💸 Pure Trade Performance: [Accumulated Earnings (7%% Sum): $%s] | [Total Fees Paid: $%s] | [Swap Trade Count: %d updates]\n",
			formatWithSpacedDecimals(state.TotalTradeProfitUSD),
			formatWithSpacedDecimals(state.AccumulatedFees),
			state.FeesPaidCount,
		)

		time.Sleep(10 * time.Millisecond)
	}

	// Final termination log outputs after loop exit
	Info(fmt.Sprintf("🏁 2000-Day Rebalancing Simulation cycle complete. Termination routines closing gracefully."))
	
	// ADDED: Detailed structural metric printout on program termination
	fmt.Printf("\n================ FINAL SIMULATION AUDIT ================\n")
	fmt.Printf("📊 Processed Blocks   : Day %d\n", state.Day-1)
	fmt.Printf("🔄 Swap Transactions  : %d updates\n", state.FeesPaidCount)
	fmt.Printf("💸 Total Fees Paid    : $%s\n", formatWithSpacedDecimals(state.AccumulatedFees))
	fmt.Printf("✨ Total Profits Won  : $%s (Net Accumulated 7%% Earnings)\n", formatWithSpacedDecimals(state.TotalTradeProfitUSD))
	fmt.Printf("========================================================\n\n")
}

// ==============================
// STATE CONTROLLER UTILITIES
// ==============================

func loadOrCreateState() PortfolioState {
	if _, err := os.Stat(CheckpointFile); os.IsNotExist(err) {
		Warn("State checkpoint matrix file not found. Generating fresh Initial state setup data structures...")
		initialState := PortfolioState{
			Day:                 1,
			UniQuantity:         2.9,
			BtcQuantity:         0.0001666,
			UniPrice:            3.448,  
			BtcPrice:            60024.0, 
			LastRatio:           1.0, 
			TotalTradeProfitUSD: 0.0,
			AccumulatedFees:     0.0,
			FeesPaidCount:       0,
			NextActionRequired:  "SELL_UNI", // Baseline start sequence trigger
			LastUniPriceUSD:     0.0,
			LastUpdatedAt:       time.Now(),
		}
		saveState(initialState)
		return initialState
	}

	data, err := os.ReadFile(CheckpointFile)
	if err != nil {
		Error(fmt.Sprintf("Critical error reading fallback configuration: %v", err))
		log.Fatal(err)
	}

	var recoveredState PortfolioState
	if err := json.Unmarshal(data, &recoveredState); err != nil {
		Error(fmt.Sprintf("State restoration routine corrupted: %v", err))
		log.Fatal(err)
	}

	if recoveredState.UniPrice <= 0 { recoveredState.UniPrice = 3.448 }
	if recoveredState.BtcPrice <= 0 { recoveredState.BtcPrice = 60024.0 }

	Info(fmt.Sprintf("🔄 System State Recovered Successfully! Day %d [Swaps Run: %d | Fees Charged: $%s]", 
		recoveredState.Day, recoveredState.FeesPaidCount, formatWithSpacedDecimals(recoveredState.AccumulatedFees)))
	return recoveredState
}

func saveState(state PortfolioState) {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		Error(fmt.Sprintf("Failed to marshal tracking metrics block object: %v", err))
		return
	}

	tmpFile := CheckpointFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		Error(fmt.Sprintf("Write access sequence tracking blocked: %v", err))
		return
	}

	if err := os.Rename(tmpFile, CheckpointFile); err != nil {
		Error(fmt.Sprintf("System pointer reference replacement failed swapping state file nodes: %v", err))
	}
}

func simulateRandomWalk(currentPrice, min, max, volatility float64) float64 {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	changePercent := (r.Float64() * 2.0 - 1.0) * volatility
	nextPrice := currentPrice * (1.0 + changePercent)

	if nextPrice < min { return min }
	if nextPrice > max { return max }
	return nextPrice
}