/******************************************************************************
 * File Name       : main.go
 * File Path       : apps/start_all/main.go
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:42 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:42 (UTC+7)
 * Description     : Unified launcher for Dexbot daemons and HTTP server.
 ******************************************************************************/
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"dexbot/infra"
)

type daemonDef struct {
	Name    string   // Human-readable name
	Path    string   // Relative path to main.go
	Args    []string // Additional args
	Enabled bool
}

/******************************************************************************
 * Function Name : isSingleContainerMode
 * Purpose       : Reads SINGLE_CONTAINER_MODE from environment. Returns true unless
 *                 explicitly set to false/0/no/off.
 ******************************************************************************/
func isSingleContainerMode() bool {
	v := strings.ToLower(os.Getenv("SINGLE_CONTAINER_MODE"))
	if v == "false" || v == "0" || v == "no" || v == "off" {
		return false
	}
	return true
}

/******************************************************************************
 * Function Name : main
 * Purpose       : Entry point for unified launcher. Detects deployment mode
 *                 and launches appropriate daemons.
 ******************************************************************************/
func main() {
	infra.InitLogger()
	singleMode := isSingleContainerMode()

	fmt.Println("")
	fmt.Println("============================================================")
	fmt.Println("  DEXBOT PLATFORM — Unified Launcher v2.0")
	if singleMode {
		fmt.Println("  MODE: single-container (all daemons)")
	} else {
		fmt.Println("  MODE: distributed (per-container daemons)")
	}
	fmt.Println("============================================================")
	fmt.Println("")

	allDaemons := []daemonDef{
		{Name: "governance", Path: "./apps/governance", Args: nil, Enabled: true},
		{Name: "school", Path: "./apps/school", Args: nil, Enabled: singleMode},
		{Name: "trading", Path: "./apps/trading", Args: nil, Enabled: true},
		{Name: "testdaemon", Path: "./testdaemon", Args: []string{"-action=start"}, Enabled: true},
		{Name: "balance", Path: "./apps/balance", Args: []string{"-action=start"}, Enabled: true},
	}

	if !singleMode {
		hostname, _ := os.Hostname()
		if strings.HasPrefix(hostname, "worker2") || os.Getenv("WORKER_ROLE") == "school" {
			allDaemons = []daemonDef{
				{Name: "school", Path: "./apps/school", Args: nil, Enabled: true},
			}
		} else {
			allDaemons = []daemonDef{
				{Name: "governance", Path: "./apps/governance", Args: nil, Enabled: true},
				{Name: "trading", Path: "./apps/trading", Args: nil, Enabled: true},
				{Name: "testdaemon", Path: "./testdaemon", Args: []string{"-action=start"}, Enabled: true},
			}
		}
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	infra.Info(fmt.Sprintf("Launching daemons (singleContainer=%v)...", singleMode))
	for _, d := range allDaemons {
		if !d.Enabled {
			continue
		}
		wg.Add(1)
		go launchDaemon(ctx, d, &wg)
		time.Sleep(800 * time.Millisecond)
	}

	if singleMode || os.Getenv("WORKER_ROLE") != "school" {
		infra.Info("Launching Python HTTP server (serve.py)...")
		wg.Add(1)
		go launchPython(ctx, &wg)
		time.Sleep(1 * time.Second)
	}

	fmt.Println("")
	fmt.Println("============================================================")
	fmt.Println("  All daemons launched.")
	fmt.Println("  Dashboard:   http://localhost:8080")
	fmt.Println("  Press Ctrl+C to stop all daemons.")
	fmt.Println("============================================================")
	fmt.Println("")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	<-sigChan

	fmt.Println("\nShutting down all daemons...")
	cancel()
	wg.Wait()
	fmt.Println("All daemons stopped. Goodbye.")
}

/******************************************************************************
 * Function Name : launchDaemon
 * Purpose       : Launches a Go daemon as a subprocess with auto-restart on crash.
 ******************************************************************************/
func launchDaemon(ctx context.Context, d daemonDef, wg *sync.WaitGroup) {
	infra.FnTrace(fmt.Sprintf("starting %s", d.Name))
	defer wg.Done()
	defer infra.FnTrace(fmt.Sprintf("%s exited", d.Name))

	goBin := "/usr/local/go/bin/go"
	if _, err := os.Stat(goBin); err != nil {
		goBin = "go"
	}

	for {
		args := []string{"run", "-a", d.Path}
		args = append(args, d.Args...)
		cmd := exec.CommandContext(ctx, goBin, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		infra.Info(fmt.Sprintf("Starting %s: %s %v", d.Name, goBin, args))
		if err := cmd.Start(); err != nil {
			infra.Error(fmt.Sprintf("Failed to start %s: %v", d.Name, err))
			return
		}

		fmt.Printf("  [OK] %-12s PID=%d\n", d.Name, cmd.Process.Pid)

		err := cmd.Wait()

		select {
		case <-ctx.Done():
			return
		default:
		}

		if err != nil {
			infra.Warn(fmt.Sprintf("%s exited: %v — restarting in 5s", d.Name, err))
			fmt.Printf("  [!!] %-12s exited — restarting in 5s...\n", d.Name)
			time.Sleep(5 * time.Second)
		} else {
			return
		}
	}
}

/******************************************************************************
 * Function Name : launchPython
 * Purpose       : Launches the Python3 HTTP server (serve.py) with auto-restart.
 ******************************************************************************/
func launchPython(ctx context.Context, wg *sync.WaitGroup) {
	infra.FnTrace("starting serve.py")
	defer wg.Done()

	dexbotRoot := "."
	if _, err := os.Stat("serve.py"); err != nil {
		for _, p := range []string{"/home/worker1/dexbot", "/home/worker2/dexbot", os.ExpandEnv("$HOME/dexbot")} {
			if _, err2 := os.Stat(p + "/serve.py"); err2 == nil {
				dexbotRoot = p
				break
			}
		}
	}

	for {
		cmd := exec.CommandContext(ctx, "python3", "serve.py")
		cmd.Dir = dexbotRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		infra.Info("Starting Python HTTP server")
		if err := cmd.Start(); err != nil {
			infra.Error("Failed to start serve.py: " + err.Error())
			return
		}

		fmt.Printf("  [OK] %-12s PID=%d\n", "serve.py", cmd.Process.Pid)

		err := cmd.Wait()

		select {
		case <-ctx.Done():
			return
		default:
		}

		if err != nil {
			infra.Warn("serve.py exited: " + err.Error() + " — restarting in 3s")
			time.Sleep(3 * time.Second)
		} else {
			return
		}
	}
}

var _ = strconv.Itoa