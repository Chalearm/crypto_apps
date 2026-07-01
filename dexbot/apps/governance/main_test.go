/******************************************************************************
 * File Name       : main_test.go
 * File Path       : apps/governance/main_test.go
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
 *   Unit tests for the Governance daemon v2.0, using shared governance.Registry per Phase 8 reorganization (myreq2.txt §3). Tests: - loadEnvSmart - UDP listener + heartbeat/legacy message parsing - UDP pr
 *
 * Responsibilities:
 *   - Implement core functionality for apps package.
 *
 * Usage :
 *   Directory : apps/governance/
 *
 *   Build :
 *     go build ./apps/governance
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./apps/governance
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
 *   [Test Functions] Test suite: TestLoadEnvSmart, TestStartUdpListenerAndMessageParsing, TestHeartbeatFormatParsing, TestSendUdpProbe
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
	"strings"
	"sync"
	"testing"
	"time"

	"dexbot/governance"
	"dexbot/infra"
)

// Helper functions
func setEnv(key, value string) { os.Setenv(key, value) }
func clearEnv(key string)      { os.Unsetenv(key) }

// mockUdpListener simulates a UDP listener responding to health probes.
func mockUdpListener(t *testing.T, port int, response string, ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	addr := net.UDPAddr{Port: port, IP: net.ParseIP("127.0.0.1")}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		t.Errorf("Mock listener: Failed to listen on UDP port %d: %v", port, err)
		return
	}
	defer conn.Close()

	buffer := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, remoteAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				t.Logf("Mock listener on port %d: UDP read error: %v", port, err)
				continue
			}
			if strings.Contains(string(buffer[:n]), "governance:probe:health_check") {
				conn.WriteToUDP([]byte(response), remoteAddr)
			}
		}
	}
}

// Test ports — use different ports from running daemons to avoid conflicts.
// When tests run inside the container alongside live daemons, the daemon ports
// (8081/8082/8083) are already bound. Tests use offset ports instead.
const (
	testGovPort    = 18081
	testSchoolPort = 18082
	testTradingPort = 18083
)

func TestLoadEnvSmart(t *testing.T) {
	dummyConfigPath := "config.env"
	originalContent := ""
	if content, err := os.ReadFile(dummyConfigPath); err == nil {
		originalContent = string(content)
	}
	defer os.WriteFile(dummyConfigPath, []byte(originalContent), 0644)
	defer os.Remove(dummyConfigPath)

	err := os.WriteFile(dummyConfigPath, []byte("TEST_ENV_VAR=test_value_from_config_env"), 0644)
	if err != nil {
		t.Fatalf("Failed to create dummy config.env: %v", err)
	}
	clearEnv("TEST_ENV_VAR")
	loadEnvSmart()
	if os.Getenv("TEST_ENV_VAR") != "test_value_from_config_env" {
		t.Errorf("loadEnvSmart did not load TEST_ENV_VAR correctly. Got: %s", os.Getenv("TEST_ENV_VAR"))
	}
	clearEnv("TEST_ENV_VAR")
}

/*
Function: TestStartUdpListenerAndMessageParsing
Description:
  Tests UDP listener with legacy 3-field messages and full heartbeat format.
  Verifies registry is updated correctly.

Lines: ~80
*/
func TestStartUdpListenerAndMessageParsing(t *testing.T) {
	infra.InitLogger()
	registry = governance.NewRegistry() // init registry for test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	governancePort = UDP_GOVERNANCE_PORT
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		startUdpListener(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: UDP_GOVERNANCE_PORT})
	if err != nil {
		t.Fatalf("Failed to dial UDP for testing: %v", err)
	}
	defer conn.Close()

	testCases := []struct {
		name         string
		message      string
		wantHealthy  bool
		wantMsg      string
	}{
		{"School Healthy Legacy", "school:healthy:School daemon is operational.", true, "School daemon is operational."},
		{"Trading Unhealthy Legacy", "trading:unhealthy:Trading loop encountered an error.", false, "Trading loop encountered an error."},
		{"Database Critical Legacy", "database:critical:Database connection lost.", false, "Database connection lost."},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			conn.Write([]byte(tc.message))
			time.Sleep(100 * time.Millisecond)

			parts := strings.SplitN(tc.message, ":", 3)
			info := registry.GetStatus(parts[0])
			if info == nil {
				t.Errorf("Expected %s to be registered", parts[0])
				return
			}
			if info.IsHealthy() != tc.wantHealthy {
				t.Errorf("Expected healthy=%v, got %v", tc.wantHealthy, info.IsHealthy())
			}
			if info.Message != tc.wantMsg {
				t.Errorf("Expected msg=%q, got %q", tc.wantMsg, info.Message)
			}
		})
	}

	// Test full heartbeat format
	t.Run("Full Heartbeat Format", func(t *testing.T) {
		hb := "testdaemon:v1.0:healthy:5.2:64.0:128.0:3:120:Test daemon ok"
		conn.Write([]byte(hb))
		time.Sleep(100 * time.Millisecond)

		info := registry.GetStatus("testdaemon")
		if info == nil {
			t.Fatal("Expected testdaemon in registry")
		}
		if info.Version != "v1.0" {
			t.Errorf("Expected version=v1.0, got %s", info.Version)
		}
		if info.CPUPercent != 5.2 {
			t.Errorf("Expected CPU=5.2, got %.1f", info.CPUPercent)
		}
		if info.MemoryMB != 64.0 {
			t.Errorf("Expected Memory=64.0, got %.1f", info.MemoryMB)
		}
		if info.ActiveTasks != 3 {
			t.Errorf("Expected ActiveTasks=3, got %d", info.ActiveTasks)
		}
	})

	cancel()
	wg.Wait()
}

/*
Function: TestHeartbeatFormatParsing
Description:
  Tests governance.ParseHeartbeat and FormatHeartbeat round-trip.

Lines: ~20
*/
func TestHeartbeatFormatParsing(t *testing.T) {
	raw := "school:v1.0:healthy:12.5:256.0:1024.0:5:3600:All ok"
	info, err := governance.ParseHeartbeat(raw)
	if err != nil {
		t.Fatalf("ParseHeartbeat failed: %v", err)
	}
	if info.Name != "school" || info.Status != "healthy" {
		t.Errorf("Parse mismatch: name=%s status=%s", info.Name, info.Status)
	}

	formatted := governance.FormatHeartbeat(info)
	if !strings.Contains(formatted, "school:v1.0:healthy:") {
		t.Errorf("Format mismatch: %s", formatted)
	}

	// Round-trip
	info2, err := governance.ParseHeartbeat(formatted)
	if err != nil {
		t.Fatalf("Round-trip parse failed: %v", err)
	}
	if info2.Name != "school" {
		t.Errorf("Round-trip name mismatch: %s", info2.Name)
	}
}

/*
Function: TestSendUdpProbe
Description:
  Tests sendUdpProbe with mock listener.

Lines: ~40
*/
func TestSendUdpProbe(t *testing.T) {
	infra.InitLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go mockUdpListener(t, UDP_SCHOOL_PORT, "school:pong:healthy", ctx, &wg)
	time.Sleep(100 * time.Millisecond)

	probeConn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: UDP_SCHOOL_PORT})
	if err != nil {
		t.Fatalf("Failed to dial UDP: %v", err)
	}
	defer probeConn.Close()

	oldSendUdpProbeFunc := sendUdpProbeFunc
	defer func() { sendUdpProbeFunc = oldSendUdpProbeFunc }()
	sendUdpProbeFunc = sendUdpProbeReal

	response, err := sendUdpProbeFunc(probeConn, "governance:probe:health_check", 200*time.Millisecond)
	if err != nil {
		t.Errorf("Unexpected probe error: %v", err)
	}
	if response != "school:pong:healthy" {
		t.Errorf("Expected school:pong:healthy, got %s", response)
	}

	// Timeout test
	noConn, _ := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9999})
	if noConn != nil {
		defer noConn.Close()
		resp, err := sendUdpProbeFunc(noConn, "ping", 100*time.Millisecond)
		if err != nil {
			t.Logf("No-response error (expected): %v", err)
		}
		if resp != "" {
			t.Errorf("Expected empty on timeout, got %s", resp)
		}
	}

	cancel()
	wg.Wait()
}

/*
Function: TestStartHealthCheckLoop
Description:
  Tests health check loop with mock UDP connections and registry.

Lines: ~130
*/
func TestStartHealthCheckLoop(t *testing.T) {
	infra.InitLogger()
	registry = governance.NewRegistry() // init for test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)
	schoolMockResponse := "school:pong:healthy"
	tradingMockResponse := "trading:pong:healthy"

	go mockUdpListener(t, UDP_SCHOOL_PORT, schoolMockResponse, ctx, &wg)
	go mockUdpListener(t, UDP_TRADING_PORT, tradingMockResponse, ctx, &wg)
	time.Sleep(200 * time.Millisecond)

	oldSchoolUdpConn := schoolUdpConn
	oldTradingUdpConn := tradingUdpConn
	defer func() {
		schoolUdpConn = oldSchoolUdpConn
		tradingUdpConn = oldTradingUdpConn
		if oldSchoolUdpConn != nil {
			oldSchoolUdpConn.Close()
		}
		if oldTradingUdpConn != nil {
			oldTradingUdpConn.Close()
		}
	}()

	var err error
	schoolAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", UDP_SCHOOL_PORT))
	schoolUdpConn, err = net.DialUDP("udp", nil, schoolAddr)
	if err != nil {
		t.Fatalf("Failed to dial School UDP: %v", err)
	}
	tradingAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", testTradingPort))
	tradingUdpConn, err = net.DialUDP("udp", nil, tradingAddr)
	if err != nil {
		t.Fatalf("Failed to dial Trading UDP: %v", err)
	}

	oldRecreateDaemonFunc := recreateDaemonFunc
	oldSendUdpProbeFunc := sendUdpProbeFunc
	recreateCalled := make(chan string, 5)

	recreateDaemonFunc = func(daemonName string, daemonPath string) {
		recreateCalled <- daemonName
		// Set mock response so NEXT health check sees healthy — do NOT register
		// directly, as the health check loop will overwrite it based on probe results.
		switch daemonName {
		case "school":
			schoolMockResponse = "school:pong:healthy"
		case "trading":
			tradingMockResponse = "trading:pong:healthy"
		}
	}
	defer func() {
		recreateDaemonFunc = oldRecreateDaemonFunc
		sendUdpProbeFunc = oldSendUdpProbeFunc
	}()

	sendUdpProbeFunc = func(conn *net.UDPConn, message string, timeout time.Duration) (string, error) {
		if strings.Contains(message, "governance:probe:health_check") {
			remoteAddr, ok := conn.RemoteAddr().(*net.UDPAddr)
			if ok {
				switch remoteAddr.Port {
				case testSchoolPort:
					if schoolMockResponse != "" {
						return schoolMockResponse, nil
					}
				case testTradingPort:
					if tradingMockResponse != "" {
						return tradingMockResponse, nil
					}
				}
			}
		}
		return "", nil
	}

	recreateThreshold = 0
	registry.Register(&governance.DaemonInfo{Name: "school", Version: "?", Status: "unhealthy", Message: "Initial"})
	registry.Register(&governance.DaemonInfo{Name: "trading", Version: "?", Status: "unhealthy", Message: "Initial"})

	os.Setenv("HEALTH_CHECK_INTERVAL_SECONDS", "1")
	defer clearEnv("HEALTH_CHECK_INTERVAL_SECONDS")

	var healthCheckWg sync.WaitGroup
	healthCheckWg.Add(1)
	go func() {
		defer healthCheckWg.Done()
		startHealthCheckLoop(ctx)
	}()

	// Case 1: Healthy
	t.Run("Healthy Daemons", func(t *testing.T) {
		schoolMockResponse = "school:pong:healthy"
		tradingMockResponse = "trading:pong:healthy"
		time.Sleep(2 * time.Second)

		s := registry.GetStatus("school")
		tr := registry.GetStatus("trading")
		if s == nil || !s.IsHealthy() {
			t.Errorf("School should be healthy: %+v", s)
		}
		if tr == nil || !tr.IsHealthy() {
			t.Errorf("Trading should be healthy: %+v", tr)
		}
		select {
		case daemon := <-recreateCalled:
			t.Errorf("Recreate called unexpectedly for %s", daemon)
		default:
		}
	})

	// Case 2: School unresponsive → recreated
	t.Run("School Unresponsive and Recreated", func(t *testing.T) {
		schoolMockResponse = ""
		tradingMockResponse = "trading:pong:healthy"
		si := registry.GetStatus("school")
		si.LastHeartbeat = time.Now().Add(-2 * time.Minute)
		registry.Register(si)

		select {
		case daemon := <-recreateCalled:
			if daemon != "school" {
				t.Errorf("Expected 'school' recreated, got %s", daemon)
			}
			// Wait for next health check to pick up healthy probe response
			time.Sleep(2 * time.Second)
			s := registry.GetStatus("school")
			if s == nil || !s.IsHealthy() {
				t.Errorf("School should be healthy after recreation: %+v", s)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for school recreation")
		}
	})

	// Case 3: Trading unresponsive → recreated
	t.Run("Trading Unresponsive and Recreated", func(t *testing.T) {
		schoolMockResponse = "school:pong:healthy"
		tradingMockResponse = ""
		ti := registry.GetStatus("trading")
		ti.LastHeartbeat = time.Now().Add(-2 * time.Minute)
		registry.Register(ti)

		select {
		case daemon := <-recreateCalled:
			if daemon != "trading" {
				t.Errorf("Expected 'trading' recreated, got %s", daemon)
			}
			time.Sleep(2 * time.Second)
			tr := registry.GetStatus("trading")
			if tr == nil || !tr.IsHealthy() {
				t.Errorf("Trading should be healthy after recreation: %+v", tr)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for trading recreation")
		}
	})

	cancel()
	healthCheckWg.Wait()
	wg.Wait()
}
