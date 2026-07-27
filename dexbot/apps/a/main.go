/******************************************************************************
 * File Name       : main.go
 * File Path       : dexbot/apps/a/main.go
 *
 * Author          : Gemini 3.1 Pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-13 12:00:00 (UTC+7)
 * Modified Date   : 2026-07-13 12:00:00 (UTC+7)
 *
 * Description     : Sample App A utilizing the new generalized API.
 *
 * Usage :
 *   Directory : apps/a/
 *   Build     : go build -o a .
 *   Run       : ./a -action=start
 *****************************************************************************
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
 */
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"dexbot/infra"
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
	action := flag.String("action", "start", "Action: start, status, terminate")
	flag.Parse()

	infra.InitLogger()

	// 1. Let the library handle standard operations first!
	if infra.HandleCLI(*action, "a") {
		return
	}

	if *action != "start" {
		infra.Error("Unknown action: " + *action)
		return
	}

	infra.LoadEnv("../../config.env")
	daemonName := "a"

	port, _ := strconv.Atoi(os.Getenv("DAEMON_A_PORT"))
	if port == 0 { port = 8084 }
	
	ip := os.Getenv("DAEMON_A_IP")
	if ip == "" { ip = "127.0.0.1" }

	workerLoop := func(ctx context.Context) {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fmt.Println("hello I am A")
				infra.Info("hello I am A")
			}
		}
	}

	infra.RunDaemonApp(daemonName, ip, port, workerLoop)
}