/******************************************************************************
 * File Name       : token_persistence.go
 * File Path       : infra/token_persistence.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:32 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:32 (UTC+7)
 *
 * Description     :
 *   Token persistence layer — loads/saves user token configurations
 *
 * Responsibilities:
 *   - Implement core functionality.
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
 *   1.0.0   | 2026-07-01 19:25:32 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
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
	"fmt"
)

// CreateUserTokensTable ensures the user_tokens table exists.
func CreateUserTokensTable() {
	if DB == nil {
		return
	}
	query := `
	CREATE TABLE IF NOT EXISTS user_tokens (
		id SERIAL PRIMARY KEY,
		account_key TEXT NOT NULL,
		ticker TEXT NOT NULL,
		address TEXT NOT NULL,
		chain_id TEXT NOT NULL DEFAULT '56',
		chain_name TEXT NOT NULL DEFAULT 'BSC',
		base_url TEXT NOT NULL DEFAULT 'https://bsc-dataseed.binance.org',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(account_key, ticker, chain_id)
	);
	CREATE INDEX IF NOT EXISTS idx_user_tokens_account ON user_tokens(account_key);
	`
	if _, err := DB.Exec(query); err != nil {
		Error("CreateUserTokensTable failed: " + err.Error())
	}
}

// TokenRecord is a DB row from user_tokens.
type TokenRecord struct {
	Ticker    string
	Address   string
	ChainID   string
	ChainName string
	BaseURL   string
}

// LoadTokensForAccount loads tokens for a private key from DB.
// If no records exist, seeds from default tokens (from tokens/tokens.go) and saves to DB.
// USD prices are NOT stored — they come from real wallet + price oracle at display time.
func LoadTokensForAccount(accountKey string, defaults []TokenRecord) ([]TokenRecord, error) {
	if DB == nil {
		return defaults, nil
	}
	CreateUserTokensTable()

	rows, err := DB.Query(`SELECT ticker, address, chain_id, chain_name, base_url
		FROM user_tokens WHERE account_key = $1 ORDER BY ticker`, accountKey)
	if err != nil {
		return defaults, err
	}
	defer rows.Close()

	var tokens []TokenRecord
	for rows.Next() {
		var t TokenRecord
		if err := rows.Scan(&t.Ticker, &t.Address, &t.ChainID, &t.ChainName, &t.BaseURL); err != nil {
			continue
		}
		tokens = append(tokens, t)
	}

	// Seed from defaults if empty
	if len(tokens) == 0 && len(defaults) > 0 {
		tokens = defaults
		if err := SaveTokensForAccount(accountKey, tokens); err != nil {
			Warn("Failed to seed default tokens: " + err.Error())
		}
	}
	return tokens, nil
}

// SaveTokensForAccount persists token list to DB for a given account.
func SaveTokensForAccount(accountKey string, tokens []TokenRecord) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	CreateUserTokensTable()

	// Delete existing
	if _, err := DB.Exec(`DELETE FROM user_tokens WHERE account_key = $1`, accountKey); err != nil {
		return err
	}

	// Insert batch
	for _, t := range tokens {
		_, err := DB.Exec(`INSERT INTO user_tokens (account_key, ticker, address, chain_id, chain_name, base_url)
			VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (account_key, ticker, chain_id) DO UPDATE
			SET address=$3, chain_name=$5, base_url=$6`,
			accountKey, t.Ticker, t.Address, t.ChainID, t.ChainName, t.BaseURL)
		if err != nil {
			Warn("SaveTokensForAccount insert failed for " + t.Ticker + ": " + err.Error())
		}
	}
	return nil
}

// DefaultTokenRecords returns the BSC defaults from tokens/tokens.go.
// USD prices intentionally NOT stored — fetched live from wallet + PancakeSwap.
func DefaultTokenRecords() []TokenRecord {
	return []TokenRecord{
		{Ticker: "USDT", Address: "0x55d398326f99059ff775485246999027b3197955", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org"},
		{Ticker: "CAKE", Address: "0x0e09fabb73bd3ade0a17ecc321fd13a19e81ce82", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org"},
		{Ticker: "USDC", Address: "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org"},
		{Ticker: "WBNB", Address: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org"},
		{Ticker: "BNB", Address: "0x0000000000000000000000000000000000000000", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org"},
		{Ticker: "ETH", Address: "0x2170Ed0880ac9A755fd29B2688956BD959F933F8", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org"},
		{Ticker: "BTT", Address: "0x352Cb5E19b12FC216548a2677bD0fce83BaE434B", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org"},
		{Ticker: "SHIB", Address: "0x2859e4544C4bB03966803b044A93563Bd2D0DD4D", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org"},
		{Ticker: "UNI", Address: "0xbf5140a22578168fd562dccf235e5d43a02ce9b1", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org"},
		{Ticker: "AUTO", Address: "0xa184088a740c695E156F91f5cC086a06bb78b827", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org"},
		{Ticker: "BSW", Address: "0x965f527d9159dce6288a2219db51fc6eef120dd1", ChainID: "56", ChainName: "BSC", BaseURL: "https://bsc-dataseed.binance.org"},
	}
}
