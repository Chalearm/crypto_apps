/******************************************************************************
 * File Name       : cli.go
 * File Path       : apps/balance/cli.go
 *
 * Author          : Gemini 3.1 Pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.4.0
 * Status          : Development
 * Created Date    : 2026-07-12 14:32:43 (UTC+7)
 * Modified Date   : 2026-07-12 15:15:00 (UTC+7)
 *
 * Description     :
 *   Handles CLI flag parsing and routes to daemon control or DB operations.
 *
 * Responsibilities:
 *   - Parse -action, -private-key, -account, -chain-id, etc.
 *   - Validate command-line prerequisites.
 *   - Provide a comprehensive help menu.
 *
 * Usage :
 *   Internal use by main.go
 *
 * Dependencies :
 *   - dexbot/infra
 *
 * Updated Parts :
 *   - parseAndRouteFlags() - Added help action support.
 *
 * New Parts :
 *   [Functions]
 *     - HandleHelp()
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)       | Author         | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-12 14:32:43     | Gemini 3.1 Pro | Initial routing implementation
 *   1.1.0   | 2026-07-12 14:55:00     | Gemini 3.1 Pro | Wired unused flags to crud actions
 *   1.2.0   | 2026-07-12 15:10:00     | Gemini 3.1 Pro | Rule1.txt header compliance
 *   1.3.0   | 2026-07-12 15:05:00     | Gemini 3.1 Pro | Added -action=terminate support
 *   1.4.0   | 2026-07-12 15:15:00     | Gemini 3.1 Pro | Added -action=help menu
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests.
 *****************************************************************************
 *
 * Notes :
 *   - Per regulator coding standard.
 */
package main

import (
	"flag"
	"fmt"
	"os"

	"dexbot/infra"
)

/******************************************************************************
 * Function Name : HandleHelp
 *
 * Purpose :
 *   Prints a detailed help menu for all supported CLI actions with examples.
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
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - None
 *
 * Dependencies :
 *   - fmt
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   35
 ******************************************************************************/
func HandleHelp() {
	fmt.Println("================================================================================")
	fmt.Println("                         DEXBOT BALANCE CLI - HELP MENU                         ")
	fmt.Println("================================================================================")
	fmt.Println("USAGE:")
	fmt.Println("  ./balance -action=[ACTION] [OPTIONS]")
	fmt.Println()
	fmt.Println("DAEMON CONTROL:")
	fmt.Println("  -action=start       : Starts the daemon process in the background.")
	fmt.Println("  -action=status      : Checks if the daemon is currently running.")
	fmt.Println("  -action=terminate   : Kills the running daemon process.")
	fmt.Println()
	fmt.Println("ACCOUNT & BALANCE:")
	fmt.Println("  -action=view-balance")
	fmt.Println("     Requires : -private-key=[METAMASK_PRIVATE_KEY]")
	fmt.Println("     Example  : ./balance -action=view-balance -private-key=abc123def456...")
	fmt.Println()
	fmt.Println("DATABASE CRUD OPERATIONS:")
	fmt.Println("  -action=add-chain")
	fmt.Println("     Requires : -account=[SHA256] -chain-name=[NAME] -chain-url-based=[URL] -chain-id=[ID]")
	fmt.Println("     Example  : ./balance -action=add-chain -account=a1b2... -chain-name=BSC -chain-url-based=https://bsc -chain-id=56")
	fmt.Println()
	fmt.Println("  -action=add-token")
	fmt.Println("     Requires : -account=[SHA256] -chain-id=[ID] -token-name=[TICKER] -token-address=[0x...]")
	fmt.Println("     Example  : ./balance -action=add-token -account=a1b2... -chain-id=56 -token-name=WBNB -token-address=0xbb4...")
	fmt.Println()
	fmt.Println("  -action=delete-account")
	fmt.Println("     Requires : -account=[SHA256]")
	fmt.Println("     Example  : ./balance -action=delete-account -account=a1b2...")
	fmt.Println()
	fmt.Println("  -action=delete-chain")
	fmt.Println("     Requires : -account=[SHA256] -chain-id=[ID]")
	fmt.Println("     Example  : ./balance -action=delete-chain -account=a1b2... -chain-id=56")
	fmt.Println()
	fmt.Println("  -action=delete-token")
	fmt.Println("     Requires : -account=[SHA256] -chain-id=[ID] -token-name=[TICKER]")
	fmt.Println("     Example  : ./balance -action=delete-token -account=a1b2... -chain-id=56 -token-name=WBNB")
	fmt.Println("================================================================================")
}

/******************************************************************************
 * Function Name : parseAndRouteFlags
 *
 * Purpose :
 *   Parses the flag arguments provided by the user and routes the execution
 *   to the appropriate daemon management or database manipulation functions.
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
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - Missing required flags triggers os.Exit(1).
 *
 * Dependencies :
 *   - flag package
 *   - HandleDaemonStart
 *   - HandleDaemonStatus
 *   - HandleDaemonTerminate
 *   - HandleHelp
 *   - ViewBalance
 *   - AddChainToAccount
 *   - AddTokenToAccount
 *   - HandleDeleteAccount
 *   - HandleDeleteChain
 *   - HandleDeleteToken
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   72
 ******************************************************************************/
func parseAndRouteFlags() {
 
/*

    flagAction     = flag.String("action", "", "Action to perform")
    flagPrivateKey = flag.String("private-key", "", "Metamask private key")
    flagAccount    = flag.String("account", "", "Account SHA256 string")
    flagChainName  = flag.String("chain-name", "", "Name of the blockchain")
    flagChainURL   = flag.String("chain-url-based", "", "RPC URL for the chain")
    flagChainID    = flag.String("chain-id", "", "Numeric Chain ID")
    flagTokenName  = flag.String("token-name", "", "Token Ticker")
    flagTokenAddr  = flag.String("token-address", "", "Token Contract Address")
*/


	if !flag.Parsed() {
		flag.Parse()
	}
 
	// 3. Route & Validate Based on Action
	switch *flagAction {
	case "help":
		HandleHelp()
	case "start":
		infra.Info("Initializing Daemon...")
		HandleDaemonStart()
	case "status":
		HandleDaemonStatus()
	case "terminate":
		infra.Info("Terminating Daemon...")
		HandleDaemonTerminate()
	case "view-balance":
		if *flagPrivateKey == "" {
			infra.Error("Missing -private-key flag for view-balance")
			os.Exit(1)
		}
		ViewBalance(*flagPrivateKey)
	case "add-chain":
		if *flagAccount == "" || *flagChainName == "" || *flagChainURL == "" || *flagChainID == "" {
			infra.Error("Missing flags for add-chain")
			os.Exit(1)
		}
		AddChainToAccount(*flagAccount, *flagChainName, *flagChainURL, *flagChainID)
	case "add-token":
		if *flagAccount == "" || *flagChainID == "" || *flagTokenName == "" || *flagTokenAddr == "" {
			infra.Error("Missing flags for add-token")
			os.Exit(1)
		}
		AddTokenToAccount(*flagAccount, *flagChainID, *flagTokenName, *flagTokenAddr)
	case "delete-account":
		if *flagAccount == "" {
			infra.Error("Missing -account flag for delete-account")
			os.Exit(1)
		}
		HandleDeleteAccount(*flagAccount)
	case "delete-chain":
		if *flagAccount == "" || *flagChainID == "" {
			infra.Error("Missing -account or -chain-id for delete-chain")
			os.Exit(1)
		}
		HandleDeleteChain(*flagAccount, *flagChainID)
	case "delete-token":
		if *flagAccount == "" || *flagChainID == "" || *flagTokenName == "" {
			infra.Error("Missing flags for delete-token")
			os.Exit(1)
		}
		HandleDeleteToken(*flagAccount, *flagChainID, *flagTokenName)
	default:
		infra.Warn("Unknown or malformed action: " + *flagAccount)
		HandleHelp()
	}
}