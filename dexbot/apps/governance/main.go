/******************************************************************************
 * File Name       : main.go
 * File Path       : apps/governance/main.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 3.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:43 (UTC+7)
 * Modified Date   : 2026-07-13 10:00:00 (UTC+7)
 *
 * Description     :
 *   Central orchestration daemon for the Dexbot system. Monitors daemon health
 *   via UDP heartbeats, manages daemon lifecycle dynamically from config.env
 *   (MANAGED_DAEMONS + DAEMON_{NAME}_* env vars), provides CLI commands,
 *   publishes dashboard HTML/JSON files, and hosts a TCP action listener.
 *
 * Responsibilities:
 *   - Parse MANAGED_DAEMONS from config.env and init UDP connections
 *   - Listen on UDP port (default 8081) for heartbeats from all daemons
 *   - Periodically health-check each daemon and recreate if unhealthy
 *   - Publish dashboard pages (index, trading/portfolio, school, predict) every 10s
 *   - Listen on TCP ACTION_PORT for UI-triggered daemon lifecycle actions
 *   - Provide CLI status, restart, stop, start, shutdown commands
 *
 * Usage :
 *   Directory : apps/governance/
 *   Build     : go build -o governance .
 *   Run       : ./governance -action=start
 *   Test      : go test ./apps/governance
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/governance (registry, heartbeat parser, commander)
 *     - dexbot/infra (logger, publisher, env loader, DB)
 *     - dexbot/webui (dashboard renderer)
 *   External :
 *     - (stdlib only)
 *
 * Configuration :
 *   - config.env (all DAEMON_* vars, MANAGED_DAEMONS, ports, intervals)
 *
 * Updated Parts :
 *   [Function]
 *     - startHealthCheckLoop() — message format corrected
 *   [Global Variables]
 *     - managedDaemons — now map[string]*DaemonConfig for dynamic init
 *
 * New Parts :
 *   [Struct]
 *     - DaemonConfig — holds IP, port, path, UDP connection per registered daemon
 *   [Function]
 *     - initDynamicDaemons() — reads MANAGED_DAEMONS + DAEMON_{NAME}_* from env
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-01 19:25:43    | deepseek-4.0-pro | Initial version
 *   2.0.0   | 2026-07-13 08:00:00    | deepseek-4.0-pro | Dynamic daemon init
 *   3.0.0   | 2026-07-13 10:00:00    | deepseek-4.0-pro | Health msg fix, format
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add live log viewer on dashboard
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dexbot/governance"
	"dexbot/infra"
	"dexbot/webui"
)

// ── CONSTANTS ──
const (
	PID_FILE              = "governance.pid"
	HEALTH_CHECK_TIMEOUT  = 500 * time.Millisecond
	UDP_GOVERNANCE_PORT   = 8081
	UDP_SCHOOL_PORT       = 8082
	UDP_TRADING_PORT      = 8083
	GOVERNANCE_WEB_PORT   = 8080
)

// ── TYPES ──

/******************************************************************************
 * Struct Name : DaemonConfig
 * Purpose     : Dynamic daemon registration from config.env.
 ******************************************************************************/
type DaemonConfig struct {
	Name        string       `json:"name"`
	IP          string       `json:"ip"`
	Port        int          `json:"port"`
	Path        string       `json:"path"`
	LogPath     string       `json:"log_path"`
	SSHUser     string       `json:"ssh_user"`
	SSHPass     string       `json:"ssh_pass"`
	SSHPort     string       `json:"ssh_port"`
	StartMethod string       `json:"start_method"`
	UDPConn     *net.UDPConn `json:"-"`
}

// ── GLOBALS ──
var (
	registry       *governance.Registry
	commander      *governance.DefaultCommander
	managedDaemons map[string]*DaemonConfig

	schoolUdpConn  *net.UDPConn
	tradingUdpConn *net.UDPConn

	governancePort int
	schoolPort     int
	tradingPort    int
	webPort        int

	recreateThreshold = 1 * time.Minute

	publisher    *infra.Publisher
	modelReg     *governance.ModelRegistry
	tokenReg     *infra.TokenRegistry
	dashRenderer *webui.Renderer
)

/******************************************************************************
 * Function Name : main
 * Purpose       : CLI dispatch or start daemon.
 * Inputs        : none (os.Args)
 * Outputs       : none
 * Return        : none
 * Number Of Lines : 20
 ******************************************************************************/
func main() {
	fs := flag.NewFlagSet("governance", flag.ContinueOnError)
	action := fs.String("action", "start", "Action: start, status, restart, stop, shutdown")
	daemon := fs.String("daemon", "", "Target daemon name")
	_ = fs.Parse(os.Args[1:])

	if *action != "start" {
		infra.InitLogger()
		initDynamicDaemons()
		handleCLI(*action, *daemon)
		return
	}
	startDaemon()
}

/******************************************************************************
 * Function Name : handleCLI
 * Purpose       : Dispatch CLI commands (status, restart, stop, start, shutdown).
 * Inputs        : action string, daemon string
 * Outputs       : prints to stdout
 * Return        : none
 * Error Cases   : unknown daemon name silently ignored
 * Number Of Lines : 25
 ******************************************************************************/
func handleCLI(action, daemon string) {
	switch action {
	case "status":
		type statusOut struct {
			Daemons []*governance.DaemonInfo `json:"daemons"`
			Count   int                       `json:"count"`
		}
		out := statusOut{}
		for _, n := range registry.List() {
			out.Daemons = append(out.Daemons, registry.GetStatus(n))
		}
		out.Count = len(out.Daemons)
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))

	case "restart", "stop", "start":
		if cfg, ok := managedDaemons[daemon]; ok {
			if cfg.UDPConn != nil {
				cfg.UDPConn.Write([]byte("governance:command:" + action))
			}
			if action == "restart" || action == "start" {
				recreateDaemon(cfg)
			}
		}
	case "shutdown":
		for _, cfg := range managedDaemons {
			if cfg.UDPConn != nil {
				cfg.UDPConn.Write([]byte("governance:command:stop"))
			}
		}
		os.Exit(0)
	}
}

/******************************************************************************
 * Function Name : initDynamicDaemons
 * Purpose       : Read MANAGED_DAEMONS from env and build DaemonConfig map.
 * Inputs        : none (os.Getenv)
 * Outputs       : populates managedDaemons map + backward compat UDP conns
 * Return        : none
 * Error Cases   : env vars missing -> defaults applied
 * Number Of Lines : 55
 ******************************************************************************/
func initDynamicDaemons() {
	managedDaemons = make(map[string]*DaemonConfig)
	daemonList := os.Getenv("MANAGED_DAEMONS")
	if daemonList == "" { daemonList = "school,trading" }

	for _, n := range strings.Split(daemonList, ",") {
		n = strings.TrimSpace(n)
		if n == "" { continue }
		upper := strings.ToUpper(n)

		port, _ := strconv.Atoi(os.Getenv("DAEMON_" + upper + "_PORT"))
		if port == 0 {
			switch n {
			case "school":  port = 8082
			case "trading": port = 8083
			default:        port = 8084
			}
		}
		ip := os.Getenv("DAEMON_" + upper + "_IP")
		if ip == "" { ip = "127.0.0.1" }
		path := os.Getenv("DAEMON_" + upper + "_PATH")
		if path == "" {
			path = fmt.Sprintf("/workspace/crypto_apps/dexbot/apps/%s/main.go", n)
		}

		cfg := &DaemonConfig{
			Name:        n, IP: ip, Port: port, Path: path,
			LogPath:     os.Getenv("DAEMON_" + upper + "_LOG"),
			SSHUser:     os.Getenv("DAEMON_" + upper + "_SSH_USER"),
			SSHPass:     os.Getenv("DAEMON_" + upper + "_SSH_PASS"),
			SSHPort:     os.Getenv("DAEMON_" + upper + "_SSH_PORT"),
			StartMethod: os.Getenv("DAEMON_" + upper + "_START_METHOD"),
		}
		if cfg.SSHPort == "" { cfg.SSHPort = "22" }

		addr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip, port))
		if conn, err := net.DialUDP("udp", nil, addr); err == nil {
			cfg.UDPConn = conn
		}
		managedDaemons[n] = cfg
		infra.Info(fmt.Sprintf("Registered daemon: %s -> %s:%d", n, ip, port))

		if n == "school" { schoolUdpConn = cfg.UDPConn; schoolPort = port }
		if n == "trading" { tradingUdpConn = cfg.UDPConn; tradingPort = port }
	}
}

/******************************************************************************
 * Function Name : startDaemon
 * Purpose       : Launch all governance goroutines and block until signal.
 * Inputs        : none
 * Outputs       : none
 * Return        : none
 * Error Cases   : Fatal if UDP listener or health check loop fails
 * Number Of Lines : 30
 ******************************************************************************/
func startDaemon() {
	registry = governance.NewRegistry()
	initDynamicDaemons()

	governancePort = UDP_GOVERNANCE_PORT
	webPort = GOVERNANCE_WEB_PORT

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startUdpListener(ctx)
	go startHealthCheckLoop(ctx)
	dashRenderer = webui.NewRenderer(registry)
	go startPublisher(ctx)
	go startActionListener(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	infra.Info("Governance shutting down.")
	cancel()
	time.Sleep(1 * time.Second)
}

/******************************************************************************
 * Function Name : startUdpListener
 * Purpose       : Listen for UDP heartbeats from daemons on governancePort.
 * Inputs        : ctx context.Context
 * Outputs       : none (runs as goroutine)
 * Return        : none
 * Error Cases   : Port bind failure logs error and returns
 * Number Of Lines : 40
 ******************************************************************************/
func startUdpListener(ctx context.Context) {
	port := governancePort
	if port == 0 { port = UDP_GOVERNANCE_PORT }
	listenAddr := os.Getenv("GOVERNANCE_ADDR")
	if listenAddr == "" { listenAddr = "127.0.0.1" }
	addr := net.UDPAddr{Port: port, IP: net.ParseIP(listenAddr)}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil { infra.Error("UDP listen failed: " + err.Error()); return }
	defer conn.Close()
	infra.Info(fmt.Sprintf("UDP listener on %s:%d", listenAddr, port))

	buf := make([]byte, 16384)
	for {
		select {
		case <-ctx.Done(): return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil { continue }
			msg := string(buf[:n])

			if strings.HasPrefix(msg, "model:sync:") {
				handleModelSync(strings.TrimPrefix(msg, "model:sync:"))
				continue
			}
			if info, err := governance.ParseHeartbeat(msg); err == nil {
				registry.Register(info)
				continue
			}
			parts := strings.SplitN(msg, ":", 3)
			if len(parts) >= 3 {
				registry.Register(&governance.DaemonInfo{
					Name: parts[0], Version: "legacy",
					Status: parts[1], Message: parts[2],
					LastHeartbeat: time.Now(),
				})
			}
		}
	}
}

/******************************************************************************
 * Function Name : handleModelSync
 * Purpose       : Parse and register model:sync JSON from School daemon.
 * Inputs        : jsonPayload string
 * Outputs       : none
 * Return        : none
 * Error Cases   : Invalid JSON silently skipped
 * Number Of Lines : 25
 ******************************************************************************/
func handleModelSync(jsonPayload string) {
	if modelReg == nil { modelReg = governance.NewModelRegistry() }
	type modelSummary struct {
		ID, Version, Category, Architecture, Status string
		Sharpe, Consistency, Profit                  float64
		Generation                                   int
	}
	var wrapper struct{ Models []modelSummary }
	if err := json.Unmarshal([]byte(jsonPayload), &wrapper); err != nil { return }
	for _, s := range wrapper.Models {
		modelReg.Register(&governance.ModelRecord{
			ID: s.ID, ModelVersion: s.Version,
			Category: s.Category, Architecture: s.Architecture,
			Status: s.Status, Generation: s.Generation,
		})
	}
}

/******************************************************************************
 * Function Name : startHealthCheckLoop
 * Purpose       : Periodically probe each managed daemon, mark healthy/unhealthy,
 *                 trigger recreation after threshold. Message format per myre6.txt.
 * Inputs        : ctx context.Context
 * Outputs       : none (runs as goroutine)
 * Return        : none
 * Error Cases   : Probe failures handled gracefully (mark unhealthy)
 * Number Of Lines : 70
 ******************************************************************************/
func startHealthCheckLoop(ctx context.Context) {
	intervalStr := os.Getenv("HEALTH_CHECK_INTERVAL_SECONDS")
	interval, _ := strconv.Atoi(intervalStr)
	if interval <= 0 { interval = 30 }
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()
	infra.Info(fmt.Sprintf("Health check starting (%ds)", interval))
	time.Sleep(time.Duration(interval) * time.Second)

	for {
		select {
		case <-ctx.Done(): return
		case <-ticker.C:
			unhealthyCount := 0
			killedCount := 0
			var lastUnhealthy string
			var lastKilled string

			for name, cfg := range managedDaemons {
				if cfg.UDPConn == nil { continue }
				cfg.UDPConn.Write([]byte("governance:probe:health_check"))
				buf := make([]byte, 1024)
				cfg.UDPConn.SetReadDeadline(time.Now().Add(HEALTH_CHECK_TIMEOUT))
				n, _, err := cfg.UDPConn.ReadFromUDP(buf)

				info := getOrCreateInfo(name)
				secondsSinceHealthy := time.Since(info.LastHeartbeat).Seconds()

				if err != nil || !strings.Contains(string(buf[:n]), "healthy") {
					prevStatus := info.Status
					info.Status = "unhealthy"
					unhealthyCount++
					lastUnhealthy = name
					if secondsSinceHealthy > recreateThreshold.Seconds() {
						recreateDaemon(cfg)
						info.RecordRestart()
						info.Status = "killed"
						killedCount++
						lastKilled = name
						info.Message = fmt.Sprintf("Killed after %.0fs unhealthy — recreating", secondsSinceHealthy)
					} else if prevStatus == "killed" {
						info.Message = fmt.Sprintf("Recreating… waiting (%.0fs since kill)", secondsSinceHealthy)
					} else {
						info.Message = fmt.Sprintf("Unhealthy (%.0fs, threshold=%.0fs)", secondsSinceHealthy, recreateThreshold.Seconds())
					}
				} else {
					info.Status = "healthy"
					info.Message = fmt.Sprintf("%s daemon operational", name)
				}
				registry.Register(info)
			}

			govInfo := getOrCreateInfo("governance")
			govInfo.Status = "healthy"
			if killedCount > 0 {
				if killedCount == 1 {
					govInfo.Message = fmt.Sprintf("detect killed daemon: %s", lastKilled)
				} else {
					govInfo.Message = fmt.Sprintf("detect %d killed daemons (last: %s)", killedCount, lastKilled)
				}
			} else if unhealthyCount == 1 {
				govInfo.Message = fmt.Sprintf("detect unhealthy from: %s", lastUnhealthy)
			} else if unhealthyCount > 1 {
				govInfo.Message = fmt.Sprintf("detect %d unhealthy daemons (last: %s)", unhealthyCount, lastUnhealthy)
			} else {
				govInfo.Message = "All managed daemons healthy"
			}
			registry.Register(govInfo)
		}
	}
}

/******************************************************************************
 * Function Name : getOrCreateInfo
 * Purpose       : Retrieve DaemonInfo from registry or create default entry.
 * Inputs        : name string
 * Outputs       : returns existing or new DaemonInfo pointer
 * Return        : *governance.DaemonInfo
 * Error Cases   : none (always returns valid info struct)
 * Complexity    : O(1) — daemon name
 * Return        : *governance.DaemonInfo
 * Number Of Lines : 8
 ******************************************************************************/
func getOrCreateInfo(name string) *governance.DaemonInfo {
	info := registry.GetStatus(name)
	if info == nil {
		info = &governance.DaemonInfo{Name: name, Version: "unknown", Status: "unknown"}
	}
	return info
}

/******************************************************************************
 * Function Name : recreateDaemon
 * Purpose       : Start a daemon process via go run or configured StartMethod.
 * Inputs        : cfg *DaemonConfig
 * Outputs       : none
 * Return        : none
 * Error Cases   : Command start failure logged
 * Number Of Lines : 20
 ******************************************************************************/
func recreateDaemon(cfg *DaemonConfig) {
	infra.Warn(fmt.Sprintf("Recreating %s daemon…", cfg.Name))
	var cmd *exec.Cmd
	if cfg.StartMethod != "" {
		cmd = exec.Command("sh", "-c", cfg.StartMethod)
	} else {
		cmd = exec.Command("/usr/local/go/bin/go", "run", cfg.Path, "-action=start")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		infra.Error(fmt.Sprintf("Failed to recreate %s: %v", cfg.Name, err))
	} else {
		infra.Info(fmt.Sprintf("%s recreated (PID via start_all)", cfg.Name))
	}
}

/******************************************************************************
 * Function Name : startPublisher
 * Purpose       : Periodically regenerate and write HTML + JSON dashboard files.
 * Inputs        : ctx context.Context
 * Outputs       : none (runs as goroutine)
 * Return        : none
 * Error Cases   : Write failures logged, publisher continues
 * Number Of Lines : 25
 ******************************************************************************/
func startPublisher(ctx context.Context) {
	outputDir := os.Getenv("WEB_OUTPUT_DIR")
	if outputDir == "" { outputDir = "web_output" }
	publisher = infra.NewPublisher(outputDir)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done(): return
		case <-ticker.C:
			if dashRenderer != nil { refreshDashboard() }
		}
	}
}

/******************************************************************************
 * Function Name : refreshDashboard
 * Purpose       : Generate all HTML pages + JSON API files from registry, DB.
 * Inputs        : none (uses globals: dashRenderer, registry, publisher)
 * Outputs       : none (writes files to web_output/)
 * Return        : none
 * Error Cases   : Render/write failures silently skipped
 * Number Of Lines : 45
 ******************************************************************************/
func refreshDashboard() {
	pages := []struct {
		name string
		fn   func(http.ResponseWriter)
	}{
		{"index", dashRenderer.Operations},
		{"training", dashRenderer.SchoolDashboard},
		{"portfolio", dashRenderer.Portfolio},
		{"predict", dashRenderer.PredictionComparison},
	}
	for _, p := range pages {
		buf := &bytes.Buffer{}
		w := &fakeResp{buf: buf}
		p.fn(w)
		publisher.WriteHTML(p.name, buf.String())
	}
	if registry != nil {
		type daemonList struct{ Daemons []*governance.DaemonInfo `json:"daemons"` }
		dl := daemonList{}
		for _, n := range registry.List() {
			dl.Daemons = append(dl.Daemons, registry.GetStatus(n))
		}
		publisher.WriteJSON("api/daemons", dl)
	}
	tables := infra.ListTables()
	if tables != nil {
		publisher.WriteJSON("api/database_tables", map[string]interface{}{"tables": tables})
		dbData := make(map[string]interface{})
		for _, tbl := range tables {
			cols, rows := infra.QueryTableRows(tbl, 5, "newest")
			if cols != nil {
				dbData[tbl] = map[string]interface{}{"columns": cols, "rows": rows}
			}
		}
		if len(dbData) > 0 { publisher.WriteJSON("api/database", dbData) }
	}
	publisher.MarkRefreshed()
}

/******************************************************************************
 * Struct Name : fakeResp
 * Purpose     : io.Writer adapter so renderToBytes can capture HTML output.
 ******************************************************************************/
type fakeResp struct{ buf *bytes.Buffer }
func (w *fakeResp) Header() http.Header         { return make(http.Header) }
func (w *fakeResp) Write(b []byte) (int, error)   { return w.buf.Write(b) }
func (w *fakeResp) WriteHeader(code int)          {}

/******************************************************************************
 * Function Name : startActionListener
 * Purpose       : TCP listener on ACTION_PORT for UI-triggered daemon actions.
 * Inputs        : ctx context.Context
 * Outputs       : none (runs as goroutine)
 * Return        : none
 * Error Cases   : Listen failure silently exits
 * Number Of Lines : 30
 ******************************************************************************/
func startActionListener(ctx context.Context) {
	port := os.Getenv("ACTION_PORT")
	if port == "" { port = "8085" }
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil { return }
	defer ln.Close()
	infra.Info("Action listener on :" + port)

	for {
		select {
		case <-ctx.Done(): return
		default:
			ln.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))
			conn, err := ln.Accept()
			if err != nil { continue }
			go func(c net.Conn) {
				defer c.Close()
				b := make([]byte, 1024)
				n, _ := c.Read(b)
				parts := strings.Fields(string(b[:n]))
				if len(parts) >= 2 {
					action, name := parts[0], parts[1]
					if cfg, ok := managedDaemons[name]; ok {
						if cfg.UDPConn != nil {
							cfg.UDPConn.Write([]byte("governance:command:" + action))
						}
						if action == "restart" || action == "start" {
							recreateDaemon(cfg)
						}
					}
				}
			}(conn)
		}
	}
}

/******************************************************************************
 * Function Name : init
 * Purpose       : Package init — sets up logger and loads env.
 * Inputs        : none
 * Outputs       : none
 * Number Of Lines : 6
 ******************************************************************************/
func init() {
	infra.InitLogger()
	loadEnvSmart()
}

/******************************************************************************
 * Function Name : loadEnvSmart
 * Purpose       : Load config.env from multiple possible paths.
 * Inputs        : none
 * Outputs       : none
 * Number Of Lines : 8
 ******************************************************************************/
func loadEnvSmart() {
	for _, p := range []string{"config.env", "../config.env", "../../config.env"} {
		if _, err := os.Stat(p); err == nil {
			infra.LoadEnv(p)
			return
		}
	}
	infra.Warn("config.env not found")
}
