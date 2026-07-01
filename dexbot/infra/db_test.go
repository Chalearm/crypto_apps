/******************************************************************************
 * File Name       : db_test.go
 * File Path       : infra/db_test.go
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
 *   Unit tests for the DB module, ensuring database connection, health checks, and schema creation functionalities behave as expected under various conditions, including missing environment variables. Tes
 *
 * Responsibilities:
 *   - Implement core functionality for infra package.
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
 *   [Test Functions] Test suite: TestInitDB_MissingEnvVars, TestCheckDBHealth_UninitializedDB, TestCreateMarketTable
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
package infra_test

import (
	"context"
	"os"
	"testing"
	"time"

	"dexbot/infra"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// Helper to set environment variables for the tests
func setEnv(key, value string) {
	os.Setenv(key, value)
}

// Helper to clear environment variables after the tests
func clearEnv(key string) {
	os.Unsetenv(key)
}

/*
Function: TestInitDB_MissingEnvVars
Description:
  Tests that `InitDB` returns an error when essential database environment variables
  (`DB_HOST`, `DB_PORT`, `DB_USER`, `DB_NAME`) are not set, ensuring graceful handling.
Input:
  - t *testing.T: The testing framework's T object.
Output:
  - none
Lines: ~20
*/
func TestInitDB_MissingEnvVars(t *testing.T) {
	// Clear DB related environment variables
	clearEnv("DB_HOST")
	clearEnv("DB_PORT")
	clearEnv("DB_USER")
	clearEnv("DB_PASS")
	clearEnv("DB_NAME")

	// Ensure infra.DB is nil before test
	infra.DB = nil

	err := infra.InitDB()
	if err == nil {
		t.Errorf("Expected InitDB to return an error when environment variables are missing, but got nil")
	}

	if infra.DB != nil {
		t.Errorf("Expected infra.DB to be nil after failed InitDB, but it was not")
	}
}

/*
Function: TestCheckDBHealth_UninitializedDB
Description:
  Tests that `CheckDBHealth` attempts to re-initialize the database if `infra.DB` is `nil`
  and returns an error if re-initialization also fails due to missing environment variables.
Input:
  - t *testing.T: The testing framework's T object.
Output:
  - none
Lines: ~25
*/
func TestCheckDBHealth_UninitializedDB(t *testing.T) {
	// Clear DB related environment variables to ensure InitDB fails if called
	clearEnv("DB_HOST")
	clearEnv("DB_PORT")
	clearEnv("DB_USER")
	clearEnv("DB_PASS")
	clearEnv("DB_NAME")

	// Explicitly set infra.DB to nil to simulate uninitialized state
	infra.DB = nil

	err := infra.CheckDBHealth()

	if err == nil {
		t.Errorf("Expected CheckDBHealth to return an error for uninitialized DB with missing env vars, but got nil")
	}

	if infra.DB != nil {
		t.Errorf("Expected infra.DB to be nil after failed re-initialization, but it was not")
	}
}

/*
Function: TestCreateMarketTable
Description:
  Tests the `createMarketTable` function to ensure it creates the `market_prices` table
  without errors and can be called multiple times idempotently.
  Note: This test requires a running PostgreSQL instance with access configured by `config.env`.
Input:
  - t *testing.T: The testing framework's T object.
Output:
  - none
Lines: ~40
*/
func TestCreateMarketTable(t *testing.T) {
	// This test requires a running PostgreSQL instance and correct config.env setup.
	// For CI/local testing, ensure Docker Compose has the DB running.

	// Set environment variables for a dummy DB connection (adjust as per your Docker setup)
	setEnv("DB_HOST", "localhost")
	setEnv("DB_PORT", "5432")
	setEnv("DB_USER", "testuser")
	setEnv("DB_PASS", "testpassword")
	setEnv("DB_NAME", "testdb")

	// Attempt to initialize DB for the test
	err := infra.InitDB()
	if err != nil {
		t.Skipf("Skipping TestCreateMarketTable because DB Init failed: %v. Ensure PostgreSQL is running and config.env is correct.", err)
	}
	defer infra.DB.Close() // Ensure DB connection is closed after test

	// Drop table if it exists from previous runs
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = infra.DB.ExecContext(ctx, "DROP TABLE IF EXISTS market_prices")

	// Test 1: Create table for the first time
	infra.CreateMarketTable() // This function is not exported, so we need to access it differently for test.
	// If createMarketTable was infra.CreateMarketTable, we would call that instead.

	// Verify table exists (basic check)
	rows, err := infra.DB.QueryContext(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'market_prices')")
	if err != nil {
		t.Fatalf("Failed to query for table existence: %v", err)
	}
	defer rows.Close()

	exists := false
	if rows.Next() {
		rows.Scan(&exists)
	}
	if !exists {
		t.Errorf("Expected market_prices table to exist, but it does not")
	}

	// Test 2: Call again (should be idempotent and not error)
	infra.CreateMarketTable() // This should not cause an error or alter the existing table negatively.

	// Clean up environment variables
	clearEnv("DB_HOST")
	clearEnv("DB_PORT")
	clearEnv("DB_USER")
	clearEnv("DB_PASS")
	clearEnv("DB_NAME")
}
