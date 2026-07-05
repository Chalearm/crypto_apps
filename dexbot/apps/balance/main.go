/******************************************************************************
 * File Name       : main.go
 * File Path       : apps/balance/main.go
 ******************************************************************************/
package main

import (
    "fmt"
    "dexbot/auth"
    "dexbot/balance"
    "dexbot/tokens"
)

func main() {
    pk := auth.LoadPrivateKey()

    // 1. Process BSC Chain Network
    fmt.Println("NETWORK: BINANCE SMART CHAIN")
    fmt.Println("------------------------------------------------------------")
    bscClient := auth.ConnectToChain("https://bsc-dataseed.binance.org/")
    bscWallet := auth.GetWalletForChain(bscClient, pk, 56)
    bscTotal := balance.Report(bscClient, bscWallet, tokens.Chains["BSC"])
    fmt.Println()

    // 2. Process Polygon Chain Network
    fmt.Println("NETWORK: POLYGON POS")
    fmt.Println("------------------------------------------------------------")
    polyClient := auth.ConnectToChain("https://polygon.drpc.org")
    polyWallet := auth.GetWalletForChain(polyClient, pk, 137)
    polygonTotal := balance.Report(polyClient, polyWallet, tokens.Chains["POLYGON"])
    fmt.Println()

    // 3. Process opBNB Layer 2 Network
    fmt.Println("NETWORK: opBNB LAYER 2")
    fmt.Println("------------------------------------------------------------")
    opBnbClient := auth.ConnectToChain("https://opbnb-mainnet-rpc.bnbchain.org")
    opBnbWallet := auth.GetWalletForChain(opBnbClient, pk, 204)
    opBnbTotal := balance.Report(opBnbClient, opBnbWallet, tokens.Chains["OPBNB"])
    fmt.Println()

    // 4. Compute Cumulative System Summary
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