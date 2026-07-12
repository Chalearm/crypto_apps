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
    "bufio"
    "fmt"
    "os"
    "strings"

    "dexbot/auth"
    "dexbot/balance"
    "dexbot/infra"
    "dexbot/tokens"
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
 *   22
 ******************************************************************************/
func main() {
    if _, err := os.Stat("config.env"); os.IsNotExist(err) {
        if _, err := os.Stat("../../config.env"); err == nil {
            os.Chdir("../..")
        }
    }

    LoadEnvConfig()
    infra.InitLogger()
    infra.SetDaemonID("balance-app")

    if len(os.Args) > 1 {
        parseAndRouteFlags()
        return
    }

    infra.Info("Starting legacy balance execution mode")
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