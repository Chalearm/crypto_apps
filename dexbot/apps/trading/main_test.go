/******************************************************************************
 * File Name       : main_test.go
 * File Path       : apps/trading/main_test.go
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
 *   Unit tests for the Trading daemon, covering its core functionalities such as environment variable loading, configuration management, UDP communication, including graceful shutdowns and health probe re
 *
 * Responsibilities:
 *   - Implement core functionality for apps package.
 *
 * Usage :
 *   Directory : apps/trading/
 *
 *   Build :
 *     go build ./apps/trading
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./apps/trading
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/apps
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
 *   [Test Functions] Test suite: TestLoadEnvSmart, TestLoadAndWriteConfig, TestSendStatusToGovernance, TestStartUdpListenerGracefulShutdown
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
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"dexbot/infra"
)

// ==============================
// HELPER FUNCTIONS FOR TESTS
// ==============================

// setEnv sets an environment variable for testing purposes.
func setEnv(key, value string) {
	os.Setenv(key, value)
}

// clearEnv unsets an environment variable after testing.
func clearEnv(key string) {
	os.Unsetenv(key)
}

// captureUDP captures UDP messages sent to a specific port, respecting a context for shutdown.
func captureUDP(t *testing.T, port int, ctx context.Context) chan string {
	messages := make(chan string, 10)
	done := make(chan struct{})
	addr := net.UDPAddr{
		Port: port,
		IP:   net.ParseIP("127.0.0.1"),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		t.Fatalf("Failed to listen on UDP port %d: %v", port, err)
	}

	go func() {
		defer conn.Close()
		defer close(done)
		buffer := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, _, err := conn.ReadFromUDP(buffer)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					t.Logf("UDP read error (non-fatal for test): %v", err)
					continue
				}
				messages <- string(buffer[:n])
			}
		}
	}()
	t.Cleanup(func() { <-done }) // wait for listener goroutine to exit
	return messages
}

// ==============================
// TEST SUITE
// ==============================

/*
Function: TestLoadEnvSmart
Description:
  Tests the `loadEnvSmart` function to ensure it correctly loads environment variables
  from the `config.env` file using the predefined paths.
Input:
  - t *testing.T: The testing framework's T object.
Output:
  - none
Lines: ~20
*/
func TestLoadEnvSmart(t *testing.T) {
	// Create a dummy config.env file for testing
	dummyConfigPath := "config.env" // Relative to trading/ cwd — checked first by loadEnvSmart
	originalContent := ""
	if content, err := os.ReadFile(dummyConfigPath); err == nil {
		originalContent = string(content)
	}
	defer os.WriteFile(dummyConfigPath, []byte(originalContent), 0644) // Restore original content
	defer os.Remove(dummyConfigPath)                                // Clean up dummy file

	dummyContent := "TEST_ENV_VAR=test_value_from_trading_config_env"
	err := os.WriteFile(dummyConfigPath, []byte(dummyContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create dummy config.env: %v", err)
	}

	clearEnv("TEST_ENV_VAR") // Clear any existing env var

	loadEnvSmart()

	if os.Getenv("TEST_ENV_VAR") != "test_value_from_trading_config_env" {
		t.Errorf("loadEnvSmart did not load TEST_ENV_VAR correctly. Got: %s", os.Getenv("TEST_ENV_VAR"))
	}
	clearEnv("TEST_ENV_VAR") // Clean up
}

/*
Function: TestLoadAndWriteConfig
Description:
  Tests `loadConfig` and `writeConfig` functions to ensure that configuration
  can be loaded, modified, and persisted correctly for the trading daemon.
Input:
  - t *testing.T: The testing framework's T object.
Output:
  - none
Lines: ~30
*/
func TestLoadAndWriteConfig(t *testing.T) {
	// Ensure the config file is clean before and after tests
	_ = os.Remove(CONFIG_FILE) // start fresh
	originalConfigContent := ""
	if content, err := os.ReadFile(CONFIG_FILE); err == nil {
		originalConfigContent = string(content)
	}
	defer os.WriteFile(CONFIG_FILE, []byte(originalConfigContent), 0644)
	defer os.Remove(CONFIG_FILE)

	// Test default config creation
	cfg := loadConfig()
	if cfg.MaxTasks != 10 { // default pulled from config.env / config.Defaults()
		t.Errorf("Default config not loaded correctly. Expected MaxTasks 10, Got %d", cfg.MaxTasks)
	}

	// Test writing and reloading config
	cfg.MaxTasks = 5
	writeConfig(cfg)

	loadedCfg := loadConfig()
	if loadedCfg.MaxTasks != 5 {
		t.Errorf("Config not written/loaded correctly. Expected MaxTasks 5, Got %d", loadedCfg.MaxTasks)
	}
}

/*
Function: TestSendStatusToGovernance
Description:
  Tests the `sendStatusToGovernance` function by capturing UDP messages sent to the
  Governance daemon's port and verifying their content.
Input:
  - t *testing.T: The testing framework's T object.
Output:
  - none
Lines: ~35
*/
func TestSendStatusToGovernance(t *testing.T) {
	infra.InitLogger()

	// Temporarily redirect udpConn for testing
	oldUdpConn := udpConn
	oldGovernanceAddr := governanceAddr
	defer func() {
		udpConn = oldUdpConn
		governanceAddr = oldGovernanceAddr
		if oldUdpConn != nil {
			oldUdpConn.Close()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Capture messages on the governance port (simulating governance listener)
	capturedMessages := captureUDP(t, UDP_GOVERNANCE_PORT, ctx)

	// Re-initialize udpConn to point to our test listener
	var err error
	governanceAddr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", UDP_GOVERNANCE_PORT))
	if err != nil {
		t.Fatalf("Failed to resolve UDP address: %v", err)
	}
	udpConn, err = net.DialUDP("udp", nil, governanceAddr)
	if err != nil {
		t.Fatalf("Failed to dial UDP: %v", err)
	}

	testMessage := "trading:healthy:Trading daemon is running."
	sendStatusToGovernance("trading", "healthy", "Trading daemon is running.")

	select {
	case msg := <-capturedMessages:
		if msg != testMessage {
			t.Errorf("Expected UDP message \"%s\", got \"%s\"", testMessage, msg)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Timeout waiting for UDP message")
	}
}

/*
Function: TestStartUdpListenerGracefulShutdown
Description:
  Tests the graceful shutdown mechanism of the `startUdpListener` function by sending
  a cancellation signal via context and verifying that the listener stops.
Input:
  - t *testing.T: The testing framework's T object.
Output:
  - none
Lines: ~30
*/
func TestStartUdpListenerGracefulShutdown(t *testing.T) {
	infra.InitLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		startUdpListener(ctx)
	}()

	// Give the listener a moment to start
	time.Sleep(100 * time.Millisecond)

	// Signal for graceful shutdown
	cancel()

	// Wait for the listener goroutine to finish with a timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success: listener shut down gracefully
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout: UDP listener did not shut down gracefully")
	}
}

/*
Function: TestRunTradingDaemonGracefulShutdown
Description:
  Tests the graceful shutdown mechanism of the `runTradingDaemon` function by sending
  a cancellation signal via context and verifying that the daemon stops.
Input:
  - t *testing.T: The testing framework's T object.
Output:
  - none
Lines: ~30
*/
func TestRunTradingDaemonGracefulShutdown(t *testing.T) {
	infra.InitLogger()

	ctx, cancel := context.WithCancel(context.Background())

	// Use a WaitGroup to wait for the daemon to shut down
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runTradingDaemon(ctx)
	}()

	// Give the daemon a moment to start
	time.Sleep(100 * time.Millisecond)

	// Simulate a signal to trigger graceful shutdown
	cancel()

	// Wait for the daemon to shut down. Use a timeout to prevent tests from hanging.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success: daemon shut down gracefully
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout: Trading daemon did not shut down gracefully")
	}
}

/*
Function: TestStartUdpListenerHealthProbeResponse
Description:
  Tests that the `startUdpListener` in the trading daemon correctly responds to
  health probe messages from the Governance daemon.
Input:
  - t *testing.T: The testing framework's T object.
Output:
  - none
Lines: ~40
*/
func TestStartUdpListenerHealthProbeResponse(t *testing.T) {
	infra.InitLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the trading daemon's UDP listener in a goroutine
	tradingPort = UDP_TRADING_PORT // ensure correct port
	var listenerWg sync.WaitGroup
	listenerWg.Add(1)
	go func() {
		defer listenerWg.Done()
		startUdpListener(ctx)
	}()

	// Give the listener a moment to start up
	time.Sleep(100 * time.Millisecond)

	// Create a UDP client (simulating Governance) to send a probe
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: UDP_TRADING_PORT})
	if err != nil {
		t.Fatalf("Failed to dial UDP for testing probe: %v", err)
	}
	defer conn.Close()

	probeMessage := "governance:probe:health_check"
	_, err = conn.Write([]byte(probeMessage))
	if err != nil {
		t.Fatalf("Failed to send probe message: %v", err)
	}

	// Wait for and capture the response from the trading daemon
	responseBuffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	n, _, err := conn.ReadFromUDP(responseBuffer)
	if err != nil {
		t.Fatalf("Failed to receive probe response: %v", err)
	}
	response := string(responseBuffer[:n])

	expectedResponse := "trading:pong:healthy"
	if response != expectedResponse {
		t.Errorf("Expected probe response \"%s\", got \"%s\"", expectedResponse, response)
	}

	cancel()
	listenerWg.Wait() // Wait for the listener to shut down
}
