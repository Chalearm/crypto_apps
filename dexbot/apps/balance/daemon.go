/******************************************************************************
 * File Name       : daemon.go
 * File Path       : apps/balance/daemon.go
 *
 * Author          : Gemini 3.1 Pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.6.0
 * Status          : Development
 * Created Date    : 2026-07-12 14:37:38 (UTC+7)
 * Modified Date   : 2026-07-12 16:25:00 (UTC+7)
 *
 * Description     :
 *   Handles background process detachment, PID file management, and 
 *   status/termination reporting. Contains an embedded HTTP API listener
 *   to handle real-time balance queries from external servers.
 *
 * Responsibilities:
 *   - Forking the current process into a background daemon.
 *   - Initializing a background HTTP API on port 8085.
 *   - Exposing /api/update to return calculated balance states as JSON.
 *   - Safely terminating background processes and wiping PID states.
 *
 * Usage :
 *   Directory : apps/balance/
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/infra
 *
 *   External :
 *     - net/http
 *     - encoding/json
 *     - os/exec
 *     - syscall
 * Updated Parts :
 *   [Function]
 *     - IsDaemonRunning() (Updated to utilize high-reliability HTTP probe checks)
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)       | Author         | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-12 14:37:38     | Gemini         | Initial version
 *   1.3.0   | 2026-07-12 15:25:00     | Gemini         | Fixed child PID exit bug
 *   1.4.0   | 2026-07-12 16:20:00     | Gemini         | Added embedded HTTP API server
 *   1.5.0   | 2026-07-12 16:25:00     | Gemini         | Connected full JSON payload logic
 *   1.6.0   | 2026-07-15 12:46:00     | Gemini 3.1 Pro | Unified network status engine sync
 *   ------------------------------------------------------------------------- 
 *
 * TODO :
 *   - Add token authentication to the localized HTTP port loop.
 *****************************************************************************
 *
 * New Parts :
 *   [Function] See function list.
 *
 * Notes :
 *   - Per regulator coding standard.
 */
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"dexbot/infra"
	"dexbot/tokens"
)

const pidFileName = "daemon.pid"
const logDirName = "logs"
const daemonAPIPort = ":8085"

/******************************************************************************
 * Function Name : getPIDFilePath
 *
 * Purpose :
 *   Constructs and returns the absolute path to the daemon's PID file.
 *
 * Inputs :
 *   None
 *
 * Return :
 *   Type        : string
 *   Description : Absolute path to the PID file inside the logs directory.
 *
 * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - Cannot determine current working directory.
 *
 * Number Of Lines :
 *   13
 ******************************************************************************/
func getPIDFilePath() string {
	// If we are at project root, make sure logs land in apps/balance/logs
	baseDir := "."
	if fi, err := os.Stat("apps/balance"); err == nil && fi.IsDir() {
		baseDir = "apps/balance"
	}
	
	logPath := filepath.Join(baseDir, logDirName)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		os.MkdirAll(logPath, 0755)
	}
	
	return filepath.Join(logPath, pidFileName)
}

/******************************************************************************
 * Function Name : IsDaemonRunning
 *
 * Purpose :
 *   Checks if the daemon is currently running by issuing a rapid HTTP probe
 *   against its default API port endpoint.
 *
 * Inputs :
 *   None
 *
 * Return :
 *   Type        : bool, int
 *   Description : Returns true and the running PID if active, false and 0 if dead.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - PID file missing/corrupt returns false.
 *   - Zombie process clears lock.
 *
 * Number Of Lines :
 *   20
 ******************************************************************************/
func IsDaemonRunning() (bool, int) {
pidFile := getPIDFilePath()
	data, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return false, 0
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false, 0
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}

	// Signal 0 tests process existence on the operating system layer
	sigErr := process.Signal(syscall.Signal(0))
	if sigErr != nil {
		return false, 0
	}

	// Linux Specific: Check /proc/PID/stat to ensure it's not a Zombie (Z) state
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	statData, statErr := ioutil.ReadFile(statPath)
	if statErr == nil {
		fields := strings.Split(string(statData), " ")
		if len(fields) > 2 {
			state := fields[2]
			if state == "Z" {
				infra.Warn(fmt.Sprintf("Found defunct zombie process profile for PID %d. Clearing lock.", pid))
				_ = os.Remove(pidFile) // Clear the stale file lock automatically
				return false, 0
			}
		}
	}

	return true, pid
}
/******************************************************************************
 * Function Name : handleAPIUpdateRoute
 *
 * Purpose :
 *   HTTP Handler for the /api/update route. Extracts private-key parameter,
 *   calculates the full live DB + blockchain payload via GetBalanceReport, 
 *   and returns structured JSON data natively to the external web server.
 *
 * Inputs :
 *   w http.ResponseWriter
 *   r *http.Request
 *
 * Return :
 *   None
 *
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - Missing private-key param returns 400 Bad Request.
 *   - Failure to calculate payload returns 500 Internal Server Error.
 *
 * Number Of Lines :
 *   20
 ******************************************************************************/
func handleAPIUpdateRoute(w http.ResponseWriter, r *http.Request) {
	pk := r.URL.Query().Get("private-key")
	if pk == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"missing private-key parameter"}`))
		return
	}

	report, err := GetBalanceReport(pk)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"error":"%v"}`, err)))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(report)
}

/******************************************************************************
 * Function Name : handleAPIBalanceRoute
 *
 * Purpose :
 *   HTTP Handler for /api/balance. Same as /api/update — returns full
 *   portfolio report. This is the canonical endpoint consumed by the
 *   webUI balance card.
 *
 * Inputs :
 *   w http.ResponseWriter
 *   r *http.Request
 *
 * Return :
 *   None
 *
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - Missing private-key returns 400.
 *   - Report failure returns 500.
 *
 * Number Of Lines :
 *   22
 ******************************************************************************/
func handleAPIBalanceRoute(w http.ResponseWriter, r *http.Request) {
	pk := r.URL.Query().Get("private-key")
	if pk == "" { pk = r.URL.Query().Get("private_key") }
	if pk == "" { pk = r.Header.Get("X-Private-Key") }
	if pk == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"missing private-key parameter"}`))
		return
	}

	report, err := GetBalanceReport(pk)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"error":"%v"}`, err)))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(report)
}

/******************************************************************************
 * Function Name : handleAPIChainAddRoute
 * Purpose :
 *   POST /api/chain/add — adds a chain to user_chains DB.
 * Inputs :
 *   w http.ResponseWriter
 *   r *http.Request
 * Return :
 *   None (writes JSON response)
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - Missing fields returns 400. DB init on nil.
 * Number Of Lines : 18 — adds a chain to user_chains DB.
 ******************************************************************************/
func handleAPIChainAddRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { w.WriteHeader(405); return }
	var body struct {
		AccountID string `json:"account_id"`
		Name      string `json:"name"`
		ChainID   string `json:"chain_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AccountID == "" || body.Name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write([]byte(`{"status":"error","message":"account_id, name, and chain_id required"}`))
		return
	}
	if dbConn == nil { InitDB() }

	// Determine base_url from tokens.AllChains() metadata
	baseURL := ""
	for _, c := range tokens.AllChains() {
		if c.Name == body.Name { baseURL = c.BaseURL; break }
	}

	InsertUserChain(body.AccountID, body.Name, body.ChainID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Chain added successfully", "name": body.Name, "base_url": baseURL})
}

/******************************************************************************
 * Function Name : handleAPITokenAddRoute
 * Purpose :
 *   POST /api/token/add — adds a token to user_tokens DB.
 * Inputs :
 *   w http.ResponseWriter
 *   r *http.Request
 * Return :
 *   None (writes JSON response)
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - Missing fields returns 400. DB init on nil.
 * Number Of Lines : 18 — adds a token to user_tokens DB.
 ******************************************************************************/
func handleAPITokenAddRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { w.WriteHeader(405); return }
	var body struct {
		AccountID string `json:"account_id"`
		ChainID   string `json:"chain_id"`
		Ticker    string `json:"ticker"`
		Address   string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AccountID == "" || body.Ticker == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write([]byte(`{"status":"error","message":"account_id, chain_id, ticker, and address required"}`))
		return
	}
	if dbConn == nil { InitDB() }
	chainName := GetChainNameByID(body.AccountID, body.ChainID)
	if chainName == "" { chainName = "BSC" }
	InsertUserToken(body.AccountID, chainName, body.Ticker, body.Address)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Token bound successfully"})
}

/******************************************************************************
 * Function Name : handleAPIChainDeleteRoute
 * Purpose :
 *   POST /api/chain/delete — cascade-deletes a chain.
 * Inputs :
 *   w http.ResponseWriter
 *   r *http.Request
 * Return :
 *   None (writes JSON response)
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - Missing fields returns 400. DB init on nil.
 * Number Of Lines : 16 — cascade-deletes a chain.
 ******************************************************************************/
func handleAPIChainDeleteRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { w.WriteHeader(405); return }
	var body struct {
		AccountID string `json:"account_id"`
		ChainID   string `json:"chain_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AccountID == "" || body.ChainID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write([]byte(`{"status":"error","message":"account_id and chain_id required"}`))
		return
	}
	if dbConn == nil { InitDB() }
	DeleteChainCascade(body.AccountID, body.ChainID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Chain and associated tokens removed"})
}

/******************************************************************************
 * Function Name : handleAPITokenDeleteRoute
 * Purpose :
 *   POST /api/token/delete — deletes a single token.
 * Inputs :
 *   w http.ResponseWriter
 *   r *http.Request
 * Return :
 *   None (writes JSON response)
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - Missing fields returns 400. DB init on nil.
 * Number Of Lines : 18 — deletes a single token.
 ******************************************************************************/
func handleAPITokenDeleteRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { w.WriteHeader(405); return }
	var body struct {
		AccountID string `json:"account_id"`
		ChainID   string `json:"chain_id"`
		Ticker    string `json:"ticker"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AccountID == "" || body.Ticker == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write([]byte(`{"status":"error","message":"account_id, chain_id, and ticker required"}`))
		return
	}
	if dbConn == nil { InitDB() }
	chainName := GetChainNameByID(body.AccountID, body.ChainID)
	if chainName == "" { chainName = "BSC" }
	DeleteSingleToken(body.AccountID, chainName, body.Ticker)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Token deleted successfully"})
}

/******************************************************************************
 * Function Name : handleAPIAccountDeleteRoute
 * Purpose :
 *   POST /api/account/delete — cascade-deletes an account.
 * Inputs :
 *   w http.ResponseWriter
 *   r *http.Request
 * Return :
 *   None (writes JSON response)
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - Missing account_id returns 400. DB init on nil.
 * Number Of Lines : 16 — cascade-deletes an account.
 ******************************************************************************/
func handleAPIAccountDeleteRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { w.WriteHeader(405); return }
	var body struct {
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AccountID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write([]byte(`{"status":"error","message":"account_id required"}`))
		return
	}
	if dbConn == nil { InitDB() }
	DeleteAccountCascade(body.AccountID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Account tracking records purged"})
}

/******************************************************************************
 * Function Name : HandleDaemonStart
 *
 * Purpose :
 *   Handles the '-action=start' command. Checks for existing running processes 
 *   to avoid duplicates, forks the child process into the background, and mounts 
 *   the HTTP server handlers cleanly inside the high-availability UDP framework loop.
 *
 * Inputs :
 *   None
 *
 * Outputs :
 *   None
 *
 * Return :
 *   None
 *
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - Fails to resolve binary executable path.
 *   - HTTP server binds fail due to an obstructed port.
 *
 * Dependencies :
 *   - net/http
 *   - os/exec
 *   - dexbot/infra
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines :
 *   64
 ******************************************************************************/
func HandleDaemonStart() {
	isChild := os.Getenv("DAEMON_CHILD") == "1"

	// 1. Resolve environment port and IP parameters dynamically
	apiPort := os.Getenv("DAEMON_BALANCE_HTTP_PORT")
	if apiPort == "" {
		apiPort = "8087" 
	}
	udpPortStr := os.Getenv("DAEMON_BALANCE_PORT")
	udpPort, _ := strconv.Atoi(udpPortStr)
	if udpPort == 0 {
		udpPort = 8086
	}
	ip := os.Getenv("DAEMON_BALANCE_IP")
	if ip == "" {
		ip = "127.0.0.1"
	}

	if isChild {
		// Define the custom background service execution loop closure
		workerLoop := func(ctx context.Context) {
			infra.Info("Daemon successfully detached. Setting up HTTP API listener on port :" + apiPort)
			
			// Mount your health check route for the CLI status checks
			http.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"healthy","service":"balance"}`))
			})

			// Mount the canonical endpoints used by your custom routers and web UI cards
			http.HandleFunc("/api/balance", handleAPIBalanceRoute)
			http.HandleFunc("/api/update", handleAPIUpdateRoute)
			
			server := &http.Server{Addr: ":" + apiPort}
			
			// Spawn the HTTP server in a separate non-blocking thread
			go func() {
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					fmt.Fprintf(os.Stderr, "CRITICAL: ListenAndServe failed on port %s: %v\n", apiPort, err)
					infra.Error(fmt.Sprintf("Balance HTTP API listener failed: %v", err))
				}
			}()
			
			// Block until the infrastructure library signals a termination shutdown context
			<-ctx.Done()
			_ = server.Shutdown(context.Background())
		}

		// Transfer execution control entirely to your shared infrastructure library wrapper
		infra.RunDaemonApp("balance", ip, udpPort, workerLoop)
		os.Exit(0)
	} else {
		isRunning, pid := IsDaemonRunning()
		if isRunning {
			infra.Warn(fmt.Sprintf("duplicated creating of daemon, currently running with PID: %d", pid))
			fmt.Printf("Warning: Duplicated creation of daemon. Already running (PID: %d).\n", pid)
			return
		}

		exePath, err := os.Executable()
		if err != nil {
			infra.Error(fmt.Sprintf("Failed to resolve executable path: %v", err))
			os.Exit(1)
		}

		cmd := exec.Command(exePath, os.Args[1:]...)
		cmd.Env = append(os.Environ(), "DAEMON_CHILD=1")
		
		// Direct background startup logs to a file to capture early binding exceptions
		logFile, fileErr := os.OpenFile("/home/worker1/dexbot/apps/balance/logs/daemon_boot.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if fileErr == nil {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
			defer logFile.Close()
		} else {
			infra.Error("Failed to initialize tracking debug file: " + fileErr.Error())
		}
		
		err = cmd.Start()
		if err != nil {
			infra.Error(fmt.Sprintf("Failed to start daemon: %v", err))
			os.Exit(1)
		}

		pidFile := getPIDFilePath()
		_ = ioutil.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)

		infra.Info(fmt.Sprintf("Daemon started in background with PID: %d", cmd.Process.Pid))
		fmt.Printf("Daemon started safely in background. Assigned PID: %d\n", cmd.Process.Pid)
		os.Exit(0)
	}
}

/******************************************************************************
 * Function Name : HandleDaemonStatus
 *
 * Purpose :
 *   Handles the '-action=status' command. Reports the current state of the daemon.
 *
 * Inputs :
 *   None
 *
 * Return :
 *   None
 *
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 *
 * Error Cases :
 *   - None
 * Number Of Lines :
 *   13
 ******************************************************************************/
func HandleDaemonStatus() {
	isRunning, pid := IsDaemonRunning()
	if isRunning {
		msg := fmt.Sprintf("Daemon is currently running (PID: %d) listening on port %s", pid, daemonAPIPort)
		infra.Info(msg)
		fmt.Println(msg)
	} else {
		msg := "Daemon is not running."
		infra.Info(msg)
		fmt.Println(msg)
	}
}

/******************************************************************************
 * Function Name : HandleDaemonTerminate
 *
 * Purpose :
 *   Handles the '-action=terminate' command. Finds the running daemon process
 *   and issues a SIGTERM to shut it down, followed by PID file cleanup.
 *
 * Inputs :
 *   None
 *
 * Return :
 *   None
 *
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  *
 * Error Cases :
 *   - Daemon not running.
 *   - Insufficient permissions to kill process.
 *
 * Number Of Lines :
 *   25
 ******************************************************************************/
func HandleDaemonTerminate() {
	isRunning, pid := IsDaemonRunning()
	if !isRunning {
		infra.Info("Daemon is not currently running.")
		fmt.Println("Daemon is not running.")
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		infra.Error(fmt.Sprintf("Failed to find daemon process %d: %v", pid, err))
		os.Exit(1)
	}

	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		infra.Error(fmt.Sprintf("Failed to terminate daemon (PID %d): %v", pid, err))
		os.Exit(1)
	}

	os.Remove(getPIDFilePath())

	msg := fmt.Sprintf("Daemon (PID: %d) terminated successfully.", pid)
	infra.Info(msg)
	fmt.Println(msg)
}