/******************************************************************************
 * File Name        : db.go
 * File Path        : apps/balance/db.go
 * Author           : Gemini 3.1 Pro & Gemini
 * Owner            : Chalearm Saelim
 * Reviewer         : Chalearm Saelim
 * Version          : 1.5.1
 * Status           : Development
 * Created Date     : 2026-07-12 14:45:00 (UTC+7)
 * Modified Date    : 2026-07-12 16:05:00 (UTC+7)
 *
 * Description      :
 *    Database access layer for the Balance application. Connects to PostgreSQL,
 *    manages dynamic network fallbacks (Container 'db' vs Localhost), and executes 
 *    cascading SQL operations for user profiles, active chains, and tracked tokens.
 *
 * DEPENDENCY TREE & STRUCTURAL MAP:
 * ───────────────────────────────────────────────────────────────────────────
 * [apps/balance/db.go] (Database Persistence Engine)
 *     │
 *     ├── Imports Internal Modules ──> [dexbot/infra] (Logger Engine)
 *     ├── Imports External Drivers ──> [github.com/lib/pq] (PostgreSQL Driver)
 *     │
 *     ├── Connection Handshake Strategy:
 *     │     ├── Stage 1: Primary Target ──> `host=db port=5432` (Container Bridge)
 *     │     └── Stage 2: Fallback Route ──> `host=localhost port=5432` (Host Interface)
 *     │
 *     └── Schema Relations (`account_key` = SHA256 identifier):
 *           ├── `user_profiles` ──> Profile records with timestamps
 *           ├── `user_chains`   ──> Dynamic chain registrations per account
 *           └── `user_tokens`   ──> ERC-20/BEP-20 token tracking entries
 *
 * FUNCTION DEPENDENCY MATRIX (Internal Methods):
 * ───────────────────────────────────────────────────────────────────────────
 * InitDB()
 *  ├── sql.Open("postgres", primaryConnStr) ──> dbConn.Ping()
 *  └── [On Failure] ──> sql.Open("postgres", fallbackConnStr) ──> dbConn.Ping()
 *
 * GetChainNameByID(accountHash, chainID) ───────> dbConn.QueryRow("SELECT chain_name...")
 * CheckUserProfileExists(accountHash) ──────────> dbConn.QueryRow("SELECT id FROM user_profiles...")
 * InsertUserProfile(accountHash) ───────────────> dbConn.Exec("INSERT INTO user_profiles...")
 * InsertUserChain(accountHash, chainName, ID) ──> dbConn.Exec("INSERT INTO user_chains...")
 * InsertUserToken(accountHash, chain, ticker, addr) -> dbConn.Exec("INSERT INTO user_tokens...")
 * DeleteAccountCascade(accountHash) ────────────> dbConn.Exec("DELETE FROM user_tokens...")
 *                                           ──> dbConn.Exec("DELETE FROM user_chains...")
 *                                           ──> dbConn.Exec("DELETE FROM user_profiles...")
 * DeleteChainCascade(accountHash, chainID) ────> GetChainNameByID()
 *                                           ──> dbConn.Exec("DELETE FROM user_tokens...")
 *                                           ──> dbConn.Exec("DELETE FROM user_chains...")
 * DeleteSingleToken(accountHash, chain, ticker) ─> dbConn.Exec("DELETE FROM user_tokens...")
 *
 * Responsibilities :
 *    - Ensures reliable database connectivity across both local and container environments.
 *    - Resolves numeric chain IDs to human-readable network names dynamically.
 *    - Performs application-level cascading deletes to maintain referential integrity.
 *    - Binds user-configured token contract addresses to relational database tables.
 *
 * Usage :
 *    Directory : apps/balance/
 *    Build     : Built as part of main package (`go build -o balance .`)
 *
 * Dependencies :
 *    Internal  : dexbot/infra
 *    External  : database/sql, github.com/lib/pq
 *
 * Change History :
 *    -------------------------------------------------------------------------
 *    Version | Date Time (UTC+7)         | Author          | Description
 *    -------------------------------------------------------------------------
 *    1.0.0   | 2026-07-12 14:45:00 (UTC+7) | Gemini 3.1 Pro  | Initial database connectivity
 *    1.5.0   | 2026-07-12 15:45:00 (UTC+7) | Gemini 3.1 Pro  | Matched chain_name schema constraints
 *    1.5.1   | 2026-07-12 16:05:00 (UTC+7) | Gemini 3.1 Pro  | Added explicit application-level cascade
 *    -------------------------------------------------------------------------
 *
 * Notes :
 *    - Per regulator coding standard rules.
 ******************************************************************************/
package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"dexbot/infra"
	_ "github.com/lib/pq"
)

var dbConn *sql.DB
/******************************************************************************
 * Function Name : InitDB
 *
 * Purpose :
 *   Initializes the database connection. Probes the primary configured host
 *   (e.g., Docker container name 'db') and automatically rolls back to a local
 *   localhost interface if connection handshakes are refused.
 *
 * Inputs :
 *   None
 *
 * Return :
 *   None
 *
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - Empty user/dbname properties trigger process termination.
 *   - Failure to establish network connectivity on both targets exits with code 1.
 *
 * Number Of Lines :
 *   52
 ******************************************************************************/
func InitDB() {
	primaryHost := os.Getenv("DB_HOST")
	if primaryHost == "" {
		primaryHost = "db"
	}
	
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASS")
	dbname := os.Getenv("DB_NAME")

	if user == "" || dbname == "" {
		infra.Error("Database config error: DB_USER or DB_NAME variables are empty.")
		os.Exit(1)
	}

	// Stage 1: Attempt connection using the primary target host option (Docker)
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		primaryHost, port, user, pass, dbname)

	var err error
	dbConn, err = sql.Open("postgres", connStr)
	if err == nil {
		err = dbConn.Ping()
		if err == nil {
			infra.Info(fmt.Sprintf("Successfully connected to database via primary host layout target: %s", primaryHost))
			return
		}
	}

	// Stage 2: Fallback route triggered if the primary path is blocked or running outside container
	infra.Warn(fmt.Sprintf("Primary database target host (%s) unreachable: %v. Swapping connection fallback to localhost...", primaryHost, err))
	
	if dbConn != nil {
		dbConn.Close()
	}

	fallbackHost := "localhost"
	fallbackConnStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		fallbackHost, port, user, pass, dbname)

	dbConn, err = sql.Open("postgres", fallbackConnStr)
	if err != nil {
		infra.Error("Failed to open fallback database connection: " + err.Error())
		os.Exit(1)
	}

	err = dbConn.Ping()
	if err != nil {
		infra.Error("Critical: Failed to connect to database on both primary and fallback localhost networks: " + err.Error())
		os.Exit(1)
	}

	infra.Info("Successfully connected to database using local machine network fallback layout.")
}
/******************************************************************************
 * Function Name : GetChainNameByID
 *
 * Purpose :
 *   Maps a numeric Chain ID to its human-readable chain name from
 *   the user_chains database schema.
 *
 * Inputs :
 *   accountHash
 *     Type        : string
 *     Description : SHA256 account identifier
 *
 *   chainID
 *     Type        : string
 *     Description : Numeric chain ID
 *
 * Return :
 *   Type        : string
 *   Description : Chain name or empty if not found.
 *
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - DB nil or query failure returns empty string.
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/
func GetChainNameByID(accountHash, chainID string) string {
	var name string
	err := dbConn.QueryRow(`SELECT chain_name FROM user_chains WHERE account_key = $1 AND chain_id = $2`, accountHash, chainID).Scan(&name)
	if err != nil {
		return ""
	}
	return name
}
/******************************************************************************
 * Function Name : CheckUserProfileExists
 *
 * Purpose :
 *   Checks if a given SHA256 account string exists in user_profiles.
 *
 * Inputs :
 *   accountHash
 *     Type        : string
 *     Range       : Valid SHA256 string
 *     Description : Target account hash.
 *
 * Outputs :
 *   None
 *
 * Return :
 *   Type        : bool
 *   Range       : true/false
 *   Description : True if exists, false otherwise.
 *
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - SQL query error (returns false and logs error).
 *
 * Dependencies :
 *   - dbConn
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   12
 ******************************************************************************/
func CheckUserProfileExists(accountHash string) bool {
	var id string
	query := `SELECT id FROM user_profiles WHERE id = $1`
	err := dbConn.QueryRow(query, accountHash).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return false
		}
		infra.Error("Error checking user profile: " + err.Error())
		return false
	}
	return true
}

/******************************************************************************
 * Function Name : InsertUserProfile
 *
 * Purpose :
 *   Creates a new record in user_profiles with timestamps.
 *
 * Inputs :
 *   accountHash
 *     Type        : string
 *     Range       : Valid SHA256 string
 *     Description : The account hash to insert.
 *
 * Outputs :
 *   None
 *
 * Return :
 *   None
 *
 * Error Cases :
 *   - Database execution failure (logs error).
 *
 * Dependencies :
 *   - dbConn
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/
func InsertUserProfile(accountHash string) {
	query := `INSERT INTO user_profiles (id, created_at, updated_at) VALUES ($1, $2, $3)`
	now := time.Now().UTC()
	_, err := dbConn.Exec(query, accountHash, now, now)
	if err != nil {
		infra.Error("Error inserting user profile: " + err.Error())
	}
}

/******************************************************************************
 * Function Name : InsertUserChain
 *
 * Purpose :
 *   Inserts a new chain mapping for the given account.
 *
 * Inputs :
 *   accountHash, chainName, chainURL, chainID
 *     Type        : string
 *     Description : Chain binding details
 *
 * Outputs :
 *   None
 *
 * Return :
 *   None
 *
 * Error Cases :
 *   - Database execution failure.
 *
 * Dependencies :
 *   - dbConn
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/
func InsertUserChain(accountHash, chainName, chainID string) {
	query := `INSERT INTO user_chains (account_key, chain_name, chain_id, created_at) VALUES ($1, $2, $3, $4)`
	now := time.Now().UTC()
	_, err := dbConn.Exec(query, accountHash, chainName, chainID, now)
	if err != nil {
		infra.Error("Error inserting user chain: " + err.Error())
	}
}

/******************************************************************************
 * Function Name : InsertUserToken
 *
 * Purpose :
 *   Inserts a new token mapping for the given account and chain.
 *
 * Inputs :
 *   accountHash, chainID, ticker, address
 *     Type        : string
 *     Description : Token mapping details.
 *
 * Outputs :
 *   None
 *
 * Return :
 *   None
 *
 * Error Cases :
 *   - Database execution failure.
 *
 * Dependencies :
 *   - dbConn
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/
func InsertUserToken(accountHash, chainName, ticker, address string) {
	query := `INSERT INTO user_tokens (account_key, chain_name, ticker, address, created_at) VALUES ($1, $2, $3, $4, $5)`
	now := time.Now().UTC()
	_, err := dbConn.Exec(query, accountHash, chainName, ticker, address, now)
	if err != nil {
		infra.Error("Error inserting user token: " + err.Error())
	}
}

/******************************************************************************
 * Function Name : DeleteAccountCascade
 *
 * Purpose :
 *   Removes the account profile. Cascades to chains and tokens.
 *
 * Inputs :
 *   accountHash
 *     Type        : string
 *     Description : Target account to delete.
 *
 * Outputs :
 *   None
 *
 * Return :
 *   None
 *
 * Error Cases :
 *   - Database execution failure.
 *
 * Dependencies :
 *   - dbConn
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   8
 ******************************************************************************/
func DeleteAccountCascade(accountHash string) {
	// 1. Delete all tokens owned by the account
	_, err := dbConn.Exec(`DELETE FROM user_tokens WHERE account_key = $1`, accountHash)
	if err != nil {
		infra.Warn("Error cleaning up user tokens: " + err.Error())
	}

	// 2. Delete all chains owned by the account
	_, err = dbConn.Exec(`DELETE FROM user_chains WHERE account_key = $1`, accountHash)
	if err != nil {
		infra.Warn("Error cleaning up user chains: " + err.Error())
	}

	// 3. Delete the profile itself
	_, err = dbConn.Exec(`DELETE FROM user_profiles WHERE id = $1`, accountHash)
	if err != nil {
		infra.Error("Error deleting account profile: " + err.Error())
	}
}

/******************************************************************************
 * Function Name : DeleteChainCascade
 *
 * Purpose :
 *   Removes a specific chain for an account.
 *
 * Inputs :
 *   accountHash, chainID
 *     Type        : string
 *     Description : Identifiers for deletion.
 *
 * Outputs :
 *   None
 *
 * Return :
 *   None
 *
 * Error Cases :
 *   - Database execution failure.
 *
 * Dependencies :
 *   - dbConn
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   8
 ******************************************************************************/
func DeleteChainCascade(accountHash, chainID string) {
	chainName := GetChainNameByID(accountHash, chainID)
	
	// If chain exists, manually wipe its tokens first
	if chainName != "" {
		_, err := dbConn.Exec(`DELETE FROM user_tokens WHERE account_key = $1 AND chain_name = $2`, accountHash, chainName)
		if err != nil {
			infra.Warn("Error cascading token deletions: " + err.Error())
		}
	}

	// Delete the chain
	query := `DELETE FROM user_chains WHERE account_key = $1 AND chain_id = $2`
	_, err := dbConn.Exec(query, accountHash, chainID)
	if err != nil {
		infra.Error("Error deleting user chain: " + err.Error())
	}
}

/******************************************************************************
 * Function Name : DeleteSingleToken
 *
 * Purpose :
 *   Removes a specific token mapping for an account.
 *
 * Inputs :
 *   accountHash, chainID, tokenName
 *     Type        : string
 *     Description : Identifiers for token deletion.
 *
 * Outputs :
 *   None
 *
 * Return :
 *   None
 *
 * Error Cases :
 *   - Database execution failure.
 *
 * Dependencies :
 *   - dbConn
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   8
 ******************************************************************************/
func DeleteSingleToken(accountHash, chainName, tokenName string) {
	query := `DELETE FROM user_tokens WHERE account_key = $1 AND chain_name = $2 AND ticker = $3`
	_, err := dbConn.Exec(query, accountHash, chainName, tokenName)
	if err != nil {
		infra.Error("Error deleting user token: " + err.Error())
	}
}