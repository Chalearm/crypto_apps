/******************************************************************************
 * File Name       : integration_test.go
 * File Path       : webui/integration_test.go
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
 *   Subsystem integration tests for the webui package. Starts a real HTTP server on a random port and makes actual HTTP requests against it, validating full page responses. Covers: - GET  /               
 *
 * Responsibilities:
 *   - Implement core functionality for webui package.
 *
 * Usage :
 *   Directory : webui/
 *
 *   Build :
 *     go build ./webui
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./webui
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/webui
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
 *   [Test Functions] Test suite: TestIntegrationOperationsPage, TestIntegrationTrainingPage, TestIntegrationPortfolioPage, TestIntegrationAPIDaemonsJSON
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
package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"dexbot/governance"
)

// ==============================
// HELPER: start test server
// ==============================

/*
Function: startTestServer
Description:
  Creates a Server with mock registry data, binds to a random port,
  starts it in a background goroutine, and returns the base URL.
  The server is automatically closed via t.Cleanup.

Input:
  - t *testing.T : Test handle

Output:
  - string : Base URL (e.g., "http://127.0.0.1:54321")

Lines: ~25
*/
func startTestServer(t *testing.T) string {
	reg := governance.NewRegistry()
	reg.Register(&governance.DaemonInfo{
		Name: "governance", Version: "v2.0", Status: "healthy",
		CPUPercent: 3.5, MemoryMB: 96, StorageMB: 512, ActiveTasks: 1,
		Uptime: 2 * time.Hour, Message: "Operational",
	})
	reg.Register(&governance.DaemonInfo{
		Name: "school", Version: "v1.4", Status: "healthy",
		CPUPercent: 7.2, MemoryMB: 180, StorageMB: 1024, ActiveTasks: 4,
		Uptime: 3*time.Hour + 15*time.Minute, Message: "Recording market data",
		RestartCount: 2,
		RestartHistory: []time.Time{
			time.Now().Add(-4 * time.Hour),
			time.Now().Add(-1 * time.Hour),
		},
	})
	reg.Register(&governance.DaemonInfo{
		Name: "trading", Version: "v1.3", Status: "healthy",
		CPUPercent: 5.1, MemoryMB: 220, StorageMB: 2048, ActiveTasks: 3,
		Uptime: 1*time.Hour + 45*time.Minute, Message: "Trading loop active",
	})

	srv := NewServer(reg, 0) // 0 = system allocates port

	// Bind to a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind test server: %v", err)
	}

	// Build the mux manually (same as ListenAndServe but with our listener)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		srv.renderer.Operations(w)
	})
	mux.HandleFunc("/training", func(w http.ResponseWriter, r *http.Request) {
		srv.renderer.Training(w)
	})
	mux.HandleFunc("/portfolio", func(w http.ResponseWriter, r *http.Request) {
		srv.renderer.Portfolio(w)
	})
	mux.HandleFunc("/predict", func(w http.ResponseWriter, r *http.Request) {
		srv.renderer.PredictionComparison(w)
	})
	mux.HandleFunc("/api/daemons", srv.apiDaemonsHandler)
	mux.HandleFunc("/api/daemon/", srv.apiDaemonActionHandler)

	hs := &http.Server{Handler: mux}
	go hs.Serve(listener)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)

	t.Cleanup(func() {
		hs.Close()
		listener.Close()
	})

	// Give the server a moment
	time.Sleep(50 * time.Millisecond)
	return baseURL
}

// ==============================
// INTEGRATION TESTS
// ==============================

/*
Function: TestIntegrationOperationsPage
Description:
  Positive: GET / returns 200 with HTML containing daemon names and metrics.

Lines: ~20
*/
func TestIntegrationOperationsPage(t *testing.T) {
	base := startTestServer(t)

	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	// Should contain daemon names
	for _, name := range []string{"governance", "school", "trading"} {
		if !strings.Contains(s, name) {
			t.Errorf("Expected page to contain %q", name)
		}
	}
	// Should contain metric data
	if !strings.Contains(s, "3.5") {
		t.Error("Expected page to contain CPU metric 3.5")
	}
	if !strings.Contains(s, "Operations") {
		t.Error("Expected page title 'Operations'")
	}
	// Should contain restart badge
	if !strings.Contains(s, "restart") {
		t.Error("Expected restart badge for school daemon")
	}
}

/*
Function: TestIntegrationTrainingPage
Description:
  Positive: GET /training returns 200 with model data.

Lines: ~15
*/
func TestIntegrationTrainingPage(t *testing.T) {
	base := startTestServer(t)

	resp, err := http.Get(base + "/training")
	if err != nil {
		t.Fatalf("GET /training failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	for _, model := range []string{"School", "Primary", "Middle", "High", "Graduate"} {
		if !strings.Contains(s, model) {
			t.Errorf("Expected training page to contain %q", model)
		}
	}
}

/*
Function: TestIntegrationPortfolioPage
Description:
  Positive: GET /portfolio returns 200 with transaction data.

Lines: ~15
*/
func TestIntegrationPortfolioPage(t *testing.T) {
	base := startTestServer(t)

	resp, err := http.Get(base + "/portfolio")
	if err != nil {
		t.Fatalf("GET /portfolio failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	s := string(body)

	for _, token := range []string{"Portfolio", "Active", "Trade"} {
		if !strings.Contains(s, token) {
			t.Errorf("Expected portfolio to contain %q", token)
		}
	}
}

/*
Function: TestIntegrationAPIDaemonsJSON
Description:
  Positive: GET /api/daemons returns valid JSON with 3 daemons.

Lines: ~20
*/
func TestIntegrationAPIDaemonsJSON(t *testing.T) {
	base := startTestServer(t)

	resp, err := http.Get(base + "/api/daemons")
	if err != nil {
		t.Fatalf("GET /api/daemons failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var out struct {
		Daemons []*governance.DaemonInfo `json:"daemons"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}
	if len(out.Daemons) != 3 {
		t.Errorf("Expected 3 daemons, got %d", len(out.Daemons))
	}
	// Verify school has restart count
	for _, d := range out.Daemons {
		if d.Name == "school" && d.RestartCount != 2 {
			t.Errorf("Expected school restartCount=2, got %d", d.RestartCount)
		}
	}
}

/*
Function: TestIntegrationAPIDaemonActionFires
Description:
  Positive: POST /api/daemon/school/restart fires OnAction and returns JSON.

Lines: ~25
*/
func TestIntegrationAPIDaemonActionFires(t *testing.T) {
	reg := governance.NewRegistry()
	reg.Register(&governance.DaemonInfo{Name: "school", Version: "v1", Status: "healthy"})

	srv := NewServer(reg, 0)
	called := make(chan [2]string, 1)
	srv.OnAction = func(name, action string) {
		called <- [2]string{name, action}
	}

	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/daemon/", srv.apiDaemonActionHandler)
	hs := &http.Server{Handler: mux}
	go hs.Serve(listener)
	defer hs.Close()
	defer listener.Close()
	time.Sleep(50 * time.Millisecond)

	base := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)

	resp, err := http.Post(base+"/api/daemon/school/restart", "application/json", nil)
	if err != nil {
		t.Fatalf("POST restart failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	select {
	case pair := <-called:
		if pair[0] != "school" || pair[1] != "restart" {
			t.Errorf("Expected school/restart, got %s/%s", pair[0], pair[1])
		}
	case <-time.After(1 * time.Second):
		t.Error("OnAction was not called")
	}
}

/*
Function: TestIntegrationNotFound
Description:
  Negative: GET /nonexistent returns 404.

Lines: ~12
*/
func TestIntegrationNotFound(t *testing.T) {
	base := startTestServer(t)

	resp, err := http.Get(base + "/nonexistent")
	if err != nil {
		t.Fatalf("GET /nonexistent failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}
}

/*
Function: TestIntegrationInvalidAction
Description:
  Negative: POST /api/daemon/x/bad_action returns 400.

Lines: ~12
*/
func TestIntegrationInvalidAction(t *testing.T) {
	base := startTestServer(t)

	resp, err := http.Post(base+"/api/daemon/school/fly_to_moon", "application/json", nil)
	if err != nil {
		t.Fatalf("POST invalid action failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid action, got %d", resp.StatusCode)
	}
}
