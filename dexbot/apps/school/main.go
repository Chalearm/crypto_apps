/******************************************************************************
 * File Name        : main.go
 * File Path        : apps/school/main.go
 * Author           : Chalearm Saelim & Gemini
 * Owner            : Chalearm Saelim
 * Reviewer         : Chalearm Saelim
 *
 * Version          : 3.3.0
 * Status           : Development
 * Created Date     : 2026-07-01 19:25:44 (UTC+7)
 * Modified Date    : 2026-07-24 17:55:00 (UTC+7)
 *
 * Description      :
 *    School Daemon Orchestrator. Direct CLI routing for set-up, create-work,
 *    and status to bypass background daemon locks cleanly.
 *****************************************************************
 *
 * Responsibilities:
 *   - Part of the dexbot platform.
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/infra
 *
 *   External :
 *     - (stdlib only)
 *
 * Updated Parts :
 *   None
 *
 * New Parts :
 *   [Function] See function list.
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)        | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-26 08:00:00 (UTC+7)      | Chalearm Saelim | Initial
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add documentation.
 *
 * Notes :
 *   - Per regulator coding standard.
 *
 * Usage :
 *   Directory : (project root)
 *   Build     : go build
 *   Run       : ./binary
 */
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"dexbot/infra"
)

// initZombieReaper harvests dead Linux child processes non-blockingly to prevent zombie accumulation
/******************************************************************************
 * Function Name : initZombieReaper
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func initZombieReaper() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGCHLD)
	go func() {
		for range sigChan {
			for {
				var status syscall.WaitStatus
				pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
				if err != nil || pid <= 0 {
					break
				}
			}
		}
	}()
}

// ==============================
// GA MASTER SUPERVISOR & PATH HELPERS
// ==============================

/******************************************************************************
 * Function Name : resolvePythonPaths
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func resolvePythonPaths() (string, string) {
	pyExec := filepath.Join("venv", "bin", "python")
	if _, err := os.Stat(pyExec); os.IsNotExist(err) {
		pyExec = filepath.Join("apps", "school", "venv", "bin", "python")
		if _, err := os.Stat(pyExec); os.IsNotExist(err) {
			pyExec = "python3"
		}
	}

	scriptPath := "ga_master.py"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		scriptPath = filepath.Join("apps", "school", "ga_master.py")
	}

	return pyExec, scriptPath
}

// ==============================
// MAIN & CLI ENTRY POINT
// ==============================
/******************************************************************************
 * Function Name : main
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func main() {
	initZombieReaper()

	fs := flag.NewFlagSet("school", flag.ContinueOnError)
	action := fs.String("action", "start", "Action: start, stop, terminate, status, set-up, create-work, restart, clear-state")
	num := fs.Int("num", 1, "Number of workers for create-work")
	_ = fs.Parse(os.Args[1:])

	pyExec, scriptPath := resolvePythonPaths()
 
	// Direct synchronous execution CLI commands (Do NOT attach to daemon lock)
	if *action == "status" || *action == "create-work" || *action == "set-up" || *action == "clear-state" {
		cmd := exec.Command(pyExec, scriptPath, fmt.Sprintf("-action=%s", *action), fmt.Sprintf("-num=%d", *num))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		return
	}

	// For terminate and stop, clean underlying process tree FIRST
	if *action == "terminate" || *action == "stop" {
		infra.Info(fmt.Sprintf("🧹 [SCHOOL DAEMON] Executing complete cluster sweep (-action=%s)...", *action))
		cmd := exec.Command(pyExec, scriptPath, fmt.Sprintf("-action=%s", *action))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()

		_ = infra.HandleCLI(*action, "school")
		return
	}

	if infra.HandleCLI(*action, "school") {
		return
	}

	ip := os.Getenv("DAEMON_SCHOOL_IP")
	if ip == "" {
		ip = "127.0.0.1"
	}

	portStr := os.Getenv("DAEMON_SCHOOL_PORT")
	port := 8082
	if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
		port = p
	}

	infra.RunDaemonApp("school", ip, port, func(ctx context.Context) {
		infra.Info("🏫 [SCHOOL DAEMON] School Daemon initialized & connected to Governance.")
		startGAMasterOrchestrator(ctx, *action, *num)
		infra.Info("🏫 [SCHOOL DAEMON] Background jobs cleaned up cleanly.")
	})
}

/******************************************************************************
 * Function Name : startGAMasterOrchestrator
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

  *
  * Function Name : startGAMasterOrchestrator
  * Purpose :
  *   Performs its designated operation.
  * Inputs :
  *   None (see function signature)
  * Return :
  *   Type        : varies
  *   Description : Result of computation.
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  * Error Cases :
  *   - None
  * Number Of Lines :
  *   10
func startGAMasterOrchestrator(ctx context.Context, initialAction string, workerNum int) {
	pyExec, scriptPath := resolvePythonPaths()

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		infra.Error(fmt.Sprintf("❌ [GA SUPERVISOR] Master script not found: %s", scriptPath))
		return
	}
	infra.Info(fmt.Sprintf("🧬 [GA SUPERVISOR] Master Active | Python: %s | Script: %s", pyExec, scriptPath))

	cleanupCluster := func(actionType string) {
		infra.Warn(fmt.Sprintf("🧹 [GA SUPERVISOR] Executing cluster cleanup (-action=%s)...", actionType))
		cleanCmd := exec.Command(pyExec, scriptPath, fmt.Sprintf("-action=%s", actionType))
		_ = cleanCmd.Run()
	}

	for {
		select {
		case <-ctx.Done():
			infra.Info("🏫 [SCHOOL DAEMON] Daemon stopping. Requesting graceful shutdown...")
			cleanupCluster("stop")
			return
		default:
		}

		infra.Info("🚀 [GA SUPERVISOR] Spawning Distributed GA Master Engine (ga_master.py)...")

		cmdArgs := []string{scriptPath, "-v", fmt.Sprintf("-action=%s", initialAction), fmt.Sprintf("-num=%d", workerNum)}
		cmd := exec.CommandContext(ctx, pyExec, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		startTime := time.Now()
		err := cmd.Run()
		duration := time.Since(startTime)

		if ctx.Err() != nil {
			infra.Info("🏫 [SCHOOL DAEMON] Context canceled. Executing graceful process stop...")
			cleanupCluster("stop")
			return
		}

		if err != nil {
			infra.Error(fmt.Sprintf("💥 [GA SUPERVISOR] Master Engine exited abruptly after %s: %v", duration, err))
			infra.Warn("🚨 [GA SUPERVISOR] Crash/OOM-Kill detected! Sweeping stale processes & restarting from checkpoint...")

			cleanupCluster("terminate")
			initialAction = "start"

			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		} else {
			infra.Info(fmt.Sprintf("✅ [GA SUPERVISOR] GA evolution cycle completed successfully in %s.", duration))
			infra.Info("⏳ [GA SUPERVISOR] Cooldown active. Next evolution pass in 5 minutes...")

			select {
			case <-ctx.Done():
				cleanupCluster("stop")
				return
			case <-time.After(5 * time.Minute):
				initialAction = "start"
			}
		}
	}
}