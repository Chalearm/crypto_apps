/******************************************************************************
 * File Name       : swap.go
 * File Path       : swap/swap.go
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
 *   Automated token swap module using multi-path routing frameworks on Decentralized Exchanges. - Blockchain-backed path discovery optimization - Dynamic gas cost calculation and conversion reporting - Hi
 *
 * Responsibilities:
 *   - Implement core functionality for swap package.
 *
 * Usage :
 *   Directory : swap/
 *
 *   Build :
 *     go build ./swap
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./swap
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/swap
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
package swap

import (
    "context"
    "fmt"
    "log"
    "math/big"
    "strings"
    "time"

    "github.com/ethereum/go-ethereum/accounts/abi"
    "github.com/ethereum/go-ethereum/accounts/abi/bind"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/ethclient"
)

const ROUTER = "0x10ED43C718714eb63d5aA57B78B54704E256024E"

const ROUTER_ABI = `[
{"name":"swapExactTokensForTokens","type":"function","inputs":[
{"type":"uint256"},{"type":"uint256"},{"type":"address[]"},{"type":"address"},{"type":"uint256"}],
"outputs":[{"type":"uint256[]"}]},
{"name":"getAmountsOut","type":"function","inputs":[
{"type":"uint256"},{"type":"address[]"}],
"outputs":[{"type":"uint256[]"}]}
]`

const ERC20_ABI = `[
{"name":"approve","type":"function","inputs":[
{"name":"spender","type":"address"},
{"name":"amount","type":"uint256"}]},
{"name":"balanceOf","type":"function","inputs":[
{"name":"account","type":"address"}],
"outputs":[{"type":"uint256"}]}
]`

var Decimals = map[string]int64{
    "BTT":  18,
    "USDC": 18,
    "SHIB": 18,
    "WBNB": 18,
    "USDT": 18,
    "ETH":  18,
    "CAKE": 18,
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

// APPROVAL ENGINE
func approve(client *ethclient.Client, auth *bind.TransactOpts, token common.Address) {
    parsed, _ := abi.JSON(strings.NewReader(ERC20_ABI))
    contract := bind.NewBoundContract(token, parsed, client, client, client)

    nonce, _ := client.PendingNonceAt(context.Background(), auth.From)
    auth.Nonce = big.NewInt(int64(nonce))

    max := new(big.Int).Sub(
        new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil),
        big.NewInt(1),
    )

    fmt.Println("Approving asset allocation rules...")
    tx, err := contract.Transact(auth, "approve", common.HexToAddress(ROUTER), max)
    if err != nil {
        log.Fatal("Approve signature failed:", err)
    }

    fmt.Println("Approval Transaction Broadcasted:", tx.Hash().Hex())
    time.Sleep(5 * time.Second)
}

// EXECUTE SWAP USING THE UPDATED MATRIX LOOKUP
func ExecuteSwap(
    client *ethclient.Client,
    auth *bind.TransactOpts,
    fromName string,
    toName string,
    from common.Address,
    to common.Address,
    amount *big.Int,
    tokens map[string]common.Address,
) {
    routerABI, _ := abi.JSON(strings.NewReader(ROUTER_ABI))
    contract := bind.NewBoundContract(common.HexToAddress(ROUTER), routerABI, client, client, client)

    // Updated path options incorporating the verified ETH routes from MetaMask layout
    testPaths := [][]common.Address{
        {from, tokens["ETH"], to},                  // 🚀 Path 1: BTT -> ETH -> SHIB
        {from, tokens["WBNB"], to},                 // Path 2: BTT -> WBNB -> SHIB
        {from, tokens["USDT"], to},                 // Path 3: BTT -> USDT -> SHIB
        {from, tokens["WBNB"], tokens["ETH"], to},  // Path 4: BTT -> WBNB -> ETH -> SHIB
        {from, tokens["USDT"], tokens["ETH"], to},  // Path 5: BTT -> USDT -> ETH -> SHIB
        {from, tokens["WBNB"], tokens["USDT"], to}, // Path 6: BTT -> WBNB -> USDT -> SHIB
        {from, to},                                 // Fallback Direct Pool
    }

    var validPath []common.Address
    var expectedAmounts []*big.Int

    // Search matrix paths directly on the blockchain
    for _, path := range testPaths {
        var out []interface{}
        err := contract.Call(nil, &out, "getAmountsOut", amount, path)
        if err == nil && len(out) > 0 {
            expectedAmounts = out[0].([]*big.Int)
            validPath = path
            break
        }
    }

    if validPath == nil {
        log.Fatal("Liquidity check failed: The underlying router cannot find any operational path configurations.")
    }

    // Calculate Slippage Protection (5%)
    expectedOut := expectedAmounts[len(expectedAmounts)-1]
    minOut := new(big.Int).Mul(expectedOut, big.NewInt(95))
    minOut.Div(minOut, big.NewInt(100))

    // Execute Approval
    approve(client, auth, from)

    // Prepare Live Block Specs
    nonce, _ := client.PendingNonceAt(context.Background(), auth.From)
    auth.Nonce = big.NewInt(int64(nonce))
    gasPrice, _ := client.SuggestGasPrice(context.Background())
    auth.GasPrice = gasPrice
    deadline := big.NewInt(time.Now().Add(5 * time.Minute).Unix())

    div := new(big.Int).Exp(big.NewInt(10), big.NewInt(Decimals[fromName]), nil)
    f := new(big.Float).Quo(new(big.Float).SetInt(amount), new(big.Float).SetInt(div))
    val, _ := f.Float64()

    fmt.Println("--------------------------------------------------")
    fmt.Printf("Active Routing Path Identified Successfully!\n")
    fmt.Printf("Executing Swap Order: %.4f %s -> %s\n", val, fromName, toName)

    tx, err := contract.Transact(auth,
        "swapExactTokensForTokens",
        amount,
        minOut,
        validPath,
        auth.From,
        deadline,
    )
    if err != nil {
        log.Fatal("Swap broadcast failed execution:", err)
    }

    fmt.Println("Path array chosen:", validPath)
    fmt.Println("TX Hash:", tx.Hash().Hex())

    reportGas(client, tx, gasPrice)
}

// GAS REPORT GENERATOR WITH HIGH PRECISION SPACING
func reportGas(client *ethclient.Client, tx *types.Transaction, gasPrice *big.Int) {
    receipt, err := bind.WaitMined(context.Background(), client, tx)
    if err != nil {
        return
    }

    gasUsed := receipt.GasUsed
    totalWei := new(big.Int).Mul(big.NewInt(int64(gasUsed)), gasPrice)

    bnb := new(big.Float).Quo(new(big.Float).SetInt(totalWei), big.NewFloat(1e18))
    bnbF, _ := bnb.Float64()
    usd := bnbF * 600

    // Use our high-precision layout formatting rules
    prettyBNBCost := formatWithSpacedDecimals(bnbF)
    prettyUSDCost := formatWithSpacedDecimals(usd)

    fmt.Println("Gas Units Consumed:", gasUsed)
    fmt.Printf("Gas Cost Metrics: %s BNB (~$%s USD)\n", prettyBNBCost, prettyUSDCost)
}

// EXPORT STRATEGY METHOD
func SwapBTTtoSHIB(
    client *ethclient.Client,
    auth *bind.TransactOpts,
    tokens map[string]common.Address,
) {
    fmt.Println("--- Debugging Addresses ---")
    fmt.Println("BTT: ", tokens["BTT"].Hex())
    fmt.Println("SHIB:", tokens["SHIB"].Hex())
    fmt.Println("ETH: ", tokens["ETH"].Hex())
    fmt.Println("---------------------------")

    // 5000 BTT with 18 standard decimals
    amt := new(big.Int).Mul(big.NewInt(5000), big.NewInt(1e18))

    ExecuteSwap(
        client,
        auth,
        "BTT",
        "SHIB",
        tokens["BTT"],
        tokens["SHIB"],
        amt,
        tokens,
    )

    fmt.Println("✅ TRANSACTION COMPLETION SUCCESS")
}