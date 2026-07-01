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
 * Created Date    : 2026-06-30 02:00:00 (UTC+7)
 * Modified Date   : 2026-06-30 02:00:00 (UTC+7)
 *
 * Description     :
 *   AccountManager provides account identity, privacy-masked display,
 *   balance formatting, and portfolio directory persistence.
 *   Per myreq4.txt §79-81:
 *     - Private key as account name with first-8-chars + ***** masking
 *     - Eye icon toggle to reveal full key
 *     - Portfolio directories keyed by account for persistence
 *     - Balance formatted with 3-digit spacing and 9 fractional digits
 *
 * Responsibilities:
 *   - Read PRIVATE_KEY from environment
 *   - MaskedKey() returns first 8 chars + "*****"
 *   - FullKey() returns the unmasked key
 *   - PortfolioDir() returns account-specific persistence directory
 *   - FormatBalance() formats numbers with 3-digit grouping + 9 decimals
 *   - GetBalanceSummary() returns mock USD/BTC totals (BSC RPC later)
 *
 * Usage :
 *   Directory : infra/
 *   Build     : go build ./infra
 *   Test      : go test ./infra -v -run Account
 *
 * Dependencies :
 *   Internal : dexbot/tokens
 *   External : os, fmt, strings, math (stdlib)
 *
 * Configuration :
 *   - config.env (PRIVATE_KEY)
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Struct]    AccountManager
 *   [Function]  NewAccountManager, MaskedKey, FullKey, PortfolioDir,
 *               FormatBalance, GetBalanceSummary, FormatAmount, BTCPrice
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-30 02:00:00   | deepseek-4.0-pro | Initial version
 *            |                        |                  | §79-81 implementation
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Integrate real BSC RPC for on-chain balance queries
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 *   - BTC price is a mock value; replace with real oracle later.
 ******************************************************************************/

package infra

import (
	"fmt"
	"math"
	"os"
	"strings"
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
 *   Description : e.g., "12afb13e*****" or "no-private-key" if empty.
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
 *   Description : e.g., "runtime/portfolio_12afb13e/".
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
	// Mock token balances — replace with BSC RPC queries
	assets := []BalanceAsset{
		{Ticker: "BNB", Amount: 12.345678912, USDPrice: 610.50, BSCAddr: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c", ChainID: "56", ChainName: "BSC"},
		{Ticker: "BTC", Amount: 0.001234567, USDPrice: BTCPriceMock, BSCAddr: "0x7130d2A12B9BCbFAe4f2634d864A1Ee1Ce3Ead9c", ChainID: "56", ChainName: "BSC"},
		{Ticker: "USDC", Amount: 5432.109876543, USDPrice: 1.00, BSCAddr: "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d", ChainID: "56", ChainName: "BSC"},
		{Ticker: "CAKE", Amount: 123.456789012, USDPrice: 2.35, BSCAddr: "0x0E09FaBB73Bd3Ade0a17ECC321fD13a19e81cE82", ChainID: "56", ChainName: "BSC"},
		{Ticker: "UNI", Amount: 3234.123456789, USDPrice: 9.52, BSCAddr: "0xBf5140A22578168FD562DCcF235E5D43A02ce9B1", ChainID: "56", ChainName: "BSC"},
		{Ticker: "ADA", Amount: 8765.432109876, USDPrice: 0.42, BSCAddr: "0x3EE2200Efb3400fAbB9AacF31297cBdD1d435D47", ChainID: "56", ChainName: "BSC"},
	}

	totalUSD := 0.0
	for i := range assets {
		assets[i].USDValue = assets[i].Amount * assets[i].USDPrice
		totalUSD += assets[i].USDValue
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
