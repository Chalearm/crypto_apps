/******************************************************************************
 * File Name       : registry.go
 * File Path       : governance/registry.go
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
 *   Shared governance types: daemon registry, heartbeat protocol, daemon status, model performance, transaction records. Extracted from apps/governance/main.go during Phase 7 reorganization (myreq2.txt §1
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
package governance

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ==============================
// DAEMON REGISTRY TYPES
// ==============================

/*
Struct: DaemonInfo
Description:
  Full heartbeat and status information for a registered daemon.
  Replaces the older DaemonStatus with expanded resource metrics
  per myreq2.txt §3.

Fields:
  - Name              string        : Daemon name (governance, school, trading, testdaemon)
  - Version           string        : Daemon version string
  - Status            string        : healthy, unhealthy, starting, stopping
  - Uptime            time.Duration : Time since last start
  - CPUPercent        float64       : CPU utilization (0.0-100.0)
  - MemoryMB          float64       : Memory usage in MB
  - StorageMB         float64       : Storage usage in MB
  - ActiveTasks       int           : Number of active tasks
  - LastCheckpoint    time.Time     : Timestamp of last state checkpoint
  - LastHeartbeat     time.Time     : Timestamp of most recent heartbeat
  - RestartCount      int           : Number of restarts since deployment
  - RestartHistory    []time.Time   : Timestamps of last 10 restarts
  - Message           string        : Human-readable status message

Lines: ~15
*/
type DaemonInfo struct {
	Name           string
	Version        string
	Status         string
	Uptime         time.Duration
	CPUPercent     float64
	MemoryMB       float64
	StorageMB      float64
	ActiveTasks    int
	LastCheckpoint time.Time
	LastHeartbeat  time.Time
	RestartCount   int
	RestartHistory []time.Time
	Message        string
}

/*
Function: IsHealthy
Description:
  Convenience method to check if daemon is in healthy state.

Input:
  - none

Output:
  - bool: true if Status == "healthy"

Lines: ~5
*/
func (d *DaemonInfo) IsHealthy() bool {
	return d.Status == "healthy"
}

/*
Function: RecordRestart
Description:
  Increments the restart counter and appends a restart timestamp.
  Trims RestartHistory to last 10 entries.

Input:
  - none

Output:
  - none

Lines: ~8
*/
func (d *DaemonInfo) RecordRestart() {
	d.RestartCount++
	d.RestartHistory = append(d.RestartHistory, time.Now())
	if len(d.RestartHistory) > 10 {
		d.RestartHistory = d.RestartHistory[len(d.RestartHistory)-10:]
	}
}

/******************************************************************************
 * Function Name : PostStatus
 *
 * Purpose :
 *   Allows external actors (test daemon, web action handler) to set a
 *   daemon's lifecycle status. Per myreq4.txt §86: status transitions
 *   through killing → building → recovering → healthy.
 *
 * Inputs :
 *   status
 *     Type        : string
 *     Range       : "killing", "building", "recovering", "healthy",
 *                   "unhealthy", "critical", "pass", "starting", "stopping"
 *     Description : Lifecycle status to apply to this daemon.
 *
 * Outputs :
 *   None (mutates DaemonInfo in-place)
 *
 * Return :
 *   None
 *
 * Error Cases :
 *   None — invalid status string is accepted but may display as unknown badge.
 *
 * Dependencies :
 *   None
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines : 7
 *
 * Notes :
 *   - Also updates LastHeartbeat to current time so the dashboard poll
 *     picks up the change immediately.
 ******************************************************************************/
func (d *DaemonInfo) PostStatus(status string) {
	d.Status = status
	d.LastHeartbeat = time.Now()
}

// ==============================
// REGISTRY
// ==============================

/*
Struct: Registry
Description:
  Thread-safe daemon registry. Tracks all registered daemons,
  their current status, and heartbeat history.

Fields:
  - mu      sync.RWMutex         : Protects concurrent access
  - daemons map[string]*DaemonInfo : Active daemon entries
  - history []DaemonInfo         : Historical status records (last 1000)

Lines: ~8
*/
type Registry struct {
	mu      sync.RWMutex
	daemons map[string]*DaemonInfo
	history []DaemonInfo
}

/*
Function: NewRegistry
Description:
  Creates and returns a new empty Registry.

Input:
  - none

Output:
  - *Registry: Initialized registry

Lines: ~8
*/
func NewRegistry() *Registry {
	return &Registry{
		daemons: make(map[string]*DaemonInfo),
	}
}

/*
Function: Register
Description:
  Adds or updates a daemon entry in the registry.

Input:
  - info *DaemonInfo : The daemon info to register

Output:
  - none

Lines: ~10
*/
func (r *Registry) Register(info *DaemonInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.daemons[info.Name]
	if exists {
		// Preserve restart history when updating
		info.RestartCount = entry.RestartCount
		info.RestartHistory = entry.RestartHistory
	}
	info.LastHeartbeat = time.Now()
	r.daemons[info.Name] = info

	// Append to history (cap at 1000)
	r.history = append(r.history, *info)
	if len(r.history) > 1000 {
		r.history = r.history[len(r.history)-1000:]
	}
}

/*
Function: Unregister
Description:
  Removes a daemon from the registry.

Input:
  - name string : Daemon name to remove

Output:
  - none

Lines: ~6
*/
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.daemons, name)
}

/*
Function: GetStatus
Description:
  Returns a copy of the daemon info for a given name.

Input:
  - name string : Daemon name

Output:
  - *DaemonInfo : Copy of daemon info, or nil if not found

Lines: ~10
*/
func (r *Registry) GetStatus(name string) *DaemonInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.daemons[name]
	if !ok {
		return nil
	}
	copy := *info
	return &copy
}

/*
Function: List
Description:
  Returns a slice of all registered daemon names.

Input:
  - none

Output:
  - []string : Alphabetically sorted daemon names

Lines: ~10
*/
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.daemons))
	for name := range r.daemons {
		names = append(names, name)
	}
	return names
}

/*
Function: AllHealthy
Description:
  Returns true if all registered daemons are healthy.

Input:
  - none

Output:
  - bool : True if every daemon has Status == "healthy"

Lines: ~10
*/
func (r *Registry) AllHealthy() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, d := range r.daemons {
		if !d.IsHealthy() {
			return false
		}
	}
	return true
}

/*
Function: History
Description:
  Returns a copy of the historical status records.

Input:
  - limit int : Max number of records to return (0 = all)

Output:
  - []DaemonInfo : Historical records, newest last

Lines: ~12
*/
func (r *Registry) History(limit int) []DaemonInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	h := r.history
	if limit > 0 && limit < len(h) {
		h = h[len(h)-limit:]
	}
	result := make([]DaemonInfo, len(h))
	copy(result, h)
	return result
}

// ==============================
// HEARTBEAT PARSING
// ==============================

/*
Function: ParseHeartbeat
Description:
  Parses a UDP heartbeat message into a DaemonInfo struct.
  Expected format: "daemon_name:version:status:cpu:mem:storage:tasks:uptime_sec:message"

Input:
  - raw string : The raw heartbeat message

Output:
  - *DaemonInfo : Parsed daemon info, or nil on parse error
  - error       : Non-nil if the message is malformed

Lines: ~25
*/
func ParseHeartbeat(raw string) (*DaemonInfo, error) {
	parts := strings.SplitN(raw, ":", 9)
	if len(parts) < 8 {
		return nil, fmt.Errorf("heartbeat: expected at least 8 fields, got %d", len(parts))
	}

	info := &DaemonInfo{
		Name:     parts[0],
		Version:  parts[1],
		Status:   parts[2],
		LastHeartbeat: time.Now(),
	}

	fmt.Sscanf(parts[3], "%f", &info.CPUPercent)
	fmt.Sscanf(parts[4], "%f", &info.MemoryMB)
	fmt.Sscanf(parts[5], "%f", &info.StorageMB)
	fmt.Sscanf(parts[6], "%d", &info.ActiveTasks)

	var uptimeSec float64
	fmt.Sscanf(parts[7], "%f", &uptimeSec)
	info.Uptime = time.Duration(uptimeSec) * time.Second

	if len(parts) >= 9 {
		info.Message = parts[8]
	}

	return info, nil
}

/*
Function: FormatHeartbeat
Description:
  Formats a DaemonInfo into a UDP heartbeat message string.

Input:
  - info *DaemonInfo : The daemon info to serialize

Output:
  - string : "name:version:status:cpu:mem:storage:tasks:uptime_sec:message"

Lines: ~8
*/
func FormatHeartbeat(info *DaemonInfo) string {
	return fmt.Sprintf("%s:%s:%s:%.2f:%.2f:%.2f:%d:%.0f:%s",
		info.Name, info.Version, info.Status,
		info.CPUPercent, info.MemoryMB, info.StorageMB,
		info.ActiveTasks,
		info.Uptime.Seconds(),
		info.Message,
	)
}

// ==============================
// DASHBOARD TYPES
// ==============================

/*
Struct: ModelPerformance
Description:
  Holds training/backtesting results for the School dashboard.
  Moved from apps/governance/main.go.

Fields:
  - Name    string  : Model name (e.g., "LSTM_v2", "Transformer_v1")
  - Score   float64 : Backtesting score (0-100)
  - WinRate float64 : Win rate as percentage (0-100)
  - Status  string  : "training", "active", "abandoned"

Lines: ~5
*/
type ModelPerformance struct {
	Name    string
	Score   float64
	WinRate float64
	Status  string
}

/*
Struct: TransactionRecord
Description:
  Represents a trading transaction for the portfolio dashboard.
  Moved from apps/governance/main.go.

Fields:
  - Timestamp  time.Time : When the transaction occurred
  - FromToken  string    : Token sold
  - ToToken    string    : Token bought
  - Amount     float64   : Trade amount in USD
  - PnL        float64   : Profit/loss in USD
  - Confidence float64   : Model confidence at execution

Lines: ~5
*/
type TransactionRecord struct {
	Timestamp  time.Time
	FromToken  string
	ToToken    string
	Amount     float64
	PnL        float64
	Confidence float64
}
