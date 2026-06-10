/*
Filename: tokens/tokens.go

Author: Gemini
Version: v1.1
Owner: Chalearm Saelim
Date: 2026-06-11

Description:
Token address registry for the balance tracker.

Features:
- Map token tickers to their respective common.Address
- Supports both BEP20 tokens and a native BNB placeholder

AI Prompt Idea:
"Create a Go registry mapping token symbols to their smart contract addresses on BNB Smart Chain."
*/

package tokens

import "github.com/ethereum/go-ethereum/common"

var Tokens = map[string]common.Address{
    "USDT": common.HexToAddress("0x55d398326f99059ff775485246999027b3197955"),
    "CAKE": common.HexToAddress("0x0e09fabb73bd3ade0a17ecc321fd13a19e81ce82"),
    "USDC": common.HexToAddress("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"),
    "WBNB": common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"),

    // Native BNB Coin (Handled dynamically via BalanceAt)
    "BNB": common.HexToAddress("0x0000000000000000000000000000000000000000"),

    // Verified Binance-Peg Ethereum
    "ETH": common.HexToAddress("0x2170Ed0880ac9A755fd29B2688956BD959F933F8"),

    // FIXED: Official BitTorrent (BTT) active contract address matching main.go
    "BTT": common.HexToAddress("0x352Cb5E19b12FC216548a2677bD0fce83BaE434B"),

    // Official Binance-Peg Shiba Inu Token
    "SHIB": common.HexToAddress("0x2859e4544C4bB03966803b044A93563Bd2D0DD4D"),

    // Official Binance-Peg Uniswap Token
    "UNI": common.HexToAddress("0xbf5140a22578168fd562dccf235e5d43a02ce9b1"),

    "AUTO": common.HexToAddress("0xa184088a740c695E156F91f5cC086a06bb78b827"),
    "BSW":  common.HexToAddress("0x965f527d9159dce6288a2219db51fc6eef120dd1"),
}