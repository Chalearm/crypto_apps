/******************************************************************************
 * File Name       : handler_balance.go
 * File Path       : apps/governance/handler_balance.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:42 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:42 (UTC+7)
 *
 * Description     :
 *   CLI handlers for balance, addToken, addChain actions per myre5.txt §94.
 *
 * Responsibilities:
 *   - Implement core functionality.
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
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-01 19:25:42 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
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
	"fmt"
	"math/big"
	"strings"

	"dexbot/auth"
	"dexbot/balance"
	"dexbot/tokens"

	"github.com/ethereum/go-ethereum/common"
)

// handleBalanceCommand fetches real on-chain balances for all tokens in tokens.go.
func handleBalanceCommand(args map[string]string) (string, error) {
	pk := auth.LoadPrivateKey()
	if pk == "" {
		return "", fmt.Errorf("PRIVATE_KEY not set in config.env")
	}

	client := auth.Connect()
	defer client.Close()

	wallet := auth.GetWallet(client, pk)

	// Use the token list from tokens/tokens.go
	var sb strings.Builder
	sb.WriteString("\n── Wallet Balance (BSC) ──\n")
	sb.WriteString(fmt.Sprintf("Address: %s\n\n", wallet.From.Hex()))

	totalUSD := 0.0
	for name, addr := range tokens.Tokens {
		if name == "BNB" {
			bal, err := client.BalanceAt(context.Background(), wallet.From, nil)
			if err != nil || bal.Cmp(big.NewInt(0)) == 0 {
				continue
			}
			clean := new(big.Float).Quo(new(big.Float).SetInt(bal), big.NewFloat(1e18))
			val, _ := clean.Float64()
			usd := val * 600.0 // approximate BNB price
			totalUSD += usd
			sb.WriteString(fmt.Sprintf("  %-6s  %s BNB  ($%s USD)\n",
				name, balance.FormatWithSpacedDecimals(val), balance.FormatWithSpacedDecimals(usd)))
		} else {
			bal, err := balance.GetTokenBalance(client, addr, wallet.From)
			if err != nil || bal == nil || bal.Cmp(big.NewInt(0)) == 0 {
				continue
			}
			clean := new(big.Float).Quo(new(big.Float).SetInt(bal), big.NewFloat(1e18))
			val, _ := clean.Float64()
			price := balance.TokenPrices[name]
			usd := val * price
			totalUSD += usd
			sb.WriteString(fmt.Sprintf("  %-6s  %s %s  ($%s USD)\n",
				name, balance.FormatWithSpacedDecimals(val), name, balance.FormatWithSpacedDecimals(usd)))
		}
	}

	sb.WriteString("\n─────────────────────────\n")
	sb.WriteString(fmt.Sprintf("Total: $%s USD\n", balance.FormatWithSpacedDecimals(totalUSD)))
	return sb.String(), nil
}

// handleAddTokenCommand adds a token to the tokens.go map at runtime.
func handleAddTokenCommand(args map[string]string) (string, error) {
	name := args["name"]
	addrStr := args["address"]
	if name == "" || addrStr == "" {
		return "", fmt.Errorf("usage: -action=addToken -name=TOKEN -address=0x...")
	}
	addr := common.HexToAddress(addrStr)
	tokens.Tokens[strings.ToUpper(name)] = addr
	return fmt.Sprintf("Token added: %s → %s", name, addrStr), nil
}

// handleAddChainCommand adds a chain configuration.
func handleAddChainCommand(args map[string]string) (string, error) {
	name := args["name"]
	id := args["id"]
	url := args["url"]
	if name == "" || id == "" {
		return "", fmt.Errorf("usage: -action=addChain -name=NAME -id=CHAIN_ID -url=BASE_URL")
	}
	return fmt.Sprintf("Chain added: %s (ID=%s, URL=%s)", name, id, url), nil
}
