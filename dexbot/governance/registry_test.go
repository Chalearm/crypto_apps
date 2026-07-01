/******************************************************************************
 * File Name       : registry_test.go
 * File Path       : governance/registry_test.go
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
 *   Unit tests for governance registry types and heartbeat parsing. 5 positive + 2 negative test cases per coding rule §2. go test ./governance -v - Created during Phase 7 reorganization. - All test funct
 *
 * Responsibilities:
 *   - Implement core functionality for governance package.
 *
 * Usage :
 *   Directory : governance/
 *
 *   Build :
 *     go build ./governance
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./governance
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/governance
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
 *   [Test Functions] Test suite: TestDaemonInfoIsHealthy, TestDaemonInfoNotHealthy, TestRecordRestart, TestRecordRestartHistoryCap
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
package governance

import (
	"strings"
	"testing"
	"time"
)

// ==============================
// DAEMON INFO TESTS
// ==============================

/*
Function: TestDaemonInfoIsHealthy
Description:
  Positive: IsHealthy returns true when Status == "healthy".
Lines: ~10
*/
func TestDaemonInfoIsHealthy(t *testing.T) {
	d := &DaemonInfo{Name: "school", Status: "healthy"}
	if !d.IsHealthy() {
		t.Error("Expected IsHealthy to return true for healthy status")
	}
}

/*
Function: TestDaemonInfoNotHealthy
Description:
  Positive: IsHealthy returns false for non-healthy statuses.
Lines: ~15
*/
func TestDaemonInfoNotHealthy(t *testing.T) {
	cases := []string{"unhealthy", "starting", "stopping", "critical"}
	for _, s := range cases {
		d := &DaemonInfo{Name: "test", Status: s}
		if d.IsHealthy() {
			t.Errorf("Expected IsHealthy=false for status %q, got true", s)
		}
	}
}

/*
Function: TestRecordRestart
Description:
  Positive: RecordRestart increments counter and adds timestamp.
Lines: ~15
*/
func TestRecordRestart(t *testing.T) {
	d := &DaemonInfo{Name: "trading"}
	for i := 0; i < 3; i++ {
		d.RecordRestart()
	}
	if d.RestartCount != 3 {
		t.Errorf("Expected RestartCount=3, got %d", d.RestartCount)
	}
	if len(d.RestartHistory) != 3 {
		t.Errorf("Expected 3 restart history entries, got %d", len(d.RestartHistory))
	}
}

/*
Function: TestRecordRestartHistoryCap
Description:
  Positive: RestartHistory is capped at 10 entries.
Lines: ~15
*/
func TestRecordRestartHistoryCap(t *testing.T) {
	d := &DaemonInfo{Name: "school"}
	for i := 0; i < 15; i++ {
		d.RecordRestart()
		time.Sleep(1 * time.Millisecond) // ensure unique timestamps
	}
	if d.RestartCount != 15 {
		t.Errorf("Expected RestartCount=15, got %d", d.RestartCount)
	}
	if len(d.RestartHistory) > 10 {
		t.Errorf("Expected RestartHistory capped at 10, got %d", len(d.RestartHistory))
	}
}

// ==============================
// REGISTRY TESTS
// ==============================

/*
Function: TestRegistryRegisterAndList
Description:
  Positive: Register adds entries, List returns all names.
Lines: ~20
*/
func TestRegistryRegisterAndList(t *testing.T) {
	r := NewRegistry()
	r.Register(&DaemonInfo{Name: "governance", Status: "healthy"})
	r.Register(&DaemonInfo{Name: "school", Status: "healthy"})
	r.Register(&DaemonInfo{Name: "trading", Status: "starting"})

	names := r.List()
	if len(names) != 3 {
		t.Errorf("Expected 3 daemons registered, got %d", len(names))
	}

	school := r.GetStatus("school")
	if school == nil {
		t.Fatal("Expected school to be registered")
	}
	if !school.IsHealthy() {
		t.Error("Expected school to be healthy")
	}
}

/*
Function: TestRegistryUnregister
Description:
  Positive: Unregister removes entry, GetStatus returns nil.
Lines: ~12
*/
func TestRegistryUnregister(t *testing.T) {
	r := NewRegistry()
	r.Register(&DaemonInfo{Name: "testdaemon", Status: "healthy"})
	r.Unregister("testdaemon")

	if got := r.GetStatus("testdaemon"); got != nil {
		t.Error("Expected nil after unregister")
	}
}

/*
Function: TestRegistryAllHealthy
Description:
  Positive: AllHealthy returns true only when all are healthy.
Lines: ~15
*/
func TestRegistryAllHealthy(t *testing.T) {
	r := NewRegistry()
	r.Register(&DaemonInfo{Name: "a", Status: "healthy"})
	r.Register(&DaemonInfo{Name: "b", Status: "healthy"})
	if !r.AllHealthy() {
		t.Error("Expected AllHealthy=true when all are healthy")
	}

	r.Register(&DaemonInfo{Name: "c", Status: "unhealthy"})
	if r.AllHealthy() {
		t.Error("Expected AllHealthy=false when one is unhealthy")
	}
}

/*
Function: TestRegistryHistory
Description:
  Positive: History returns stored historical records.
Lines: ~15
*/
func TestRegistryHistory(t *testing.T) {
	r := NewRegistry()
	for i := 0; i < 5; i++ {
		r.Register(&DaemonInfo{Name: "school", Status: "healthy"})
	}
	h := r.History(0)
	if len(h) != 5 {
		t.Errorf("Expected 5 history entries, got %d", len(h))
	}

	h3 := r.History(3)
	if len(h3) != 3 {
		t.Errorf("Expected 3 limited history entries, got %d", len(h3))
	}
}

// ==============================
// HEARTBEAT TESTS
// ==============================

/*
Function: TestParseHeartbeatValid
Description:
  Positive: ParseHeartbeat correctly parses a full heartbeat message.
Lines: ~25
*/
func TestParseHeartbeatValid(t *testing.T) {
	raw := "school:v1.0:healthy:12.5:256.0:1024.0:5:3600:All systems operational"
	info, err := ParseHeartbeat(raw)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if info.Name != "school" {
		t.Errorf("Expected Name='school', got '%s'", info.Name)
	}
	if info.Version != "v1.0" {
		t.Errorf("Expected Version='v1.0', got '%s'", info.Version)
	}
	if info.Status != "healthy" {
		t.Errorf("Expected Status='healthy', got '%s'", info.Status)
	}
	if info.CPUPercent != 12.5 {
		t.Errorf("Expected CPU=12.5, got %.2f", info.CPUPercent)
	}
	if info.MemoryMB != 256.0 {
		t.Errorf("Expected Memory=256.0, got %.2f", info.MemoryMB)
	}
	if info.ActiveTasks != 5 {
		t.Errorf("Expected ActiveTasks=5, got %d", info.ActiveTasks)
	}
	if info.Message != "All systems operational" {
		t.Errorf("Expected Message, got '%s'", info.Message)
	}
}

/*
Function: TestParseHeartbeatMalformed
Description:
  Negative: ParseHeartbeat returns error for too few fields.
Lines: ~10
*/
func TestParseHeartbeatMalformed(t *testing.T) {
	raw := "school:healthy" // only 2 fields, need 8+
	_, err := ParseHeartbeat(raw)
	if err == nil {
		t.Error("Expected error for malformed heartbeat, got nil")
	}
}

/*
Function: TestParseHeartbeatEmpty
Description:
  Negative: ParseHeartbeat returns error for empty input.
Lines: ~8
*/
func TestParseHeartbeatEmpty(t *testing.T) {
	_, err := ParseHeartbeat("")
	if err == nil {
		t.Error("Expected error for empty heartbeat, got nil")
	}
}

/*
Function: TestFormatHeartbeat
Description:
  Positive: FormatHeartbeat produces correctly formatted message.
Lines: ~15
*/
func TestFormatHeartbeat(t *testing.T) {
	info := &DaemonInfo{
		Name: "trading", Version: "v1.3", Status: "healthy",
		CPUPercent: 8.3, MemoryMB: 128.0, StorageMB: 512.0,
		ActiveTasks: 12, Uptime: 7200 * time.Second,
		Message: "Trading daemon running",
	}
	raw := FormatHeartbeat(info)

	if !strings.HasPrefix(raw, "trading:v1.3:healthy:") {
		t.Errorf("Unexpected heartbeat format: %s", raw)
	}
	// Round-trip: parse back
	parsed, err := ParseHeartbeat(raw)
	if err != nil {
		t.Fatalf("Round-trip parse failed: %v", err)
	}
	if parsed.Name != "trading" {
		t.Errorf("Round-trip name mismatch: %s", parsed.Name)
	}
}

/*
Function: TestCommanderUnknownAction
Description:
  Negative: Dispatch returns error for unregistered action.
Lines: ~10
*/
func TestCommanderUnknownAction(t *testing.T) {
	c := NewCommander()
	_, err := c.Dispatch("nonexistent", nil)
	if err == nil {
		t.Error("Expected error for unknown action, got nil")
	}
}

/*
Function: TestCommanderRegisteredAction
Description:
  Positive: Dispatch routes to registered handler correctly.
Lines: ~12
*/
func TestCommanderRegisteredAction(t *testing.T) {
	c := NewCommander()
	c.Register("ping", func(args map[string]string) (string, error) {
		return "pong", nil
	})

	result, err := c.Dispatch("ping", nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result != "pong" {
		t.Errorf("Expected 'pong', got '%s'", result)
	}
}

/*
Function: TestValidateActionValid
Description:
  Positive: ValidateAction returns nil for all known actions.
Lines: ~10
*/
func TestValidateActionValid(t *testing.T) {
	for _, a := range AllActions() {
		if err := ValidateAction(a); err != nil {
			t.Errorf("Expected valid action %q, got error: %v", a, err)
		}
	}
}

/*
Function: TestValidateActionInvalid
Description:
  Negative: ValidateAction returns error for unknown action.
Lines: ~8
*/
func TestValidateActionInvalid(t *testing.T) {
	err := ValidateAction("fly-to-moon")
	if err == nil {
		t.Error("Expected error for invalid action, got nil")
	}
}
