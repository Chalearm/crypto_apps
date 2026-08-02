/******************************************************************************
 * File Name        : main.go
 * File Path        : apps/school/main.go
 * Author           : Chalearm Saelim & Gemini
 * Owner            : Chalearm Saelim
 * Reviewer         : Chalearm Saelim
 * Version          : 3.7.0
 * Status           : Development
 * Created Date     : 2026-07-01 19:25:44 (UTC+7)
 * Modified Date    : 2026-07-31 15:20:00 (UTC+7)
 *
 * Description      :
 *    School Daemon Orchestrator. Serves as process supervisor for the underlying
 *    Python GA Master engine (`ga_master.py`), non-blockingly reaps zombie child 
 *    processes, handles CLI actions, and provides auto-recovery/restart loops on crash.
 *
 * DEPENDENCY TREE & STRUCTURAL MAP:
 * ───────────────────────────────────────────────────────────────────────────
 * [apps/school/main.go] (School Daemon Orchestrator)
 *      │
 *      ├── Imports Internal Module ──> [dexbot/infra] (Logger, Daemon Runner, CLI Dispatch)
 *      │
 *      ├── Subprocess Execution Pipeline:
 *      │      ├── Resolves Python Venv Path ─> /opt/venv/bin/python3 OR venv/bin/python
 *      │      ├── Direct Synchronous CLI ───> Executes `ga_master.py` [-action=create-work|set-up|etc]
 *      │      └── Daemon Supervisor ────────> Spawns and supervises `ga_master.py` [-action=start]
 *      │
 *      ├── Checkpoint Telemetry Inspector:
 *      │      └── printStatusReport() ──────> Parses lstm_ga_checkpoint.json directly for `-action=status`
 *      │
 *      └── OS Subprocess Reaper:
 *             └── Non-blocking wait loop on SIGCHLD to prevent zombie process accumulation
 *
 * FUNCTION DEPENDENCY MATRIX (Internal Methods):
 * ───────────────────────────────────────────────────────────────────────────
 * main()
 *  ├── initZombieReaper()
 *  ├── resolvePythonPaths()
 *  ├── printStatusReport() ──────────────> Parses & renders live checkpoint metrics
 *  ├── [If direct CLI action] ───────────> exec.Command(python, script, args...).Run()
 *  ├── [If stop/terminate action] ──────> infra.HandleCLI()
 *  └── infra.RunDaemonApp()
 *        └── startGAMasterOrchestrator()
 *            ├── resolvePythonPaths()
 *            └── Process Lifecycle Loop:
 *                  ├── exec.CommandContext(python, ga_master.py, args...).Run()
 *                  ├── [On Error/Crash] ───> Sweeps cluster & restarts loop
 *                  └── [On Success] ───────> Enters 5-minute cooldown period
 *
 * Responsibilities :
 *    - Resolves virtual environment paths across container and local environments.
 *    - Reaps dead Linux child processes non-blockingly using `syscall.Wait4`.
 *    - Parses active `lstm_ga_checkpoint.json` files to display real-time cluster telemetry.
 *    - Supervises Python GA process lifecycle with automatic recovery on out-of-memory or crashes.
 *    - Forwards log rotation, checkpoint configuration, and lookahead buffer size parameters to `ga_master.py`.
 *
 * Usage :
 *    Directory : apps/school/
 *    Build     : go build -o school apps/school/main.go
 *    Run       : ./school -action=status
 *
 * Dependencies :
 *    Internal  : dexbot/infra
 *    External  : stdlib (context, encoding/json, flag, os, exec, signal, filepath, strconv, syscall, time)
 *
 * Change History :
 *    -------------------------------------------------------------------------
 *    Version | Date Time (UTC+7)         | Author          | Description
 *    -------------------------------------------------------------------------
 *    1.0.0   | 2026-07-26 08:00:00 (UTC+7) | Chalearm Saelim | Initial release
 *    3.5.0   | 2026-07-30 16:10:00 (UTC+7) | Chalearm Saelim | Added -buffer-size support for asynchronous queue
 *    3.6.0   | 2026-07-31 15:15:00 (UTC+7) | Chalearm Saelim | Enhanced telemetry logging & process supervisor
 *    3.7.0   | 2026-07-31 15:20:00 (UTC+7) | Chalearm Saelim | Integrated direct printStatusReport JSON inspector
 *    -------------------------------------------------------------------------
 *
 * Notes :
 *    - Per regulator coding standard rules.
 ******************************************************************************/
package main

import (
	"context"
	//"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	//"strings"

	"dexbot/infra"
)

// CheckpointData represents the top-level structure of lstm_ga_checkpoint.json
type CheckpointData struct {
	RunID             string       `json:"run_id"`
	Generation        int          `json:"generation"`
	CurrentGeneration int          `json:"current_generation"`
	MaxGenerations    int          `json:"max_generations"`
	EvaluatedCount    int          `json:"evaluated_count"`
	TotalPopulation   int          `json:"total_population"`
	Chromosomes       []Chromosome `json:"chromosomes"`
	Population        []Chromosome `json:"chromosome_population"`
	Timestamp         float64      `json:"timestamp"`
}

// Chromosome represents individual model structures in the checkpoint
type Chromosome struct {
	ID               string    `json:"id"`
	FitnessEvaluated bool      `json:"fitness_evaluated"`
	PerfVector       []float64 `json:"perf_vector"`
}

// ConfigParams holds runtime options for GA orchestrator
type ConfigParams struct {
	SaveMin    int
	SavePct    float64
	RotateMin  int
	RotateMB   float64
	BufferSize int
}

/******************************************************************************
 * Function Name : initZombieReaper
 *
 * Purpose :
 *    Registers a signal handler for SIGCHLD and harvests dead child processes
 *    non-blockingly using syscall.Wait4 to avoid zombie process accumulation in Linux.
 *
 * Inputs :
 *    None
 *
 * Return :
 *    None
 *
 * Complexity :
 *    Time  : O(1) continuous background signal channel loop.
 *    Space : O(1)
 *
 * Error Cases :
 *    - Ignores syscall wait errors when no child process remains.
 *
 * Number Of Lines :
 *    18
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

/******************************************************************************
 * Function Name : resolvePythonPaths
 *
 * Purpose :
 *    Resolves local virtualenv Python executable binary and target ga_master.py
 *    script file paths across container paths (/opt/venv) and local directories.
 *
 * Inputs :
 *    None
 *
 * Return :
 *    Type        : (string, string)
 *    Description : Python binary path and ga_master.py script file path.
 *
 * Complexity :
 *    Time  : O(1) stat check sequence.
 *    Space : O(1)
 *
 * Error Cases :
 *    - Fallback defaults to system `python3` if no virtualenv bin is located.
 *
 * Number Of Lines :
 *    22
 ******************************************************************************/
func resolvePythonPaths() (string, string) {
	pyExec := "/opt/venv/bin/python3"
	if _, err := os.Stat(pyExec); os.IsNotExist(err) {
		pyExec = filepath.Join("venv", "bin", "python")
		if _, err := os.Stat(pyExec); os.IsNotExist(err) {
			pyExec = filepath.Join("apps", "school", "venv", "bin", "python")
			if _, err := os.Stat(pyExec); os.IsNotExist(err) {
				pyExec = "python3"
			}
		}
	}

	scriptPath := "ga_master.py"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		scriptPath = filepath.Join("apps", "school", "ga_master.py")
	}

	return pyExec, scriptPath
}


/******************************************************************************
 * Function Name : printStatusReport
 *
 * Purpose :
 *    Inspects and renders current execution progress directly from active JSON
 *    checkpoint files, and queries Celery/Redis broker inspect telemetry to display
 *    connected worker nodes, concurrency capacity, and real-time task assignments.
 *
 * Inputs :
 *    None
 *
 * Return :
 *    Type        : None
 *    Description : Prints complete cluster telemetry and worker details to stdout.
 *
 * Complexity :
 *    Time  : O(N + W) where N is chromosome count and W is active worker nodes.
 *    Space : O(N) for unmarshaling JSON data structures.
 ******************************************************************************/
func printStatusReport() {
	pyExec, scriptPath := resolvePythonPaths()
	// Call Python directly for status
	cmd := exec.Command(pyExec, scriptPath, "-action=status")
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

/******************************************************************************
 * Function Name : main
 *
 * Purpose :
 *    Main entry point for School Daemon Orchestrator. Parses CLI parameters,
 *    executes direct status checks, manages cluster cleanup actions, or initializes
 *    the supervised daemon process.
 *
 * Inputs :
 *    None (Reads flags from os.Args)
 *
 * Return :
 *    Type        : None
 *    Description : Executes requested CLI actions or launches background daemon.
 *
 * Complexity :
 *    Time  : O(1) for flag parsing and synchronous command execution.
 *    Space : O(1)
 *
 * Error Cases :
 *    - Exits cleanly on CLI action completion or parameter validation failure.
 *
 * Number Of Lines :
 *    85
 ******************************************************************************/
func main() {
	initZombieReaper()

	fs := flag.NewFlagSet("school", flag.ContinueOnError)
	action := fs.String("action", "start", "Action: start, update, stop, terminate, status, set-up, create-work, restart, clear-state")
	num := fs.Int("num", 1, "Number of workers for create-work")
	generations := fs.Int("generations", 50, "Target maximum generations limit (Unbounded, e.g., 50, 100, 2334)")
	saveMin := fs.Int("save-min", 20, "Checkpoint save interval in minutes")
	savePct := fs.Float64("save-pct", 25.0, "Selection percentage")
	rotateMin := fs.Int("rotate-min", 30, "Log rotation interval in minutes")
	rotateMB := fs.Float64("rotate-mb", 30.0, "Log rotation size threshold in MB")
	bufferSize := fs.Int("buffer-size", 25, "Maximum lookahead task buffer size in Redis")
	warmStart := fs.Bool("warm-start", false, "Seed top candidate architectures from prior run checkpoint")
	_ = fs.Parse(os.Args[1:])

	cfg := ConfigParams{
		SaveMin:    *saveMin,
		SavePct:    *savePct,
		RotateMin:  *rotateMin,
		RotateMB:   *rotateMB,
		BufferSize: *bufferSize,
	}

	pyExec, scriptPath := resolvePythonPaths()

	// --------------------------------------------------------------------------
	// 1. DIRECT STATUS DISPATCH (Prevents duplicate status report executions)
	// --------------------------------------------------------------------------
	if *action == "status" {
		printStatusReport()
		return
	}

	// --------------------------------------------------------------------------
	// 2. SYNCHRONOUS DIRECT CLI COMMAND EXECUTION
	// --------------------------------------------------------------------------
	if *action == "create-work" || *action == "set-up" || *action == "clear-state" || *action == "update" {
		infra.Info(fmt.Sprintf("⚡ [CLI DISPATCH] Executing command: -action=%s (Workers: %d | Target Gen: %d)", *action, *num, *generations))
		cmdArgs := []string{
			scriptPath,
			fmt.Sprintf("-action=%s", *action),
			fmt.Sprintf("-num=%d", *num),
			fmt.Sprintf("-generations=%d", *generations),
			fmt.Sprintf("-save-min=%d", cfg.SaveMin),
			fmt.Sprintf("-save-pct=%.1f", cfg.SavePct),
			fmt.Sprintf("-rotate-min=%d", cfg.RotateMin),
			fmt.Sprintf("-rotate-mb=%.1f", cfg.RotateMB),
			fmt.Sprintf("-buffer-size=%d", cfg.BufferSize),
			fmt.Sprintf("-warm-start=%t", *warmStart),
		}
		cmd := exec.Command(pyExec, cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Run(); err != nil {
			infra.Error(fmt.Sprintf("❌ [CLI ERROR] Action '%s' failed: %v", *action, err))
		} else {
			infra.Info(fmt.Sprintf("✅ [CLI SUCCESS] Action '%s' completed successfully.", *action))
		}
		return
	}

	// --------------------------------------------------------------------------
	// 3. CLUSTER SWEEP FOR STOP / TERMINATE
	// --------------------------------------------------------------------------
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

	// --------------------------------------------------------------------------
	// 4. DAEMON APPLICATION INITIALIZATION
	// --------------------------------------------------------------------------
	infra.RunDaemonApp("school", ip, port, func(ctx context.Context) {
		infra.Info("🏫 [SCHOOL DAEMON] School Daemon initialized & connected to Governance.")
		startGAMasterOrchestrator(ctx, *action, *num, *generations, *warmStart, cfg)
		infra.Info("🏫 [SCHOOL DAEMON] Background jobs cleaned up cleanly.")
	})
}

/******************************************************************************
 * Function Name : startGAMasterOrchestrator
 *
 * Purpose :
 *    Supervises the ga_master.py process inside an execution loop, forwarding
 *    configuration flags, managing process stdout/stderr, and providing auto-recovery
 *    with cluster sweeps on process crashes or out-of-memory events.
 *
 * Inputs :
 *    - ctx            : Context for daemon lifecycle control and cancellation.
 *    - initialAction  : Primary action string ("start", etc.).
 *    - workerNum      : Number of worker processes.
 *    - maxGenerations : Maximum target generations ceiling limit.
 *    - warmStart      : Boolean flag enabling warm-start candidate seeding.
 *    - cfg            : ConfigParams struct containing save, log rotation, and buffer settings.
 *
 * Return :
 *    Type        : None
 *    Description : Spawns and supervises Python GA Master process.
 *
 * Complexity :
 *    Time  : Continuous daemon loop execution.
 *    Space : O(1)
 *
 * Error Cases :
 *    - Logs errors and executes cluster sweeps on script crash before restarting loop.
 *
 * Number Of Lines :
 *    80
 ******************************************************************************/
func startGAMasterOrchestrator(ctx context.Context, initialAction string, workerNum int, maxGenerations int, warmStart bool, cfg ConfigParams) {
	pyExec, scriptPath := resolvePythonPaths()

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		infra.Error(fmt.Sprintf("❌ [GA SUPERVISOR] Master script not found: %s", scriptPath))
		return
	}
	infra.Info(fmt.Sprintf("🧬 [GA SUPERVISOR] Master Active | Python: %s | Script: %s | Target Gen: %d | Warm Start: %t", 
		pyExec, scriptPath, maxGenerations, warmStart))

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

		infra.Info(fmt.Sprintf("🚀 [GA SUPERVISOR] Spawning Distributed GA Master Engine (ga_master.py) [Action: %s | Workers: %d | Target Gen: %d | Buffer: %d]...", 
			initialAction, workerNum, maxGenerations, cfg.BufferSize))

		cmdArgs := []string{
			scriptPath,
			"-v",
			fmt.Sprintf("-action=%s", initialAction),
			fmt.Sprintf("-num=%d", workerNum),
			fmt.Sprintf("-generations=%d", maxGenerations),
			fmt.Sprintf("-save-min=%d", cfg.SaveMin),
			fmt.Sprintf("-save-pct=%.1f", cfg.SavePct),
			fmt.Sprintf("-rotate-min=%d", cfg.RotateMin),
			fmt.Sprintf("-rotate-mb=%.1f", cfg.RotateMB),
			fmt.Sprintf("-buffer-size=%d", cfg.BufferSize),
			fmt.Sprintf("-warm-start=%t", warmStart),
		}
		
		cmd := exec.CommandContext(ctx, pyExec, cmdArgs...)
		cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
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
			infra.Error(fmt.Sprintf("💥 [GA SUPERVISOR ERROR] Process failed after %s: %v", duration, err))
			infra.Warn("🔄 [GA SUPERVISOR RECOVERY] Sweeping cluster state and initiating restart in 5s...")
			cleanupCluster("terminate")
			initialAction = "start"

			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		} else {
			infra.Info(fmt.Sprintf("✅ [GA SUPERVISOR SUCCESS] GA evolution cycle completed successfully in %s.", duration))
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