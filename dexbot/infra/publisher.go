/******************************************************************************
 * File Name       : publisher.go
 * File Path       : infra/publisher.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-06-28 14:15:00 (UTC+7)
 * Modified Date   : 2026-06-28 17:00:00 (UTC+7)
 *
 * Description     :
 *   File-based dashboard publisher. Replaces embedded HTTP server per
 *   myreq3.txt §68-70. Governance writes HTML/JSON to output directory;
 *   external HTTP server (serve.py) publishes static files.
 *
 * Responsibilities:
 *   - Write HTML pages to configured output directory
 *   - Write JSON API responses to api/ subdirectory
 *   - Write arbitrary files via WriteRaw
 *   - Directory health check for writability
 *   - Thread-safe via mutex
 *
 * Usage :
 *   Directory : infra/
 *   Build     : go build ./infra
 *   Run       : pub := infra.NewPublisher("web_output")
 *               pub.WriteHTML("index", "<html>...</html>")
 *               pub.WriteJSON("api/daemons", data)
 *   Test      : go test ./infra -v
 *
 * Dependencies :
 *   Internal : None
 *   External : encoding/json, os, path/filepath, sync, time (stdlib)
 *
 * Configuration :
 *   - WEB_OUTPUT_DIR environment variable (default: web_output)
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Struct]  Publisher
 *   [Function] NewPublisher, WriteHTML, WriteJSON, WriteRaw, Dir,
 *             LastRefresh, MarkRefreshed, HealthCheck
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-28 14:15:00   | deepseek-4.0-pro | Initial version
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add WriteCSV for spreadsheet export
 *   - Add incremental refresh (only changed files)
 *
 * Notes :
 *   - Per myreq3.txt §68-70: Governance does NOT run an HTTP server.
 *   - Output files are served by an external HTTP server (serve.py).
 ******************************************************************************/

package infra

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ==============================
// PUBLISHER
// ==============================

/*
Struct: Publisher
Description:
  Manages dashboard file output. Writes HTML pages, JSON API responses,
  and any other static content to a configurable output directory.
  Thread-safe via mutex.

Fields:
  - mu      sync.Mutex : Protects file writes
  - dir     string     : Output directory path
  - refresh time.Time  : Last refresh timestamp

Lines: ~5
*/
type Publisher struct {
	mu      sync.Mutex
	dir     string
	refresh time.Time
}

/*
Function: NewPublisher
Description:
  Creates a new Publisher for the given output directory.
  Creates the directory and api/ subdirectory if missing.

Input:
  - dir string : Output directory path (e.g., "web_output")

Output:
  - *Publisher : Initialized publisher

Lines: ~12
*/
func NewPublisher(dir string) *Publisher {
	os.MkdirAll(dir, 0755)
	os.MkdirAll(filepath.Join(dir, "api"), 0755)
	return &Publisher{dir: dir, refresh: time.Now()}
}

/*
Function: WriteHTML
Description:
  Writes an HTML file to the output directory.

Input:
  - name    string : File name without extension (e.g., "index" → index.html)
  - content string : Full HTML content

Output:
  - error : Non-nil if write fails

Lines: ~8
*/
func (p *Publisher) WriteHTML(name, content string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	path := filepath.Join(p.dir, name+".html")
	return os.WriteFile(path, []byte(content), 0644)
}

/*
Function: WriteJSON
Description:
  Writes a JSON file to the output directory (typically under api/).

Input:
  - name string      : File name without extension (e.g., "api/daemons" → api/daemons.json)
  - data interface{} : Any JSON-serializable value

Output:
  - error : Non-nil if marshal or write fails

Lines: ~12
*/
func (p *Publisher) WriteJSON(name string, data interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(p.dir, name+".json")
	return os.WriteFile(path, bytes, 0644)
}

/*
Function: WriteRaw
Description:
  Writes arbitrary bytes to a file in the output directory.

Input:
  - name string : File name with extension (e.g., "styles.css")
  - data []byte : Raw file content

Output:
  - error : Non-nil if write fails

Lines: ~6
*/
func (p *Publisher) WriteRaw(name string, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	path := filepath.Join(p.dir, name)
	return os.WriteFile(path, data, 0644)
}

/*
Function: Dir
Description:
  Returns the output directory path.

Input:
  - none

Output:
  - string : Output directory path

Lines: ~3
*/
func (p *Publisher) Dir() string {
	return p.dir
}

/*
Function: LastRefresh
Description:
  Returns the timestamp of the last RefreshAll call.

Input:
  - none

Output:
  - time.Time : Last refresh time

Lines: ~5
*/
func (p *Publisher) LastRefresh() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.refresh
}

/*
Function: MarkRefreshed
Description:
  Updates the last refresh timestamp. Called after a full refresh cycle.

Input:
  - none

Output:
  - none

Lines: ~5
*/
func (p *Publisher) MarkRefreshed() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.refresh = time.Now()
}

/*
Function: HealthCheck
Description:
  Returns nil if the output directory is writable.

Input:
  - none

Output:
  - error : Non-nil if directory check fails

Lines: ~8
*/
func (p *Publisher) HealthCheck() error {
	testFile := filepath.Join(p.dir, ".health_check")
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		return fmt.Errorf("publisher directory not writable: %w", err)
	}
	os.Remove(testFile)
	return nil
}
