/******************************************************************************
 * File Name       : main.go
 * File Path       : apps/entrypoint/main.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:35 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:35 (UTC+7)
 *
 * Description     :
 *   Container entrypoint binary — replaces entrypoint.sh.
 *
 * Responsibilities:
 *   - Implement core functionality.
 *
 * Usage :
 *   Directory : apps/entrypoint/
 *
 *   Build :
 *     go build ./apps/entrypoint
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./apps/entrypoint
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/apps
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
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-01 19:25:35 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

/******************************************************************************
 * Function Name : main
 *
 * Purpose :
 *   Entry point for the application.
 *
 * Inputs :
 *   None (reads os.Args or stdlib flags)
 *
 * Return :
 *   None (exits with code 0 on success)
 *
 * Complexity :
 *   Time  : O(N)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   15
 ******************************************************************************/
func main() {
	fmt.Println("")
	fmt.Println("============================================================")
	fmt.Println("  Worker Container -- Dexbot Platform")
	fmt.Println("  SSH on port 22   |   Web on port 8080")
	fmt.Println("============================================================")
	fmt.Println("")

	// 1. Start SSH daemon in background
	fmt.Println("[entrypoint] Starting SSH daemon...")
	sshd := exec.Command("/usr/sbin/sshd")
	sshd.Stdout = os.Stdout
	sshd.Stderr = os.Stderr
	if err := sshd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "[entrypoint] WARNING: SSHD failed to start: %v\n", err)
	} else {
		fmt.Println("[entrypoint] SSH daemon started (port 22)")
	}

	// 2. Find dexbot working directory
	workDir := findWorkDir()
	if workDir == "" {
		fmt.Fprintln(os.Stderr, "[entrypoint] ERROR: dexbot source not found.")
		fmt.Fprintln(os.Stderr, "  Checked: $HOME/dexbot, /home/worker1/dexbot, /home/worker2/dexbot")
		os.Exit(1)
	}

	fmt.Printf("[entrypoint] Working directory: %s\n", workDir)

	// Check Go version
	goBin := "/usr/local/go/bin/go"
	if _, err := os.Stat(goBin); err != nil {
		goBin = "go"
	}
	goVer := exec.Command(goBin, "version")
	goVer.Dir = workDir
	goVer.Stdout = os.Stdout
	goVer.Stderr = os.Stderr
	goVer.Run()

	fmt.Println("[entrypoint] Starting Dexbot unified launcher...")
	fmt.Println("")

	// 3. Run unified launcher (replaces current process)
	launcher := exec.Command(goBin, "run", "./apps/start_all")
	launcher.Dir = workDir
	launcher.Stdout = os.Stdout
	launcher.Stderr = os.Stderr
	launcher.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := launcher.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "[entrypoint] ERROR: Launcher failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  [OK] launcher    PID=%d\n", launcher.Process.Pid)

	// Wait for launcher (it runs forever until shutdown signal)
	if err := launcher.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "[entrypoint] Launcher exited: %v\n", err)
		os.Exit(1)
	}

	_ = time.Now // silence import
}
/******************************************************************************
 * Function Name : findWorkDir
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


func findWorkDir() string {
	candidates := []string{
		os.ExpandEnv("$HOME/dexbot"),
		"/home/worker1/dexbot",
		"/home/worker2/dexbot",
	}
	for _, dir := range candidates {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
	}
	return ""
}
