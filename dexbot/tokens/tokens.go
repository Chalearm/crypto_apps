/******************************************************************************
 * File Name       : tokens.go
 * File Path       : tokens/tokens.go
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
 *   Token address registry for the balance tracker. - Map token tickers to their respective common.Address - Supports both BEP20 tokens and a native BNB placeholder AI Prompt Idea: "Create a Go registry m
 *
 * Responsibilities:
 *   - Implement core functionality for tokens package.
 *
 * Usage :
 *   Directory : tokens/
 *
 *   Build :
 *     go build ./tokens
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./tokens
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/tokens
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