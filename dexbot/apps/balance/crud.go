/******************************************************************************
 * File Name        : crud.go
 * File Path        : apps/balance/crud.go
 * Author           : Gemini 3.1 Pro & Gemini
 * Owner            : Chalearm Saelim
 * Reviewer         : Chalearm Saelim
 * Version          : 1.2.0
 * Status           : Development
 * Created Date     : 2026-07-12 14:37:38 (UTC+7)
 * Modified Date    : 2026-07-12 15:45:00 (UTC+7)
 *
 * Description      :
 *    Command-line CRUD handler for accounts, networks, and tokens. Validates incoming
 *    parameters, resolves network identifiers dynamically, and bridges CLI invocation 
 *    directly to persistence operations in `db.go`.
 *
 * DEPENDENCY TREE & STRUCTURAL MAP:
 * ───────────────────────────────────────────────────────────────────────────
 * [apps/balance/crud.go] (CLI CRUD Controller)
 *     │
 *     ├── Imports Internal Modules ──> [dexbot/infra] (Logger Module)
 *     │
 *     ├── Invoked By ───────────────> `cli.go` (`parseAndRouteFlags()`)
 *     │
 *     └── Delegation Target ────────> `db.go` (SQL Execution Functions)
 *           ├── AddChainToAccount ──> `InsertUserChain()`
 *           ├── AddTokenToAccount ──> `GetChainNameByID()` -> `InsertUserToken()`
 *           ├── HandleDeleteAccount ─> `DeleteAccountCascade()`
 *           ├── HandleDeleteChain ──> `DeleteChainCascade()`
 *           └── HandleDeleteToken ──> `GetChainNameByID()` -> `DeleteSingleToken()`
 *
 * FUNCTION DEPENDENCY MATRIX (Internal Methods):
 * ───────────────────────────────────────────────────────────────────────────
 * AddChainToAccount(accountHash, chainName, chainURL, chainID)
 *  ├── InitDB() [If dbConn == nil]
 *  └── InsertUserChain(accountHash, chainName, chainID)
 *
 * AddTokenToAccount(accountHash, chainID, tokenName, tokenAddress)
 *  ├── InitDB() [If dbConn == nil]
 *  ├── GetChainNameByID(accountHash, chainID)
 *  └── InsertUserToken(accountHash, chainName, tokenName, tokenAddress)
 *
 * HandleDeleteAccount(accountHash)
 *  ├── InitDB() [If dbConn == nil]
 *  └── DeleteAccountCascade(accountHash)
 *
 * HandleDeleteChain(accountHash, chainID)
 *  ├── InitDB() [If dbConn == nil]
 *  └── DeleteChainCascade(accountHash, chainID)
 *
 * HandleDeleteToken(accountHash, chainID, tokenName)
 *  ├── InitDB() [If dbConn == nil]
 *  ├── GetChainNameByID(accountHash, chainID)
 *  └── DeleteSingleToken(accountHash, chainName, tokenName)
 *
 * Responsibilities :
 *    - Intercepts administrative user commands to insert or remove profile records.
 *    - Translates numeric Chain ID parameters to relational schema names before execution.
 *    - Logs operational audit details to standard out and system log files.
 *
 * Usage :
 *    Directory : apps/balance/
 *    Build     : Built as part of main package (`go build -o balance .`)
 *
 * Dependencies :
 *    Internal  : dexbot/infra
 *    External  : stdlib (fmt)
 *
 * Change History :
 *    -------------------------------------------------------------------------
 *    Version | Date Time (UTC+7)         | Author          | Description
 *    -------------------------------------------------------------------------
 *    1.0.0   | 2026-07-12 14:37:38 (UTC+7) | Gemini 3.1 Pro  | Initial CLI CRUD controller
 *    1.1.1   | 2026-07-12 15:45:00 (UTC+7) | Gemini 3.1 Pro  | Fixed argument matching for chain inserts
 *    1.2.0   | 2026-07-12 15:45:00 (UTC+7) | Gemini 3.1 Pro  | Integrated dynamic ID-to-Name mapper
 *    -------------------------------------------------------------------------
 *
 * Notes :
 *    - Per regulator coding standard rules.
 ******************************************************************************/
package main

import (
	"fmt"
	"dexbot/infra"
)

/******************************************************************************
 * Function Name : AddChainToAccount
 *
 * Purpose :
 *   Binds a new chain to an existing account profile in the database.
 *   Accepts the chainURL from CLI but drops it before DB insertion.
 *
 * Inputs :
 *   accountHash, chainName, chainURL, chainID
 *     Type        : string
 *     Description : Binding values for the new chain.
 *
 * Outputs :
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
 *   - Database execution failure.
 *
 * Dependencies :
 *   - infra.Info
 *   - InitDB
 *   - InsertUserChain
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/
func AddChainToAccount(accountHash, chainName, chainURL, chainID string) {
	if dbConn == nil {
		InitDB()
	}

	infra.Info(fmt.Sprintf("Adding chain %s (ID: %s, URL: %s) to account %s", chainName, chainID, chainURL, accountHash))
	// Pass only the 3 arguments that match the updated db.go schema
	InsertUserChain(accountHash, chainName, chainID)
	fmt.Printf("Successfully added chain %s to account %s\n", chainName, accountHash)
}

/******************************************************************************
 * Function Name : AddTokenToAccount
 *
 * Purpose :
 *   Binds a new token. Because CLI passes chainID and the DB expects 
 *   chainName, it dynamically resolves the name using GetChainNameByID.
 *
 * Inputs :
 *   accountHash, chainID, tokenName, tokenAddress
 *     Type        : string
 *     Description : Token binding parameters.
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
 *   - infra.Info
 *   - InitDB
 *   - InsertUserToken
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   16
 ******************************************************************************/
func AddTokenToAccount(accountHash, chainID, tokenName, tokenAddress string) {
	if dbConn == nil {
		InitDB()
	}

	chainName := GetChainNameByID(accountHash, chainID)
	if chainName == "" {
		infra.Error(fmt.Sprintf("Failed to resolve Chain ID %s to a valid chain_name. Add chain first.", chainID))
		return
	}

	infra.Info(fmt.Sprintf("Adding token %s (%s) on chain %s to account %s", tokenName, tokenAddress, chainName, accountHash))
	InsertUserToken(accountHash, chainName, tokenName, tokenAddress)
	fmt.Printf("Successfully added token %s to account %s\n", tokenName, accountHash)
}

/******************************************************************************
 * Function Name : HandleDeleteAccount
 *
 * Purpose :
 *   Handles cascade deletion of an account, removing associated chains and tokens.
 *
 * Inputs :
 *   accountHash
 *     Type        : string
 *     Description : Target account hash to delete.
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
 *   - infra.Info
 *   - InitDB
 *   - DeleteAccountCascade
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/
func HandleDeleteAccount(accountHash string) {
	if dbConn == nil {
		InitDB()
	}

	infra.Info(fmt.Sprintf("Initiating cascade deletion for account: %s", accountHash))
	DeleteAccountCascade(accountHash)
	fmt.Printf("Successfully deleted account %s and all related chains/tokens.\n", accountHash)
}

/******************************************************************************
 * Function Name : HandleDeleteChain
 *
 * Purpose :
 *   Deletes a specific chain linked to an account and all its related tokens.
 *
 * Inputs :
 *   accountHash, chainID
 *     Type        : string
 *     Description : Identifiers for the chain deletion.
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
 *   - infra.Info
 *   - InitDB
 *   - DeleteChainCascade
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/
func HandleDeleteChain(accountHash, chainID string) {
	if dbConn == nil {
		InitDB()
	}

	infra.Info(fmt.Sprintf("Initiating cascade deletion for chain %s on account %s", chainID, accountHash))
	DeleteChainCascade(accountHash, chainID)
	fmt.Printf("Successfully deleted chain ID %s for account %s\n", chainID, accountHash)
}

/******************************************************************************
 * Function Name : HandleDeleteToken
 *
 * Purpose :
 *   Deletes a single token. Resolves chainID to chainName dynamically for DB mapping.
 *
 * Inputs :
 *   accountHash, chainID, tokenName
 *     Type        : string
 *     Description : Identifiers for the token deletion.
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
 *   - infra.Info
 *   - InitDB
 *   - DeleteSingleToken
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   16
 ******************************************************************************/
func HandleDeleteToken(accountHash, chainID, tokenName string) {
	if dbConn == nil {
		InitDB()
	}

	chainName := GetChainNameByID(accountHash, chainID)
	if chainName == "" {
		infra.Error(fmt.Sprintf("Failed to resolve Chain ID %s. Chain might not exist.", chainID))
		return
	}

	infra.Info(fmt.Sprintf("Deleting token %s on chain %s for account %s", tokenName, chainName, accountHash))
	DeleteSingleToken(accountHash, chainName, tokenName)
	fmt.Printf("Successfully deleted token %s for account %s\n", tokenName, accountHash)
}