/******************************************************************************
 * File Name       : db.go
 * File Path       : infra/db.go
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
 *   Centralized database layer for Dexbot daemons, utilizing PostgreSQL and configured via environment variables. It provides functionalities for initializing the database connection, checking its health,
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
package infra

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// DB is the global database connection pool accessible by other modules.
var DB *sql.DB

/*
Function: InitDB
Description:
  Initializes the global PostgreSQL database connection pool and ensures the `market_prices`
  table schema exists. It reads connection details from environment variables and attempts
  to establish and verify the connection.
Input:
  - none
Output:
  - error: Returns an error if the database connection or schema creation fails; otherwise, `nil`.
Lines: ~45
*/
func InitDB() error {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASS")
	dbname := os.Getenv("DB_NAME")

	if host == "" || port == "" || user == "" || dbname == "" {
		Warn("Database configuration (DB_HOST, DB_PORT, DB_USER, DB_NAME) is incomplete in config.env. Skipping DB initialization.")
		return fmt.Errorf("incomplete database configuration")
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		Error("Failed to open database connection: " + err.Error())
		return err
	}

	// Use a context with a timeout for the initial database ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = DB.PingContext(ctx); err != nil {
		Error("Failed to connect to database after opening (ping failed): " + err.Error())
		// Ensure DB connection is closed if ping fails
		DB.Close()
		DB = nil
		return err
	}

	Info("Database connection successfully established. Creating schema if not exists...")
	CreateMarketTable() // Ensure market_prices table exists

	Info("Database initialization complete.")
	return nil
}

/*
Function: CheckDBHealth
Description:
  Checks the health of the global database connection by performing a ping operation.
  This function is crucial for daemons to monitor the database's availability.
Input:
  - none
Output:
  - error: Returns `nil` if the database is healthy and reachable; otherwise, returns an error.
Lines: ~20
*/
var CheckDBHealth = func() error {
	if DB == nil {
		Warn("Database connection is not initialized. Attempting to re-initialize.")
		// Attempt to re-initialize the DB if it's nil
		// This could be problematic if InitDB relies on a running daemon setup.
		// For robust error handling, consider if re-initialization here is truly safe/desirable.
		return InitDB() // If InitDB fails, it will log its own error.
	}

	// Use a context with a timeout for the health check ping
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := DB.PingContext(ctx); err != nil {
		Error("Database health check failed (ping error): " + err.Error())
		return err
	}

	// Info("Database health OK.") // Too verbose for a periodic health check
	return nil
}

/*
Function: CreateMarketTable
Description:
  Creates the `market_prices` table in the connected database if it does not already exist.
  This table is used to store historical market data for various cryptocurrencies.
Input:
  - none
Output:
  - none
Lines: ~20
*/
func CreateMarketTable() {
	query := `
    CREATE TABLE IF NOT EXISTS market_prices (
        id SERIAL PRIMARY KEY,
        symbol TEXT NOT NULL,
        price DOUBLE PRECISION NOT NULL,
        volume DOUBLE PRECISION NOT NULL,
        high_24h DOUBLE PRECISION DEFAULT 0,
        low_24h DOUBLE PRECISION DEFAULT 0,
        market_cap DOUBLE PRECISION DEFAULT 0,
        open_price DOUBLE PRECISION DEFAULT 0,
        close_price DOUBLE PRECISION DEFAULT 0,
        change_pct DOUBLE PRECISION DEFAULT 0,
        base_asset TEXT NOT NULL DEFAULT '',
        quote_asset TEXT NOT NULL DEFAULT '',
        exchange TEXT NOT NULL DEFAULT 'BSC',
        chain_id TEXT NOT NULL DEFAULT '56',
        block_number BIGINT DEFAULT 0,
        tx_count INTEGER DEFAULT 0,
        source TEXT NOT NULL DEFAULT 'dexbot',
        timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_market_prices_symbol_ts ON market_prices(symbol, timestamp DESC);
    CREATE INDEX IF NOT EXISTS idx_market_prices_chain ON market_prices(chain_id, timestamp DESC);
    `

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := DB.ExecContext(ctx, query)
	if err != nil {
		Error("Failed to create market_prices table: " + err.Error())
		return
	}

	Info("'market_prices' table is ready or already existed.")
}

/******************************************************************************
 * Function Name : ListTables
 *
 * Purpose :
 *   Returns the names of all tables in the public schema of the connected
 *   database. Per myreq4.txt §87: enables the database browser dropdown.
 *
 * Inputs :
 *   None
 *
 * Return :
 *   Type        : []string
 *   Description : Table names in alphabetical order.
 *
 * Error Cases :
 *   - DB not initialized : returns empty slice
 *   - Query fails        : returns empty slice, logs error
 *
 * Dependencies :
 *   - DB (global *sql.DB)
 *
 * Complexity :
 *   Time  : O(1) — single system catalog query
 *   Space : O(n) where n = number of tables
 *
 * Number Of Lines : 25
 ******************************************************************************/
func ListTables() []string {
	if DB == nil {
		return nil
	}
	query := `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name`
	rows, err := DB.Query(query)
	if err != nil {
		Error("ListTables query failed: " + err.Error())
		return nil
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			tables = append(tables, name)
		}
	}
	return tables
}

/******************************************************************************
 * Function Name : QueryTableRows
 *
 * Purpose :
 *   Fetches up to `limit` rows from a given table with optional sort order.
 *   Per myreq4.txt §87: displays DB table data on the web dashboard.
 *
 * Inputs :
 *   tableName
 *     Type        : string
 *     Range       : valid public table name
 *     Description : Table to query.
 *
 *   limit
 *     Type        : int
 *     Range       : 1 ~ 25
 *     Description : Maximum number of rows to return.
 *
 *   sortOrder
 *     Type        : string
 *     Range       : "newest", "oldest", "default"
 *     Description : Sort direction (newest=DESC, oldest=ASC, default=none).
 *
 * Outputs :
 *   None (returned via []map[string]interface{})
 *
 * Return :
 *   Type        : ([]string, [][]string)
 *   Description : Column names and data rows (all values as strings).
 *
 * Dependencies :
 *   - DB (global *sql.DB)
 *
 * Complexity :
 *   Time  : O(n) where n = number of rows returned
 *   Space : O(n)
 *
 * Number Of Lines : 40
 ******************************************************************************/
func QueryTableRows(tableName string, limit int, sortOrder string) ([]string, [][]string) {
	if DB == nil {
		return nil, nil
	}
	if limit < 1 {
		limit = 5
	}
	if limit > 25 {
		limit = 25
	}

	orderClause := ""
	switch sortOrder {
	case "newest":
		orderClause = " ORDER BY id DESC"
	case "oldest":
		orderClause = " ORDER BY id ASC"
	default:
		// no explicit order — DB order
	}

	query := fmt.Sprintf("SELECT * FROM %s%s LIMIT %d", tableName, orderClause, limit)
	rows, err := DB.Query(query)
	if err != nil {
		Error("QueryTableRows failed for " + tableName + ": " + err.Error())
		return nil, nil
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		Error("QueryTableRows columns failed: " + err.Error())
		return nil, nil
	}

	var results [][]string
	for rows.Next() {
		vals := make([]interface{}, len(columns))
		valPtrs := make([]interface{}, len(columns))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}
		if err := rows.Scan(valPtrs...); err != nil {
			continue
		}
		row := make([]string, len(columns))
		for i, v := range vals {
			if v == nil {
				row[i] = "NULL"
			} else {
				row[i] = fmt.Sprintf("%v", v)
			}
		}
		results = append(results, row)
	}
	return columns, results
}

/******************************************************************************
 * Function Name : InsertMarketData
 *
 * Purpose :
 *   Inserts a market price record into market_prices. Used by the School
 *   daemon to record price data for training (§13, §91).
 *
 * Inputs :
 *   symbol  string  — Token ticker (e.g., "BNB")
 *   price   float64 — Current price
 *   volume  float64 — 24h volume
 *   high24  float64 — 24h high
 *   low24   float64 — 24h low
 *   base    string  — Base asset
 *   quote   string  — Quote asset
 *   chainID string  — Chain identifier
 *
 * Return :
 *   Type        : error
 *   Description : nil on success, error otherwise.
 *
 * Complexity : O(1), Number Of Lines : 25
 ******************************************************************************/
func InsertMarketData(symbol string, price, volume, high24, low24 float64, base, quote, chainID string) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	query := `INSERT INTO market_prices (symbol, price, volume, high_24h, low_24h, base_asset, quote_asset, chain_id, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'dexbot')`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := DB.ExecContext(ctx, query, symbol, price, volume, high24, low24, base, quote, chainID)
	if err != nil {
		Error("InsertMarketData failed for " + symbol + ": " + err.Error())
		return err
	}
	return nil
}

/******************************************************************************
 * Function Name : FetchTrainingData
 *
 * Purpose :
 *   Fetches the last N market price records for a given symbol as
 *   feature-target pairs for model training (§90, §91).
 *
 * Inputs :
 *   symbol  string — Token ticker
 *   limit   int    — Max records (e.g., 300)
 *
 * Return :
 *   Type        : (features [][]float64, targets []float64)
 *   Description : Feature matrix and target vector suitable for TrainingEngine.
 *
 * Complexity : O(n), Number Of Lines : 30
 ******************************************************************************/
func FetchTrainingData(symbol string, limit int) ([][]float64, []float64) {
	if DB == nil || limit <= 0 {
		return nil, nil
	}
	query := `SELECT price, volume, high_24h, low_24h FROM market_prices
		WHERE symbol = $1 ORDER BY timestamp DESC LIMIT $2`
	rows, err := DB.Query(query, symbol, limit)
	if err != nil {
		Error("FetchTrainingData query failed: " + err.Error())
		return nil, nil
	}
	defer rows.Close()

	var features [][]float64
	var targets []float64
	for rows.Next() {
		var price, volume, high, low float64
		if err := rows.Scan(&price, &volume, &high, &low); err != nil {
			continue
		}
		// Feature: [price, volume, high, low], Target: price (next-step forecast)
		features = append(features, []float64{price, volume, high, low})
		targets = append(targets, price)
	}
	return features, targets
}
