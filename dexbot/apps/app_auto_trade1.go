package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"dexbot/auth"
	"dexbot/swap"
	"dexbot/tokens"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	STATE_FILE  = "tasks_state.json"
	CONFIG_FILE = "config.json"
	PID_FILE    = "dexbot.pid"
)

type TaskStatus string

const (
	StatusCreated   TaskStatus = "CREATED"
	StatusBought    TaskStatus = "BOUGHT"
	StatusCompleted TaskStatus = "COMPLETED"
	StatusFailed    TaskStatus = "FAILED"
)

type GlobalConfig struct {
	MaxTasks      int     `json:"max_tasks"`
	MinPriceLimit float64 `json:"min_price_limit"`
	MaxPriceLimit float64 `json:"max_price_limit"`
}

type TradeTask struct {
	ID                string     `json:"id"`
	Status            TaskStatus `json:"status"`
	FromToken         string     `json:"from_token"`
	ToToken           string     `json:"to_token"`
	BuyAmountUnits    float64    `json:"buy_amount_units"`
	BuyPriceUSDC      float64    `json:"buy_price_usdc"`
	BuyTimestamp      time.Time  `json:"buy_timestamp"`
	ForecastSellTime  time.Time  `json:"forecast_sell_time"`
	ForecastProfitPct float64    `json:"forecast_profit_pct"`
	TargetPriceUSDC   float64    `json:"target_price_usdc"`
	SellPriceUSDC     float64    `json:"sell_price_usdc"`
	SellTimestamp     time.Time  `json:"sell_timestamp"`
}

type TaskManager struct {
	mu    sync.Mutex
	Tasks map[string]*TradeTask `json:"tasks"`
}

func NewTaskManager() *TaskManager {
	return &TaskManager{Tasks: make(map[string]*TradeTask)}
}

func (tm *TaskManager) Save() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	data, _ := json.MarshalIndent(tm, "", "  ")
	_ = os.WriteFile(STATE_FILE, data, 0644)
}

func (tm *TaskManager) Load() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	data, err := os.ReadFile(STATE_FILE)
	if err == nil {
		_ = json.Unmarshal(data, tm)
	}
}

func loadConfig() GlobalConfig {
	var config GlobalConfig
	data, err := os.ReadFile(CONFIG_FILE)
	if err != nil {
		config = GlobalConfig{MaxTasks: 5, MinPriceLimit: 0.00000001, MaxPriceLimit: 2.0}
		writeConfig(config)
		return config
	}
	_ = json.Unmarshal(data, &config)
	return config
}

func writeConfig(config GlobalConfig) {
	data, _ := json.MarshalIndent(config, "", "  ")
	_ = os.WriteFile(CONFIG_FILE, data, 0644)
}

func main() {
	cmdAction := flag.String("action", "start", "Action to perform: 'start', 'terminate', 'report'")
	daemonFlag := flag.Bool("daemon", false, "Internal usage flag to identify detached background status")
	flag.Parse()

	// 📊 ACTION: REPORT GENERATOR WITH LIVE BALANCE TRACKER
	if *cmdAction == "report" {
		client := auth.Connect()
		pk := auth.LoadPrivateKey()
		wallet := auth.GetWallet(client, pk)

		manager := NewTaskManager()
		manager.Load()

		fmt.Println("\n==================================================================================")
		fmt.Println("                       📊 DEXBOT AUTOMATION LIVE REPORT                          ")
		fmt.Println("==================================================================================")
		
		// Fetch Live Wallet Balances
		bnbBalanceRaw, err := client.BalanceAt(context.Background(), wallet.From, nil)
		var bnbBalance float64 = 0.0
		if err == nil {
			bnbBalance, _ = new(big.Float).Quo(new(big.Float).SetInt(bnbBalanceRaw), big.NewFloat(1e18)).Float64()
		}

		usdcRaw := queryBalanceOnChain(client, wallet.From, tokens.Tokens["USDC"])
		usdcBalance, _ := new(big.Float).Quo(new(big.Float).SetInt(usdcRaw), big.NewFloat(1e18)).Float64()

		bttRaw := queryBalanceOnChain(client, wallet.From, tokens.Tokens["BTT"])
		bttBalance, _ := new(big.Float).Quo(new(big.Float).SetInt(bttRaw), big.NewFloat(1e18)).Float64()

		shibRaw := queryBalanceOnChain(client, wallet.From, tokens.Tokens["SHIB"])
		shibBalance, _ := new(big.Float).Quo(new(big.Float).SetInt(shibRaw), big.NewFloat(1e18)).Float64()

		fmt.Println("💳 WALLET ADDRESS:", wallet.From.Hex())
		fmt.Printf("💰 LIVE BALANCES : %.5f BNB | %.2f USDC | %.2f BTT | %.2f SHIB\n", bnbBalance, usdcBalance, bttBalance, shibBalance)
		fmt.Println("==================================================================================")

		if len(manager.Tasks) == 0 {
			fmt.Println("No recorded tracking tasks found in database history state files.")
			return
		}

		var created, bought, completed, failed int
		for _, t := range manager.Tasks {
			switch t.Status {
			case StatusCreated:   created++
			case StatusBought:    bought++
			case StatusCompleted: completed++
			case StatusFailed:    failed++
			}

			fmt.Printf("🔹 Task ID: %s | Status: [%-9s] | Path: %s -> %s\n", t.ID, t.Status, t.FromToken, t.ToToken)
			fmt.Printf("   ├─ Allocation Size : %.2f %s\n", t.BuyAmountUnits, t.FromToken)
			
			if t.Status != StatusCreated {
				fmt.Printf("   ├─ Entry Asset Price: %.8f USDC  | Bought At: %s\n", t.BuyPriceUSDC, t.BuyTimestamp.Format("2006-01-02 15:04:05"))
				fmt.Printf("   ├─ Forecast Return  : +%.2f%% Target | Required Stop: %.8f USDC\n", t.ForecastProfitPct, t.TargetPriceUSDC)
				fmt.Printf("   ├─ Predicted Exit   : %s\n", t.ForecastSellTime.Format("2006-01-02 15:04:05"))
			}
			if t.Status == StatusCompleted {
				fmt.Printf("   └─ Final Exit Price : %.8f USDC  | Closed At: %s\n", t.SellPriceUSDC, t.SellTimestamp.Format("2006-01-02 15:04:05"))
			}
			fmt.Println("----------------------------------------------------------------------------------")
		}

		fmt.Printf("\n📈 SUMMARY: Active-Buys: %d | Pending-Setup: %d | Closed-Profit: %d | Failed: %d\n", bought, created, completed, failed)
		fmt.Println("==================================================================================")
		return
	}

	// 🛑 ACTION: TERMINATE DAEMON
	if *cmdAction == "terminate" {
		pidData, err := os.ReadFile(PID_FILE)
		if err != nil {
			fmt.Println("❌ Error: Could not read process ID tracking file. Daemon is not active.")
			return
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(string(pidData)))
		process, err := os.FindProcess(pid)
		if err == nil {
			fmt.Printf("🛑 Sending shutdown signal to background runtime daemon [PID: %d]...\n", pid)
			_ = process.Signal(syscall.SIGTERM)
			fmt.Println("✅ Termination signal completed. States saved safely to file.")
			_ = os.Remove(PID_FILE)
			return
		}
		return
	}

	// 🚀 ACTION: START DAEMON DETACHED
	if !*daemonFlag {
		cmd := exec.Command(os.Args[0], "-daemon")
		cmd.Stdout, cmd.Stderr, cmd.Stdin = nil, nil, nil
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		err := cmd.Start()
		if err != nil {
			log.Fatalf("Failed to initialize daemon: %v", err)
		}
		
		_ = os.WriteFile(PID_FILE, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
		fmt.Printf("🚀 Dexbot successfully detached into background daemon! [PID: %d]\n", cmd.Process.Pid)
		fmt.Println("Options:\n -> Watch Live Report: go run apps/app_auto_trade1.go -action report\n -> Kill Bot Gracefully: go run apps/app_auto_trade1.go -action terminate")
		os.Exit(0)
	}

	// BACKGROUND DAEMON ENGINE CORE LIFECYCLE LOOP
	logFile, _ := os.OpenFile("dexbot.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	log.SetOutput(logFile)
	log.Println("--- Dexbot Engine Daemon Initialized Successfully ---")

	client := auth.Connect()
	pk := auth.LoadPrivateKey()
	wallet := auth.GetWallet(client, pk)

	manager := NewTaskManager()
	manager.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Termination signal captured. Flushing tasks map state data safely...")
		manager.Save()
		_ = os.Remove(PID_FILE)
		os.Exit(0)
	}()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runtimeConfig := loadConfig()
			activeTasksCount := 0
			for _, t := range manager.Tasks {
				if t.Status == StatusCreated || t.Status == StatusBought {
					activeTasksCount++
				}
			}

			if activeTasksCount < runtimeConfig.MaxTasks {
				needed := runtimeConfig.MaxTasks - activeTasksCount
				log.Printf("[Config Update] Expanding runtime loops. Generating %d new programmatic tasks...", needed)
				
				pairs := [][]string{
					{"USDC", "SHIB"},
					{"USDC", "BTT"},
					{"BTT", "SHIB"},
				}

				for i := 0; i < needed; i++ {
					rand.Seed(time.Now().UnixNano() + int64(i))
					selectedPair := pairs[rand.Intn(len(pairs))]
					taskID := fmt.Sprintf("auto_task_%d_%d", time.Now().UnixNano(), i)
					
					baseAmt := 0.35 
					if selectedPair[0] == "BTT" {
						baseAmt = 2000000.0 
					} else if selectedPair[0] == "SHIB" {
						baseAmt = 100000.0
					}

					manager.Tasks[taskID] = &TradeTask{
						ID:             taskID,
						Status:         StatusCreated,
						FromToken:      selectedPair[0],
						ToToken:        selectedPair[1],
						BuyAmountUnits: baseAmt,
					}
				}
				manager.Save()
			}

			var wg sync.WaitGroup
			for _, t := range manager.Tasks {
				if t.Status == StatusCompleted || t.Status == StatusFailed {
					continue
				}
				wg.Add(1)
				go func(task *TradeTask) {
					defer wg.Done()
					processTaskLifecycle(client, wallet, task, manager)
				}(t)
			}
			wg.Wait()
		}
	}
}

func processTaskLifecycle(client *ethclient.Client, wallet *bind.TransactOpts, task *TradeTask, manager *TaskManager) {
	switch task.Status {
	case StatusCreated:
		currentPrice := queryPriceOnChain(client, task.FromToken, "USDC")
		if currentPrice == 0 {
			return
		}

		rawAmount := floatToBigInt(task.BuyAmountUnits, swap.Decimals[task.FromToken])
		log.Printf("[%s] Executing entry transaction mapping: %s -> %s", task.ID, task.FromToken, task.ToToken)
		
		swap.ExecuteSwap(client, wallet, task.FromToken, task.ToToken, tokens.Tokens[task.FromToken], tokens.Tokens[task.ToToken], rawAmount, tokens.Tokens)

		task.BuyPriceUSDC = currentPrice
		task.BuyTimestamp = time.Now()
		task.Status = StatusBought

		engine1Forecast(task)
		manager.Save()
		log.Printf("[%s] Target metrics saved. Threshold Limit Destination: %.8f USDC", task.ID, task.TargetPriceUSDC)

	case StatusBought:
		currentPriceToSell := queryPriceOnChain(client, task.ToToken, "USDC")
		if currentPriceToSell == 0 {
			return
		}

		now := time.Now()
		if now.After(task.ForecastSellTime) || currentPriceToSell >= task.TargetPriceUSDC {
			log.Printf("[%s] Exiting cycle position: Current Price %.8f met target parameters.", task.ID, currentPriceToSell)
			outBalance := queryBalanceOnChain(client, wallet.From, tokens.Tokens[task.ToToken])
			if outBalance.Cmp(big.NewInt(0)) == 0 {
				task.Status = StatusFailed
				manager.Save()
				return
			}

			swap.ExecuteSwap(client, wallet, task.ToToken, task.FromToken, tokens.Tokens[task.ToToken], tokens.Tokens[task.FromToken], outBalance, tokens.Tokens)

			task.SellPriceUSDC = currentPriceToSell
			task.SellTimestamp = now
			task.Status = StatusCompleted
			manager.Save()
			log.Printf("[%s] Position safely closed out.", task.ID)
		}
	}
}

func engine1Forecast(task *TradeTask) {
	rand.Seed(time.Now().UnixNano())
	minDuration := time.Hour
	maxDuration := 7 * 24 * time.Hour
	randomDuration := time.Duration(rand.Int63n(int64(maxDuration-minDuration))) + minDuration
	task.ForecastSellTime = time.Now().Add(randomDuration)

	for {
		generatedProfit := 5.0 + rand.Float64()*10.0
		if generatedProfit >= 8.0 && generatedProfit <= 12.0 {
			task.ForecastProfitPct = generatedProfit
			break
		}
	}
	task.TargetPriceUSDC = task.BuyPriceUSDC * (1.0 + (task.ForecastProfitPct / 100.0))
}

func queryPriceOnChain(client *ethclient.Client, tokenName string, referenceName string) float64 {
	if tokenName == referenceName {
		return 1.0
	}
	routerABI, _ := abi.JSON(strings.NewReader(swap.ROUTER_ABI))
	contract := bind.NewBoundContract(common.HexToAddress(swap.ROUTER), routerABI, client, client, client)
	path := []common.Address{tokens.Tokens[tokenName], tokens.Tokens[referenceName]}
	amtIn := floatToBigInt(1.0, swap.Decimals[tokenName])
	var out []interface{}
	err := contract.Call(nil, &out, "getAmountsOut", amtIn, path)
	if err != nil || len(out) == 0 {
		return 0.0
	}
	expectedAmounts := out[0].([]*big.Int)
	expectedOut := expectedAmounts[len(expectedAmounts)-1]
	div := new(big.Int).Exp(big.NewInt(10), big.NewInt(swap.Decimals[referenceName]), nil)
	f := new(big.Float).Quo(new(big.Float).SetInt(expectedOut), new(big.Float).SetInt(div))
	val, _ := f.Float64()
	return val
}

func queryBalanceOnChain(client *ethclient.Client, account common.Address, tokenAddress common.Address) *big.Int {
	parsed, _ := abi.JSON(strings.NewReader(swap.ERC20_ABI))
	contract := bind.NewBoundContract(tokenAddress, parsed, client, client, client)
	var out []interface{}
	err := contract.Call(nil, &out, "balanceOf", account)
	if err != nil || len(out) == 0 {
		return big.NewInt(0)
	}
	return out[0].(*big.Int)
}

func floatToBigInt(val float64, decimals int64) *big.Int {
	bigval := new(big.Float).SetFloat64(val)
	multiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(decimals), nil))
	bigval.Mul(bigval, multiplier)
	result := new(big.Int)
	bigval.Int(result)
	return result
}
