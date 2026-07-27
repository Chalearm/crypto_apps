/******************************************************************************
 * File Name       : publisher.go
 * File Path       : infra/publisher.go
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:31 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:31 (UTC+7)
 * Description     : File-based dashboard publisher for HTML, JSON, and static content.
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

type Publisher struct {
  mu      sync.Mutex
  dir     string
  refresh time.Time
}

/******************************************************************************
 * Function Name : NewPublisher
 * Purpose       : Initializes a new Publisher and output directory structure.
 ******************************************************************************/
func NewPublisher(dir string) *Publisher {
  os.MkdirAll(dir, 0755)
  os.MkdirAll(filepath.Join(dir, "api"), 0755)
  return &Publisher{dir: dir, refresh: time.Now()}
}

/******************************************************************************
 * Function Name : WriteHTML
 * Purpose       : Writes an HTML content file to the target output directory.
 ******************************************************************************/
func (p *Publisher) WriteHTML(name, content string) error {
  p.mu.Lock()
  defer p.mu.Unlock()
  path := filepath.Join(p.dir, name+".html")
  return os.WriteFile(path, []byte(content), 0644)
}

/******************************************************************************
 * Function Name : WriteJSON
 * Purpose       : Encodes and writes data as JSON to the target output directory.
 ******************************************************************************/
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

/******************************************************************************
 * Function Name : WriteRaw
 * Purpose       : Writes raw bytes to a file under the publisher directory.
 ******************************************************************************/
func (p *Publisher) WriteRaw(name string, data []byte) error {
  p.mu.Lock()
  defer p.mu.Unlock()
  path := filepath.Join(p.dir, name)
  return os.WriteFile(path, data, 0644)
}

/******************************************************************************
 * Function Name : Dir
 * Purpose       : Returns the configured publisher output directory path.
 ******************************************************************************/
func (p *Publisher) Dir() string {
  return p.dir
}

/******************************************************************************
 * Function Name : LastRefresh
 * Purpose       : Returns the timestamp of the last dashboard refresh.
 ******************************************************************************/
func (p *Publisher) LastRefresh() time.Time {
  p.mu.Lock()
  defer p.mu.Unlock()
  return p.refresh
}

/******************************************************************************
 * Function Name : MarkRefreshed
 * Purpose       : Updates the last refresh timestamp to the current time.
 ******************************************************************************/
func (p *Publisher) MarkRefreshed() {
  p.mu.Lock()
  defer p.mu.Unlock()
  p.refresh = time.Now()
}

/******************************************************************************
 * Function Name : HealthCheck
 * Purpose       : Verifies write permissions for the publisher directory.
 ******************************************************************************/
func (p *Publisher) HealthCheck() error {
  testFile := filepath.Join(p.dir, ".health_check")
  if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
    return fmt.Errorf("publisher directory not writable: %w", err)
  }
  os.Remove(testFile)
  return nil
}