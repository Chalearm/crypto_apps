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
 * Created Date    : 2026-07-01 19:25:44 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:44 (UTC+7)
 *
 * Description     :
 *   Token address registry for the balance tracker. - Map token tickers to their respective common.Address - Supports both BEP20 tokens and a native BNB placeholder AI Prompt Idea: "Create a Go registry m
 *
 * Responsibilities:
 *   - - Implement core functionality for tokens package.
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
 *   1.0.0   | 2026-07-01 19:25:44 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
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

// Chains must be declared first so it is allocated in memory before Tokens assignments
var Chains = map[string]map[string]common.Address{
    "BSC": {
        "USDT": common.HexToAddress("0x55d398326f99059ff775485246999027b3197955"),
        "CAKE": common.HexToAddress("0x0e09fabb73bd3ade0a17ecc321fd13a19e81ce82"),
        "USDC": common.HexToAddress("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"),
        "BUSD": common.HexToAddress("0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56"), // 👈 FIXED: Correct Official Address
        "WBNB": common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"),
        "BNB":  common.HexToAddress("0x0000000000000000000000000000000000000000"), // Native
        "ETH":  common.HexToAddress("0x2170Ed0880ac9A755fd29B2688956BD959F933F8"),
        "BTT":  common.HexToAddress("0x352Cb5E19b12FC216548a2677bD0fce83BaE434B"),
        "SHIB": common.HexToAddress("0x2859e4544C4bB03966803b044A93563Bd2D0DD4D"),
        "UNI":  common.HexToAddress("0xbf5140a22578168fd562dccf235e5d43a02ce9b1"),
        "AUTO": common.HexToAddress("0xa184088a740c695E156F91f5cC086a06bb78b827"),
        "BSW":  common.HexToAddress("0x965f527d9159dce6288a2219db51fc6eef120dd1"),
        "BTC":  common.HexToAddress("0x7130d2a12b9bcbfae4f2634d864a1ee1ce3ead9c"),
    },
    "POLYGON": {
        "MATIC": common.HexToAddress("0x0000000000000000000000000000000000000000"), // Native
        "USDT":  common.HexToAddress("0xc2132d05d31c914a87c6611c10748aeb04b58e8f"),
        "USDC":  common.HexToAddress("0x3c499c542cef5e3811e1192ce70d8cc03d5c3359"),
    },
    "ETHEREUM": {
        "ETH":  common.HexToAddress("0x0000000000000000000000000000000000000000"), // Native
        "USDT": common.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7"),
        "USDC": common.HexToAddress("0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"),
    },
    "OPBNB": { // 👈 Added opBNB Layer 2 Definition Registry
        "BNB":  common.HexToAddress("0x0000000000000000000000000000000000000000"), // Native L2 Gas Token
        "USDT": common.HexToAddress("0x9e5aac1ba1a2e6aed6b32689dfcf62a509ca96f3"), // Official opBNB USDT
    },
}

// ✅ FIXED: Initialized below Chains map so it correctly links the BSC map for backward compatibility
var Tokens = Chains["BSC"]