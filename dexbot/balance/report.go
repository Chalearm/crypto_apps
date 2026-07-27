/******************************************************************************
 * File Name       : report.go
 * File Path       : balance/report.go
 *
 * Author          : Chalearm Saelim
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-26 06:03:11 (UTC+7)
 * Modified Date   : 2026-07-26 06:03:11 (UTC+7)
 *
 * Description     :
 *   Balance reporting functions for on-chain token value calculation.
 *
 * Usage :
 *   Directory : balance/
 *   Package   : dexbot/balance
 *****************************************************************************
 *
 * Responsibilities:
 *   - Part of the dexbot platform.
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/infra
 *
 *   External :
 *     - (stdlib only)
 *
 * Updated Parts :
 *   None
 *
 * New Parts :
 *   [Function] See function list.
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)        | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-26 08:00:00 (UTC+7)      | Chalearm Saelim | Initial
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add documentation.
 *
 * Notes :
 *   - Per regulator coding standard.
 */

package balance

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

// Added a view call for decimals
const ERC20_ABI = `[
{
    "name":"balanceOf",
    "type":"function",
    "inputs":[{"name":"account","type":"address"}],
    "outputs":[{"name":"","type":"uint256"}],
    "stateMutability":"view"
},
{
    "name":"decimals",
    "type":"function",
    "inputs":[],
    "outputs":[{"name":"","type":"uint8"}],
    "stateMutability":"view"
}
]`

var TokenPrices = map[string]float64{
	"BNB":   553.843,    
	"USDC":  1.00,
	"BUSD":  1.00,
	"MATIC": 0.072535,  
	"USDT":  1.00,        
	"BTT":   0.00000026,
	"SHIB":  0.000025,
	"AUTO":  600.0,
	"BSW":   0.3,
	"WBNB":  553.843,
	"UNI":   3.35,
	"ETH":   3500.0,
}
/******************************************************************************
 * Function Name : FormatWithSpacedDecimals
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func FormatWithSpacedDecimals(val float64) string {
	rawStr := fmt.Sprintf("%.12f", val)
	parts := strings.Split(rawStr, ".")

	intPart := parts[0]
	decPart := parts[1]

	var intResult []string
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			intResult = append(intResult, ",")
		}
		intResult = append(intResult, string(c))
	}
	formattedInt := strings.Join(intResult, "")

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
/******************************************************************************
 * Function Name : Report
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func Report(
	client bind.ContractBackend,
	auth *bind.TransactOpts,
	tokenList map[string]common.Address,
) float64 {
	parsed, err := abi.JSON(strings.NewReader(ERC20_ABI))
	if err != nil {
		log.Fatal("ABI parse error:", err)
	}

	isBSCChain := false
	if _, hasWBNB := tokenList["WBNB"]; hasWBNB {
		isBSCChain = true
	} else if _, hasCake := tokenList["CAKE"]; hasCake {
		isBSCChain = true
	}

	var networkTotalUSD float64

	for name, addr := range tokenList {
		var balance *big.Int
		tokenDecimals := uint8(18) // Default fallback
		isNative := addr.Hex() == "0x0000000000000000000000000000000000000000" || (isBSCChain && name == "BNB")

		if isNative {
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
			
			// 1. Fetch Dynamic Token Decimals to handle 6-decimal tokens like Polygon USDT
			var decResult []interface{}
			errDec := contract.Call(nil, &decResult, "decimals")
			if errDec == nil && len(decResult) > 0 {
				if d, ok := decResult[0].(uint8); ok {
					tokenDecimals = d
				}
			}

			// 2. Fetch Balance
			var result []interface{}
			err := contract.Call(nil, &result, "balanceOf", auth.From)
			if err != nil || len(result) == 0 {
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

		// Calculate using the dynamic decimal scale factor (e.g. 10^6 or 10^18)
		divisor := math.Pow10(int(tokenDecimals))
		clean := new(big.Float).Quo(
			new(big.Float).SetInt(balance),
			big.NewFloat(divisor),
		)

		value, _ := clean.Float64()
		priceUSD := TokenPrices[name]
		usd := value * priceUSD

		networkTotalUSD += usd

		prettyTokenAmt := FormatWithSpacedDecimals(value)
		prettyUSDAmt := FormatWithSpacedDecimals(usd)

		fmt.Printf("%s: %s tokens ($%s USD)\n", name, prettyTokenAmt, prettyUSDAmt)
	}

	prettyNetTotal := FormatWithSpacedDecimals(networkTotalUSD)
	fmt.Printf("Subtotal for Network: $%s USD\n", prettyNetTotal)
	fmt.Println("------------------------------------------------------------")

	return networkTotalUSD
}
/******************************************************************************
 * Function Name : GetTokenBalance
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func GetTokenBalance(client bind.ContractBackend, tokenAddr, owner common.Address) (*big.Int, error) {
	parsed, err := abi.JSON(strings.NewReader(ERC20_ABI))
	if err != nil {
		return nil, err
	}
	contract := bind.NewBoundContract(tokenAddr, parsed, client, client, client)
	var result []interface{}
	if err := contract.Call(nil, &result, "balanceOf", owner); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("empty result")
	}
	bal, ok := result[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("type assertion failed")
	}
	return bal, nil
}