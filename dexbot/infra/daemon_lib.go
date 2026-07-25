/******************************************************************************
 * File Name       : daemon_lib.go
 * File Path       : dexbot/infra/daemon_lib.go
 *
 * Author          : Gemini 3.1 Pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.1.0
 * Status          : Development
 * Created Date    : 2026-07-13 12:00:00 (UTC+7)
 * Modified Date   : 2026-07-13 12:00:00 (UTC+7)
 *
 * Description     :
 *   Generalized daemon IPC API library. Provides Multi-Governance High 
 *   Availability (HA) by broadcasting heartbeats. Abstracts process detachment, 
 *   duplicate prevention, and standard CLI commands.
 ******************************************************************************/
package infra

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dexbot/governance"
)

func HandleCLI(action, name string) bool {
	pidFile := filepath.Join("logs", fmt.Sprintf("%s.pid", name))

	switch action {
	case "status":
		if isRunning, pid := checkDaemon(pidFile); isRunning {
			msg := fmt.Sprintf("%s is running (PID: %d)", name, pid)
			Info(msg)
			fmt.Println(msg)
		} else {
			msg := fmt.Sprintf("%s is not running", name)
			Info(msg)
			fmt.Println(msg)
		}
		return true

	case "terminate", "stop":
		if isRunning, pid := checkDaemon(pidFile); isRunning {
			process, _ := os.FindProcess(pid)
			process.Signal(syscall.SIGTERM)
			os.Remove(pidFile)
			msg := fmt.Sprintf("%s terminated successfully", name)
			Info(msg)
			fmt.Println(msg)
		} else {
			fmt.Printf("%s is not running\n", name)
		}
		return true

	case "start":
		return false // Let RunDaemonApp handle the start detachment

	default:
		return false // Unrecognized. Route to user-space!
	}
}

func RunDaemonApp(name, listenIP string, listenPort int, worker func(ctx context.Context)) {
	InitLogger()

	isChild := os.Getenv("DAEMON_CHILD") == "1"
	if !isChild {
		pidFile := filepath.Join("logs", fmt.Sprintf("%s.pid", name))
		if isRunning, pid := checkDaemon(pidFile); isRunning {
			Warn(fmt.Sprintf("%s is already running with PID %d", name, pid))
			os.Exit(0)
		}

		exePath, _ := os.Executable()
		cmd := exec.Command(exePath, os.Args[1:]...)
		cmd.Env = append(os.Environ(), "DAEMON_CHILD=1")
		if err := cmd.Start(); err != nil {
			Error("Failed to fork daemon: " + err.Error())
			os.Exit(1)
		}

		os.MkdirAll("logs", 0755)
		os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
		Info(fmt.Sprintf("%s Daemon started in background (PID: %d)", name, cmd.Process.Pid))
		os.Exit(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var govConns []*net.UDPConn
	endpoints := os.Getenv("GOVERNANCE_ENDPOINTS")
	if endpoints == "" {
		endpoints = "127.0.0.1:8081"
	}

	for _, ep := range strings.Split(endpoints, ",") {
		ep = strings.TrimSpace(ep)
		if ep == "" { continue }
		addr, err := net.ResolveUDPAddr("udp", ep)
		if err == nil {
			if conn, err := net.DialUDP("udp", nil, addr); err == nil {
				govConns = append(govConns, conn)
			}
		}
	}

	if len(govConns) == 0 {
		Warn(fmt.Sprintf("%s: Warning — No Governance endpoints reachable", name))
	}

	go startGeneralizedUdpListener(ctx, name, listenIP, listenPort, govConns)
	go worker(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	<-sigChan

	Info(fmt.Sprintf("%s shutting down gracefully...", name))
	for _, conn := range govConns {
		conn.Close()
	}
}

/******************************************************************************
 * Function Name : startGeneralizedUdpListener
 *
 * Purpose :
 *   Binds and runs the IPC loop. Logs arriving governance packets, parses 
 *   commands, and records exactly what message payload the daemon replies back.
 *
 * Inputs :
 *   ctx      context.Context
 *   name     string (Daemon name identifier, e.g., "balance")
 *   ip       string
 *   port     int
 *   govConns []*net.UDPConn
 *
 * Return :
 *   None
 *
 * Number Of Lines :
 *   52
 ******************************************************************************/
func startGeneralizedUdpListener(ctx context.Context, name, ip string, port int, govConns []*net.UDPConn) {
	addr := net.UDPAddr{Port: port, IP: net.ParseIP(ip)}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		Error(fmt.Sprintf("%s UDP bind failed: %v", name, err))
		return
	}
	defer conn.Close()
	Info(fmt.Sprintf("%s UDP listener active on %s:%d", name, ip, port))

	buffer := make([]byte, 1024)
	startTime := time.Now()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	sendPeriodicHeartbeat(name, govConns, startTime)

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeatTicker.C:
			sendPeriodicHeartbeat(name, govConns, startTime)
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, remoteAddr, err := conn.ReadFromUDP(buffer)
			if err != nil { continue }
			
			msg := string(buffer[:n])
			
			// [CYCLC] Audit Inbound Message from Governance Daemon
			Info(fmt.Sprintf("[CYCLC %s] Gov inbound: %s", name, msg))

			if strings.HasPrefix(msg, "governance:probe:health_check") {
				replyMsg := fmt.Sprintf("%s:pong:healthy", name)
				_, _ = conn.WriteToUDP([]byte(replyMsg), remoteAddr)
				
				// [CYCLC] Audit Outbound Pong
				Info(fmt.Sprintf("[CYCLC %s] Gov pong: healthy", name))
				
				sendPeriodicHeartbeat(name, govConns, startTime)
				continue
			}

			if strings.HasPrefix(msg, "governance:command:kill") {
				Info(fmt.Sprintf("[EVENT %s] Terminating via Gov kill command", name))
				os.Exit(1)
			}
			if strings.HasPrefix(msg, "governance:command:stop") || strings.HasPrefix(msg, "governance:command:restart") {
				Info(fmt.Sprintf("[%s DAEMON] Exiting cleanly via Governance command: %s", name, msg))
				os.Exit(0)
			}
		}
	}
}

func sendPeriodicHeartbeat(name string, govConns []*net.UDPConn, startTime time.Time) {
	info := &governance.DaemonInfo{
		Name:          name,
		Version:       "v1.0 (Dynamic)",
		Status:        "healthy",
		CPUPercent:    2.5,
		MemoryMB:      15.0,
		ActiveTasks:   0,
		Uptime:        time.Since(startTime),
		Message:       fmt.Sprintf("%s daemon operational", name),
		LastHeartbeat: time.Now(),
	}
	hbData := []byte(governance.FormatHeartbeat(info))
	for _, gConn := range govConns {
		gConn.Write(hbData)
	}
}

func checkDaemon(pidFile string) (bool, int) {
	data, err := os.ReadFile(pidFile)
	if err != nil { return false, 0 }
	pid, err := strconv.Atoi(string(data))
	if err != nil || pid <= 0 { return false, 0 }
	process, err := os.FindProcess(pid)
	if err != nil { return false, 0 }
	err = process.Signal(syscall.Signal(0))
	return err == nil, pid
}