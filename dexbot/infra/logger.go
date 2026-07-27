/******************************************************************************
 * File Name       : logger.go
 * File Path       : infra/logger.go
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:30 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:30 (UTC+7)
 * Description     : Centralized logging subsystem for Dexbot.
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
	currentLevel    = "INFO"
	logFilePath     = "logs/system.log"
	logToTerminal   = true
	logToFile       = true
	logCallerFormat = "full"
	fnTraceEnabled  = false
	logFormat       = "text"

	daemonID      = "default"
	correlationID = ""
)

/******************************************************************************
 * Function Name : SetDaemonID
 * Purpose       : Sets the daemon identifier for log prefixing.
 ******************************************************************************/
func SetDaemonID(id string) {
	daemonID = id
}

/******************************************************************************
 * Function Name : DaemonID
 * Purpose       : Returns the current daemon identifier.
 ******************************************************************************/
func DaemonID() string {
	return daemonID
}

/******************************************************************************
 * Function Name : SetCorrelationID
 * Purpose       : Sets a request-scoped correlation ID.
 ******************************************************************************/
func SetCorrelationID(cid string) {
	correlationID = cid
}

/******************************************************************************
 * Function Name : CorrelationID
 * Purpose       : Returns the current correlation ID.
 ******************************************************************************/
func CorrelationID() string {
	return correlationID
}

/******************************************************************************
 * Function Name : NewCorrelationID
 * Purpose       : Generates and sets a simple unique ID for cycle tracing.
 ******************************************************************************/
func NewCorrelationID() string {
	id := fmt.Sprintf("%d-%d", time.Now().UnixNano()%1000000000, time.Now().Unix()%10000)
	correlationID = id
	return id
}

// ==============================
// LOG WRITER INTERFACE
// ==============================

type LogWriter interface {
	Format(ts time.Time, level, caller, msg string) []byte
}

type TextWriter struct{}

func (tw *TextWriter) Format(ts time.Time, level, caller, msg string) []byte {
	if caller != "" {
		return []byte(fmt.Sprintf("[%s][%s]%s %s\n",
			ts.Format("2006-01-02 15:04:05"), level, caller, msg))
	}
	return []byte(fmt.Sprintf("[%s][%s] %s\n",
		ts.Format("2006-01-02 15:04:05"), level, msg))
}

type JSONWriter struct{}

type jsonLogEntry struct {
	Timestamp string `json:"ts"`
	Level     string `json:"level"`
	Caller    string `json:"caller,omitempty"`
	Function  string `json:"fn,omitempty"`
	Message   string `json:"msg"`
}

func (jw *JSONWriter) Format(ts time.Time, level, caller, msg string) []byte {
	entry := jsonLogEntry{
		Timestamp: ts.UTC().Format(time.RFC3339),
		Level:     level,
		Message:   msg,
	}
	if caller != "" {
		entry.Caller = caller
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

var activeWriter LogWriter = &TextWriter{}

// ==============================
// CORE LOGGING FUNCTIONS
// ==============================

/******************************************************************************
 * Function Name : InitLogger
 * Purpose       : Initializes logger parameters from environment settings.
 ******************************************************************************/
func InitLogger() {
	ReloadLoggerConfig()
	fmt.Println("[LOGGER][INFO] Logger initialized.")
}

/******************************************************************************
 * Function Name : FnTrace
 * Purpose       : Logs function entry for tracing execution flow.
 ******************************************************************************/
func FnTrace(msg string) {
	if !fnTraceEnabled {
		return
	}
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

/******************************************************************************
 * Function Name : ReloadLoggerConfig
 * Purpose       : Reloads logger options dynamically from environment variables.
 ******************************************************************************/
func ReloadLoggerConfig() {
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

	switch strings.ToLower(fnTrace) {
	case "on", "true", "1", "yes":
		fnTraceEnabled = true
	default:
		fnTraceEnabled = false
	}

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

	switch strings.ToUpper(level) {
	case "":
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
		fmt.Printf("[LOGGER][WARN] Invalid LOG_LEVEL='%s', defaulting to INFO\n", level)
	}

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

	if path := os.Getenv("LOG_FILE_PATH"); path != "" {
		logFilePath = path
	} else {
		logFilePath = "logs/system.log"
	}
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

/******************************************************************************
 * Function Name : ensureLogDir
 * Purpose       : Ensures that target log directory exists.
 ******************************************************************************/
func ensureLogDir() {
	dir := filepath.Dir(logFilePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
	}
}

/******************************************************************************
 * Function Name : getCallerInfo
 * Purpose       : Returns caller location string for log entries.
 ******************************************************************************/
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

	fn := runtime.FuncForPC(pc)
	funcName := "?"
	if fn != nil {
		funcName = fn.Name()
		if idx := strings.LastIndex(funcName, "/"); idx != -1 {
			funcName = funcName[idx+1:]
		}
	}

	return fmt.Sprintf(" (%s:%d %s)", file, line, funcName)
}

/******************************************************************************
 * Function Name : writeLog
 * Purpose       : Writes formatted log message to terminal and/or log file.
 ******************************************************************************/
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
		f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("WARNING: Log file error (%s): %v\n", logFilePath, err)
			return
		}
		defer f.Close()
		f.Write(formatted)
	}
}

/******************************************************************************
 * Function Name : shouldLog
 * Purpose       : Checks whether a level should be logged given current threshold.
 ******************************************************************************/
func shouldLog(level string) bool {
	if currentLevel == "OFF" {
		return false
	}

	levels := map[string]int{
		"TRACE": 0,
		"INFO":  1,
		"WARN":  2,
		"ERROR": 3,
	}

	return levels[level] >= levels[currentLevel]
}

/******************************************************************************
 * Function Name : Info / Warn / Error
 * Purpose       : Log severity wrappers.
 ******************************************************************************/
func Info(msg string)  { writeLog("INFO", msg) }
func Warn(msg string)  { writeLog("WARN", msg) }
func Error(msg string) { writeLog("ERROR", msg) }