/******************************************************************************
 * File Name       : report.go
 * File Path       : balance/report.go
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
 *   Wallet balance reporting module. - Fetch token balances on-chain (Supports both BEP20 tokens and Native BNB) - Convert to USD (static price mapping) - Pretty formatting output for both token amounts a
 *
 * Responsibilities:
 *   - Implement core functionality for balance package.
 *
 * Usage :
 *   Directory : balance/
 *
 *   Build :
 *     go build ./balance
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./balance
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/balance
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
 *   1.0.0   | 2026-06-30 00:53:07 (UTC+7)   | deepseek-4.0-pro | Initial version — rule1.txt header batch
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package balance

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

const ERC20_ABI = `[
{
    "name":"balanceOf",
    "type":"function",
    "inputs":[{"name":"account","type":"address"}],
    "outputs":[{"name":"","type":"uint256"}],
    "stateMutability":"view"
}
]`

// ✅ TEMP STATIC PRICES
var tokenPrices = map[string]float64{
	"USDC": 1.0,
	"BTT":  0.0000003,
	"SHIB": 0.000025,
	"AUTO": 600.0,
	"BSW":  0.3,
	"WBNB": 600.0,
	"BNB":  600.0,
	"UNI":  3.35,
}

// Custom parser to format: 12,345.123 456 789 012
func formatWithSpacedDecimals(val float64) string {
	rawStr := fmt.Sprintf("%.12f", val)
	parts := strings.Split(rawStr, ".")

	intPart := parts[0]
	decPart := parts[1]

	// 1. Format Integer side with commas (e.g., 1000 -> 1,000)
	var intResult []string
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			intResult = append(intResult, ",")
		}
		intResult = append(intResult, string(c))
	}
	formattedInt := strings.Join(intResult, "")

	// 2. Format Decimal side with 3-digit spacing spaces
	var decResult []string
	for i, c := range decPart {
		if i > 0 && i%3 == 0 {
			decResult = append(decResult, " ")
		}
		decResult = append(decResult, string(c))
	}
	formattedDec := strings.Join(decResult, "")

	return formattedInt + "." + formattedDec
}

func Report(
	client bind.ContractBackend,
	auth *bind.TransactOpts,
	tokenList map[string]common.Address,
) {

	parsed, err := abi.JSON(strings.NewReader(ERC20_ABI))
	if err != nil {
		log.Fatal("ABI parse error:", err)
	}

	fmt.Println("WALLET BALANCE")
	fmt.Println("------------------------------------------------------------")

	for name, addr := range tokenList {
		var balance *big.Int

		// Native BNB doesn't implement ERC20. We verify if our client supports BalanceAt
		if name == "BNB" {
			type nativeBalanceReader interface {
				BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
			}

			if reader, ok := client.(nativeBalanceReader); ok {
				var err error
				balance, err = reader.BalanceAt(context.Background(), auth.From, nil)
				if err != nil {
					continue
				}
			} else {
				continue
			}
		} else {
			contract := bind.NewBoundContract(addr, parsed, client, client, client)
			var result []interface{}

			err := contract.Call(nil, &result, "balanceOf", auth.From)
			if err != nil {
				continue
			}

			if len(result) == 0 {
				continue
			}

			var ok bool
			balance, ok = result[0].(*big.Int)
			if !ok {
				continue
			}
		}

		if balance.Cmp(big.NewInt(0)) == 0 {
			continue
		}

		// FIX: Replaced big.BigFloat with big.NewFloat
		clean := new(big.Float).Quo(
			new(big.Float).SetInt(balance),
			big.NewFloat(1e18),
		)

		value, _ := clean.Float64()

		// USD calculation
		priceUSD := tokenPrices[name]
		usd := value * priceUSD

		// Apply our custom spatial layout format to both token amounts and USD
		prettyTokenAmt := formatWithSpacedDecimals(value)
		prettyUSDAmt := formatWithSpacedDecimals(usd)

		fmt.Printf("%s: %s tokens ($%s USD)\n", name, prettyTokenAmt, prettyUSDAmt)
	}

	fmt.Println("------------------------------------------------------------")
}
