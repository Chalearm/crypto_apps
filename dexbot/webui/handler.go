/******************************************************************************
 * File Name       : handler.go
 * File Path       : webui/handler.go
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
 *   HTTP server wrapper and API handlers for the Dexbot web dashboard. Extracted from apps/governance/main.go during Phase 10 (myreq2.txt §8). Routes: GET  /                             → Operations dashb
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
package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"dexbot/governance"
)

// ==============================
// SERVER
// ==============================

/*
Struct: Server
Description:
  HTTP server wrapping the dashboard renderer and API endpoints.
  Uses a callback pattern (OnAction) so the governance daemon can
  execute real actions (spawn process, send UDP signals).

Fields:
  - renderer   *Renderer                    : HTML page renderer
  - registry   *governance.Registry         : Daemon registry
  - port       int                          : Listen port
  - OnAction   func(name, action string)    : Called on POST /api/daemon/{name}/{action}

Lines: ~8
*/
type Server struct {
	renderer *Renderer
	registry *governance.Registry
	port     int
	OnAction func(name, action string)
}

/*
Function: NewServer
Description:
  Creates a new HTTP dashboard server.

Input:
  - registry *governance.Registry : Daemon registry
  - port     int                  : Listen port

Output:
  - *Server: Initialized server

Lines: ~12
*/
func NewServer(registry *governance.Registry, port int) *Server {
	return &Server{
		renderer: NewRenderer(registry),
		registry: registry,
		port:     port,
	}
}

/*
Function: Renderer
Description:
  Returns the embedded Renderer for direct access (e.g. SetPorts).

Input:
  - none

Output:
  - *Renderer

Lines: ~3
*/
func (s *Server) Renderer() *Renderer {
	return s.renderer
}

/*
Function: ListenAndServe
Description:
  Registers all routes and starts the HTTP server. Blocks.

Input:
  - none

Output:
  - error: Non-nil if server fails to start

Lines: ~20
*/
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	// Dashboard pages
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		s.renderer.Operations(w)
	})
	mux.HandleFunc("/training", func(w http.ResponseWriter, r *http.Request) {
		s.renderer.Training(w)
	})
	mux.HandleFunc("/portfolio", func(w http.ResponseWriter, r *http.Request) {
		s.renderer.Portfolio(w)
	})
	mux.HandleFunc("/predict", func(w http.ResponseWriter, r *http.Request) {
		s.renderer.PredictionComparison(w)
	})

	// API
	mux.HandleFunc("/api/daemons", s.apiDaemonsHandler)
	mux.HandleFunc("/api/daemon/", s.apiDaemonActionHandler)

	addr := fmt.Sprintf(":%d", s.port)
	return http.ListenAndServe(addr, mux)
}

// ==============================
// API HANDLERS
// ==============================

/*
Function: apiDaemonsHandler
Description:
  Returns all daemon status as JSON.

Input:
  - w http.ResponseWriter
  - r *http.Request

Output:
  - none

Lines: ~12
*/
func (s *Server) apiDaemonsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	names := s.registry.List()
	type out struct {
		Daemons []*governance.DaemonInfo `json:"daemons"`
	}
	o := out{}
	for _, n := range names {
		o.Daemons = append(o.Daemons, s.registry.GetStatus(n))
	}
	json.NewEncoder(w).Encode(o)
}

/*
Function: apiDaemonActionHandler
Description:
  Handles POST actions on /api/daemon/{name}/{action}.
  Supported: restart, stop, start.

Input:
  - w http.ResponseWriter
  - r *http.Request

Output:
  - none

Lines: ~35
*/
func (s *Server) apiDaemonActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/daemon/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid path: /api/daemon/{name}/{action}", http.StatusBadRequest)
		return
	}
	name, action := parts[0], parts[1]

	switch action {
	case "restart", "stop", "start":
		// Update registry for stop
		if action == "stop" {
			info := s.registry.GetStatus(name)
			if info != nil {
				info.Status = "stopping"
				s.registry.Register(info)
			}
		}
		// Fire callback
		if s.OnAction != nil {
			s.OnAction(name, action)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","action":"%s","daemon":"%s"}`, action, name)
	default:
		http.Error(w, "Unknown action: "+action, http.StatusBadRequest)
	}
}
