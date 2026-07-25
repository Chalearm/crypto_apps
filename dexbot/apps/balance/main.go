/******************************************************************************
 * File Name       : main.go
 * File Path       : apps/balance/main.go
 *
 * Author          : Gemini 3.1 Pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.3.0
 * Status          : Development
 * Created Date    : 2026-07-12 14:32:43 (UTC+7)
 * Modified Date   : 2026-07-12 15:30:00 (UTC+7)
 *
 * Description     :
 *   Main entry point of the Balance Tracker. Automatically changes directory
 *   to project root, parses config.env configuration settings, and boots the 
 *   CLI router or legacy tracking engine.
 *
 * Responsibilities:
 *   - Auto-detect project root to resolve file pathways cleanly.
 *   - Parse config.env key-value pairs manually into active process environment.
 *   - Route execution based on flags or fallback to standard balance summary.
 *
 * Usage :
 *   Directory : apps/balance/
 *
 *   Build :
 *     go build -o balance .
 *
 *   Run :
 *     ./balance
 *     ./balance -action=start
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/auth
 *     - dexbot/balance
 *     - dexbot/tokens
 *     - dexbot/infra
 *
 *   External :
 *     - (stdlib only)
 *
 * Updated Parts :
 *   [Function]
 *     - main() (Integrated LoadEnvConfig logic invocation)
 *
 * New Parts :
 *   [Function]
 *     - LoadEnvConfig()
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)       | Author         | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-01 19:25:44     | GPT-4          | Initial version
 *   1.1.0   | 2026-07-12 14:32:43     | Gemini 3.1 Pro | Added Daemon & CLI routing
 *   1.2.0   | 2026-07-12 15:10:00     | Gemini 3.1 Pro | Auto-Chdir fix for paths
 *   1.3.0   | 2026-07-12 15:30:00     | Gemini 3.1 Pro | Added custom config.env parser
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add structural unit testing frameworks.
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package main

import (
    "context" 
    "bufio"
    "fmt"
    "os"
    "flag"
    "strings"
    "strconv"
    "net/http"

    "dexbot/auth"
    "dexbot/balance"
    "dexbot/infra"
    "dexbot/tokens"
)
// Centralized Global CLI flags defined once for the entire main package space
var (
    flagAction     = flag.String("action", "", "Action to perform")
    flagPrivateKey = flag.String("private-key", "", "Metamask private key")
    flagAccount    = flag.String("account", "", "Account SHA256 string")
    flagChainName  = flag.String("chain-name", "", "Name of the blockchain")
    flagChainURL   = flag.String("chain-url-based", "", "RPC URL for the chain")
    flagChainID    = flag.String("chain-id", "", "Numeric Chain ID")
    flagTokenName  = flag.String("token-name", "", "Token Ticker")
    flagTokenAddr  = flag.String("token-address", "", "Token Contract Address")
)

/******************************************************************************
 * Function Name : LoadEnvConfig
 *
 * Purpose :
 *   Manually parses config.env line by line, extracts environment variables, 
 *   skips comment lines starting with '#', and binds values using os.Setenv.
 *
 * Inputs :
 *   None
 *
 * Outputs :
 *   None
 *
 * Return :
 *   None
 *
 * Error Cases :
 *   - Cannot open config.env file (logs error and continues gracefully).
 *
 * Dependencies :
 *   - os.Open
 *   - bufio.NewScanner
 *   - os.Setenv
 *
 * Complexity :
 *   Time  : O(N) where N is lines count in config.env
 *   Space : O(1)
 *
 * Number Of Lines :
 *   28
 ******************************************************************************/
func LoadEnvConfig() {
    file, err := os.Open("config.env")
    if err != nil {
        return
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        // Skip empty lines or commented metadata lines
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        parts := strings.SplitN(line, "=", 2)
        if len(parts) == 2 {
            key := strings.TrimSpace(parts[0])
            val := strings.TrimSpace(parts[1])
            os.Setenv(key, val)
        }
    }
}

/******************************************************************************
 * Function Name : main
 *
 * Purpose :
 *   Entry point for the application. Fixes working directory, loads settings 
 *   from config.env, initializes logger, and processes instructions.
 *
 * Inputs :
 *   None
 *
 * Outputs :
 *   None
 *
 * Return :
 *   None
 *
 * Error Cases :
 *   - Invalid flag parameters.
 *
 * Dependencies :
 *   - os.Chdir
 *   - LoadEnvConfig
 *   - infra.InitLogger
 *   - parseAndRouteFlags
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   23
 ******************************************************************************/
func main() {
    // 1. Directory and Env setup (Kept exactly as you had it)
    if _, err := os.Stat("config.env"); os.IsNotExist(err) {
        if _, err := os.Stat("../../config.env"); err == nil {
            os.Chdir("../..")
        }
    }

    LoadEnvConfig() // Make sure this matches your local function name
    infra.InitLogger()
    infra.SetDaemonID("balance-app")

    flag.Parse()

    if *flagAction == "" && len(os.Args) == 1 {
        runLegacyBalance()
        return
    }

    if *flagAction == "start" {
        HandleDaemonStart()
        return
    }

    parseAndRouteFlags()
    return
}

/******************************************************************************
 * Function Name : runLegacyBalance
 * Purpose :
 *   Execute legacy text-mode balance report for all chains (BSC/POLYGON/OPBNB).
 * Inputs :
 *   none (uses private key from env/CLI args)
 * Return :
 *   none (prints to stdout)
 * Error Cases :
/******************************************************************************
 * Function Name : runLegacyBalance
 *
 * Purpose :
 *   Execute legacy text-mode balance report for all chains (BSC/POLYGON/OPBNB).
 *
 * Inputs :
 *   none (uses private key from env/CLI args)
 *
 * Return :
 *   none (prints to stdout)
 *
 * Error Cases :
 *   - nil pointer if chain RPC unreachable (panics from GetWalletForChain).
 *
 * Number Of Lines :
 *   5
 ******************************************************************************/
func runLegacyBalance() {
    pk := auth.LoadPrivateKey()

    fmt.Println("NETWORK: BINANCE SMART CHAIN")
    fmt.Println("------------------------------------------------------------")
    bscClient := auth.ConnectToChain("https://bsc-dataseed.binance.org/")
    bscWallet := auth.GetWalletForChain(bscClient, pk, 56)
    bscTotal := balance.Report(bscClient, bscWallet, tokens.Chains["BSC"])
    fmt.Println()

    fmt.Println("NETWORK: POLYGON POS")
    fmt.Println("------------------------------------------------------------")
    polyClient := auth.ConnectToChain("https://polygon.drpc.org")
    polyWallet := auth.GetWalletForChain(polyClient, pk, 137)
    polygonTotal := balance.Report(polyClient, polyWallet, tokens.Chains["POLYGON"])
    fmt.Println()

    fmt.Println("NETWORK: opBNB LAYER 2")
    fmt.Println("------------------------------------------------------------")
    opBnbClient := auth.ConnectToChain("https://opbnb-mainnet-rpc.bnbchain.org")
    opBnbWallet := auth.GetWalletForChain(opBnbClient, pk, 204)
    opBnbTotal := balance.Report(opBnbClient, opBnbWallet, tokens.Chains["OPBNB"])
    fmt.Println()

    globalTotal := bscTotal + polygonTotal + opBnbTotal

    fmt.Println("CROSS-CHAIN PORTFOLIO SUMMARY")
    fmt.Println("============================================================")
    prettyTotalBSC := balance.FormatWithSpacedDecimals(bscTotal)
    prettyTotalPoly := balance.FormatWithSpacedDecimals(polygonTotal)
    prettyTotalOpBNB := balance.FormatWithSpacedDecimals(opBnbTotal)
    prettyGlobal := balance.FormatWithSpacedDecimals(globalTotal)

    fmt.Printf("TOTAL BSC CHAIN BALANCE: $%s USD\n", prettyTotalBSC)
    fmt.Printf("TOTAL POLYGON BALANCE  : $%s USD\n", prettyTotalPoly)
    fmt.Printf("TOTAL opBNB BALANCE    : $%s USD\n", prettyTotalOpBNB)
    fmt.Printf("TOTAL GLOBAL BALANCE   : $%s USD\n", prettyGlobal)
    fmt.Println("============================================================")
}
/******************************************************************************
 * Function Name : bootDaemon
 *
 * Purpose :
 *   Wraps the background HTTP API routing server logic inside the generalized 
 *   high-availability infrastructure package callback loop, mounting both web UI
 *   data endpoints and internal CLI tracking pathways.
 *
 * Inputs :
 *   None
 *
 * Outputs :
 *   None
 *
 * Return :
 *   None
 *
 * Error Cases :
 *   - Obstructed port maps crash the background web thread loop.
 *
 * Dependencies :
 *   - net/http
 *   - context
 *   - dexbot/infra
 *
 * Complexity :
 *   Time  : O(1) concurrent socket scheduling loops
 *   Space : O(1) allocation frames
 *
 * Number Of Lines :
 *   48
 ******************************************************************************/
func bootDaemon() {
    udpPort, _ := strconv.Atoi(os.Getenv("DAEMON_BALANCE_PORT"))
    if udpPort == 0 { 
        udpPort = 8086 
    } 

    apiPort := os.Getenv("DAEMON_BALANCE_HTTP_PORT")
    if apiPort == "" { 
        apiPort = "8087" // FIXED: Default to 8087 to match your custom config.env spec
    }

    ip := os.Getenv("DAEMON_BALANCE_IP")
    if ip == "" { 
        ip = "127.0.0.1" 
    }

    workerLoop := func(ctx context.Context) {
        infra.Info(fmt.Sprintf("Balance HTTP API starting on port %s", apiPort))
        
        // 1. Dedicated CLI status tracking loop endpoint
        http.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            w.Write([]byte(`{"status":"healthy","service":"balance"}`))
        })
        
        // 2. Canonical web UI card data endpoints
        http.HandleFunc("/api/update", handleAPIUpdateRoute)
        http.HandleFunc("/api/balance", handleAPIBalanceRoute)
        
        // 3. Relational data sync route handlers
        http.HandleFunc("/api/chain/add", handleAPIChainAddRoute)
        http.HandleFunc("/api/token/add", handleAPITokenAddRoute)
        http.HandleFunc("/api/chain/delete", handleAPIChainDeleteRoute)
        http.HandleFunc("/api/token/delete", handleAPITokenDeleteRoute)
        http.HandleFunc("/api/account/delete", handleAPIAccountDeleteRoute)
        
        server := &http.Server{Addr: ":" + apiPort}
        
        // Execute listener context in a parallel non-blocking thread routine
        go func() {
            if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                infra.Error(fmt.Sprintf("Balance HTTP API failed: %v", err))
            }
        }()
        
        <-ctx.Done()
        _ = server.Shutdown(context.Background())
    }

    // Hand off execution completely to the shared infrastructure library!
    infra.RunDaemonApp("balance", ip, udpPort, workerLoop)
}