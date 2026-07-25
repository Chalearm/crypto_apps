/*

./main -asset=bitcoin -ticker=BTC-USD -start=2026-07-01 -end=2026-07-21 -interval=daily -transformVolume -cyclicalTime
./main -asset=ethereum -ticker=ETH-USD -start=2026-07-01 -end=2026-07-21 -interval=daily -transformVolume -cyclicalTime
./main -asset=solana -ticker=SOL-USD -start=2026-07-01 -end=2026-07-21 -interval=daily -transformVolume -cyclicalTime
./main -asset=binance -ticker=BNB-USD -start=2026-07-01 -end=2026-07-21 -interval=daily -transformVolume -cyclicalTime
./main -asset=uniswap -ticker=UNI7083-USD -start=2022-07-01 -end=2026-06-30 -interval=daily -transformVolume -cyclicalTime
 
# validate
./main -asset=gold -ticker=GC=F -start=2026-07-01 -end=2026-07-21 -interval=daily -transformVolume -weekend -cyclicalTime
./main -asset=oil -ticker=BZ=F -start=2026-07-01 -end=2026-07-21 -interval=daily -transformVolume -weekend -cyclicalTime
./main -asset=spy -ticker=SPY -start=2026-07-01 -end=2026-07-21 -interval=daily -transformVolume -weekend -cyclicalTime
./main -asset=fed_rate -ticker=^IRX -start=2026-07-01 -end=2026-07-21 -interval=daily -transformVolume -weekend -cyclicalTime

*/

package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type MacroConfig struct {
	Asset           string
	Ticker          string
	StartDate       string
	EndDate         string
	Interval        string
	TransformVolume bool
	CyclicalTime    bool
	WeekendFill     bool
}

type BinanceKline []interface{}

type BinanceBookTicker struct {
	Symbol   string `json:"symbol"`
	BidPrice string `json:"bidPrice"`
	BidQty   string `json:"bidQty"`
	AskPrice string `json:"askPrice"`
	AskQty   string `json:"askQty"`
}

type YahooResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close  []float64 `json:"close"`
					Volume []int64   `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"chart"`
}

type DataRow struct {
	Date         string
	Open         float64 // Retained: Core requirement for structural crypto momentum
	Close        float64
	Volume       int64
	TakerVolume  int64
	MakerVolume  int64
	PriceLogRet  float64 // ln(Close_t / Close_t-1)
	VolLogRet    float64
	IntradayBody float64 // New Feature: ln(Close_t / Open_t) (Crypto Only)
	HourSin, HourCos, MinSin, MinCos float64
	DayWkSin, DayWkCos, DayYrSin, DayYrCos float64
}
func main() {
	flag.Usage = func() { showHelpMenu() }

	config, helpRequested := parseFlags()
	if helpRequested {
		showHelpMenu()
		return
	}

	startTime, err := time.Parse("2006-01-02", config.StartDate)
	if err != nil {
		fmt.Printf("❌ Invalid start date format: %v. Use YYYY-MM-DD\n", err)
		os.Exit(1)
	}
	endTime, err := time.Parse("2006-01-02", config.EndDate)
	if err != nil {
		fmt.Printf("❌ Invalid end date format: %v. Use YYYY-MM-DD\n", err)
		os.Exit(1)
	}

	fmt.Printf("🌐 Fetching Asset Portfolio: %s (%s) | Timeframe Target: %s...\n", config.Asset, config.Ticker, config.Interval)

	var rawRows []DataRow
	isHighFreq := isCryptoHighFrequency(config.Interval)

	if isHighFreq {
		rawRows, err = fetchFromBinance(config.Ticker, config.Interval, startTime, endTime)
	} else {
		rawRows, err = fetchFromYahooFallback(config.Ticker, config.Interval, startTime, endTime)
	}

	if err != nil {
		fmt.Printf("💥 Critical Extraction Failure: %v\n", err)
		os.Exit(1)
	}

	if len(rawRows) == 0 {
		fmt.Println("⚠️ No historical data arrays returned within specified boundaries.")
		return
	}

	// Route the complete multi-chunk array down to our conditional calendar filter
	var processedRows []DataRow
	if config.WeekendFill && config.Interval == "1d" && !isHighFreq {
		processedRows = generateContinuousCalendarFill(startTime, endTime, rawRows)
	} else {
		processedRows = rawRows
	}

	// Core Mathematical Features & Multi-Timeframe Cyclical Engineering
	// Processes ALL paginated rows inside processedRows seamlessly
	for i := 0; i < len(processedRows); i++ {
		var t time.Time
		if isHighFreq {
			t, _ = time.Parse("2006-01-02 15:04", processedRows[i].Date)
		} else {
			t, _ = time.Parse("2006-01-02", processedRows[i].Date)
		}

		if i > 0 {
			// 1. Price Log Return calculation
			if processedRows[i-1].Close > 0 && processedRows[i].Close > 0 {
				processedRows[i].PriceLogRet = math.Log(processedRows[i].Close / processedRows[i-1].Close)
			}
			// 2. Scale-Invariant Volume Log Transformation
			if config.TransformVolume {
				processedRows[i].VolLogRet = math.Log(float64(processedRows[i].Volume+1) / float64(processedRows[i-1].Volume+1))
			}
			// UPGRADE: Calculate Intraday Body Log Ratio for High-Frequency Crypto
			if isHighFreq && processedRows[i].Open > 0 && processedRows[i].Close > 0 {
				processedRows[i].IntradayBody = math.Log(processedRows[i].Close / processedRows[i].Open)
			}
		}

		// MULTI-SCALE CYCLICAL TIME ENGINE
		if config.CyclicalTime {
			// Intraday Hour Node (0 - 23)
			hourVal := float64(t.Hour())
			processedRows[i].HourSin = math.Sin(2 * math.Pi * hourVal / 24.0)
			processedRows[i].HourCos = math.Cos(2 * math.Pi * hourVal / 24.0)

			// Intraday Minute Node (0 - 59)
			minVal := float64(t.Minute())
			processedRows[i].MinSin = math.Sin(2 * math.Pi * minVal / 60.0)
			processedRows[i].MinCos = math.Cos(2 * math.Pi * minVal / 60.0)

			// Weekly Macro Node (0 - 6)
			weekday := float64(t.Weekday())
			processedRows[i].DayWkSin = math.Sin(2 * math.Pi * weekday / 7.0)
			processedRows[i].DayWkCos = math.Cos(2 * math.Pi * weekday / 7.0)

			// Yearly Macro Seasonality Node (1 - 366)
			yearday := float64(t.YearDay())
			processedRows[i].DayYrSin = math.Sin(2 * math.Pi * yearday / 365.25)
			processedRows[i].DayYrCos = math.Cos(2 * math.Pi * yearday / 365.25)
		}
	}

	// String format target destination outputs matching your requested setup
	rawFilename := fmt.Sprintf("%s_%s_%s_%s.csv", config.Asset, config.StartDate, config.EndDate, config.Interval)
	returnFilename := fmt.Sprintf("%s_%s_%s_%s_transformed.csv", config.Asset, config.StartDate, config.EndDate, config.Interval)

	// CRITICAL FIX: Explicitly pass the full processedRows array to both disk writer modules
	writeRawCSV(rawFilename, processedRows, isHighFreq)
	writeTransformedCSV(returnFilename, processedRows, config.TransformVolume, config.CyclicalTime, isHighFreq)
}

func parseFlags() (MacroConfig, bool) {
	asset := flag.String("asset", "gold", "Name tag used for naming output CSV targets")
	ticker := flag.String("ticker", "GC=F", "Valid ticker token symbol pairing tracking target asset")
	start := flag.String("start", "2023-01-01", "Chronological search start point in YYYY-MM-DD format")
	end := flag.String("end", "2026-06-30", "Chronological search cutoff point in YYYY-MM-DD format")
	interval := flag.String("interval", "1d", "Resolution partitioning step across multi-model frequencies")
	transVol := flag.Bool("transformVolume", false, "Convert raw volumes to stationary Log Change features")
	cycTime := flag.Bool("cyclicalTime", false, "Append engineered calendar sin/cos coordinates")
	weekend := flag.Bool("weekend", false, "Forward-fill weekend matrix data blocks using the previous Friday close")
	help := flag.Bool("help", false, "Renders the complete operational lookup menu context manual")

	flag.Parse()

	normInt := strings.ToLower(*interval)
	if normInt == "1month" || normInt == "monthly" { normInt = "1mo" }
	if normInt == "daily" { normInt = "1d" }

	return MacroConfig{
		Asset:           strings.ToLower(*asset),
		Ticker:          strings.ToUpper(*ticker),
		StartDate:       *start,
		EndDate:         *end,
		Interval:        normInt,
		TransformVolume: *transVol,
		CyclicalTime:    *cycTime,
		WeekendFill:     *weekend,
	}, *help
}

func isCryptoHighFrequency(interval string) bool {
	return interval == "15m" || interval == "1h" || interval == "4h"
}

func fetchFromBinance(symbol, interval string, start, end time.Time) ([]DataRow, error) {
	fmt.Println("⚡ Initiating Paginated Binance Klines Spot API Engine...")
	
	var allRecords []DataRow
	currentStartMs := start.UnixNano() / int64(time.Millisecond)
	endMs := end.UnixNano() / int64(time.Millisecond)

	client := &http.Client{Timeout: 15 * time.Second}

	for {
		// Break the loop if our sliding start window passes the target end date
		if currentStartMs >= endMs {
			break
		}

		url := fmt.Sprintf("https://api.binance.com/api/v3/klines?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=1000", 
			symbol, interval, currentStartMs, endMs)

		resp, err := client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("network error during pagination slice: %v", err)
		}
		
		// Handle rate limiting gracefully if you pull massive history
		if resp.StatusCode == http.StatusTooManyRequests {
			fmt.Println("⏳ Rate limit hit (429). Sleeping for 5 seconds...")
			resp.Body.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("exchange rejected query frame, status: %d", resp.StatusCode)
		}

		var klines []BinanceKline
		if err := json.NewDecoder(resp.Body).Decode(&klines); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		// If Binance returns no more data, we have reached the end of available history
		if len(klines) == 0 {
			break
		}

		var lastLogTimeMs int64
		for _, k := range klines {
			if len(k) < 10 { continue }
			openTimeRaw := int64(k[0].(float64))
			lastLogTimeMs = openTimeRaw // Track the latest timestamp in this chunk

			openPrice, _ := strconv.ParseFloat(k[1].(string), 64)
			closePrice, _ := strconv.ParseFloat(k[4].(string), 64)
			totalVolume, _ := strconv.ParseFloat(k[5].(string), 64)
			takerBuyVolume, _ := strconv.ParseFloat(k[9].(string), 64)

			timeAnchor := time.Unix(0, openTimeRaw*int64(time.Millisecond)).UTC()

			allRecords = append(allRecords, DataRow{
				Date:        timeAnchor.Format("2006-01-02 15:04"),
				Open:        openPrice,
				Close:       closePrice,
				Volume:      int64(totalVolume),
				TakerVolume: int64(takerBuyVolume),
				MakerVolume: int64(totalVolume) - int64(takerBuyVolume),
			})
		}

		fmt.Printf("📥 Fetched chunk up to: %s (Total records: %d)\n", 
			allRecords[len(allRecords)-1].Date, len(allRecords))

		// CRITICAL FOR PAGINATION: Set the next startTime to 1 millisecond AFTER the last received candle
		currentStartMs = lastLogTimeMs + 1
		
		// Polite cooling delay to avoid getting your IP banned by Binance's firewall
		time.Sleep(50 * time.Millisecond)
	}

	return allRecords, nil
}

func fetchFromYahooFallback(ticker, interval string, start, end time.Time) ([]DataRow, error) {
	yahooInterval := "1d"
	if interval == "1mo" { yahooInterval = "1mo" }
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?period1=%d&period2=%d&interval=%s", ticker, start.Unix(), end.Unix(), yahooInterval)

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	var yData YahooResponse
	if err := json.NewDecoder(resp.Body).Decode(&yData); err != nil { return nil, err }
	if len(yData.Chart.Result) == 0 { return nil, fmt.Errorf("empty fallback payload returned") }
	res := yData.Chart.Result[0]

	var records []DataRow
	for i, t := range res.Timestamp {
		if i >= len(res.Indicators.Quote[0].Close) || res.Indicators.Quote[0].Close[i] == 0 { continue }
		var vol int64 = 0
		if len(res.Indicators.Quote[0].Volume) > i { vol = res.Indicators.Quote[0].Volume[i] }
		records = append(records, DataRow{
			Date:  time.Unix(t, 0).UTC().Format("2006-01-02"),
			Close: res.Indicators.Quote[0].Close[i],
			Volume: vol,
		})
	}
	return records, nil
}

func generateContinuousCalendarFill(start, end time.Time, yahooData []DataRow) []DataRow {
	yahooMap := make(map[string]DataRow)
	for _, row := range yahooData { yahooMap[row.Date] = row }
	var completedTimeline []DataRow
	var lastKnownClose float64 = 0.0
	var lastKnownVolume int64 = 0

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dateKey := d.Format("2006-01-02")
		val, exists := yahooMap[dateKey]
		if exists {
			lastKnownClose = val.Close
			lastKnownVolume = val.Volume
			completedTimeline = append(completedTimeline, val)
		} else {
			completedTimeline = append(completedTimeline, DataRow{
				Date: dateKey, Close: lastKnownClose, Volume: lastKnownVolume,
			})
		}
	}
	return completedTimeline
}

func writeRawCSV(filename string, rows []DataRow, isHighFreq bool) {
	file, _ := os.Create(filename)
	defer file.Close()
	w := csv.NewWriter(file)
	defer w.Flush()

	var headers []string
	if isHighFreq {
		// Crypto High-Freq needs Open for body calculations, plus order flow metrics
		headers = []string{"timestamp", "open", "close", "volume", "taker_buy_volume", "maker_sell_volume"}
	} else {
		// Macro baseline only tracks what it actually has: Close and Volume
		headers = []string{"timestamp", "close", "volume"}
	}
	w.Write(headers)

	for _, r := range rows {
		var record []string
		
		if isHighFreq {
			record = []string{
				r.Date,
				strconv.FormatFloat(r.Open, 'f', 12, 64),  // Expanded to 12 digits
				strconv.FormatFloat(r.Close, 'f', 12, 64), // Expanded to 12 digits
				strconv.FormatInt(r.Volume, 10),
				strconv.FormatInt(r.TakerVolume, 10),
				strconv.FormatInt(r.MakerVolume, 10),
			}
		} else {
			record = []string{
				r.Date,
				strconv.FormatFloat(r.Close, 'f', 12, 64), // Expanded to 12 digits
				strconv.FormatInt(r.Volume, 10),
			}
		}
		w.Write(record)
	}
	fmt.Printf("💾 Raw Data Matrix successfully saved: %s\n", filename)
}
func writeTransformedCSV(filename string, rows []DataRow, incVol, incTime, isHighFreq bool) {
	file, _ := os.Create(filename)
	defer file.Close()
	w := csv.NewWriter(file)
	defer w.Flush()

	// Streamlined ML Header: spread_log_change is removed
	headers := []string{"timestamp", "price_log_return"}
	if incVol { 
		headers = append(headers, "volume_log_change") 
	}
	if incTime {
		// In headers setup:
		if isHighFreq {
			headers = append(headers, "intraday_body_log_change", "hour_sin", "hour_cos", "min_sin", "min_cos")
		}
		headers = append(headers, "day_wk_sin", "day_wk_cos", "day_yr_sin", "day_yr_cos")
	}

	w.Write(headers)

	for i := 1; i < len(rows); i++ {
		record := []string{rows[i].Date, strconv.FormatFloat(rows[i].PriceLogRet, 'f', 12, 64)}
		if incVol { 
			record = append(record, strconv.FormatFloat(rows[i].VolLogRet, 'f', 12, 64)) 
		}
		if incTime {
			if isHighFreq {
				record = append(record,
					strconv.FormatFloat(rows[i].IntradayBody, 'f', 12, 64),
					strconv.FormatFloat(rows[i].HourSin, 'f', 12, 64), strconv.FormatFloat(rows[i].HourCos, 'f', 12, 64),
					strconv.FormatFloat(rows[i].MinSin, 'f', 12, 64), strconv.FormatFloat(rows[i].MinCos, 'f', 12, 64),
				)
			}
			record = append(record,
				strconv.FormatFloat(rows[i].DayWkSin, 'f', 12, 64), strconv.FormatFloat(rows[i].DayWkCos, 'f', 12, 64),
				strconv.FormatFloat(rows[i].DayYrSin, 'f', 12, 64), strconv.FormatFloat(rows[i].DayYrCos, 'f', 12, 64),
			)
		}
		w.Write(record)
	}
	fmt.Printf("💾 Transformed ML Matrix successfully saved: %s\n", filename)
}

func showHelpMenu() {
	fmt.Println("================================================================================")
	fmt.Println("          MACRO-MICRO EXCHANGE DATA FETCHER — SYSTEM MANUAL & MATRIX")
	fmt.Println("================================================================================")
	fmt.Println("Usage:")
	fmt.Println("  ./main [flags]")
	fmt.Println("  go run macro_yahoo_fetcher.go [flags]")
	fmt.Println("\nFlags:")
	fmt.Println("  -asset            string  Name tag used for naming output CSV targets")
	fmt.Println("                            (default: \"gold\")")
	fmt.Println("  -ticker           string  Valid ticker or coin pair (e.g., GC=F, BTCUSDT, UNIBTC, UNI-ETH)")
	fmt.Println("                            (default: \"GC=F\")")
	fmt.Println("  -start            string  Chronological search start point in YYYY-MM-DD format")
	fmt.Println("                            (default: \"2023-01-01\")")
	fmt.Println("  -end              string  Chronological search cutoff point in YYYY-MM-DD format")
	fmt.Println("                            (default: \"2026-06-30\")")
	fmt.Println("  -interval         string  Resolution partitioning step across multi-model frameworks:")
	fmt.Println("                            Crypto Spot High-Freq (Binance): 15m, 1h, 4h")
	fmt.Println("                            Macro / Crypto Baseline (Yahoo): 1d, 1month")
	fmt.Println("                            (default: \"1d\")")
	fmt.Println("  -transformVolume  bool    Convert raw volumes to stationary Log Change features")
	fmt.Println("                            via ln((Vol_t + 1) / (Vol_t-1 + 1)) (default: false)")
	fmt.Println("  -cyclicalTime     bool    Append engineered Day-of-Week & Day-of-Year sin/cos")
	fmt.Println("                            coordinates for neural networks (default: false)")
	fmt.Println("  -weekend          bool    Forward-fill weekend matrix data blocks using the")
	fmt.Println("                            previous Friday close parameters. Only applies to")
	fmt.Println("                            macro asset '1d' steps. (default: false)")
	fmt.Println("  -help             bool    Renders this operational lookup menu context directly")

	fmt.Println("\n--------------------------------------------------------------------------------")
	fmt.Println("             PRESET ASSET MAPPING MATRIX FOR MODEL 1 & MODEL 2")
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Printf("%-15s | %-10s | %-50s\n", "ASSET PARAM", "TICKER", "DESCRIPTION & SOURCE GATEWAY")
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Printf("%-15s | %-10s | %-50s\n", "bitcoin", "BTCUSDT", "Bitcoin vs USDT Spot. High-Freq (Binance) / Daily (Yahoo).")
	fmt.Printf("%-15s | %-10s | %-50s\n", "ethereum", "ETHUSDT", "Ethereum vs USDT Spot. High-Freq (Binance) / Daily (Yahoo).")
	fmt.Printf("%-15s | %-10s | %-50s\n", "uni_btc_cross", "UNIBTC", "Relative strength cross-pair. Alpha isolation book (Binance).")
	fmt.Printf("%-15s | %-10s | %-50s\n", "uni_eth_cross", "UNIETH", "Relative strength cross-pair. Alpha isolation book (Binance).")
	fmt.Printf("%-15s | %-10s | %-50s\n", "gold", "GC=F", "Gold Futures (COMEX). Core safe-haven hedge tracker (Yahoo).")
	fmt.Printf("%-15s | %-10s | %-50s\n", "oil", "BZ=F", "Brent Crude Oil Futures. Primary inflation driver (Yahoo).")
	fmt.Printf("%-15s | %-10s | %-50s\n", "fed_rate", "^IRX", "13-Week US Treasury Bill Index for baseline rates (Yahoo).")
	fmt.Println("--------------------------------------------------------------------------------")

	fmt.Println("\nDATA TRANSFORMATIONS & TIMESCALE CYCLES EXPLANATION:")
	fmt.Println("  1. Price Log Return & Volume Log Change:")
	fmt.Println("     Converts absolute variables into scale-invariant distributions for LSTMs.")
	fmt.Println("\n  2. Upgraded Cyclical Time Engine (-cyclicalTime=true):")
	fmt.Println("     The system dynamically changes its cyclical output layer based on interval resolution:")
	fmt.Println("     A. Traditional Macro Sequences (1d, 1mo):")
	fmt.Println("        Appends 4 features: day_wk_sin, day_wk_cos, day_yr_sin, day_yr_cos.")
    fmt.Println("     Maps timestamps into continuous unit circles to preserve seasonality:")
	fmt.Println("     - Day of Week: 7-day cycle mapping raw days (0-6) into sin/cos loops.")
	fmt.Println("     - Day of Year: 365.25-day cycle mapping calendar markers into sin/cos loops.")
	fmt.Println("     B. High-Frequency Sequences (15m, 1h, 4h):")
	fmt.Println("        Appends 8 features to isolate intraday liquidity sessions:")
	fmt.Println("        - Hour of Day Cycle (0-23h)  -> hour_sin, hour_cos")
	fmt.Println("        - Minute of Hour Cycle (0-59m) -> min_sin, min_cos")
	fmt.Println("        - Standard Day-of-Week and Day-of-Year macro cycles.")
	fmt.Println("\n  3. Order Flow Metrics & Weekend Forward Fills:")
	fmt.Println("     Identical to high-resolution taker buy matrices and macro weekend gap-fill blocks.")
	fmt.Println("\n  4. Cyclical Time Features (-cyclicalTime=true):")
	fmt.Println("     Maps timestamps into continuous unit circles to preserve seasonality:")
	fmt.Println("     - Day of Week: 7-day cycle mapping raw days (0-6) into sin/cos loops.")
	fmt.Println("     - Day of Year: 365.25-day cycle mapping calendar markers into sin/cos loops.")
	fmt.Println("\n  5. Weekend Forward Fill (-weekend=true):")
	fmt.Println("     Generates an unbroken chronological timeline for traditional macro markets.")
	fmt.Println("     Gaps on Saturdays and Sundays are populated with Friday's closing values.")
	fmt.Println("     Note: Automatically bypassed for crypto assets as they trade 24/7/365.")
	fmt.Println("================================================================================")
} 