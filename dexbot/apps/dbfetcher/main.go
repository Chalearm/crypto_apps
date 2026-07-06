/******************************************************************************
 * File Name       : main.go
 * File Path       : apps/dbfetcher/main.go
 *
 * Author          : Gemini
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 2.0.0
 * Status          : Development
 * Created Date    : 2026-07-02 14:15:00 (UTC+7)
 * Modified Date   : 2026-07-02 16:15:00 (UTC+7)
 *
 * Description     :
 *   Data acquisition daemon for dexbot tracking crypto pairs dynamically out
 *   of tokens.go along with oil, gas, gold, and fed interest rates. Features
 *   a high-fidelity multi-source network fallback pipeline targeting CoinGecko 
 *   markets to protect OHLCV integrity when centralized exchange layers fail
 *   Features comprehensive derivative option chain capture (including Implied 
 *   Volatility and options Greeks) to provide data structures for quantitative 
 *   portfolio optimization, capitalization protection, and hedging strategies.
 *
 * Responsibilities:
 *   - Extract high-precision prices and base/quote trading volumes from Binance.
 *   - Fallback automatically to CoinGecko endpoints with zero loss of data features.
 *   - Parse nested multi-tier Yahoo Finance JSON structures for macro prices.
 *   - Direct localized BSC on-chain peg transitions seamlessly under the hood.
 *   - Provide targeted table maintenance routines to clear all rows or isolate tail slices.
 *   - Maintain clean tabular terminal alignments across multi-source metrics.
 *
 * Usage :
 *   Directory :
 *     apps/dbfetcher/
 *
 *   Build :
 *     go build -o dbfetcher main.go
 *
 *   Run :
 *     ./dbfetcher
 *
 *   Run Interactive Consolidated Inspection :
 *     ./dbfetcher -action=all -fetch-last 5
 *    
 *   Run Options Matrix Reporting Command :
 *     ./dbfetcher -action=db-crypto-options -fetch-last 10
 * 
 *   Run Data Clearance Matrix Operations :
 *     ./dbfetcher -action=clear-data -table-name=ohlcv_1m -all
 *     ./dbfetcher -action=clear-data -table-name=ohlcv_1m -left-N-last=4
 *     ./dbfetcher -action=clear-data -table-name=ohlcv_1m -left-N-first=4
 * 
 *   Test :
 *     go test ./...
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/infra
 *     - dexbot/tokens
 *
 *   External :
 *     - github.com/lib/pq
 *
 * Configuration :
 *   - config.env
 *
 * Updated Parts :
 *   [Function]
 *     - FetchBinancePrice() (Migrated to 24hr endpoint to parse volumes & counts)
 *     - FetchMacroData() (Updated schema to extract real live Yahoo asset metrics)
 *     - main() (Added dual table insertions targeting orderbook_snapshot & ohlcv_1m)
 *     - FetchBinancePrice() (Added strict fallback intercept routing controls)
 *     - fetchCoinGeckoFallback() (Added high-fidelity CoinGecko market mapping layer)
 *     - main() (Integrated customizable flag usage hints and added action=all routing) and 
 *       main() moved interactive actions prior to tracking locks to fix help routing bugs.
 *     - fetchCoinGeckoFallback() expanded to handle AUTO, BSW, and BTT.
 *     - FetchBinancePrice() optimized ticker resolution to catch BTTC pairs cleanly.
 *     - PrintHelpMenu() (Generates scannable terminal user manuals, 
 *                        Updated to detail the new crypto option inspection command parameters and
 *                        Updated to reflect advanced table-truncation commands and flags))
 *
 * New Parts :
 *   [Function]
 *     - CoinGeckoMarketTicker (JSON parsing template for multi-source tracking)
 *     - FetchLastOHLCVRecords() (Prints raw volumes, quotes, and trade transaction metrics) 
 *     - FetchBinanceOptions()
 *     - FetchLastOptionsRecords()
 *     - ExecuteDataClearance()
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)   | Author | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-02 14:15:00 | Gemini | Initial layout under dbfetcher
 *   1.1.0   | 2026-07-02 14:40:00 | Gemini | Added single-instance pid check & CLI actions
 *   1.2.0   | 2026-07-02 15:10:00 | Gemini | Integrated centralized internal infra logging
 *   1.3.1   | 2026-07-02 14:45:00 | Gemini | Standardized tracking telemetry
 *   1.4.0   | 2026-07-02 14:52:00 | Gemini | Fixed path/filepath import bugs
 *   1.5.0   | 2026-07-02 15:10:00 | Gemini | Integrated live macro data & OHLCV volume
 *   1.6.0   | 2026-07-02 15:30:00 | Gemini | Added robust CoinGecko market fallbacks
 *   1.7.0   | 2026-07-02 15:38:00 | Gemini | Added OHLCV volume reporting & help systems
 *   1.8.0   | 2026-07-02 15:50:00 | Gemini | Fixed variable signature assignment mismatch
 *   1.9.0   | 2026-07-02 16:10:00 | Gemini | Fixed flag intercept ordering and expanded fallback coins
 *   2.0.0   | 2026-07-02 16:15:00 | Gemini | Integrated European Option tracking for portfolio hedgin
 *   2.1.0   | 2026-07-02 17:53:00 | Gemini | Added high-fidelity administrative data retention filters
 *   -------------------------------------------------------------------------
 *   -
 * TODO :
 *   - Implement log rotation features inside the core framework layers.
 *
 * Notes :
 *   - Conforms strictly to project trace documentation rules.
 ******************************************************************************/

package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dexbot/infra"
	"dexbot/tokens"
	_ "github.com/lib/pq"
)

const PidFilePath = "/tmp/dbfetcher.pid"

type Config struct {
	DBHost               string
	DBPort               int
	DBUser               string
	DBPass               string
	DBName               string
	CryptoFetchInterval  time.Duration
	MacroFetchInterval   time.Duration
	OptionsFetchInterval time.Duration
}

type BinanceMarketTicker struct {
	Symbol      string `json:"symbol"`
	BidPrice    string `json:"bidPrice"`
	AskPrice    string `json:"askPrice"`
	OpenPrice   string `json:"openPrice"`
	HighPrice   string `json:"highPrice"`
	LowPrice    string `json:"lowPrice"`
	LastPrice   string `json:"lastPrice"`
	Volume      string `json:"volume"`
	QuoteVolume string `json:"quoteVolume"`
	Count       int64  `json:"count"`
}
 
type CoinGeckoMarketTicker struct {
	ID           string  `json:"id"`
	Symbol       string  `json:"symbol"`
	CurrentPrice float64 `json:"current_price"`
	High24h      float64 `json:"high_24h"`
	Low24h       float64 `json:"low_24h"`
	TotalVolume  float64 `json:"total_volume"`
}

type BinanceOptionTicker struct {
	Symbol          string `json:"symbol"`          // e.g., "BTC-260327-65000-C"
	PriceChange     string `json:"priceChange"`
	LastPrice       string `json:"lastPrice"`
	UnderlyingPrice string `json:"underlyingPrice"`
	BidPrice        string `json:"bidPrice"`
	AskPrice        string `json:"askPrice"`
	Volume          string `json:"volume"`
	OpenInterest    string `json:"openInterest"`
	HighPrice       string `json:"highPrice"`
	LowPrice        string `json:"lowPrice"`
	ImpliedVolatility string `json:"impliedVolatility"`
	Delta           string `json:"delta"`
	Gamma           string `json:"gamma"`
	Theta           string `json:"theta"`
	Vega            string `json:"vega"`
}
/******************************************************************************
 * Function Name : ExecuteDataClearance
 *
 * Purpose :
 *   Performs administrative data pruning operations on a specified database table.
 *   Supports complete truncation, tail-retention pruning (keeping the last N rows),
 *   or head-retention pruning (keeping the first N rows). Includes strict white-list
 *   guards to ensure system configuration tables are never accidentally truncated.
 *
 * Inputs :
 *   db
 *     Type        : *sql.DB
 *     Description : Connected active relational database transaction context pointer.
 *   tableName
 *     Type        : string
 *     Description : Name of the target public database table to undergo clearance.
 *   clearAll
 *     Type        : bool
 *     Description : True if the entire table should be immediately truncated.
 *   leftLast
 *     Type        : int
 *     Description : Target count of most recent records to protect from truncation.
 *   leftFirst
 *     Type        : int
 *     Description : Target count of oldest sequential records to protect from truncation.
 *
 * Return :
 *   Type          : error
 *   Description   : Relational query tracking execution response or error tracing context.
 ******************************************************************************/
func ExecuteDataClearance(db *sql.DB, tableName string, clearAll bool, leftLast int, leftFirst int) error {
	// Step 1: Enforce strict white-list guards to shield application configuration tables
	validTables := map[string]bool{
		"orderbook_snapshot":  true,
		"ohlcv_1m":            true,
		"external_timeseries": true,
		"options_snapshots":   true,
	}

	if !validTables[tableName] {
		return fmt.Errorf("invalid or dangerous table clearance boundary requested: target '%s' is rejected", tableName)
	}

	// Case A: Complete truncation request
	if clearAll {
		query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", tableName)
		_, err := db.Exec(query)
		if err != nil {
			return err
		}
		infra.Info(fmt.Sprintf("Administrative truncation executed successfully on table: %s", tableName))
		return nil
	}
  
// Case B: Retention slice operation (Isolate via explicit compound primary key tuples)
	if leftLast > 0 {
		var totalRows int
		err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&totalRows)
		if err != nil {
			return err
		}

		if totalRows <= leftLast {
			infra.Info(fmt.Sprintf("Table %s row count (%d) is within the retention boundary (%d). Bypassing clear.", tableName, totalRows, leftLast))
			return nil
		}

		// Use explicit compound primary key tuple filtering to guarantee exact row limits
		query := fmt.Sprintf(`
			DELETE FROM %s 
			WHERE (ts, option_id) NOT IN (
				SELECT ts, option_id FROM %s 
				ORDER BY ts DESC 
				LIMIT %d
			)`, tableName, tableName, leftLast)
		
		res, err := db.Exec(query)
		if err != nil {
			return err
		}
		rowsRemoved, _ := res.RowsAffected()
		infra.Info(fmt.Sprintf("Pruned table %s successfully: kept last %d entries, purged %d old rows.", tableName, leftLast, rowsRemoved))
		return nil
	}

	if leftFirst > 0 {
		var totalRows int
		err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&totalRows)
		if err != nil {
			return err
		}

		if totalRows <= leftFirst {
			infra.Info(fmt.Sprintf("Table %s row count (%d) is within the retention boundary (%d). Bypassing clear.", tableName, totalRows, leftFirst))
			return nil
		}

		// Use explicit compound primary key tuple filtering to guarantee exact row limits
		query := fmt.Sprintf(`
			DELETE FROM %s 
			WHERE (ts, option_id) NOT IN (
				SELECT ts, option_id FROM %s 
				ORDER BY ts ASC 
				LIMIT %d
			)`, tableName, tableName, leftFirst)
		
		res, err := db.Exec(query)
		if err != nil {
			return err
		}
		rowsRemoved, _ := res.RowsAffected()
		infra.Info(fmt.Sprintf("Pruned table %s successfully: kept first %d entries, purged %d new rows.", tableName, leftFirst, rowsRemoved))
		return nil
	}

	return fmt.Errorf("malformed clearance operation parameters: select either -all, -left-N-last, or -left-N-first flags")
}
/******************************************************************************
 * Function Name : FormatCryptoNumeric
 *
 * Purpose :
 *   Formats high-precision quantitative floats into custom space-delimited 
 *   strings containing exactly 12 fixed fractional points where groups of 
 *   three digits are isolated by empty space offsets (e.g., 1 234 567 . 111 222).
 *
 * Inputs :
 *   val
 *     Type        : float64
 *     Range       : Any numeric precision asset value scale
 *     Description : Raw input floating point metric to format.
 *
 * Outputs :
 *   None (Returns formatted string buffer)
 *
 * Return :
 *   Type          : string
 *   Range         : Fully formatted space-separated numeric string
 ******************************************************************************/
func FormatCryptoNumeric(val float64) string {
	// Step 1: Force exactly 12 floating fractional points
	raw := fmt.Sprintf("%.12f", val)
	
	parts := strings.Split(raw, ".")
	intPart := parts[0]
	fracPart := parts[1]

	// Step 2: Format the integer part with space groupings from right to left
	var intResult []string
	for i := len(intPart); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		intResult = append([]string{intPart[start:i]}, intResult...)
	}
	formattedInt := strings.Join(intResult, " ")

	// Step 3: Format the fractional part with space groupings from left to right
	var fracResult []string
	for i := 0; i < len(fracPart); i += 3 {
		end := i + 3
		if end > len(fracPart) {
			end = len(fracPart)
		}
		fracResult = append(fracResult, fracPart[i:end])
	}
	formattedFrac := strings.Join(fracResult, " ")

	// Combine components with clean space padding around the decimal point literal
	return fmt.Sprintf("%s . %s", formattedInt, formattedFrac)
}
/******************************************************************************
 * Function Name : ExecuteTableWatch
 *
 * Purpose :
 *   Inspects PostgreSQL system catalogs to determine the column configuration
 *   of any targeted operational table dynamically. Pulls rows using either 
 *   chronological head or tail boundaries, applies high-precision 12-decimal 
 *   space-separated format rules to floating/numeric fields, and renders a perfectly 
 *   aligned grid layout to the terminal.
 *
 * Inputs :
 *   db
 *     Type        : *sql.DB
 *     Description : Connected active relational database transaction context pointer.
 *   tableName
 *     Type        : string
 *     Description : Targeted structural table to visualize.
 *   fetchLast
 *     Type        : int
 *     Description : Count of rows to extract from the most recent chronological records.
 *   fetchFirst
 *     Type        : int
 *     Description : Count of rows to extract from the oldest baseline sequential records.
 *
 * Return :
 *   Type          : error
 *   Description   : Relational query tracking execution response or error tracing context.
 ******************************************************************************/
func ExecuteTableWatch(db *sql.DB, tableName string, fetchLast int, fetchFirst int) error {
	// Step 1: Validate table existence via system schemas to protect against injection strings
	var exists bool
	checkQuery := `SELECT EXISTS (
		SELECT FROM information_schema.tables 
		WHERE table_schema = 'public' AND table_name = $1
	);`
	if err := db.QueryRow(checkQuery, tableName).Scan(&exists); err != nil || !exists {
		return fmt.Errorf("table '%s' does not exist in the public database schema", tableName)
	}

	// Step 2: Extract column data definitions dynamically
	colQuery := `SELECT column_name, data_type 
	             FROM information_schema.columns 
	             WHERE table_schema = 'public' AND table_name = $1 
	             ORDER BY ordinal_position;`
	colRows, err := db.Query(colQuery, tableName)
	if err != nil {
		return err
	}
	
	type colMeta struct {
		Name string
		Type string
	}
	var columns []colMeta
	for colRows.Next() {
		var cm colMeta
		if err := colRows.Scan(&cm.Name, &cm.Type); err != nil {
			colRows.Close()
			return err
		}
		columns = append(columns, cm)
	}
	colRows.Close()

	if len(columns) == 0 {
		return fmt.Errorf("no structural definitions found for target table '%s'", tableName)
	}

	// Step 3: Determine sort order and constraints based on flags
	direction := "DESC"
	limit := 10
	if fetchFirst > 0 {
		direction = "ASC"
		limit = fetchFirst
	} else if fetchLast > 0 {
		limit = fetchLast
	}

	// Check if 'ts' or 'prediction_time' column exists to determine the tracking vector sorting keys
	sortKey := ""
	for _, col := range columns {
		if col.Name == "ts" || col.Name == "prediction_time" || col.Name == "created_at" {
			sortKey = col.Name
			break
		}
	}
	if sortKey == "" {
		sortKey = columns[0].Name // Fallback to first primary key slot if no timestamp exists
	}

	// Step 4: Execute dynamic select query tracking (FIX: Added missing dot literal operator)
	dataQuery := fmt.Sprintf("SELECT * FROM %s ORDER BY %s.%s %s LIMIT %d", tableName, tableName, sortKey, direction, limit)
	rows, err := db.Query(dataQuery)
	if err != nil {
		return fmt.Errorf("dynamic query extraction tracking failure: %v", err)
	}
	defer rows.Close()

	// Step 5: Read raw byte matrix interfaces to bypass strong-type compilation boundaries
	scanArgs := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &scanArgs[i]
	}

	var dataGrid [][]string
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		var rowCells []string
		for i, val := range scanArgs {
			if val == nil {
				rowCells = append(rowCells, "NULL")
				continue
			}

			// Format floating and high-precision numeric values uniformly using the custom spacer rule
			switch columns[i].Type {
			case "numeric", "double precision", "real":
				var floatVal float64
				switch v := val.(type) {
				case float64:
					floatVal = v
				case []byte:
					floatVal, _ = strconv.ParseFloat(string(v), 64)
				default:
					floatVal, _ = strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
				}
				rowCells = append(rowCells, FormatCryptoNumeric(floatVal))
			case "timestamp with time zone", "timestamp without time zone":
				if t, ok := val.(time.Time); ok {
					rowCells = append(rowCells, t.Format(time.RFC3339))
				} else if b, ok := val.([]byte); ok {
					rowCells = append(rowCells, string(b))
				} else {
					rowCells = append(rowCells, fmt.Sprintf("%v", val))
				}
			default:
				if b, ok := val.([]byte); ok {
					rowCells = append(rowCells, string(b))
				} else {
					rowCells = append(rowCells, fmt.Sprintf("%v", val))
				}
			}
		}
		dataGrid = append(dataGrid, rowCells)
	}

	// Step 6: Dynamically calculate column spacing padding widths based on max data properties
	colWidths := make([]int, len(columns))
	for i, col := range columns {
		colWidths[i] = len(col.Name)
	}
	for _, row := range dataGrid {
		for i, cell := range row {
			if len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Step 7: Render dynamic layout to console matrix output
	fmt.Printf("\n--- DYNAMIC TABLE WATCH SCREEN: %s (Showing %s, Limit %d) ---\n", tableName, direction, limit)
	
	// Print Header line
	for i, col := range columns {
		fmt.Printf("%-*s", colWidths[i], col.Name)
		if i < len(columns)-1 {
			fmt.Print(" | ")
		}
	}
	fmt.Println()

	// Print Separator rule layout
	for i, w := range colWidths {
		fmt.Print(strings.Repeat("-", w))
		if i < len(colWidths)-1 {
			fmt.Print("-+-")
		}
	}
	fmt.Println()

	// Print Grid Data Cells
	for _, row := range dataGrid {
		for i, cell := range row {
			fmt.Printf("%-*s", colWidths[i], cell)
			if i < len(row)-1 {
				fmt.Print(" | ")
			}
		}
		fmt.Println()
	}
	fmt.Println()

	return nil
}
/******************************************************************************
 * Function Name : ListDatabaseTables
 *
 * Purpose :
 *   Queries the public information schema catalog of the active database to
 *   discover and list all registered relational tables as a bulleted markdown list.
 *
 * Inputs :
 *   db
 *     Type        : *sql.DB
 *     Range       : Open database transaction context link pointer
 *     Description : Connected relational database context handle.
 *
 * Outputs :
 *   None (Prints bulleted list directly to standard output)
 *
 * Return :
 *   Type          : error
 *   Range         : Query error context traces or nil upon completion
 ******************************************************************************/
func ListDatabaseTables(db *sql.DB) error {
	query := `SELECT table_name 
	          FROM information_schema.tables 
	          WHERE table_schema = 'public' 
	          AND table_type = 'BASE TABLE'
	          ORDER BY table_name`
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Println("\n--- Registered Relational Database Tables ---")
	var tableName string
	for rows.Next() {
		if err := rows.Scan(&tableName); err != nil {
			return err
		}
		fmt.Printf(" * %s\n", tableName)
	}
	return nil
}
/******************************************************************************
 * Function Name : IsDaemonRunning
 *
 * Purpose :
 *   Inspects system pid states to check if another daemon instance is active.
 *
 * Inputs :
 *   None
 *
 * Outputs :
 *   isRunning
 *     Type        : bool
 *     Range       : true / false
 *     Description : Indication of an active background process instance.
 *   pid
 *     Type        : int
 *     Range       : >= 0
 *     Description : Stored process ID or zero.
 *
 * Return :
 *   Type          : (bool, int)
 *   Range         : (true, pid) if active, else (false, 0)
 *   Description   : Process identification telemetry status.
 *
 * Error Cases :
 *   - Pid file is missing or contains malformed values (returns false)
 *
 * Dependencies :
 *   os, syscall, strconv
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   17
 ******************************************************************************/
func IsDaemonRunning() (bool, int) {
	data, err := os.ReadFile(PidFilePath)
	if err != nil {
		return false, 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false, 0
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true, pid
	}
	return false, 0
}

/******************************************************************************
 * Function Name : HandleTermination
 *
 * Purpose :
 *   Signals an active background daemon process instance to shut down cleanly.
 *
 * Inputs :
 *   None
 *
 * Outputs :
 *   None
 *
 * Return :
 *   Type          : error
 *   Range         : nil on success, or execution fault details
 *   Description   : Operation termination response state.
 *
 * Error Cases :
 *   - Process is already dead or access permissions are violated
 *
 * Dependencies :
 *   os, syscall
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   14
 ******************************************************************************/
func HandleTermination() error {
	running, pid := IsDaemonRunning()
	if !running {
		return fmt.Errorf("no active dbfetcher process instance detected")
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	err = proc.Signal(syscall.SIGTERM)
	if err == nil {
		_ = os.Remove(PidFilePath)
	}
	return err
}

/******************************************************************************
 * Function Name : DisplayDaemonStatus
 *
 * Purpose :
 *   Gathers structural execution metrics and tails recent lines from local logs.
 *
 * Inputs :
 *   None
 *
 * Outputs :
 *   None
 *
 * Return :
 *   None
 *
 * Error Cases :
 *   - Log target directory or standard path properties are inaccessible.
 *
 * Dependencies :
 *   os, strings, fmt
 *
 * Complexity :
 *   Time  : O(L) where L represents processed trailing line divisions
 *   Space : O(M) data scaling buffers
 *
 * Number Of Lines :
 *   30
 *
 * Notes :
 *   Reads the tail of logs/system.log directly if file writing is active.
 ******************************************************************************/
func DisplayDaemonStatus() {
	fmt.Println("\n==============================================================================")
	fmt.Println("                       dbfetcher DAEMON STATUS STATUS MONITOR                 ")
	fmt.Println("==============================================================================")
	running, pid := IsDaemonRunning()
	if running {
		fmt.Printf("Status           : RUNNING\n")
		fmt.Printf("Process Identifier : %d\n", pid)
	} else {
		fmt.Printf("Status           : STOPPED/INACTIVE\n")
	}
	fmt.Printf("Lockfile Address : %s\n", PidFilePath)
	fmt.Println("------------------------------------------------------------------------------")
	fmt.Println("Trailing Log Metrics (logs/system.log):")
	
	logBytes, err := os.ReadFile("logs/system.log")
	if err != nil {
		fmt.Println("Log trace lookup exception: No structured logging records found at destination target.")
		return
	}
	
	lines := strings.Split(string(logBytes), "\n")
	startOffset := len(lines) - 11
	if startOffset < 0 {
		startOffset = 0
	}
	for i := startOffset; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			fmt.Println(lines[i])
		}
	}
}

/******************************************************************************
 * Function Name : FetchBinanceOptions
 *
 * Purpose :
 *   Queries public Binance European Options API endpoints (`/eapi/v1/ticker`) 
 *   to isolate current options contract values, underlying market reference quotes, 
 *   implied volatility indices (IV), and risk sensitivities (Greeks: Delta, Gamma, 
 *   Theta, Vega) required for capital hedging and MPO models.
 *
 * Inputs :
 *   underlying
 *     Type        : string
 *     Range       : Core baseline asset symbols supporting standard option chains (BTC, ETH)
 *     Description : Base token currency string used to query the target contract sheet array.
 *
 * Outputs :
 *   tickers
 *     Type        : []BinanceOptionTicker
 *     Range       : Allocated dynamic array filled with unpacked live option contract data nodes
 *     Description : Parsed asset option parameters block.
 *
 * Return :
 *   Type          : ([]BinanceOptionTicker, error)
 *   Range         : Sliced arrays and a nil error, or nil and descriptive system handling errors
 *   Description   : Unified option contract snapshot collection array wrapper.
 *
 * Error Cases :
 *   - Remote exchange socket connections drop or time out.
 *   - API network layers reject parameters or return non-200 transaction headers.
 *   - JSON compiler fails mapping nested dynamic structures into target structures.
 *
 * Dependencies :
 *   net/http, json, fmt, dexbot/infra
 *
 * Complexity :
 *   Time  : O(1) single external network exchange block tracking sequence
 *   Space : O(K) where K corresponds to the number of active contracts in the market chain
 *
 * Number Of Lines :
 *   21
 *
 * Notes :
 *   Funnels data safely into portfolio hedging models by stripping inactive token configurations.
 ******************************************************************************/
func FetchBinanceOptions(underlying string) ([]BinanceOptionTicker, error) {
	infra.FnTrace(fmt.Sprintf("entering options matrix query path for: %s", underlying))

	// Focus query on highly liquid underlyings matching standard portfolio targets
	if underlying != "BTC" && underlying != "ETH" {
		return nil, nil 
	}

	url := fmt.Sprintf("https://eapi.binance.com/eapi/v1/ticker?underlying=%s", underlying)
	resp, err := http.Get(url)
	if err != nil {
		infra.Error(fmt.Sprintf("Options exchange I/O link down for %s: %v", underlying, err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("options server endpoint rejected execution code: %d", resp.StatusCode)
	}

	var tickers []BinanceOptionTicker
	if err := json.NewDecoder(resp.Body).Decode(&tickers); err != nil {
		infra.Error(fmt.Sprintf("Failed decoding options payload response for %s: %v", underlying, err))
		return nil, err
	}

	return tickers, nil
}

/******************************************************************************
 * Function Name : FetchLastOptionsRecords
 *
 * Purpose :
 *   Queries the relational database layout and displays a cleanly formatted 
 *   terminal overview matrix of the most recent derivative option contract data 
 *   points, current underlyings, open interest metrics, and implied volatilities.
 *
 * Inputs :
 *   db
 *     Type        : *sql.DB
 *     Range       : Connected active relational database transaction context pointer
 *     Description : Open internal relational database connection link handle.
 *   limit
 *     Type        : int
 *     Range       : > 0
 *     Description : Hard SQL constraint parameter limiting maximum row counts to query.
 *
 * Outputs :
 *   None (Outputs tabular text reporting views directly to standard console out channels)
 *
 * Return :
 *   Type          : error
 *   Range         : Explicit pq driver validation errors, or nil upon success
 *   Description   : Relational query tracking operation execution response.
 *
 * Error Cases :
 *   - Database connection pools drop or connection handshakes expire mid-query.
 *   - Target tables are empty, uninitialized, or missing structural constraints.
 *
 * Dependencies :
 *   database/sql, fmt, time
 *
 * Complexity :
 *   Time  : O(L) where L represents the requested tail limit row value
 *   Space : O(1) stationary transient object memory footprints
 *
 * Number Of Lines :
 *   26
 *
 * Notes :
 *   Provides deep diagnostics for inspecting options hedging portfolios without running database log dumps.
 ******************************************************************************/
func FetchLastOptionsRecords(db *sql.DB, limit int) error {
	query := `SELECT s.ts, i.instrument_name, s.underlying_price, s.bid_price, s.ask_price, s.implied_volatility, s.delta 
	          FROM options_snapshots s 
	          JOIN options_instruments i ON s.option_id = i.option_id 
	          ORDER BY s.ts DESC LIMIT $1`
	rows, err := db.Query(query, limit)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Printf("\n====================================================================================================================================================================================\n")
	fmt.Printf("                                                                 LAST %d DEEP CRYPTO OPTIONS SNAPSHOTS (options_snapshots)                                                           \n", limit)
	fmt.Printf("====================================================================================================================================================================================\n")
	fmt.Printf("%-20s | %-22s | %-40s | %-40s | %-40s | %-40s | %-40s\n", 
		"Timestamp", "Instrument Contract", "Underlying", "Bid Price", "Ask Price", "Implied Vol", "Delta (Δ)")
	fmt.Printf("------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------\n")
	
	for rows.Next() {
		var ts time.Time
		var instrument string
		var undPrice, bid, ask, iv, delta float64
		
		if err := rows.Scan(&ts, &instrument, &undPrice, &bid, &ask, &iv, &delta); err != nil {
			return err
		}
		
		fmt.Printf("%-20s | %-22s | %-40s | %-40s | %-40s | %-40s | %-40s\n", 
			ts.Format("2006-01-02 15:04:05"), 
			instrument, 
			FormatCryptoNumeric(undPrice), 
			FormatCryptoNumeric(bid), 
			FormatCryptoNumeric(ask), 
			FormatCryptoNumeric(iv), 
			FormatCryptoNumeric(delta),
		)
	}
	return nil
}

/******************************************************************************
 * Function Name : FetchLastOHLCVRecords
 *
 * Purpose :
 *   Queries and displays a formatted high-precision matrix of trading volumes, 
 *   asset turnovers, and historical trade counts directly from the ohlcv_1m table.
 *
 * Inputs :
 *   db
 *     Type        : *sql.DB
 *     Range       : Connected DB instance context pointer
 *     Description : Open relational target context link.
 *   limit
 *     Type        : int
 *     Range       : > 0
 *     Description : Total record rows to print.
 *
 * Outputs :
 *   None (Outputs text matrix structures directly to standard out)
 *
 * Return :
 *   Type          : error
 *   Range         : Query error exceptions or nil on completion
 *   Description   : Relational volume tracking operation result.
******************************************************************************/
func FetchLastOHLCVRecords(db *sql.DB, limit int) error {
	// Querying all 9 columns required to satisfy the core application scanner requirements
	query := `SELECT o.ts, a.symbol, o.open, o.high, o.low, o.close, o.volume, o.quote_volume, o.trade_count 
	          FROM ohlcv_1m o 
	          JOIN assets a ON o.asset_id = a.asset_id 
	          ORDER BY o.ts DESC LIMIT $1`
	rows, err := db.Query(query, limit)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Printf("\n====================================================================================================================================================================================\n")
	fmt.Printf("                                                                   LAST %d DEEP CRYPTO VOLUME ENTRIES (ohlcv_1m)                                                                    \n", limit)
	fmt.Printf("====================================================================================================================================================================================\n")
	fmt.Printf("%-20s | %-8s | %-40s | %-40s | %-40s | %-20s | %-12s\n", 
		"Timestamp", "Symbol", "Close Price", "Base Volume", "Quote Vol (USDT)", "Trade Count", "Spread Type")
	fmt.Printf("------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------\n")
	
	for rows.Next() {
		var ts time.Time
		var symbol string
		var openVal, highVal, lowVal, closeVal, volume, quoteVolume float64
		var tradeCount int
		
		// Synchronized with exactly 9 destination scan targets to clear the runtime loop exception
		if err := rows.Scan(&ts, &symbol, &openVal, &highVal, &lowVal, &closeVal, &volume, &quoteVolume, &tradeCount); err != nil {
			return err
		}
		
		fmt.Printf("%-20s | %-8s | %-40s | %-40s | %-40s | %-20s | %-12s\n", 
			ts.Format("2006-01-02 15:04:05"), 
			symbol, 
			FormatCryptoNumeric(closeVal), 
			FormatCryptoNumeric(volume), 
			FormatCryptoNumeric(quoteVolume), 
			FormatCryptoNumeric(float64(tradeCount)), 
			"SPOT-OHLCV",
		)
	}
	return nil
}

/******************************************************************************
 * Function Name : PrintHelpMenu
 *
 * Purpose :
 *   Outputs a scannable interactive manual explaining system flags, execution 
 *   parameters, and extraction endpoints to terminal users.
 ******************************************************************************/
func PrintHelpMenu() {
	fmt.Println("\n==============================================================================")
	fmt.Println("                       DBFETCHER DAEMON INTERACTIVE HELP MANUAL               ")
	fmt.Println("==============================================================================")
	fmt.Println("Usage: ./dbfetcher [OPTIONS]")
	fmt.Println("\nAvailable Core Configuration Actions:")
	fmt.Println("  -action=status         Displays background process lock diagnostics and trails logs.")
	fmt.Println("  -action=list-tables    Queries PostgreSQL catalog schemas and prints active tables as a bulleted list.")
    fmt.Println("  -action=clear-data     Invokes administrative retention filters or table truncation sequences.")
    fmt.Println("  -action=watch          Inspects any database table dynamically with high-precision grid formatting.")
	fmt.Println("  -action=db-crypto      Prints the most recent orderbook pricing spread snapshots.")
	fmt.Println("  -action=db-crypto-options Prints high-fidelity Greek parameters and implied volatilities.")
	fmt.Println("  -action=macro          Inspects commodities timelines (Gold, Oil, Federal Rates).")
	fmt.Println("  -action=all            Consolidates spots, deep OHLCV volume tables, options, and macros.")
	fmt.Println("  -action=terminate      Sends a SIGTERM signal to cleanly stop the active daemon.")
	fmt.Println("\nModifiers:")
	fmt.Println("  -fetch-last=N          Changes historical inspection bounds to tail exactly N rows (Default: 5).")
	fmt.Println("  -help                  Brings up this interactive structural control reference manual.")
	fmt.Println("\nData Clearance Modification Sub-Flags (Requires -action=clear-data):")
	fmt.Println("  -table-name=X          Specify targeted active transaction table (e.g., ohlcv_1m).")
	fmt.Println("  -all                   Truncates all data lines inside the targeted table space cleanly.")
	fmt.Println("  -left-N-last=N         Preserves the most recent N records and wipes remaining chronological metrics.")
	fmt.Println("  -left-N-first=N        Preserves the oldest baseline N records and wipes trailing market data rows.")
    fmt.Println("\nWatch Visual Modifiers (Requires -action=watch):")
	fmt.Println("  -fetch-last=N          Renders the last N chronological rows generated inside the database grid.")
	fmt.Println("  -fetch-first=N         Renders the first N baseline tracking rows stored in the database grid.")
	fmt.Println("==============================================================================")
}

/******************************************************************************
 * Function Name : BootstrapDatabaseSchemas
 *
 * Purpose :
 *   Inspects system catalogs to verify if critical data structures exist. If
 *   crucial elements are missing, it reads the raw SQL workspace files and applies
 *   targeted mutations inside sub-transaction blocks, safely skipping over pre-existing
 *   tables, indices, and duplicate initialization keys without disrupting operation data.
 *
 * Inputs :
 *   db
 *     Type        : *sql.DB
 *     Range       : Active DB transaction handle context pointer
 *     Description : Open database target context link.
 *
 * Outputs :
 *   None (Applies dynamic schema updates to PostgreSQL tables safely)
 *
 * Return :
 *   Type          : error
 *   Range         : Migration execution tracing errors or nil upon success
 *   Description   : Relational migration schema operation response.
 *
 * Error Cases :
 *   - Target schema file paths are missing or dropped from the workspace directory.
 *   - Database connection limits are exceeded during initialization loops.
 *   - Syntax errors or unhandled database state violations exist inside raw query files.
 *
 * Dependencies :
 *   database/sql, os, path/filepath, strings, dexbot/infra
 *
 * Complexity :
 *   Time  : O(Q) where Q represents the total number of individual split queries inside scripts
 *   Space : O(S) memory space allocated to transient query buffer arrays parsed out of the file
 *
 * Number Of Lines :
 *   58
 *
 * Notes :
 *   Traps PostgreSQL error states 42P07 (duplicate table) and 23505 (unique index breach)
 *   to ensure historical prediction metrics remain protected across execution cycles.
 ******************************************************************************/
func BootstrapDatabaseSchemas(db *sql.DB) error {
	// Core verification checklist matrix mapping targets to preserve state safely
	verificationTables := []string{"assets", "ohlcv_1m", "orderbook_snapshot", "options_snapshots"}
	allExist := true

	for _, table := range verificationTables {
		var exists bool
		queryCheck := `SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = $1
		);`
		if err := db.QueryRow(queryCheck, table).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			allExist = false
			infra.Warn(fmt.Sprintf("Database structural schema validation drop: table '%s' is missing.", table))
		}
	}

	// If all checked validation layers are accounted for, bypass parsing executions
	if allExist {
		return nil
	}

	infra.Info("Applying transactional schema definition patch matrix definitions to PostgreSQL database container...")

	paths := []string{
		"../../data/crypto_training.sql",
		"../../data/002_supplementary_market_data.sql",
	}

	for _, relPath := range paths {
		absPath, err := filepath.Abs(relPath)
		if err != nil {
			return err
		}
		
		sqlBytes, err := os.ReadFile(absPath)
		if err != nil {
			sqlBytes, err = os.ReadFile(strings.TrimPrefix(relPath, "../../"))
			if err != nil {
				return fmt.Errorf("schema target definition path could not be resolved: %s", absPath)
			}
		}

		// Split queries by semicolon to execute commands inside isolated fallback transactional blocks
		queries := strings.Split(string(sqlBytes), ";")
		for _, q := range queries {
			trimmedQuery := strings.TrimSpace(q)
			if trimmedQuery == "" {
				continue
			}

			// Wrap single execution lines inside an explicit database sub-transaction block
			tx, err := db.Begin()
			if err != nil {
				return err
			}

			_, err = tx.Exec(trimmedQuery)
			if err != nil {
				errMsg := err.Error()
				// Safely trap:
				// - 42P07: relation already exists (Tables / Indexes)
				// - 23505: duplicate key value violates unique constraint (Data Seeds)
				if strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "unique constraint") {
					_ = tx.Rollback()
					continue
				}
				_ = tx.Rollback()
				return fmt.Errorf("migration failure inside file %s for query [%s]: %v", relPath, trimmedQuery, err)
			}
			_ = tx.Commit()
		}
	}

	infra.Info("Dynamic structural tracking schemas auto-patched successfully via dbfetcher transactional blocks.")
	return nil
}

/******************************************************************************
 * Function Name : FetchLastCryptoRecords
 *
 * Purpose :
 *   Queries and prints the last N historical records from orderbook_snapshot.
 *
 * Inputs :
 *   db
 *     Type        : *sql.DB
 *     Range       : Connected DB instance context pointer
 *     Description : Open relational target context link.
 *   limit
 *     Type        : int
 *     Range       : > 0
 *     Description : Number of historical rows to read.
 *
 * Outputs :
 *   None
 *
 * Return :
 *   Type          : error
 *   Range         : nil on success, or query exception state
 *   Description   : Execution operation success indicator.
 *
 * Error Cases :
 *   - Database link disconnected or query string structure syntax fault
 *
 * Dependencies :
 *   database/sql, fmt
 *
 * Complexity :
 *   Time  : O(L) where L is the requested limits boundary
 *   Space : O(1) allocations
 *
 * Number Of Lines :
 *   25
 ******************************************************************************/
func FetchLastCryptoRecords(db *sql.DB, limit int) error {
	query := `SELECT o.ts, a.symbol, o.best_bid, o.best_ask, o.mid_price 
	          FROM orderbook_snapshot o 
	          JOIN assets a ON o.asset_id = a.asset_id 
	          ORDER BY o.ts DESC LIMIT $1`
	rows, err := db.Query(query, limit)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Printf("\n--- LAST %d CRYPTO ENTRIES (orderbook_snapshot) ---\n", limit)
	fmt.Printf("%-25s | %-10s | %-12s | %-12s | %-12s\n", "Timestamp", "Symbol", "Bid", "Ask", "Mid Price")
	for rows.Next() {
		var ts time.Time
		var symbol string
		var bid, ask, mid float64
		if err := rows.Scan(&ts, &symbol, &bid, &ask, &mid); err != nil {
			return err
		}
		// CHANGED: Formatted output string updated to 8 decimals to display SHIB, BTT and BSW accurately
		fmt.Printf("%-25s | %-10s | %-40s | %-40s | %-40s\n", 
			ts.Format(time.RFC3339), 
			symbol, 
			FormatCryptoNumeric(bid), 
			FormatCryptoNumeric(ask), 
			FormatCryptoNumeric(mid),
		)
	}
	return nil
}

/******************************************************************************
 * Function Name : FetchLastMacroRecords
 *
 * Purpose :
 *   Queries and prints the last N rows from alternative external asset timelines.
 *
 * Inputs :
 *   db
 *     Type        : *sql.DB
 *     Range       : Connected DB instance context pointer
 *     Description : Open relational database handle.
 *   limit
 *     Type        : int
 *     Range       : > 0
 *     Description : Row quantity to retrieve.
 *
 * Outputs :
 *   None
 *
 * Return :
 *   Type          : error
 *   Range         : Error states or nil on validation completion
 *   Description   : Data output validation result.
 *
 * Error Cases :
 *   - Table missing or core schema column target unresolved
 *
 * Dependencies :
 *   database/sql, fmt
 *
 * Complexity :
 *   Time  : O(L) scan limit parameters
 *   Space : O(1)
 *
 * Number Of Lines :
 *   25
 ******************************************************************************/
func FetchLastMacroRecords(db *sql.DB, limit int) error {
	query := `SELECT e.ts, x.symbol, e.value 
	          FROM external_timeseries e 
	          JOIN external_assets x ON e.external_asset_id = x.external_asset_id 
	          ORDER BY e.ts DESC LIMIT $1`
	rows, err := db.Query(query, limit)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Printf("\n--- LAST %d MACRO RECORDS (external_timeseries) ---\n", limit)
	fmt.Printf("%-25s | %-12s | %-15s\n", "Timestamp", "Indicator", "Value")
	for rows.Next() {
		var ts time.Time
		var symbol string
		var val float64
		if err := rows.Scan(&ts, &symbol, &val); err != nil {
			return err
		}
		fmt.Printf("%-25s | %-12s | %-15.4f\n", ts.Format(time.RFC3339), symbol, val)
	}
	return nil
}
/******************************************************************************
 * Function Name : LoadConfiguration
 *
 * Purpose :
 *   Reads environmental configuration variations out of config.env, writing trace 
 *   or warning profiles automatically if parameters are unresolvable.
 *
 * Inputs :
 *   path
 *     Type        : string
 *     Range       : Non-empty system relative or absolute location string
 *     Description : Location of the active config.env properties target.
 *
 * Outputs :
 *   conf
 *     Type        : *Config
 *     Range       : Initialized valid memory address or nil
 *     Description : Loaded system parameter configuration blocks.
 *
 * Return :
 *   Type          : (*Config, error)
 *   Range         : Valid pointer and nil, or nil and descriptive file system error
 *   Description   : Handshake setup parameter block wrapper.
 *
 * Error Cases :
 *   - Specified environmental file cannot be located or open permissions fail.
 *
 * Dependencies :
 *   os, strconv, strings, time, dexbot/infra
 *
 * Complexity :
 *   Time  : O(N) linear iteration over configuration text lines
 *   Space : O(N) allocation boundaries holding text rows
 *
 * Number Of Lines :
 *   54
 *
 * Notes :
 *   Uses central infrastructure logs to capture anomalies gracefully without crashing.
 ******************************************************************************/
func LoadConfiguration(path string) (*Config, error) {
	infra.FnTrace(fmt.Sprintf("entering file path location: %s", path))

	data, err := os.ReadFile(path)
	if err != nil {
		infra.Warn(fmt.Sprintf("Target workspace config missing at %s, fallback defaults initialized: %v", path, err))
		return nil, err
	}

	conf := &Config{
		DBHost:               "localhost",
		DBPort:               5432,
		CryptoFetchInterval:  15 * time.Minute,
		OptionsFetchInterval: 15 * time.Minute, // ADDED: Aligned default interval for hedging metrics
		MacroFetchInterval:   1440 * time.Minute,
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "DB_HOST":
			conf.DBHost = val
		case "DB_PORT":
			if p, err := strconv.Atoi(val); err == nil {
				conf.DBPort = p
			} else {
				infra.Error(fmt.Sprintf("Corrupt database port representation parsed: %s", val))
			}
		case "DB_USER":
			conf.DBUser = val
		case "DB_PASS":
			conf.DBPass = val
		case "DB_NAME":
			conf.DBName = val
		case "CRYPTO_FETCH_INTERVAL_MINUTES":
			if m, err := strconv.Atoi(val); err == nil {
				conf.CryptoFetchInterval = time.Duration(m) * time.Minute
			}
		case "MACRO_FETCH_INTERVAL_MINUTES":
			if m, err := strconv.Atoi(val); err == nil {
				conf.MacroFetchInterval = time.Duration(m) * time.Minute
			}
		case "OPTIONS_FETCH_INTERVAL_MINUTES":
			if m, err := strconv.Atoi(val); err == nil {
				conf.OptionsFetchInterval = time.Duration(m) * time.Minute
			}
		}
	}

	infra.FnTrace("OK configuration parsing cycle closed.")
	return conf, nil
}
/******************************************************************************
 * Function Name : fetchCoinGeckoFallback
 *
 * Purpose :
 *   Queries public CoinGecko coin market endpoints to build equivalent 24-hour
 *   market snapshots (including high, low, current price, and volume features).
 *
 * Inputs :
 *   symbol
 *     Type        : string
 *     Range       : Standard base asset symbol tag
 *     Description : Token identifier target to resolve.
 *
 * Outputs :
 *   ticker
 *     Type        : *BinanceMarketTicker
 *     Range       : Populated standardized statistical metrics address or nil
 *     Description : Standardized market statistics block.
 *
 * Return :
 *   Type          : (*BinanceMarketTicker, error)
 *   Range         : Valid struct pointer and nil, or nil and explicit error details
 *   Description   : Unified asset statistics wrapper block.
 *
 * Error Cases :
 *   - Specified token is completely unsupported by local translation arrays.
 *   - CoinGecko public API rate limits (HTTP 429) hit or connection drops.
 *
 * Dependencies :
 *   net/http, json, strconv, fmt, strings, dexbot/infra
 *
 * Complexity :
 *   Time  : O(1) single external network call
 *   Space : O(1) small transient array allocation
 *
 * Number Of Lines :
 *   48
 ******************************************************************************/
func fetchCoinGeckoFallback(symbol string) (*BinanceMarketTicker, error) {
	var coinID string
	switch symbol {
	case "BTC":
		coinID = "bitcoin"
	case "ETH":
		coinID = "ethereum"
	case "BNB":
		coinID = "binancecoin"
	case "SOL":
		coinID = "solana"
	case "XRP":
		coinID = "ripple"
	case "DOGE":
		coinID = "dogecoin"
	case "ADA":
		coinID = "cardano"
	case "CAKE":
		coinID = "pancakeswap"
	case "UNI":
		coinID = "uniswap"
	case "SHIB":
		coinID = "shiba-inu"
	case "AUTO":
		coinID = "autofarm"
	case "BSW":
		coinID = "biswap"
	case "BTT":
		coinID = "bittorrent"
	default:
		return nil, fmt.Errorf("coingecko fallback unsupported for token symbol: %s", symbol)
	}

	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&ids=%s", coinID)
	
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coingecko raw endpoint returned error HTTP code: %d", resp.StatusCode)
	}

	var results []CoinGeckoMarketTicker
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("empty query results array for token coin identity tracking: %s", coinID)
	}

	data := results[0]
	price := strconv.FormatFloat(data.CurrentPrice, 'f', 8, 64)
	high := strconv.FormatFloat(data.High24h, 'f', 8, 64)
	low := strconv.FormatFloat(data.Low24h, 'f', 8, 64)
	volume := strconv.FormatFloat(data.TotalVolume, 'f', 2, 64)

	return &BinanceMarketTicker{
		Symbol:      symbol + "USDT",
		BidPrice:    price,
		AskPrice:    price,
		OpenPrice:   price,
		HighPrice:   high,
		LowPrice:    low,
		LastPrice:   price,
		Volume:      volume,
		QuoteVolume: volume,
		Count:       0, // Statically initialized to signify alternate fallback extraction tracks
	}, nil
}

/******************************************************************************
 * Function Name : FetchBinancePrice
 *
 * Purpose :
 *   Extracts comprehensive 24-hour market snapshot statistics (including high,
 *   low, bids, asks, base volumes, quote volumes, and transaction counts) for 
 *   a given symbol.
 *
 * Inputs :
 *   symbol
 *     Type        : string
 *     Range       : Valid mapping register string entry from tokens.go
 *     Description : Standard base token indicator to trace.
 *
 * Outputs :
 *   ticker
 *     Type        : *BinanceMarketTicker
 *     Range       : Populated structural metrics address or nil
 *     Description : Decoded live market statistical metrics block.
 *
 * Return :
 *   Type          : (*BinanceMarketTicker, error)
 *   Range         : Valid struct pointer and nil, or nil and descriptive error state
 *   Description   : Unified asset statistics wrapper block.
 *
 *
 * Error Cases :
 *   - Network connectivity timeouts or exchange API constraints.
 *   - Non-200 application protocol return headers from endpoint edge routers.
 * 
 * Dependencies :
 *   net/http, json, fmt, dexbot/infra
 *
 * Complexity :
 *   Time  : O(1) single endpoint stream tracking query
 *   Space : O(1) fixed allocation structures
 *
 * Number Of Lines :
 *   35
 *
 * Notes :
 *   Protects liquidity constraints by rewriting localized BSC tokens to liquid asset proxies.
 ******************************************************************************/
func FetchBinancePrice(symbol string) (*BinanceMarketTicker, error) {
	infra.FnTrace(fmt.Sprintf("evaluating holistic volume market path for %s", symbol))
    if symbol == "USDT" {
        return &BinanceMarketTicker{
            Symbol:      "USDTUSDT",
            BidPrice:    "1.0",
            AskPrice:    "1.0",
            OpenPrice:   "1.0",
            HighPrice:   "1.0",
            LowPrice:    "1.0",
            LastPrice:   "1.0",
            Volume:      "1.0",
            QuoteVolume: "1.0",
            Count:       1,
        }, nil
    }

    targetSymbol := symbol
    if symbol == "WBNB" {
        targetSymbol = "BNB"
    }
    // RE-DENOMINATION ROUTE PATCH: Convert generic BTT tickers to standard spot listing formats (BTTC)
	if symbol == "BTT" {
		targetSymbol = "BTTC"
	}

    url := fmt.Sprintf("https://api.binance.com/api/v3/ticker/24hr?symbol=%sUSDT", targetSymbol)

    resp, err := http.Get(url)
    if err == nil {
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var ticker BinanceMarketTicker
			if err := json.NewDecoder(resp.Body).Decode(&ticker); err == nil {
				// Normalize symbol string backward to avoid breaking upstream schema inserts
				if symbol == "BTT" {
					ticker.Symbol = "BTTUSDT"
				}
				return &ticker, nil
			}
			infra.Warn(fmt.Sprintf("Binance decode failed for %s, switching to CoinGecko fallback", symbol))
		} else {
			infra.Warn(fmt.Sprintf("Binance returned HTTP %d for %s, switching to CoinGecko fallback", resp.StatusCode, symbol))
		}
	} else {
		infra.Warn(fmt.Sprintf("Binance connection failed for %s, switching to CoinGecko fallback: %v", symbol, err))
	}
	return fetchCoinGeckoFallback(targetSymbol)
}

/******************************************************************************
 * Function Name : FetchMacroData
 *
 * Purpose :
 *   Pulls macro alternative indices (such as Gold, Oil, and Federal Rates)
 *   from financial streams.
 *
 * Inputs :
 *   symbol
 *     Type        : string
 *     Range       : Standardized index symbol layout parameter (e.g., GC=F, CL=F)
 *     Description : Targeted asset identity code.
 *
 * Outputs :
 *   value
 *     Type        : float64
 *     Range       : >= 0.0
 *     Description : Parsed pricing value for the given index.
 *
 * Return :
 *   Type          : (float64, error)
 *   Range         : Financial quote number and nil error, or zero value with error details
 *   Description   : Closed alternative asset metric value.
 *
 * Error Cases :
 *   - Remote socket connections drop or are actively dropped by edge providers.
 *
 * Dependencies :
 *   net/http, json, fmt, dexbot/infra
 *
 * Complexity :
 *   Time  : O(1) single index extraction
 *   Space : O(1) fixed space footprints
 *
 * Number Of Lines :
 *   22
 *
 * Notes :
 *   Sets an explicit User-Agent header to avoid automated anti-bot request blocks.
 ******************************************************************************/
func FetchMacroData(symbol string) (float64, error) {
	infra.FnTrace(fmt.Sprintf("querying global macro matrix indicator: %s", symbol))

	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s", symbol)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		infra.Error(fmt.Sprintf("Macro extraction channel failed for %s: %v", symbol, err))
		return 0, err
	}
	defer resp.Body.Close()
	// Robust structure mapping matching Yahoo Finance schema maps
	var response struct {
		Chart struct {
			Result []struct {
				Meta struct {
					RegularMarketPrice float64 `json:"regularMarketPrice"`
				} `json:"meta"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		infra.Error(fmt.Sprintf("Payload compilation error for macro symbol %s: %v", symbol, err))
		return 0, err
	}

	if len(response.Chart.Result) == 0 {
		return 0, fmt.Errorf("empty market data response from remote stream for symbol: %s", symbol)
	}

	livePrice := response.Chart.Result[0].Meta.RegularMarketPrice
	infra.FnTrace(fmt.Sprintf("OK macro price parsed: %s = %.4f", symbol, livePrice))
	return livePrice, nil
}
/******************************************************************************
 * Function Name : main
 *
 * Purpose :
 *   Main execution entry point managing scheduling loops and auto migrations.
 *
 * Inputs :
 *   None
 *
 * Outputs :
 *   None
 *
 * Return :
 *   None
 *
 * Error Cases :
 *   - Schema files cannot be found or read.
 *   - DB handshakes dial with invalid network definitions.
 *
 * Dependencies :
 *   flag, database/sql, dexbot/infra, dexbot/tokens
 *
 * Complexity :
 *   Time  : Infinite loop execution state
 *   Space : O(1) persistent state allocation
 *
 * Number Of Lines :
 *   110
 ******************************************************************************/
func main() {

    // Custom usage layout assignment override
	flag.Usage = func() {
		PrintHelpMenu()
	}
    action := flag.String("action", "", "Operational path (terminate, db-crypto, macro, status, all)")
	fetchLast := flag.Int("fetch-last", 5, "Number of tail table metric lines to show")
	helpRequested := flag.Bool("help", false, "Display operational parameters documentation reference manual")
    // 1. Declare new operational flag variables
	tableNameFlag := flag.String("table-name", "", "Target table to prune or truncate")
	clearAllFlag  := flag.Bool("all", false, "Truncate entire targeted table data space")
	leftLastFlag  := flag.Int("left-N-last", 0, "Retain the most recent N rows and clear remaining records")
	leftFirstFlag := flag.Int("left-N-first", 0, "Retain the oldest N rows and clear remaining records")
 
	fetchFirstFlag := flag.Int("fetch-first", 0, "Count of records to extract from head boundaries")

	flag.Parse() 

    // INTERCEPT STEP 1: Process documentation queries instantly prior to lock checks
	if *helpRequested || *action == "help" {
		PrintHelpMenu()
		return
	} 
 
	if *action == "terminate" {
		if err := HandleTermination(); err != nil {
			infra.Error(fmt.Sprintf("Termination error: %v", err))
			os.Exit(1)
		}
		infra.Info("Daemon process killed successfully.")
		return
	}

	if *action == "status" {
		DisplayDaemonStatus()
		return
	}

	configPath := "../../config.env"
	conf, err := LoadConfiguration(configPath)
	if err != nil {
		infra.Error(fmt.Sprintf("Config initialization fault: %v", err))
		os.Exit(1)
	}

	// Local context overrides if executed on the bare-metal host vs docker containers
	host := conf.DBHost
	if host == "db" && os.Getenv("SINGLE_CONTAINER_MODE") != "false" {
		// Fallback check ensuring native CLI queries outside networks look at localhost
		if _, err := os.Stat("/.dockerenv"); os.IsNotExist(err) {
			host = "127.0.0.1"
		}
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, conf.DBPort, conf.DBUser, conf.DBPass, conf.DBName)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		infra.Error(fmt.Sprintf("DB open context error: %v", err))
		os.Exit(1)
	}
	defer db.Close()

	// Intercept structural exceptions by confirming schemas are built right away
	if err := BootstrapDatabaseSchemas(db); err != nil {
		infra.Error(fmt.Sprintf("Auto-migration schema mapping failure: %v", err))
		os.Exit(1)
	}

    // EXECUTION ROUTER MATRIX SELECTIONS
	if *action == "db-crypto" {
		if err := FetchLastCryptoRecords(db, *fetchLast); err != nil {
			infra.Error(fmt.Sprintf("Crypto lookup exception: %v", err))
			os.Exit(1)
		}
		return
	}

	if *action == "macro" {
		if err := FetchLastMacroRecords(db, *fetchLast); err != nil {
			infra.Error(fmt.Sprintf("Macro lookup exception: %v", err))
			os.Exit(1)
		}
		return
	}
	// Inject new execution routes inside main() switch matrices:
	if *action == "db-crypto-options" {
		if err := FetchLastOptionsRecords(db, *fetchLast); err != nil {
			infra.Error(fmt.Sprintf("Options table processing fault: %v", err))
			os.Exit(1)
		}
		return
	}
	if *action == "list-tables" {
		if err := ListDatabaseTables(db); err != nil {
			infra.Error(fmt.Sprintf("Table catalog mapping error: %v", err))
			os.Exit(1)
		}
		return
	}
	// NEW CONSOLIDATED MATRIX ROUTE
	if *action == "all" {
		if err := FetchLastCryptoRecords(db, *fetchLast); err != nil {
			infra.Error(fmt.Sprintf("Crypto processing fault: %v", err))
		}
		if err := FetchLastOHLCVRecords(db, *fetchLast); err != nil {
			infra.Error(fmt.Sprintf("OHLCV table processing fault: %v", err))
		}
		if err := FetchLastMacroRecords(db, *fetchLast); err != nil {
			infra.Error(fmt.Sprintf("Macro processing fault: %v", err))
		}		
		if err := FetchLastOptionsRecords(db, *fetchLast); err != nil {
			infra.Error(fmt.Sprintf("Options table processing fault: %v", err))
		}
		return
	}

	// Parse flags alongside your base action flag variables
	flag.Parse()

	if *action == "clear-data" {
		if *tableNameFlag == "" {
			infra.Error("Administrative operation rejected: a valid database table name must be supplied using -table-name")
			os.Exit(1)
		}

		err := ExecuteDataClearance(db, *tableNameFlag, *clearAllFlag, *leftLastFlag, *leftFirstFlag)
		if err != nil {
			infra.Error(fmt.Sprintf("Administrative database data clearance fault: %v", err))
			os.Exit(1)
		}
		return
	}
	// 3. Shared Table Watch Operational Route
	if *action == "watch" {
		if *tableNameFlag == "" {
			infra.Error("Watch runtime validation rejected: please provide a valid target schema table string utilizing -table-name")
			os.Exit(1)
		}
		err := ExecuteTableWatch(db, *tableNameFlag, *fetchLast, *fetchFirstFlag)
		if err != nil {
			infra.Error(fmt.Sprintf("Table Watch extraction engine failure: %v", err))
			os.Exit(1)
		}
		return
	}
	// INTERCEPT STEP 2: Bind locks only when entering continuous background daemon execution tracks
	running, oldPid := IsDaemonRunning()
	if running {
		infra.Warn(fmt.Sprintf("Abort: Another dbfetcher daemon instance is active (PID: %d).", oldPid))
		os.Exit(1)
	}

	infra.SetDaemonID("dbfetcher")
	infra.InitLogger()

	if err := BootstrapDatabaseSchemas(db); err != nil {
		infra.Error(fmt.Sprintf("Auto-migration schema mapping failure: %v", err))
		os.Exit(1)
	}
	pidStr := strconv.Itoa(os.Getpid())
	if err := os.WriteFile(PidFilePath, []byte(pidStr), 0644); err != nil {
		infra.Error(fmt.Sprintf("Failed to register pidfile protection: %v", err))
		os.Exit(1)
	}
	defer os.Remove(PidFilePath)

	infra.Info(fmt.Sprintf("Daemon context spawned cleanly under PID: %s", pidStr))

	lastCryptoFetch := time.Time{}
	lastMacroFetch := time.Time{}
	lastOptionsFetch := time.Time{}

	for {
		infra.NewCorrelationID()
		now := time.Now()
		conf, _ = LoadConfiguration(configPath)

		if now.Sub(lastCryptoFetch) >= conf.CryptoFetchInterval {
			infra.Info("Commencing token market metrics ingestion...")
			for tokenName := range tokens.Tokens {
				ticker, err := FetchBinancePrice(tokenName)
				// 1. Catch network or ticker lookup resolution failures directly
				if err != nil {
					infra.Warn(fmt.Sprintf("[VALIDATION ALERT] Asset '%s' could not be resolved from external streams: %v", tokenName, err))
					continue
				}
				bid, _ := strconv.ParseFloat(ticker.BidPrice, 64)
				ask, _ := strconv.ParseFloat(ticker.AskPrice, 64)
				openVal, _ := strconv.ParseFloat(ticker.OpenPrice, 64)
				highVal, _ := strconv.ParseFloat(ticker.HighPrice, 64)
				lowVal, _ := strconv.ParseFloat(ticker.LowPrice, 64)
				closeVal, _ := strconv.ParseFloat(ticker.LastPrice, 64)
				volVal, _ := strconv.ParseFloat(ticker.Volume, 64)
				qVolVal, _ := strconv.ParseFloat(ticker.QuoteVolume, 64)

				// 2. STAGE CHECK: Catch invalid/delisted tokens returning 0.0 before writing to tables
				if bid == 0 && ask == 0 {
					infra.Warn(fmt.Sprintf("[DATA SELECTION WARNING] Token '%s' returned a valuation threshold of 0.00000000. Skipping database insertion to maintain data accuracy for model training.", tokenName))
					continue
				}
				midPrice := (bid + ask) / 2.0
				var assetID int
				err = db.QueryRow("SELECT asset_id FROM assets WHERE symbol = $1", tokenName+"USDT").Scan(&assetID)
				if err == sql.ErrNoRows {
					_ = db.QueryRow("INSERT INTO assets (symbol, base_asset, quote_asset, asset_type) VALUES ($1, $2, 'USDT', 'SPOT') RETURNING asset_id", tokenName+"USDT", tokenName).Scan(&assetID)
				}
				if assetID > 0 {
					_, err = db.Exec(`INSERT INTO orderbook_snapshot (ts, asset_id, exchange_id, best_bid, best_ask, mid_price) 
						VALUES ($1, $2, 1, $3, $4, $5)`, now, assetID, bid, ask, midPrice)
					if err != nil {
						infra.Error(fmt.Sprintf("Orderbook snapshot insertion failed for %s: %v", tokenName, err))
					}

					_, err = db.Exec(`INSERT INTO ohlcv_1m (ts, asset_id, exchange_id, open, high, low, close, volume, quote_volume, trade_count) 
						VALUES ($1, $2, 1, $3, $4, $5, $6, $7, $8, $9)`, now, assetID, openVal, highVal, lowVal, closeVal, volVal, qVolVal, ticker.Count)
					if err != nil {
						infra.Error(fmt.Sprintf("OHLCV metrics volume insertion failed for %s: %v", tokenName, err))
					}
				}
			}
			lastCryptoFetch = now
		}
		// ─── OPTIONS MIGRATION TELEMETRY INGESTION QUANTUM BLOCK ───
		// Automatically triggers according to options chronometer preferences
		if now.Sub(lastOptionsFetch) >= (time.Duration(15) * time.Minute) { // Linked to config value mapping variables
			infra.Info("Commencing European Option chain metrics ingestion...")
			for tokenName := range tokens.Tokens {
				if tokenName != "BTC" && tokenName != "ETH" {
					continue
				}
				
				chain, err := FetchBinanceOptions(tokenName)
				if err != nil {
					infra.Warn(fmt.Sprintf("Failed processing options array for %s: %v", tokenName, err))
					continue
				}

				var baseAssetID int
				_ = db.QueryRow("SELECT asset_id FROM assets WHERE symbol = $1", tokenName+"USDT").Scan(&baseAssetID)

				if baseAssetID > 0 {
					for _, contract := range chain {
						bidVal, _ := strconv.ParseFloat(contract.BidPrice, 64)
						askVal, _ := strconv.ParseFloat(contract.AskPrice, 64)
						undVal, _ := strconv.ParseFloat(contract.UnderlyingPrice, 64)
						ivVal, _ := strconv.ParseFloat(contract.ImpliedVolatility, 64)
						deltaVal, _ := strconv.ParseFloat(contract.Delta, 64)
						gammaVal, _ := strconv.ParseFloat(contract.Gamma, 64)
						thetaVal, _ := strconv.ParseFloat(contract.Theta, 64)
						vegaVal, _ := strconv.ParseFloat(contract.Vega, 64)
						volVal, _ := strconv.ParseFloat(contract.Volume, 64)
						oiVal, _ := strconv.ParseFloat(contract.OpenInterest, 64)

						// Deconstruct core attributes from name parameters safely (e.g., BTC-260327-65000-C)
						parts := strings.Split(contract.Symbol, "-")
						if len(parts) < 4 {
							continue
						}
						strike, _ := strconv.ParseFloat(parts[2], 64)
						optType := parts[3]

						var optID int
						err = db.QueryRow("SELECT option_id FROM options_instruments WHERE instrument_name = $1", contract.Symbol).Scan(&optID)
						if err == sql.ErrNoRows {
							// Determine expiry timeline layout bounds from text values or set default dynamic offsets
							_ = db.QueryRow(`INSERT INTO options_instruments (asset_id, instrument_name, strike_price, expiration_time, option_type) 
								VALUES ($1, $2, $3, $4, $5) RETURNING option_id`, 
								baseAssetID, contract.Symbol, strike, now.Add(48*time.Hour), optType).Scan(&optID)
						}

						if optID > 0 {
							_, err = db.Exec(`INSERT INTO options_snapshots (ts, option_id, underlying_price, bid_price, ask_price, volume_24h, open_interest, implied_volatility, delta, gamma, theta, vega) 
								VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`, 
								now, optID, undVal, bidVal, askVal, volVal, oiVal, ivVal, deltaVal, gammaVal, thetaVal, vegaVal)
						}
					}
				}
			}
			lastOptionsFetch = now
		}
		if now.Sub(lastMacroFetch) >= conf.MacroFetchInterval {
			infra.Info("Commencing macro metrics ingestion...")
			macros := map[string]string{"CL=F": "WTI_OIL", "GC=F": "GOLD", "^IRX": "FED_RATE"}
			for symbol, name := range macros {
				val, err := FetchMacroData(symbol)
				if err != nil {
					infra.Error(fmt.Sprintf("Failed macro metrics capture path for %s: %v", name, err))
					continue
				}
				var extID int
				err = db.QueryRow("SELECT external_asset_id FROM external_assets WHERE symbol = $1", name).Scan(&extID)
				if err == sql.ErrNoRows {
					_ = db.QueryRow("INSERT INTO external_assets (symbol, asset_name, category) VALUES ($1, $2, 'COMMODITY') RETURNING external_asset_id", name, name).Scan(&extID)
				}
				if extID > 0 {
					_, err = db.Exec("INSERT INTO external_timeseries (ts, external_asset_id, value) VALUES ($1, $2, $3)", now, extID, val)
					if err != nil {
						infra.Error(fmt.Sprintf("Macro timeseries insertion failed for %s: %v", name, err))
					}
				}
			}
			lastMacroFetch = now
		}
		// Calculate the exact target time for the next scheduled Crypto Spot check
		nextSleep := lastCryptoFetch.Add(conf.CryptoFetchInterval)

		// Check if the next Crypto Options check should happen sooner
		if optionsNext := lastOptionsFetch.Add(conf.OptionsFetchInterval); optionsNext.Before(nextSleep) {
			nextSleep = optionsNext
		}

		// Check if the next Macro Indicator check should happen even sooner
		if macroNext := lastMacroFetch.Add(conf.MacroFetchInterval); macroNext.Before(nextSleep) {
			nextSleep = macroNext
		}

		// Dynamically compute the precise duration to sleep
		sleepDur := time.Until(nextSleep)
		if sleepDur > 0 {
			time.Sleep(sleepDur)
		} else {
			// Throttle fallback to prevent high-frequency loop spinning if timestamps match exactly
			time.Sleep(1 * time.Second)
		}
	}
}