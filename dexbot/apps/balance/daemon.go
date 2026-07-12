/******************************************************************************
 * File Name       : daemon.go
 * File Path       : apps/balance/daemon.go
 *
 * Author          : Gemini 3.1 Pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.5.0
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
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)       | Author         | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-12 14:37:38     | Gemini         | Initial version
 *   1.3.0   | 2026-07-12 15:25:00     | Gemini         | Fixed child PID exit bug
 *   1.4.0   | 2026-07-12 16:20:00     | Gemini         | Added embedded HTTP API server
 *   1.5.0   | 2026-07-12 16:25:00     | Gemini         | Connected full JSON payload logic
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add token authentication to the localized HTTP port loop.
 ******************************************************************************/
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"dexbot/infra"
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
 * Error Cases :
 *   - Cannot determine current working directory.
 *
 * Number Of Lines :
 *   13
 ******************************************************************************/
func getPIDFilePath() string {
	cwd, err := os.Getwd()
	if err != nil {
		infra.Error("Failed to get current working directory: " + err.Error())
		return filepath.Join(logDirName, pidFileName)
	}
	
	logPath := filepath.Join(cwd, logDirName)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		os.MkdirAll(logPath, 0755)
	}
	
	return filepath.Join(logPath, pidFileName)
}

/******************************************************************************
 * Function Name : IsDaemonRunning
 *
 * Purpose :
 *   Checks if the daemon process is currently running by reading the PID
 *   file and sending a signal 0 to the process.
 *
 * Inputs :
 *   None
 *
 * Return :
 *   Type        : bool, int
 *   Description : Returns true and the PID if running, false and 0 otherwise.
 *
 * Error Cases :
 *   - PID file missing.
 *   - Process no longer exists (stale PID file).
 *
 * Number Of Lines :
 *   24
 ******************************************************************************/
func IsDaemonRunning() (bool, int) {
	pidFile := getPIDFilePath()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false, 0
	}

	pidStr := string(data)
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return false, 0
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}

	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false, 0
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
 * Function Name : HandleDaemonStart
 *
 * Purpose :
 *   Handles the '-action=start' command. Detaches the process to the background
 *   and registers the embedded HTTP API server to listen for socket queries.
 *
 * Inputs :
 *   None
 *
 * Return :
 *   None
 *
 * Error Cases :
 *   - Daemon already running (exits program).
 *   - HTTP server fails to bind to port.
 *
 * Number Of Lines :
 *   40
 ******************************************************************************/
func HandleDaemonStart() {
	isChild := os.Getenv("DAEMON_CHILD") == "1"

	if isChild {
		infra.Info("Daemon successfully detached. Setting up HTTP API listener on port " + daemonAPIPort)
		
		http.HandleFunc("/api/update", handleAPIUpdateRoute)
		
		err := http.ListenAndServe(daemonAPIPort, nil)
		if err != nil {
			infra.Error(fmt.Sprintf("Daemon HTTP API server failed to start: %v", err))
			os.Exit(1)
		}
	} else {
		isRunning, pid := IsDaemonRunning()
		if isRunning {
			infra.Warn(fmt.Sprintf("duplicated creating of daemon, currently running with PID: %d", pid))
			os.Exit(0)
		}

		exePath, err := os.Executable()
		if err != nil {
			infra.Error(fmt.Sprintf("Failed to resolve executable path: %v", err))
			os.Exit(1)
		}

		cmd := exec.Command(exePath, os.Args[1:]...)
		cmd.Env = append(os.Environ(), "DAEMON_CHILD=1")
		
		err = cmd.Start()
		if err != nil {
			infra.Error(fmt.Sprintf("Failed to start daemon: %v", err))
			os.Exit(1)
		}

		pidFile := getPIDFilePath()
		err = os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
		if err != nil {
			infra.Error(fmt.Sprintf("Failed to write PID file: %v", err))
			os.Exit(1)
		}

		infra.Info(fmt.Sprintf("Daemon started in background with PID: %d", cmd.Process.Pid))
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