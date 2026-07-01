/******************************************************************************
 * File Name       : logger_test.go
 * File Path       : infra/logger_test.go
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
 *   Unit tests for logging system, including dynamic configuration and log level handling. Tests: [OK] log file creation [OK] multi-level logging [OK] dynamic configuration via ReloadLoggerConfig [OK] log
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
 *   [Test Functions] Test suite: TestLoggerCreatesFile, TestLoggerMultipleWrites, TestReloadLoggerConfig, TestLogLevels
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
    "bufio"
    "encoding/json"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

// Helper function to clean up log files and reset environment variables after tests
func CleanUp() {
	_ = os.RemoveAll(filepath.Dir(logFilePath)) // Clean up log directory
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("LOG_OUTPUT")
	os.Unsetenv("LOG_FILE_PATH")
	os.Unsetenv("LOG_CALLER_FORMAT")
	os.Unsetenv("LOG_FORMAT")
	// Reset package variables to their defaults for subsequent tests
	currentLevel = "INFO"
	logFilePath = "logs/system.log"
	logToTerminal = true
	logToFile = true
	logCallerFormat = "full"
	logFormat = "text"
	activeWriter = &TextWriter{}
}

/*
Function: TestLoggerCreatesFile
Description:
Ensures that the log file is created in the specified path when logging is enabled.

Input:
- t *testing.T: The testing object.

Output:
- Error if the log file does not exist after logging.

Lines: ~20
*/
func TestLoggerCreatesFile(t *testing.T) {
    defer CleanUp()
    CleanUp() // Ensure a clean state before test

    InitLogger() // Initialize with default config (logToFile = true)
    Info("test log entry for file creation")

    if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
        t.Errorf("log file should exist at %s, but it does not", logFilePath)
    }
}

/*
Function: TestLoggerMultipleWrites
Description:
Tests that multiple log entries at different levels are processed without errors.
This test primarily checks for runtime issues during multiple write operations
and doesn't assert content, as content checks are handled in other tests.

Input:
- t *testing.T: The testing object.

Output:
- No error expected.

Lines: ~15
*/
func TestLoggerMultipleWrites(t *testing.T) {
    defer CleanUp()
    CleanUp()

    InitLogger() // Initialize logger

    Info("This is an info message.")
    Warn("This is a warning message.")
    Error("This is an error message.")

    // Check if the file was created and contains content
    fileContent, err := os.ReadFile(logFilePath)
    if err != nil {
        t.Fatalf("Failed to read log file: %v", err)
    }
    if !strings.Contains(string(fileContent), "This is an info message.") ||
        !strings.Contains(string(fileContent), "This is a warning message.") ||
        !strings.Contains(string(fileContent), "This is an error message.") {
        t.Errorf("Log file does not contain all expected messages.")
    }
}

/*
Function: TestReloadLoggerConfig
Description:
Verifies that ReloadLoggerConfig correctly updates the logger's behavior
based on environment variables for log level, output destination, and file path.

Input:
- t *testing.T: The testing object.

Output:
- Error if logger configuration is not reloaded correctly.

Lines: ~80
*/
func TestReloadLoggerConfig(t *testing.T) {
    defer CleanUp()
    CleanUp() // Start with a clean slate

    // --- Test 1: Change Log Level to WARN ---
    os.Setenv("LOG_LEVEL", "WARN")
    ReloadLoggerConfig()
    if currentLevel != "WARN" {
        t.Errorf("Expected log level to be WARN, got %s", currentLevel)
    }
    // Info message should NOT be logged
    Info("This info should not appear.")
    Warn("This warning should appear.")
    fileContent, _ := os.ReadFile(logFilePath)
    if strings.Contains(string(fileContent), "This info should not appear.") {
        t.Errorf("Info message logged despite level being WARN.")
    }
    if !strings.Contains(string(fileContent), "This warning should appear.") {
        t.Errorf("Warning message did not log when level is WARN.")
    }
    _ = os.Remove(logFilePath) // Clean for next sub-test

    // --- Test 2: Change Log Output to Terminal Only ---
    os.Setenv("LOG_LEVEL", "INFO") // Reset level
    os.Setenv("LOG_OUTPUT", "terminal")
    ReloadLoggerConfig()
    if !logToTerminal || logToFile {
        t.Errorf("Expected logToTerminal=true and logToFile=false, got %t, %t", logToTerminal, logToFile)
    }
    Info("Terminal output test.") // This should only print to console, not file
    if _, err := os.Stat(logFilePath); !os.IsNotExist(err) {
        t.Errorf("Log file should not exist when output is 'terminal' only.")
    }
    _ = os.Remove(logFilePath) // Clean for next sub-test

    // --- Test 3: Change Log File Path ---
    newLogPath := "test_logs/custom.log"
    os.Setenv("LOG_OUTPUT", "file") // Ensure file logging is on
    os.Setenv("LOG_FILE_PATH", newLogPath)
    ReloadLoggerConfig()
    if logFilePath != newLogPath {
        t.Errorf("Expected log file path to be %s, got %s", newLogPath, logFilePath)
    }
    Info("Custom path log.")
    if _, err := os.Stat(newLogPath); os.IsNotExist(err) {
        t.Errorf("Log file should exist at custom path %s", newLogPath)
    }
    // Verify old path does not exist
    if _, err := os.Stat("logs/system.log"); !os.IsNotExist(err) {
        t.Errorf("Old log file path 'logs/system.log' should not exist after path change.")
    }
    _ = os.RemoveAll(filepath.Dir(newLogPath)) // Clean for next sub-test

    // --- Test 4: Invalid LOG_OUTPUT should default to 'both' ---
    os.Setenv("LOG_OUTPUT", "invalid_option")
    ReloadLoggerConfig()
    if !logToTerminal || !logToFile {
        t.Errorf("Expected logToTerminal=true and logToFile=true for invalid LOG_OUTPUT, got %t, %t", logToTerminal, logToFile)
    }
}

/*
Function: TestLogLevels
Description:
Tests the shouldLog function and ensures that log messages are correctly filtered
based on the configured currentLevel.

Input:
- t *testing.T: The testing object.

Output:
- Error if log levels are not respected.

Lines: ~50
*/
func TestLogLevels(t *testing.T) {
    defer CleanUp()
    CleanUp()

    // Test INFO level
    os.Setenv("LOG_LEVEL", "INFO")
    ReloadLoggerConfig()
    if !shouldLog("INFO") {
        t.Errorf("Should log INFO when level is INFO")
    }
    if !shouldLog("WARN") {
        t.Errorf("Should log WARN when level is INFO")
    }
    if !shouldLog("ERROR") {
        t.Errorf("Should log ERROR when level is INFO")
    }

    // Test WARN level
    os.Setenv("LOG_LEVEL", "WARN")
    ReloadLoggerConfig()
    if shouldLog("INFO") {
        t.Errorf("Should NOT log INFO when level is WARN")
    }
    if !shouldLog("WARN") {
        t.Errorf("Should log WARN when level is WARN")
    }
    if !shouldLog("ERROR") {
        t.Errorf("Should log ERROR when level is WARN")
    }

    // Test ERROR level
    os.Setenv("LOG_LEVEL", "ERROR")
    ReloadLoggerConfig()
    if shouldLog("INFO") {
        t.Errorf("Should NOT log INFO when level is ERROR")
    }
    if shouldLog("WARN") {
        t.Errorf("Should NOT log WARN when level is ERROR")
    }
    if !shouldLog("ERROR") {
        t.Errorf("Should log ERROR when level is ERROR")
    }

    // Additionally, verify actual logging behavior for filtering
    os.Setenv("LOG_LEVEL", "WARN")
    os.Setenv("LOG_OUTPUT", "file") // Ensure file output for checking
    os.Setenv("LOG_FILE_PATH", "test_logs/level_test.log")
    ReloadLoggerConfig()

    Info("This is an INFO message.")
    Warn("This is a WARN message.")
    Error("This is an ERROR message.")

    file, err := os.Open(logFilePath)
    if err != nil {
        t.Fatalf("Failed to open log file for level test: %v", err)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    var lines []string
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }

    infoLogged := false
    warnLogged := false
    errorLogged := false
    for _, line := range lines {
        if strings.Contains(line, "INFO") {
            infoLogged = true
        }
        if strings.Contains(line, "WARN") {
            warnLogged = true
        }
        if strings.Contains(line, "ERROR") {
            errorLogged = true
        }
    }

    if infoLogged {
        t.Errorf("INFO message was logged when LOG_LEVEL is WARN.")
    }
    if !warnLogged {
        t.Errorf("WARN message was NOT logged when LOG_LEVEL is WARN.")
    }
    if !errorLogged {
        t.Errorf("ERROR message was NOT logged when LOG_LEVEL is WARN.")
    }
}

/*
Function:
    TestLogLevelOff
Description:
    Verify that LOG_LEVEL=OFF suppresses all logging.
Input:
    t *testing.T
Output:
    none
Return:
    none
Lines:
    ~30
*/
func TestLogLevelOff(t *testing.T) {
    defer CleanUp()
    CleanUp()
    os.Setenv("LOG_LEVEL", "OFF")
    os.Setenv("LOG_OUTPUT", "file")
    os.Setenv("LOG_FILE_PATH", "test_logs/off.log")
    ReloadLoggerConfig()
    Info("info message")
    Warn("warn message")
    Error("error message")
    if _, err := os.Stat(logFilePath); err == nil {
        data, _ := os.ReadFile(logFilePath)

        if len(data) > 0 {
            t.Errorf(
                "Expected empty log file when LOG_LEVEL=OFF",
            )
        }
    }
}

/*
Function:
    TestInvalidLogLevelFallback
Description:
    Verify invalid LOG_LEVEL falls back to INFO.

Input:
    t *testing.T
Output:
    none
Return:
    none
Lines:
    ~25
*/
func TestInvalidLogLevelFallback(t *testing.T) {
    defer CleanUp()
    CleanUp()
    os.Setenv("LOG_LEVEL", "BANANA")
    ReloadLoggerConfig()
    if currentLevel != "INFO" {
        t.Errorf(
            "Expected INFO fallback, got %s",
            currentLevel,
        )
    }
}

/*
Function:
    TestReloadResetsOutputDefaults
Description:
    Verify ReloadLoggerConfig resets output
    settings when LOG_OUTPUT is removed.
Input:
    t *testing.T
Output:
    none
Return:
    none
Lines:
    ~30
*/
func TestReloadResetsOutputDefaults(t *testing.T) {
    defer CleanUp()
    CleanUp()
    os.Setenv("LOG_OUTPUT", "file")
    ReloadLoggerConfig()
    if logToTerminal {
        t.Errorf("Expected terminal disabled")
    }
    if !logToFile {
        t.Errorf("Expected file enabled")
    }
    os.Unsetenv("LOG_OUTPUT")
    ReloadLoggerConfig()
    if !logToTerminal {
        t.Errorf("Expected terminal enabled after reset")
    }

    if !logToFile {
        t.Errorf("Expected file enabled after reset")
    }
}
/*
Function:
    TestReloadResetsFilePathDefaults
Description:
    Verify ReloadLoggerConfig resets log path
    when LOG_FILE_PATH is removed.
Input:
    t *testing.T
Output:
    none
Return:
    none
Lines:
    ~30
*/
func TestReloadResetsFilePathDefaults(t *testing.T) {
    defer CleanUp()
    CleanUp()
    os.Setenv("LOG_FILE_PATH", "custom/test.log")
    ReloadLoggerConfig()
    if logFilePath != "custom/test.log" {
        t.Errorf(
            "Expected custom path, got %s",
            logFilePath,
        )
    }
    os.Unsetenv("LOG_FILE_PATH")
    ReloadLoggerConfig()

    if logFilePath != "logs/system.log" {
        t.Errorf(
            "Expected default path after reset, got %s",
            logFilePath,
        )
    }
}
/*
Function:
    TestInvalidOutputFallsBackToBoth
Description:
    Verify invalid LOG_OUTPUT defaults
    to terminal+file output.
Input:
    t *testing.T
Output:
    none
Return:
    none
Lines:
    ~20
*/
func TestInvalidOutputFallsBackToBoth(t *testing.T) {
    defer CleanUp()
    CleanUp()
    os.Setenv("LOG_OUTPUT", "INVALID_VALUE")
    ReloadLoggerConfig()
    if !logToTerminal {
        t.Errorf("Expected terminal enabled")
    }
    if !logToFile {
        t.Errorf("Expected file enabled")
    }
}
/*
Function:
    TestDefaultOutputIsBoth
Description:
    Verify default output mode is terminal+file.
Input:
    t *testing.T
Output:
    none
Return:
    none
Lines:
    ~20
*/
func TestDefaultOutputIsBoth(t *testing.T) {
    defer CleanUp()
    CleanUp()
    os.Unsetenv("LOG_OUTPUT")
    ReloadLoggerConfig()
    if !logToTerminal {
        t.Errorf("Expected terminal enabled")
    }
    if !logToFile {
        t.Errorf("Expected file enabled")
    }
}

/*
Function: TestCallerFormatFull
Description:
  Verify full caller format includes file, line, and function name.
Input:
  t *testing.T
Output:
  none
Lines: ~30
*/
func TestCallerFormatFull(t *testing.T) {
	defer CleanUp()
	CleanUp()
	os.Setenv("LOG_LEVEL", "INFO")
	os.Setenv("LOG_OUTPUT", "file")
	os.Setenv("LOG_FILE_PATH", "test_logs/caller_full.log")
	os.Setenv("LOG_CALLER_FORMAT", "full")
	ReloadLoggerConfig()

	Info("caller full format test")

	data, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "TestCallerFormatFull") {
		t.Errorf("Expected function name 'TestCallerFormatFull' in log output, got: %s", content)
	}
	if !strings.Contains(content, "logger_test.go:") {
		t.Errorf("Expected file:line in log output, got: %s", content)
	}
}

/*
Function: TestCallerFormatShort
Description:
  Verify short caller format includes only file and line, no function name.
Input:
  t *testing.T
Output:
  none
Lines: ~30
*/
func TestCallerFormatShort(t *testing.T) {
	defer CleanUp()
	CleanUp()
	os.Setenv("LOG_LEVEL", "INFO")
	os.Setenv("LOG_OUTPUT", "file")
	os.Setenv("LOG_FILE_PATH", "test_logs/caller_short.log")
	os.Setenv("LOG_CALLER_FORMAT", "short")
	ReloadLoggerConfig()

	Info("caller short format test")

	data, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "main.") {
		t.Errorf("Short format should NOT contain function name, got: %s", content)
	}
	if !strings.Contains(content, "logger_test.go:") && !strings.Contains(content, ".go:") {
		t.Errorf("Expected file:line in log output, got: %s", content)
	}
}

/*
Function: TestCallerFormatOff
Description:
  Verify caller info is absent when LOG_CALLER_FORMAT=off.
Input:
  t *testing.T
Output:
  none
Lines: ~30
*/
func TestCallerFormatOff(t *testing.T) {
	defer CleanUp()
	CleanUp()
	os.Setenv("LOG_LEVEL", "INFO")
	os.Setenv("LOG_OUTPUT", "file")
	os.Setenv("LOG_FILE_PATH", "test_logs/caller_off.log")
	os.Setenv("LOG_CALLER_FORMAT", "off")
	ReloadLoggerConfig()

	Info("caller off format test")

	data, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "logger_test.go") || strings.Contains(content, ".go:") {
		t.Errorf("Caller info should be absent when LOG_CALLER_FORMAT=off, got: %s", content)
	}
	if !strings.Contains(content, "caller off format test") {
		t.Errorf("Log message itself should still appear, got: %s", content)
	}
}

/*
Function: TestCallerFormatDefaultIsFull
Description:
  Verify default caller format is 'full' when LOG_CALLER_FORMAT is not set.
Input:
  t *testing.T
Output:
  none
Lines: ~25
*/
func TestCallerFormatDefaultIsFull(t *testing.T) {
	defer CleanUp()
	CleanUp()
	os.Unsetenv("LOG_CALLER_FORMAT")
	ReloadLoggerConfig()

	if logCallerFormat != "full" {
		t.Errorf("Expected default logCallerFormat='full', got '%s'", logCallerFormat)
	}
}

/*
Function: TestCallerFormatInvalidFallback
Description:
  Verify invalid LOG_CALLER_FORMAT falls back to 'full'.
Input:
  t *testing.T
Output:
  none
Lines: ~25
*/
func TestCallerFormatInvalidFallback(t *testing.T) {
	defer CleanUp()
	CleanUp()
	os.Setenv("LOG_CALLER_FORMAT", "garbage_value")
	ReloadLoggerConfig()

	if logCallerFormat != "full" {
		t.Errorf("Expected fallback to 'full' for invalid value, got '%s'", logCallerFormat)
	}
}

/*
Function: TestCallerFormatOffNoCallerInfo
Description:
  Negative test: ensure no filename or line number appears
  in logged output when LOG_CALLER_FORMAT=off.
Input:
  t *testing.T
Output:
  none
Lines: ~30
*/
func TestCallerFormatOffNoCallerInfo(t *testing.T) {
	defer CleanUp()
	CleanUp()
	os.Setenv("LOG_LEVEL", "INFO")
	os.Setenv("LOG_OUTPUT", "file")
	os.Setenv("LOG_FILE_PATH", "test_logs/neg_no_caller.log")
	os.Setenv("LOG_CALLER_FORMAT", "off")
	ReloadLoggerConfig()

	Warn("negative: no caller info expected")
	Error("negative: error without caller info")

	data, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	content := string(data)
	if strings.Contains(content, ".go:") {
		t.Errorf("NEGATIVE: expected no '.go:' caller reference, got: %s", content)
	}
}

/*
Function: TestCallerFormatFullHasFunctionName
Description:
  Negative test: confirm 'short' format explicitly DOES NOT
  include function name, while 'full' format DOES.
Input:
  t *testing.T
Output:
  none
Lines: ~35
*/
func TestCallerFormatFullHasFunctionName(t *testing.T) {
	defer CleanUp()
	CleanUp()
	os.Setenv("LOG_LEVEL", "INFO")
	os.Setenv("LOG_OUTPUT", "file")
	os.Setenv("LOG_FILE_PATH", "test_logs/neg_full_func.log")
	os.Setenv("LOG_CALLER_FORMAT", "full")
	ReloadLoggerConfig()

	Info("full format function name check")

	data, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	content := string(data)
	hasFuncName := strings.Contains(content, "TestCallerFormatFullHasFunctionName")
	if !hasFuncName {
		t.Errorf("NEGATIVE: full format must include function name, got: %s", content)
	}
}

// ==============================
// JSON FORMAT TESTS (Phase 15 — §6)
// ==============================

/*
Function: TestJSONFormatProducesValidJSON
Description:
  Positive: LOG_FORMAT=json writes valid JSON lines to file.

Lines: ~20
*/
func TestJSONFormatProducesValidJSON(t *testing.T) {
	defer CleanUp()
	CleanUp()
	os.Setenv("LOG_LEVEL", "INFO")
	os.Setenv("LOG_OUTPUT", "file")
	os.Setenv("LOG_FILE_PATH", "test_logs/json_test.log")
	os.Setenv("LOG_FORMAT", "json")
	ReloadLoggerConfig()

	Info("json format test message")
	Warn("json warn with special chars: key=value")
	Error("json error: something failed")

	data, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 3 {
		t.Fatalf("Expected at least 3 log lines, got %d", len(lines))
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
			t.Errorf("Expected JSON object, got: %s", line)
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("Invalid JSON: %v — line: %s", err, line)
		}
	}
}

/*
Function: TestJSONFormatContainsFields
Description:
  Positive: JSON logs contain ts, level, msg fields.

Lines: ~25
*/
func TestJSONFormatContainsFields(t *testing.T) {
	defer CleanUp()
	CleanUp()
	os.Setenv("LOG_LEVEL", "INFO")
	os.Setenv("LOG_OUTPUT", "file")
	os.Setenv("LOG_FILE_PATH", "test_logs/json_fields.log")
	os.Setenv("LOG_FORMAT", "json")
	os.Setenv("LOG_CALLER_FORMAT", "full")
	ReloadLoggerConfig()

	Info("test with fields")

	data, _ := os.ReadFile(logFilePath)
	var entry map[string]interface{}
	json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry)

	if _, ok := entry["ts"]; !ok {
		t.Error("JSON log missing 'ts' field")
	}
	if _, ok := entry["level"]; !ok {
		t.Error("JSON log missing 'level' field")
	}
	if _, ok := entry["msg"]; !ok {
		t.Error("JSON log missing 'msg' field")
	}
	if entry["level"] != "INFO" {
		t.Errorf("Expected level=INFO, got %v", entry["level"])
	}
	if entry["msg"] != "test with fields" {
		t.Errorf("Expected msg='test with fields', got %v", entry["msg"])
	}
}

/*
Function: TestJSONFormatDefaultIsText
Description:
  Positive: When LOG_FORMAT is not set, format stays text.

Lines: ~15
*/
func TestJSONFormatDefaultIsText(t *testing.T) {
	defer CleanUp()
	CleanUp()
	os.Unsetenv("LOG_FORMAT")
	ReloadLoggerConfig()

	if logFormat != "text" {
		t.Errorf("Expected default format=text, got %s", logFormat)
	}
	_, isJSON := activeWriter.(*JSONWriter)
	if isJSON {
		t.Error("Expected TextWriter when no LOG_FORMAT set")
	}
}

/*
Function: TestJSONFormatSwitchesWriter
Description:
  Positive: ReloadLoggerConfig switches between TextWriter and JSONWriter.

Lines: ~15
*/
func TestJSONFormatSwitchesWriter(t *testing.T) {
	defer CleanUp()
	CleanUp()

	os.Setenv("LOG_FORMAT", "json")
	ReloadLoggerConfig()
	if _, ok := activeWriter.(*JSONWriter); !ok {
		t.Error("Expected JSONWriter when LOG_FORMAT=json")
	}

	os.Setenv("LOG_FORMAT", "text")
	ReloadLoggerConfig()
	if _, ok := activeWriter.(*TextWriter); !ok {
		t.Error("Expected TextWriter when LOG_FORMAT=text")
	}
}

/*
Function: TestJSONFormatNoCallerWhenOff
Description:
  Positive: LOG_FORMAT=json + LOG_CALLER_FORMAT=off produces no caller field.

Lines: ~20
*/
func TestJSONFormatNoCallerWhenOff(t *testing.T) {
	defer CleanUp()
	CleanUp()
	os.Setenv("LOG_LEVEL", "INFO")
	os.Setenv("LOG_OUTPUT", "file")
	os.Setenv("LOG_FILE_PATH", "test_logs/json_no_caller.log")
	os.Setenv("LOG_FORMAT", "json")
	os.Setenv("LOG_CALLER_FORMAT", "off")
	ReloadLoggerConfig()

	Info("no caller test")

	data, _ := os.ReadFile(logFilePath)
	var entry map[string]interface{}
	json.Unmarshal([]byte(strings.TrimSpace(string(data))), &entry)

	if _, ok := entry["caller"]; ok {
		t.Error("JSON log should not have 'caller' when LOG_CALLER_FORMAT=off")
	}
}

// ==============================
// NEGATIVE TESTS
// ==============================

/*
Function: TestJSONFormatInvalidFallsBack
Description:
  Negative: Invalid LOG_FORMAT falls back to text.

Lines: ~12
*/
func TestJSONFormatInvalidFallsBack(t *testing.T) {
	defer CleanUp()
	CleanUp()
	os.Setenv("LOG_FORMAT", "xml")
	ReloadLoggerConfig()

	if logFormat != "text" {
		t.Errorf("Expected fallback to text for invalid format, got %s", logFormat)
	}
}

/*
Function: TestJSONFormatWithLogLevelOff
Description:
  Negative: LOG_FORMAT=json + LOG_LEVEL=OFF produces empty file.

Lines: ~15
*/
func TestJSONFormatWithLogLevelOff(t *testing.T) {
	defer CleanUp()
	CleanUp()
	os.Setenv("LOG_LEVEL", "OFF")
	os.Setenv("LOG_OUTPUT", "file")
	os.Setenv("LOG_FILE_PATH", "test_logs/json_off.log")
	os.Setenv("LOG_FORMAT", "json")
	ReloadLoggerConfig()

	Info("should not appear")
	Warn("should not appear")

	data, _ := os.ReadFile(logFilePath)
	if len(data) > 0 {
		t.Errorf("Expected empty file when LOG_LEVEL=OFF, got %d bytes", len(data))
	}
}
