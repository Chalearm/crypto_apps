/*
Filename: balance/report.go

Author: M365 Copilot (GPT-5)
Version: v1.1
Owner: Chalearm Saelim
Date: 2026-06-10

Description:
Wallet balance reporting module.

Features:
- Fetch token balances on-chain
- Convert to USD (static price mapping)
- Pretty formatting output

AI Prompt Idea:
"Create a Go tool to read ERC20 balances and display formatted wallet report with USD estimation."
*/

package balance

import (
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

        contract := bind.NewBoundContract(addr, parsed, client, client, client)

        var result []interface{}

        err := contract.Call(nil, &result, "balanceOf", auth.From)
        if err != nil {
            continue
        }

        if len(result) == 0 {
            continue
        }

        balance, ok := result[0].(*big.Int)
        if !ok {
            continue
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

        // Apply our custom spatial layout format
        prettyTokenAmt := formatWithSpacedDecimals(value)

        fmt.Printf("%s: %s tokens ($%.4f USD)\n", name, prettyTokenAmt, usd) 
    }

    fmt.Println("------------------------------------------------------------")
}

