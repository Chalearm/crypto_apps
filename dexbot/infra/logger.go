/******************************************************************************
 * File Name       : logger.go
 * File Path       : infra/logger.go
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
 *   Centralized logging subsystem for Dexbot. Provides INFO, WARN, ERROR logging levels, OFF mode, runtime configuration reload, terminal and/or file output, automatic log directory creation, customizable
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
 *   [Types] Struct definitions in this file
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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ==============================
// GLOBAL VARIABLES
// ==============================

var (
	currentLevel    = "INFO"            // Current active logging level (INFO, WARN, ERROR, OFF)
	logFilePath     = "logs/system.log" // Path to the log file
	logToTerminal   = true              // Flag to enable/disable logging to terminal
	logToFile       = true              // Flag to enable/disable logging to file
	logCallerFormat = "full"            // Caller info format: "short", "full", "off"
	fnTraceEnabled  = false             // Enable per-function entry/exit trace logging
	logFormat       = "text"            // Log output format: "text" or "json"

	// §76: Per-daemon identity for structured logs
	daemonID      = "default"           // Current daemon identifier (governance/school/trading/testdaemon)
	correlationID = ""                  // Current request correlation ID (UUID, set per-cycle)
)

// SetDaemonID sets the daemon identifier for log prefixing (§76).
// Should be called once at startup before any goroutines log.
func SetDaemonID(id string) {
	daemonID = id
}

// DaemonID returns the current daemon identifier.
func DaemonID() string {
	return daemonID
}

// SetCorrelationID sets a request-scoped correlation ID (§76).
func SetCorrelationID(cid string) {
	correlationID = cid
}

// CorrelationID returns the current correlation ID.
func CorrelationID() string {
	return correlationID
}

// NewCorrelationID generates and sets a simple unique ID for cycle tracing.
func NewCorrelationID() string {
	id := fmt.Sprintf("%d-%d", time.Now().UnixNano()%1000000000, time.Now().Unix()%10000)
	correlationID = id
	return id
}

// ==============================
// LOG WRITER INTERFACE (Phase 15 — §6)
// ==============================

/*
Interface: LogWriter
Description:
  Abstraction for log output formatting. Implementations write structured
  or unstructured log entries to an io.Writer or terminal/file.

Methods:
  - Format(ts time.Time, level, caller, msg string) []byte

Lines: ~5
*/
type LogWriter interface {
	Format(ts time.Time, level, caller, msg string) []byte
}

/*
Struct: TextWriter
Description:
  Traditional text log format: "[2006-01-02 15:04:05][INFO] (file:line fn) msg"
/*
Struct: TextWriter
Description:
  Traditional text log format: "[2006-01-02 15:04:05][INFO] (file:line fn) msg"

Lines: ~3
*/
type TextWriter struct{}

func (tw *TextWriter) Format(ts time.Time, level, caller, msg string) []byte {
	if caller != "" {
		return []byte(fmt.Sprintf("[%s][%s]%s %s\n",
			ts.Format("2006-01-02 15:04:05"), level, caller, msg))
	}
	return []byte(fmt.Sprintf("[%s][%s] %s\n",
		ts.Format("2006-01-02 15:04:05"), level, msg))
}


/*
Struct: JSONWriter
Description:
  Structured JSON log format. Each log line is a JSON object:
  {"ts":"2006-01-02T15:04:05Z","level":"INFO","caller":"file:line","fn":"FuncName","msg":"..."}

Fields:
  - none (stateless)

Lines: ~15
*/
type JSONWriter struct{}

// jsonLogEntry is the structured JSON record emitted by JSONWriter.
type jsonLogEntry struct {
	Timestamp string `json:"ts"`
	Level     string `json:"level"`
	Caller    string `json:"caller,omitempty"`
	Function  string `json:"fn,omitempty"`
	Message   string `json:"msg"`
}

/*
Function: Format
Description:
  Formats a log entry as a single-line JSON object.

Input:
  - ts     time.Time : Timestamp
  - level  string    : Log level
  - caller string    : Caller info (file:line)
  - msg    string    : Log message

Output:
  - []byte : JSON line with trailing newline

Lines: ~20
*/
func (jw *JSONWriter) Format(ts time.Time, level, caller, msg string) []byte {
	entry := jsonLogEntry{
		Timestamp: ts.UTC().Format(time.RFC3339),
		Level:     level,
		Message:   msg,
	}
	if caller != "" {
		entry.Caller = caller
		// Extract function name from caller string like " (file:line pkg.FuncName)"
		if idx := strings.LastIndex(caller, " "); idx != -1 {
			fn := strings.TrimSpace(caller[idx:])
			fn = strings.Trim(fn, "()")
			if fn != "" {
				entry.Function = fn
			}
		}
	}
	data, _ := json.Marshal(entry)
	return append(data, '\n')
}

// activeWriter is the current log format writer — switched by ReloadLoggerConfig.
var activeWriter LogWriter = &TextWriter{}

// ==============================
// CORE LOGGING FUNCTIONS
// ==============================

/*
Function: InitLogger
Description:
  Initializes the logger subsystem by reloading its configuration from environment variables.
  This function should be called once at the application startup.
Input:
  - none
Output:
  - none
Return:
  - none
Lines: ~5
*/
func InitLogger() {
	ReloadLoggerConfig()
	fmt.Println("[LOGGER][INFO] Logger initialized.")
}

/*
Function: FnTrace
Description:
  Logs a function trace message at TRACE level. Only emitted when
  FN_TRACE=on in the environment. Useful for maintainability —
  shows exactly which functions are entered/exited and their status.

  Usage pattern:
    infra.FnTrace("entering")
    defer infra.FnTrace("OK")
    infra.FnTrace("processing batch=%d", n)

Input:
  - msg string : Trace message (format string optional)

Output:
  - none

Lines: ~10
*/
func FnTrace(msg string) {
	if !fnTraceEnabled {
		return
	}
	// Get caller function name (skip 1 frame: FnTrace → actual caller)
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		writeLog("TRACE", msg)
		return
	}
	fn := runtime.FuncForPC(pc)
	funcName := "?"
	if fn != nil {
		funcName = fn.Name()
		if idx := strings.LastIndex(funcName, "."); idx != -1 {
			funcName = funcName[idx+1:]
		}
	}
	writeLog("TRACE", fmt.Sprintf("%s() → %s", funcName, msg))
}

/*
Function: ReloadLoggerConfig
Description:
  Reloads logger configuration from environment variables. This function resets all logger
  settings to defaults before applying environment-based overrides. It supports `LOG_LEVEL`,
  `LOG_OUTPUT`, and `LOG_FILE_PATH` environment variables.
Input:
  - none
Output:
  - none
Return:
  - none
Side Effects:
  - Updates global logger configuration (`currentLevel`, `logFilePath`, `logToTerminal`, `logToFile`)
  - Creates the log directory if it does not exist.
  - Prints startup diagnostics and warnings for invalid configuration values.
Lines: ~70
*/
func ReloadLoggerConfig() {
	// Reset defaults every reload
	currentLevel = "INFO"
	logFilePath = "logs/system.log"
	logToTerminal = true
	logToFile = true
	logCallerFormat = "full"
	fnTraceEnabled = false
	logFormat = "text"
	activeWriter = &TextWriter{}

	output := strings.TrimSpace(os.Getenv("LOG_OUTPUT"))
	level := strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	callerFmt := strings.TrimSpace(os.Getenv("LOG_CALLER_FORMAT"))
	fnTrace := strings.TrimSpace(os.Getenv("FN_TRACE"))
	format := strings.TrimSpace(os.Getenv("LOG_FORMAT"))

	// Function trace toggle
	switch strings.ToLower(fnTrace) {
	case "on", "true", "1", "yes":
		fnTraceEnabled = true
	default:
		fnTraceEnabled = false
	}

	// Set log format (text or json)
	switch strings.ToLower(format) {
	case "", "text":
		logFormat = "text"
		activeWriter = &TextWriter{}
	case "json":
		logFormat = "json"
		activeWriter = &JSONWriter{}
	default:
		fmt.Printf("[LOGGER][WARN] Invalid LOG_FORMAT='%s', defaulting to text\n", format)
		logFormat = "text"
		activeWriter = &TextWriter{}
	}

	// Set caller format
	switch strings.ToLower(callerFmt) {
	case "":
		logCallerFormat = "full"
	case "short":
		logCallerFormat = "short"
	case "full":
		logCallerFormat = "full"
	case "off":
		logCallerFormat = "off"
	default:
		fmt.Printf("[LOGGER][WARN] Invalid LOG_CALLER_FORMAT='%s', defaulting to full\n", callerFmt)
		logCallerFormat = "full"
	}

	// Set log level
	switch strings.ToUpper(level) {
	case "": // Default to INFO if not specified
		currentLevel = "INFO"
	case "INFO":
		currentLevel = "INFO"
	case "WARN":
		currentLevel = "WARN"
	case "ERROR":
		currentLevel = "ERROR"
	case "OFF":
		currentLevel = "OFF"
	default:
		fmt.Printf(
			"[LOGGER][WARN] Invalid LOG_LEVEL='%s', defaulting to INFO\n",
			level,
		)
	}

	// Set log output destination
	if output != "" {
		logToTerminal = false
		logToFile = false
		switch strings.ToLower(output) {
		case "terminal":
			logToTerminal = true
		case "file":
			logToFile = true
		case "both":
			logToTerminal = true
			logToFile = true
		default:
			fmt.Println("[WARN] Invalid LOG_OUTPUT in config.env, defaulting to 'both'")
			logToTerminal = true
			logToFile = true
		}
	}

	// Set log file path
	if path := os.Getenv("LOG_FILE_PATH"); path != "" {
		logFilePath = path
	} else {
		logFilePath = "logs/system.log" // Default log file path
	}
	// Ensure log directory exists after potential path change
	ensureLogDir()

	fmt.Printf(
		"[LOGGER] level=%s output=%s file=%s terminal=%t fileOutput=%t caller=%s format=%s\n",
		currentLevel,
		output,
		logFilePath,
		logToTerminal,
		logToFile,
		logCallerFormat,
		logFormat,
	)
}

/*
Function: ensureLogDir
Description:
  Creates the directory specified by `logFilePath` if it does not already exist.
  Permissions are set to 0755 (read/write/execute for owner, read/execute for group/others).
Input:
  - none
Output:
  - none
Lines: ~10
*/
func ensureLogDir() {
	dir := filepath.Dir(logFilePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
	}
}

/*
Function: getCallerInfo
Description:
  Extracts file name, line number, and function name from the call stack.
  Skips the specified number of frames to reach the actual caller.
Input:
  - skip int: Number of stack frames to skip (e.g., 3 for Info/Warn/Error → writeLog → getCallerInfo → actual caller).
Output:
  - string: Formatted caller string per logCallerFormat, or empty string if format is "off".
Lines: ~20
*/
func getCallerInfo(skip int) string {
	if logCallerFormat == "off" {
		return ""
	}

	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}

	file = filepath.Base(file)

	if logCallerFormat == "short" {
		return fmt.Sprintf(" (%s:%d)", file, line)
	}

	// "full" format: include function name
	fn := runtime.FuncForPC(pc)
	funcName := "?"
	if fn != nil {
		funcName = fn.Name()
		// Trim package path, keep only the last segment (e.g., "main.processTask")
		if idx := strings.LastIndex(funcName, "/"); idx != -1 {
			funcName = funcName[idx+1:]
		}
		// Further trim: remove package name prefix if it duplicates what we already show
		// e.g., "infra.CheckDBHealth" → keep full, it's useful
	}

	return fmt.Sprintf(" (%s:%d %s)", file, line, funcName)
}

/*
Function: writeLog
Description:
  Writes a log message to the configured destinations (console, file).
  It includes the caller's filename, line number, and optionally function name
  based on LOG_CALLER_FORMAT for better traceability.
Input:
  - level string: The log level (e.g., "INFO", "WARN", "ERROR").
  - msg string: The actual log message content.
Output:
  - none
Lines: ~40
*/
func writeLog(level string, msg string) {
	if !shouldLog(level) {
		return
	}

	callerInfo := getCallerInfo(3)
	ts := time.Now()
	formatted := activeWriter.Format(ts, level, callerInfo, msg)

	if logToTerminal {
		os.Stdout.Write(formatted)
	}

	if logToFile {
		f, err := os.OpenFile(logFilePath,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

		if err != nil {
			fmt.Printf("WARNING: Log file error (%s): %v\n", logFilePath, err)
			return
		}

		defer f.Close()
		f.Write(formatted)
	}
}

/*
Function: shouldLog
Description:
  Determines whether a given log message should be emitted based on the `currentLevel`.
  Messages with a level lower than `currentLevel` are discarded.
Input:
  - level string: The log level of the message to be checked (e.g., "INFO", "WARN", "ERROR").
Output:
  - bool: `true` if the message should be emitted, `false` otherwise.
Return:
  - bool
Lines: ~20
*/
func shouldLog(level string) bool {

	if currentLevel == "OFF" {
		return false // If logging is off, no messages should be logged
	}

	levels := map[string]int{
		"TRACE": 0,
		"INFO":  1,
		"WARN":  2,
		"ERROR": 3,
	}

	// Compare the integer representation of the log levels
	return levels[level] >= levels[currentLevel]
}

/*
Function: Info
Description:
  Logs an informational message. These messages are typically used for general operational insights.
Input:
  - msg string: The informational message to log.
Output:
  - none
Lines: ~1
*/
func Info(msg string) { writeLog("INFO", msg) }

/*
Function: Warn
Description:
  Logs a warning message. Warnings indicate potential issues that do not prevent the application
  from running but might require attention.
Input:
  - msg string: The warning message to log.
Output:
  - none
Lines: ~1
*/
func Warn(msg string) { writeLog("WARN", msg) }

/*
Function: Error
Description:
  Logs an error message. Errors indicate problems that have occurred and might affect the application's
  functionality or stability.
Input:
  - msg string: The error message to log.
Output:
  - none
Lines: ~1
*/
func Error(msg string) { writeLog("ERROR", msg) }
