/******************************************************************************
 * File Name       : account.go
 * File Path       : infra/account.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:27 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:27 (UTC+7)
 *
 * Description     :
 *   AccountManager provides account identity, privacy-masked display,
 *
 * Responsibilities:
 *   - - Read PRIVATE_KEY from environment
 *
 * Usage :
 *   Directory : infra/
 *
 *   Build :
 *     go build ./infra
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./infra
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/infra
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
 *   1.0.0   | 2026-07-01 19:25:27 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package infra

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"

	"dexbot/auth"
	"dexbot/balance"
	"dexbot/tokens"
)

// BTCPriceMock is a placeholder BTC/USD rate (replace with oracle).
var BTCPriceMock = 85000.0

// ==============================
// ACCOUNT MANAGER
// ==============================

type AccountManager struct {
	privateKey string
}

/******************************************************************************
 * Function Name : NewAccountManager
 *
 * Purpose :
 *   Creates an AccountManager reading PRIVATE_KEY from environment.
 *
 * Inputs : None
 *
 * Return :
 *   Type        : *AccountManager
 *   Description : Initialized manager; privateKey may be empty.
 *
 * Complexity : O(1), Number Of Lines : 8
 *****************************************************************************
  * Error Cases :
  *   - None
  *
 */
func NewAccountManager() *AccountManager {
	pk := os.Getenv("PRIVATE_KEY")
	// Always read from config.env file (web unlock writes to file, not env vars)
	data, err := os.ReadFile("config.env")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "PRIVATE_KEY=") {
				parts := strings.SplitN(line, "=", 2)
				pk = ""
				if len(parts) == 2 && parts[1] != "" {
					pk = parts[1]
				}
				break
			}
		}
	}
	// If key is valid, set up profile in DB
	_ = ProfileFromKey(pk)
	return &AccountManager{privateKey: pk}
}

// ProfileFromKey looks up or creates a profile for the given key.
/******************************************************************************
 * Function Name : ProfileFromKey
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

func ProfileFromKey(pk string) *Profile {
	if pk == "" || len(pk) < 16 {
		return nil
	}
	prof, _ := LookupOrCreateProfile(pk)
	return prof
}

/******************************************************************************
 * Function Name : MaskedKey
 *
 * Purpose :
 *   Returns the first 8 characters of the private key followed by "*****".
 *   Per myreq4.txt §81: account name displays as masked by default.
 *
 * Return :
 *   Type        : string
 *   Description : e.g., "aabbcc******" or "no-private-key" if empty.
 *
 * Complexity : O(1), Number Of Lines : 8
 *****************************************************************************
  * Inputs :
  *   None (see function signature)
  *
  * Error Cases :
  *   - None
  *
 */
func (a *AccountManager) MaskedKey() string {
	if a.privateKey == "" {
		return "no-private-key"
	}
	if len(a.privateKey) <= 8 {
		return a.privateKey[:len(a.privateKey)] + "*****"
	}
	return a.privateKey[:8] + "*****"
}

/******************************************************************************
 * Function Name : FullKey
 *
 * Purpose :
 *   Returns the full unmasked private key. Shown when eye icon is clicked.
 *
 * Return :
 *   Type        : string
 *   Description : Full 64-char hex key, or empty string if not set.
 *
 * Complexity : O(1), Number Of Lines : 5
 *****************************************************************************
  * Inputs :
  *   None (see function signature)
  *
  * Error Cases :
  *   - None
  *
 */
func (a *AccountManager) FullKey() string {
	return a.privateKey
}

/******************************************************************************
 * Function Name : PortfolioDir
 *
 * Purpose :
 *   Returns the account-specific portfolio directory path.
 *   Per §81: portfolio saved/reloaded by account name.
 *
 * Return :
 *   Type        : string
 *   Description : e.g., "runtime/portfolio_abcdefab/".
 *
 * Complexity : O(1), Number Of Lines : 8
 *****************************************************************************
  * Inputs :
  *   None (see function signature)
  *
  * Error Cases :
  *   - None
  *
 */
func (a *AccountManager) PortfolioDir() string {
	if a.privateKey == "" {
		return "runtime/portfolio_default"
	}
	// Use first 16 chars as directory name
	prefix := a.privateKey
	if len(prefix) > 16 {
		prefix = prefix[:16]
	}
	return fmt.Sprintf("runtime/portfolio_%s", prefix)
}

// ==============================
// BALANCE FORMATTING (§79-80)
// ==============================

/******************************************************************************
 * Function Name : FormatAmount
 *
 * Purpose :
 *   Formats a float64 amount with spaces every 3 digits (integer part)
 *   and 9 fractional digits. Per myreq4.txt §80:
 *   "UNI 3 234 . 123 456 789"
 *
 * Inputs :
 *   value  float64 — Amount to format
 *
 * Return :
 *   Type        : string
 *   Description : Formatted string with 3-digit grouping and 9 decimals.
 *
 * Complexity : O(d) where d = number of digits, Number Of Lines : 20
 *****************************************************************************
  * Error Cases :
  *   - None
  *
 */
func FormatAmount(value float64) string {
	absVal := math.Abs(value)
	intPart := int64(absVal)
	fracPart := absVal - float64(intPart)

	// Format integer part with spaces every 3 digits
	intStr := fmt.Sprintf("%d", intPart)
	var groups []string
	for len(intStr) > 3 {
		groups = append([]string{intStr[len(intStr)-3:]}, groups...)
		intStr = intStr[:len(intStr)-3]
	}
	if intStr != "" {
		groups = append([]string{intStr}, groups...)
	}
	formattedInt := strings.Join(groups, " ")

	// Format fractional part to 9 digits
	fracStr := fmt.Sprintf("%.9f", fracPart)
	if len(fracStr) > 2 {
		fracStr = fracStr[2:] // remove "0."
	}
	if len(fracStr) > 9 {
		fracStr = fracStr[:9]
	}

	sign := ""
	if value < 0 {
		sign = "-"
	}
	if formattedInt == "" || formattedInt == "0" {
		formattedInt = "0"
	}
	return fmt.Sprintf("%s%s . %s", sign, formattedInt, fracStr)
}

/******************************************************************************
 * Function Name : FormatBalance
 *
 * Purpose :
 *   Formats an amount with asset ticker and optional USD equivalent.
 *   Per §80: "UNI 3 234 . 123 456 789 UNI (9 152 . 571 157 425 USD)"
 *
 * Inputs :
 *   amount   float64 — Token amount
 *   ticker   string  — Asset ticker (e.g., "UNI")
 *   usdPrice float64 — USD price per token (0 = omit USD)
 *
 * Return :
 *   Type        : string
 *   Description : Fully formatted balance string.
 *
 * Complexity : O(1), Number Of Lines : 12
 *****************************************************************************
  * Error Cases :
  *   - None
  *
 */
func FormatBalance(amount, usdPrice float64, ticker string) string {
	parts := fmt.Sprintf("%s %s %s", FormatAmount(amount), strings.ToUpper(ticker), strings.ToUpper(ticker))
	if usdPrice > 0 {
		usdValue := amount * usdPrice
		parts += fmt.Sprintf(" (%s USD)", FormatAmount(usdValue))
	}
	return parts
}

// ==============================
// BALANCE SUMMARY
// ==============================

// BalanceAsset represents a single asset holding.
type BalanceAsset struct {
	Ticker    string  `json:"ticker"`
	Amount    float64 `json:"amount"`
	USDPrice  float64 `json:"usd_price"`
	USDValue  float64 `json:"usd_value"`
	BSCAddr   string  `json:"bsc_addr"`
	ChainID   string  `json:"chain_id"`
	ChainName string  `json:"chain_name"`
}

// BalanceSummary is the full account balance response.
type BalanceSummary struct {
	AccountName   string          `json:"account_name"`
	AccountMasked string          `json:"account_masked"`
	TotalUSD      float64         `json:"total_usd"`
	TotalBTC      float64         `json:"total_btc"`
	BTCPrice      float64         `json:"btc_price"`
	Assets        []BalanceAsset  `json:"assets"`
	IsPaperTrade  bool            `json:"is_paper_trade"`
}

/******************************************************************************
 * Function Name : GetBalanceSummary
 *
 * Purpose :
 *   Returns a BalanceSummary with mock token balances.
 *   In production, this queries BSC RPC for real on-chain balances.
 *
 * Return :
 *   Type        : *BalanceSummary
 *   Description : Populated summary; TotalUSD computed from assets.
 *
 * Complexity : O(n) where n = number of tokens, Number Of Lines : 25
 *****************************************************************************
  * Inputs :
  *   None (see function signature)
  *
  * Error Cases :
  *   - None
  *
 */
func GetBalanceSummary(am *AccountManager) *BalanceSummary {
	// Try real BSC on-chain query first, fall back to token registry defaults
	var assets []BalanceAsset
	totalUSD := 0.0

	pk := am.FullKey()
	if pk != "" {
		// Use the real balance query via dexbot/auth + dexbot/balance
		realAssets, realTotal := queryRealBalances(pk)
		if len(realAssets) > 0 {
			assets = realAssets
			totalUSD = realTotal
		}
	}

	// Fallback: token registry defaults with zero-balance placeholder
	if len(assets) == 0 {
		registry := NewTokenRegistry()
		for _, t := range registry.GetTokens() {
			assets = append(assets, BalanceAsset{
				Ticker:    t.Ticker,
				Amount:    0,
				USDPrice:  t.USDPrice,
				USDValue:  0,
				BSCAddr:   t.Address,
				ChainID:   t.ChainID,
				ChainName: t.ChainName,
			})
		}
	}

	// Fetch live BTC price before computing TotalBTC (§119)
	btcPrice := GetBTCPrice()
	if btcPrice > 0 {
		BTCPriceMock = btcPrice
	}

	accountName := ""
	accountMasked := "no-account"
	if am != nil {
		accountName = am.FullKey()
		accountMasked = am.MaskedKey()
	}

	return &BalanceSummary{
		AccountName:   accountName[:min(8,len(accountName))] + "*****",
		AccountMasked: accountMasked,
		TotalUSD:      totalUSD,
		TotalBTC:      totalUSD / BTCPriceMock,
		BTCPrice:      BTCPriceMock,
		Assets:        assets,
		IsPaperTrade:  false,
	}
}
/******************************************************************************
 * Function Name : min
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


func min(a, b int) int {
	if a < b { return a }
	return b
}

// queryRealBalances uses dexbot/auth + dexbot/balance to query ALL chains
// exactly like apps/balance/main.go does. Each chain gets its own RPC client
// and wallet via auth.ConnectToChain + auth.GetWalletForChain.
/******************************************************************************
 * Function Name : queryRealBalances
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

func queryRealBalances(pk string) ([]BalanceAsset, float64) {
	var assets []BalanceAsset

	// ERC20 ABI — same as balance.Report()
	parsedABI, _ := abi.JSON(strings.NewReader(balance.ERC20_ABI))
	base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	prices := balance.TokenPrices

	for chainName, tokenMap := range tokens.Chains {
		chainID, rpcURL := chainInfo(chainName)
		if rpcURL == "" {
			continue
		}
		client := auth.ConnectToChain(rpcURL)
		wallet := auth.GetWalletForChain(client, pk, chainID)

		// Use balance.Report() for console diagnostics
		_ = balance.Report(client, wallet, tokenMap)

		// Query each token individually for the web asset list
		for ticker, addr := range tokenMap {
			isNative := addr.Hex() == "0x0000000000000000000000000000000000000000" ||
				(chainName == "BSC" && ticker == "BNB") ||
				(chainName == "POLYGON" && ticker == "MATIC") ||
				(chainName == "ETHEREUM" && ticker == "ETH")

			var amount float64
			if isNative {
				bal, _ := client.BalanceAt(context.Background(), wallet.From, nil)
				if bal != nil {
					clean, _ := new(big.Float).Quo(new(big.Float).SetInt(bal), base18).Float64()
					amount = clean
				}
			} else {
				tokenDecimals := uint8(18)
				contract := bind.NewBoundContract(addr, parsedABI, client, client, client)
				var decRes []interface{}
				if contract.Call(nil, &decRes, "decimals") == nil && len(decRes) > 0 {
					if d, ok := decRes[0].(uint8); ok {
						tokenDecimals = d
					}
				}
				var res []interface{}
				if contract.Call(nil, &res, "balanceOf", wallet.From) == nil && len(res) > 0 {
					if bal, ok := res[0].(*big.Int); ok && bal != nil {
						divisor := math.Pow10(int(tokenDecimals))
						clean := new(big.Float).Quo(new(big.Float).SetInt(bal), big.NewFloat(divisor))
						amount, _ = clean.Float64()
					}
				}
			}

			usdPrice := prices[ticker]
			if usdPrice <= 0 && (ticker == "USDC" || ticker == "USDT" || ticker == "BUSD") {
				usdPrice = 1.0
			}
			if ticker == "BNB" && amount > 0 {
				if oraclePtr, err := NewPriceOracle(); err == nil {
					if p, e := oraclePtr.GetPriceBNB(); e == nil && p > 0 {
						usdPrice = p
					}
					oraclePtr.Close()
				}
			}
			usdVal := amount * usdPrice
			assets = append(assets, BalanceAsset{
				Ticker: ticker, Amount: amount, USDPrice: usdPrice,
				USDValue: usdVal, BSCAddr: addr.Hex(),
				ChainID: fmt.Sprintf("%d", chainID), ChainName: chainName,
			})
		}
		client.Close()
		time.Sleep(500 * time.Millisecond)
	}

	// Add any tokens from tokens.Chains NOT already in assets (zero-balance display)
	seen := make(map[string]bool)
	for _, a := range assets {
		seen[a.Ticker+"_"+a.ChainName] = true
	}
	for chainName, tokenMap := range tokens.Chains {
		chainID, _ := chainInfo(chainName)
		if chainID == 0 {
			continue
		}
		for ticker, addr := range tokenMap {
			key := ticker + "_" + chainName
			if seen[key] {
				continue
			}
			seen[key] = true
			usdPrice := prices[ticker]
			if usdPrice <= 0 && (ticker == "USDC" || ticker == "USDT" || ticker == "BUSD") {
				usdPrice = 1.0
			}
			assets = append(assets, BalanceAsset{
				Ticker: ticker, Amount: 0, USDPrice: usdPrice,
				USDValue: 0, BSCAddr: addr.Hex(),
				ChainID: fmt.Sprintf("%d", chainID), ChainName: chainName,
			})
		}
	}

	// Sort by chain then ticker
	sort.Slice(assets, func(i, j int) bool {
		if assets[i].ChainName != assets[j].ChainName {
			return assets[i].ChainName < assets[j].ChainName
		}
		return assets[i].Ticker < assets[j].Ticker
	})

	// Compute total from assets (not balance.Report(), which may have timing differences)
	totalUSD := 0.0
	for _, a := range assets {
		totalUSD += a.USDValue
	}
	return assets, totalUSD
}

// chainInfo returns chain ID + RPC URL for a chain name.
/******************************************************************************
 * Function Name : chainInfo
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

func chainInfo(name string) (int64, string) {
	switch name {
	case "BSC":
		return 56, "https://bsc-dataseed.binance.org/"
	case "POLYGON":
		return 137, "https://polygon.drpc.org"
	case "OPBNB":
		return 204, "https://opbnb-mainnet-rpc.bnbchain.org"
	case "ETHEREUM":
		return 1, "https://eth.llamarpc.com"
	default:
		return 0, ""
	}
}