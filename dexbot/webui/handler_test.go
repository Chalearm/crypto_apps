/******************************************************************************
 * File Name       : handler_test.go
 * File Path       : webui/handler_test.go
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
 *   Unit tests for webui package. 5 positive + 2 negative test cases per coding rule §2. go test ./webui -v - Created during Phase 10 reorganization. - All test functions below.
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
 *   [Test Functions] Test suite: TestOperationsPageRenders, TestTrainingPageRenders, TestPortfolioPageRenders, TestAPIDaemonsReturnsJSON
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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dexbot/governance"
)

// ==============================
// SETUP
// ==============================

func newTestServer() *Server {
	reg := governance.NewRegistry()
	reg.Register(&governance.DaemonInfo{
		Name: "governance", Version: "v2.0", Status: "healthy",
		CPUPercent: 3.5, MemoryMB: 96.0, StorageMB: 512.0,
		ActiveTasks: 1, Message: "Running",
	})
	reg.Register(&governance.DaemonInfo{
		Name: "school", Version: "v1.4", Status: "healthy",
		CPUPercent: 7.2, MemoryMB: 180.0, StorageMB: 1024.0,
		ActiveTasks: 4, Message: "School daemon running",
	})
	return NewServer(reg, 0)
}

// ==============================
// POSITIVE TESTS
// ==============================

/*
Function: TestOperationsPageRenders
Description:
  Positive: Operations page returns 200 with daemon names.

Lines: ~15
*/
func TestOperationsPageRenders(t *testing.T) {
	srv := newTestServer()
	_ = httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	srv.renderer.Operations(rec)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "governance") || !strings.Contains(body, "school") {
		t.Error("Expected daemon names in page body")
	}
	if !strings.Contains(body, "Operations") {
		t.Error("Expected Operations title")
	}
}

/*
Function: TestTrainingPageRenders
Description:
  Positive: Training page returns 200 with model names.

Lines: ~15
*/
func TestTrainingPageRenders(t *testing.T) {
	srv := newTestServer()
	_ = httptest.NewRequest("GET", "/training", nil)
	rec := httptest.NewRecorder()

	srv.renderer.SchoolDashboard(rec)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Primary School") || !strings.Contains(body, "Middle School") {
		t.Error("Expected 4-tier school page content")
	}
}

/*
Function: TestPortfolioPageRenders
Description:
  Positive: Portfolio page returns 200 with transaction data.

Lines: ~15
*/
func TestPortfolioPageRenders(t *testing.T) {
	srv := newTestServer()
	_ = httptest.NewRequest("GET", "/portfolio", nil)
	rec := httptest.NewRecorder()

	srv.renderer.Portfolio(rec)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Portfolio") && !strings.Contains(body, "Active") {
		t.Error("Expected portfolio page content")
	}
}

/*
Function: TestAPIDaemonsReturnsJSON
Description:
  Positive: GET /api/daemons returns valid JSON with daemon list.

Lines: ~20
*/
func TestAPIDaemonsReturnsJSON(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/daemons", nil)
	rec := httptest.NewRecorder()

	srv.apiDaemonsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Expected JSON content type, got %s", ct)
	}

	var out struct {
		Daemons []*governance.DaemonInfo `json:"daemons"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	if len(out.Daemons) != 2 {
		t.Errorf("Expected 2 daemons in JSON, got %d", len(out.Daemons))
	}
}

/*
Function: TestAPIDaemonActionFiresCallback
Description:
  Positive: POST /api/daemon/school/restart calls OnAction callback.

Lines: ~20
*/
func TestAPIDaemonActionFiresCallback(t *testing.T) {
	srv := newTestServer()
	called := make(chan [2]string, 1)
	srv.OnAction = func(name, action string) {
		called <- [2]string{name, action}
	}

	req := httptest.NewRequest("POST", "/api/daemon/school/restart", nil)
	rec := httptest.NewRecorder()
	srv.apiDaemonActionHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	select {
	case pair := <-called:
		if pair[0] != "school" || pair[1] != "restart" {
			t.Errorf("Expected school/restart, got %s/%s", pair[0], pair[1])
		}
	default:
		t.Error("OnAction was not called")
	}
}

// ==============================
// NEGATIVE TESTS
// ==============================

/*
Function: TestAPIDaemonActionRejectsGET
Description:
  Negative: GET /api/daemon/x/restart returns 405 Method Not Allowed.

Lines: ~12
*/
func TestAPIDaemonActionRejectsGET(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/api/daemon/school/restart", nil)
	rec := httptest.NewRecorder()

	srv.apiDaemonActionHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", rec.Code)
	}
}

/*
Function: TestAPIDaemonActionUnknownPath
Description:
  Negative: POST /api/daemon/ (missing action) returns 400.

Lines: ~12
*/
func TestAPIDaemonActionUnknownPath(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("POST", "/api/daemon/school", nil)
	rec := httptest.NewRecorder()

	srv.apiDaemonActionHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rec.Code)
	}
}

/*
Function: TestRendererSetPorts
Description:
  Positive: SetPorts updates port values.

Lines: ~10
*/
func TestRendererSetPorts(t *testing.T) {
	srv := newTestServer()
	r := srv.Renderer()
	r.SetPorts(9090, 9091, 9092, 9093)

	if r.govPort != 9090 || r.webPort != 9093 {
		t.Errorf("Expected ports updated, got gov=%d web=%d", r.govPort, r.webPort)
	}
}
