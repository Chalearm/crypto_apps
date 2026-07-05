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

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

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
 ******************************************************************************/
func NewAccountManager() *AccountManager {
	pk := os.Getenv("PRIVATE_KEY")
	return &AccountManager{privateKey: pk}
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
 *   Description : e.g., "****" or "no-private-key" if empty.
 *
 * Complexity : O(1), Number Of Lines : 8
 ******************************************************************************/
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
 ******************************************************************************/
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
 *   Description : e.g., "runtime/portfolio_/".
 *
 * Complexity : O(1), Number Of Lines : 8
 ******************************************************************************/
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
 ******************************************************************************/
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
 ******************************************************************************/
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
 ******************************************************************************/
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
		AccountName:   accountName,
		AccountMasked: accountMasked,
		TotalUSD:      totalUSD,
		TotalBTC:      totalUSD / BTCPriceMock,
		BTCPrice:      BTCPriceMock,
		Assets:        assets,
		IsPaperTrade:  false,
	}
}

// queryRealBalances connects to BSC and reads real on-chain token balances
// for the given private key. Falls back silently on any error.
func queryRealBalances(pk string) ([]BalanceAsset, float64) {
	// Connect to BSC
	client, err := ethclient.Dial("https://bsc-dataseed.binance.org/")
	if err != nil {
		return nil, 0
	}
	defer client.Close()

	// Derive wallet from private key
	privateKey, err := crypto.HexToECDSA(pk)
	if err != nil {
		return nil, 0
	}
	chainID := big.NewInt(56)
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return nil, 0
	}

	// ERC20 ABI for balanceOf
	erc20ABI, err := abi.JSON(strings.NewReader(`[{"name":"balanceOf","type":"function","inputs":[{"name":"account","type":"address"}],"outputs":[{"name":"","type":"uint256"}],"stateMutability":"view"}]`))
	if err != nil {
		return nil, 0
	}

	base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))

	// Use the SAME token list as the CLI balance command (tokens/tokens.go)
	tokenList := tokens.Tokens
	prices := balance.TokenPrices

	// Try to get real-time BNB price from PancakeSwap
	bnbPrice := 610.50
	var oraclePtr *PriceOracle
	oraclePtr, err = NewPriceOracle()
	if err == nil {
		if p, e := oraclePtr.GetPriceBNB(); e == nil && p > 0 {
			bnbPrice = p
		}
		oraclePtr.Close()
	}

	// Collect tickers alphabetically
	type tokInfo struct {
		Ticker string
		Addr   common.Address
	}
	var sorted []tokInfo
	for ticker, addr := range tokenList {
		sorted = append(sorted, tokInfo{Ticker: ticker, Addr: addr})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Ticker < sorted[j].Ticker })

	var assets []BalanceAsset
	totalUSD := 0.0
	for _, tok := range sorted {
		ticker := strings.ToUpper(tok.Ticker)
		addr := tok.Addr
		var amount float64

		if ticker == "BNB" {
			bal, e := client.BalanceAt(context.Background(), auth.From, nil)
			if e == nil && bal != nil {
				clean := new(big.Float).Quo(new(big.Float).SetInt(bal), base18)
				amount, _ = clean.Float64()
			}
		} else if addr != (common.Address{}) && addr.Hex() != "0x0000000000000000000000000000000000000000" {
			contract := bind.NewBoundContract(addr, erc20ABI, client, client, client)
			var result []interface{}
			if e := contract.Call(nil, &result, "balanceOf", auth.From); e == nil && len(result) > 0 {
				if bal, ok := result[0].(*big.Int); ok && bal != nil {
					clean := new(big.Float).Quo(new(big.Float).SetInt(bal), base18)
					amount, _ = clean.Float64()
				}
			}
		}

		// Determine USD price — prefer live PancakeSwap price for BNB, use static fallback
		usdPrice := prices[ticker]
		if ticker == "BNB" && bnbPrice > 0 {
			usdPrice = bnbPrice // ALWAYS use live PancakeSwap BNB price
		}
		if usdPrice <= 0 && (ticker == "USDC" || ticker == "USDT") {
			usdPrice = 1.0
		}

		usdValue := amount * usdPrice
		totalUSD += usdValue

		// Show ALL tokens — even zero balance (will be dimmed on web)
		assets = append(assets, BalanceAsset{
			Ticker:    ticker,
			Amount:    amount,
			USDPrice:  usdPrice,
			USDValue:  usdValue,
			BSCAddr:   addr.Hex(),
			ChainID:   "56",
			ChainName: "BSC",
		})
	}

	return assets, totalUSD
}
