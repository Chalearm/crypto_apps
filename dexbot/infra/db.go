/******************************************************************************
 * File Name       : db.go
 * File Path       : infra/db.go
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:28 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:28 (UTC+7)
 * Description     : Centralized PostgreSQL database layer for Dexbot daemons.
 ******************************************************************************/
package infra

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

/******************************************************************************
 * Function Name : InitDB
 * Purpose       : Initializes the database connection pool.
 ******************************************************************************/
func InitDB() error {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASS")
	dbname := os.Getenv("DB_NAME")

	hosts := []string{host}
	if host == "" || host == "127.0.0.1" {
		hosts = []string{host, "db", "127.0.0.1"}
	}
	if host == "db" {
		hosts = []string{"db", "127.0.0.1"}
	}

	if user == "" || dbname == "" {
		user = "trader"
		password = "secret"
		dbname = "traderdb"
		if port == "" {
			port = "5432"
		}
	}

	var lastErr error
	for _, h := range hosts {
		if h == "" {
			continue
		}
		connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			h, port, user, password, dbname)

		var err error
		DB, err = sql.Open("postgres", connStr)
		if err != nil {
			lastErr = err
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = DB.PingContext(ctx)
		cancel()
		if err != nil {
			DB.Close()
			DB = nil
			lastErr = err
			continue
		}

		Info(fmt.Sprintf("Database connection established via %s:%s", h, port))
		DB.SetMaxOpenConns(10)
		DB.SetMaxIdleConns(5)
		DB.SetConnMaxLifetime(5 * time.Minute)
		CreateMarketTable()
		CreateUserTokensTable()
		Info("Database initialization complete.")
		return nil
	}
	if lastErr != nil {
		Error("InitDB all hosts failed: " + lastErr.Error())
	}
	return fmt.Errorf("database connection failed after trying %v", hosts)
}

/******************************************************************************
 * Function Name : CheckDBHealth
 * Purpose       : Checks the health of the database connection using ping.
 ******************************************************************************/
var CheckDBHealth = func() error {
	if DB == nil {
		Warn("Database connection is not initialized. Attempting to re-initialize.")
		return InitDB()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := DB.PingContext(ctx); err != nil {
		Error("Database health check failed (ping error): " + err.Error())
		return err
	}

	return nil
}

/******************************************************************************
 * Function Name : CreateMarketTable
 * Purpose       : Creates the market_prices table if it does not exist.
 ******************************************************************************/
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
        ts TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_market_prices_symbol_ts ON market_prices(symbol, ts DESC);
    CREATE INDEX IF NOT EXISTS idx_market_prices_chain ON market_prices(chain_id, ts DESC);
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
 * Purpose       : Lists all public schema tables in the database.
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
 * Purpose       : Queries table rows with limits and sorting for dashboard display.
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
	if sortOrder == "newest" || sortOrder == "oldest" {
		orderCol := ""
		for _, col := range []string{"id", "created_at", "ts", "timestamp"} {
			var exists bool
			checkQ := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='%s' AND column_name='%s')", tableName, col)
			if err := DB.QueryRow(checkQ).Scan(&exists); err == nil && exists {
				orderCol = col
				break
			}
		}
		if orderCol != "" {
			dir := "DESC"
			if sortOrder == "oldest" {
				dir = "ASC"
			}
			orderClause = fmt.Sprintf(" ORDER BY %s %s", orderCol, dir)
		}
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
 * Purpose       : Inserts a market price record into market_prices.
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
 * Purpose       : Fetches the last N market records for training feature-target pairs.
 ******************************************************************************/
func FetchTrainingData(symbol string, limit int) ([][]float64, []float64) {
	if DB == nil || limit <= 0 {
		return nil, nil
	}
	query := `SELECT price, volume, high_24h, low_24h FROM market_prices
		WHERE symbol = $1 ORDER BY ts DESC LIMIT $2`
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
		features = append(features, []float64{price, volume, high, low})
		targets = append(targets, price)
	}
	return features, targets
}