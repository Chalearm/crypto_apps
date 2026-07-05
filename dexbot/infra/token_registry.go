/******************************************************************************
 * File Name       : token_registry.go
 * File Path       : infra/token_registry.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:33 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:33 (UTC+7)
 *
 * Description     :
 *   Dynamic token registry for user-configurable asset tracking.
 *
 * Responsibilities:
 *   - - Load default tokens from dexbot/tokens
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
 *   1.0.0   | 2026-07-01 19:25:33 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
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
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// TokenEntry represents a single user-configured token.
// USD prices are fetched live from wallet + price oracle — not stored.
type TokenEntry struct {
	Ticker    string  `json:"ticker"`
	Address   string  `json:"address"`
	ChainID   string  `json:"chain_id"`
	ChainName string  `json:"chain_name"`
	BaseURL   string  `json:"base_url"`
	USDPrice  float64 `json:"usd_price"` // live price, not persisted
}

// TokenRegistry manages dynamic token configurations.
type TokenRegistry struct {
	mu     sync.RWMutex
	Tokens []TokenEntry          `json:"tokens"`
	path   string
}

var defaultTokens = []TokenEntry{
	{Ticker: "BNB", Address: "0x0000000000000000000000000000000000000000", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org", USDPrice: 610.50},
	{Ticker: "USDT", Address: "0x55d398326f99059ff775485246999027b3197955", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org", USDPrice: 1.00},
	{Ticker: "USDC", Address: "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org", USDPrice: 1.00},
	{Ticker: "CAKE", Address: "0x0E09FaBB73Bd3Ade0a17ECC321fD13a19e81cE82", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org", USDPrice: 2.35},
	{Ticker: "WBNB", Address: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org", USDPrice: 610.50},
	{Ticker: "ETH", Address: "0x2170Ed0880ac9A755fd29B2688956BD959F933F8", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org", USDPrice: 3400.00},
	{Ticker: "BTT", Address: "0x352Cb5E19b12FC216548a2677bD0fce83BaE434B", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org", USDPrice: 0.0000003},
	{Ticker: "SHIB", Address: "0x2859e4544C4bB03966803b044A93563Bd2D0DD4D", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org", USDPrice: 0.000025},
	{Ticker: "UNI", Address: "0xBf5140A22578168FD562DCcF235E5D43A02ce9B1", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org", USDPrice: 3.35},
	{Ticker: "AUTO", Address: "0xa184088a740c695E156F91f5cC086a06bb78b827", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org", USDPrice: 600.0},
	{Ticker: "BSW", Address: "0x965f527d9159dce6288a2219db51fc6eef120dd1", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org", USDPrice: 0.30},
}

// NewTokenRegistry loads or creates the token registry.
func NewTokenRegistry() *TokenRegistry {
	path := "runtime/token_registry.json"
	r := &TokenRegistry{path: path, Tokens: defaultTokens}
	r.Load()
	r.Save()
	return r
}

// AddToken appends a new token entry and saves.
func (r *TokenRegistry) AddToken(t TokenEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, existing := range r.Tokens {
		if existing.Ticker == t.Ticker && existing.ChainID == t.ChainID {
			r.Tokens[i] = t // update existing
			r.Save()
			return nil
		}
	}
	r.Tokens = append(r.Tokens, t)
	return r.Save()
}

// RemoveToken removes a token by ticker and chain ID.
func (r *TokenRegistry) RemoveToken(ticker, chainID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, t := range r.Tokens {
		if t.Ticker == ticker && t.ChainID == chainID {
			r.Tokens = append(r.Tokens[:i], r.Tokens[i+1:]...)
			return r.Save()
		}
	}
	return fmt.Errorf("token %s/%s not found", ticker, chainID)
}

// ListTokens returns a copy of all registered tokens.
func (r *TokenRegistry) ListTokens() []TokenEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]TokenEntry, len(r.Tokens))
	copy(out, r.Tokens)
	return out
}

// GetTokens returns the raw slice (for iteration).
func (r *TokenRegistry) GetTokens() []TokenEntry {
	return r.ListTokens()
}

// Save persists the registry to JSON.
func (r *TokenRegistry) Save() error {
	os.MkdirAll("runtime", 0755)
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

// Load reads the registry from JSON; falls back to defaults.
func (r *TokenRegistry) Load() {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return // use defaults
	}
	var loaded TokenRegistry
	if err := json.Unmarshal(data, &loaded); err != nil {
		return
	}
	if len(loaded.Tokens) > 0 {
		r.Tokens = loaded.Tokens
	}
}

// AsBalanceAssets converts token entries to BalanceAsset slice for the dashboard.
// Amounts are zero — real balances come from the BSC wallet query.
func (r *TokenRegistry) AsBalanceAssets() []BalanceAsset {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var assets []BalanceAsset
	for _, t := range r.Tokens {
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
	return assets
}
