/******************************************************************************
 * File Name       : registry.go
 * File Path       : governance/registry.go
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:46 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:46 (UTC+7)
 * Description     : Shared governance types: daemon registry, heartbeat protocol,
 *                   daemon status, model performance, transaction records.
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

/******************************************************************************
 * Function Name : IsHealthy
 * Purpose       : Checks if daemon status is healthy.
 ******************************************************************************/
func (d *DaemonInfo) IsHealthy() bool {
	return d.Status == "healthy"
}

/******************************************************************************
 * Function Name : RecordRestart
 * Purpose       : Records a restart event and updates restart history.
 ******************************************************************************/
func (d *DaemonInfo) RecordRestart() {
	d.RestartCount++
	d.RestartHistory = append(d.RestartHistory, time.Now())
	if len(d.RestartHistory) > 10 {
		d.RestartHistory = d.RestartHistory[len(d.RestartHistory)-10:]
	}
}

/******************************************************************************
 * Function Name : PostStatus
 * Purpose       : Updates daemon status and updates LastHeartbeat timestamp.
 ******************************************************************************/
func (d *DaemonInfo) PostStatus(status string) {
	d.Status = status
	d.LastHeartbeat = time.Now()
}

// ==============================
// REGISTRY
// ==============================

type Registry struct {
	mu      sync.RWMutex
	daemons map[string]*DaemonInfo
	history []DaemonInfo
}

/******************************************************************************
 * Function Name : NewRegistry
 * Purpose       : Constructs a new thread-safe Registry instance.
 ******************************************************************************/
func NewRegistry() *Registry {
	return &Registry{
		daemons: make(map[string]*DaemonInfo),
	}
}

/******************************************************************************
 * Function Name : Register
 * Purpose       : Registers or updates daemon info entry.
 ******************************************************************************/
func (r *Registry) Register(info *DaemonInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.daemons[info.Name]
	if exists {
		info.RestartCount = entry.RestartCount
		info.RestartHistory = entry.RestartHistory
	}
	info.LastHeartbeat = time.Now()
	r.daemons[info.Name] = info

	r.history = append(r.history, *info)
	if len(r.history) > 1000 {
		r.history = r.history[len(r.history)-1000:]
	}
}

/******************************************************************************
 * Function Name : Unregister
 * Purpose       : Removes a daemon from the registry by name.
 ******************************************************************************/
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.daemons, name)
}

/******************************************************************************
 * Function Name : GetStatus
 * Purpose       : Retrieves a copy of DaemonInfo for a given daemon name.
 ******************************************************************************/
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

/******************************************************************************
 * Function Name : List
 * Purpose       : Returns a slice of all registered daemon names.
 ******************************************************************************/
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.daemons))
	for name := range r.daemons {
		names = append(names, name)
	}
	return names
}

/******************************************************************************
 * Function Name : AllHealthy
 * Purpose       : Checks whether all registered daemons are healthy.
 ******************************************************************************/
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

/******************************************************************************
 * Function Name : History
 * Purpose       : Returns historical status records up to the specified limit.
 ******************************************************************************/
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

/******************************************************************************
 * Function Name : ParseHeartbeat
 * Purpose       : Parses a raw delimited heartbeat string into DaemonInfo.
 ******************************************************************************/
func ParseHeartbeat(raw string) (*DaemonInfo, error) {
	parts := strings.SplitN(raw, ":", 9)
	if len(parts) < 8 {
		return nil, fmt.Errorf("heartbeat: expected at least 8 fields, got %d", len(parts))
	}

	info := &DaemonInfo{
		Name:          parts[0],
		Version:       parts[1],
		Status:        parts[2],
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

/******************************************************************************
 * Function Name : FormatHeartbeat
 * Purpose       : Formats a DaemonInfo struct into a delimited heartbeat string.
 ******************************************************************************/
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

type ModelPerformance struct {
	Name    string
	Score   float64
	WinRate float64
	Status  string
}

type TransactionRecord struct {
	Timestamp  time.Time
	FromToken  string
	ToToken    string
	Amount     float64
	PnL        float64
	Confidence float64
}