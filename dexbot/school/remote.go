/******************************************************************************
 * File Name       : remote.go
 * File Path       : school/remote.go
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
 *   Remote school sub-daemon client. Sends model training tasks to remote school instances over UDP and collects results. Each remote school trains a subset of the model population on a different VM/proce
 *
 * Responsibilities:
 *   - Implement core functionality for school package.
 *
 * Usage :
 *   Directory : school/
 *
 *   Build :
 *     go build ./school
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./school
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/school
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
package school

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"dexbot/infra"
)

// ==============================
// TYPES
// ==============================

/*
Struct: RemoteClient
Description:
  Manages UDP connections to remote school sub-daemons.
  Distributes models for training and collects fitness results.

Fields:
  - addrs     []string      : Remote addresses (ip:port)
  - timeout   time.Duration : UDP read timeout
  - studentsPerNode int     : Max models per remote node

Lines: ~5
*/
type RemoteClient struct {
	addrs           []string
	timeout         time.Duration
	studentsPerNode int
}

/*
Struct: TrainingResult
Description:
  Parsed fitness result returned by a remote school after training.

Fields:
  - ModelName  string  : Model that was trained
  - RemoteAddr string  : Which remote school trained it
  - Sharpe     float64 : Sharpe ratio from remote
  - Sortino    float64 : Sortino ratio
  - Profit     float64 : Profit
  - Drawdown   float64 : Maximum drawdown
  - Accuracy   float64 : Prediction accuracy
  - Consistency float64 : Consistency score
  - Efficiency float64 : Capital efficiency
  - Err        string  : Non-empty if training failed

Lines: ~10
*/
type TrainingResult struct {
	ModelName   string
	RemoteAddr  string
	Sharpe      float64
	Sortino     float64
	Profit      float64
	Drawdown    float64
	Accuracy    float64
	Consistency float64
	Efficiency  float64
	Err         string
}

/*
Function: NewRemoteClient
Description:
  Creates a new remote school client. If addrs is empty, remote
  training is disabled — the main school does everything locally.

Input:
  - addrs           []string      : Remote addresses (empty = local only)
  - timeoutSec      int           : UDP timeout in seconds
  - studentsPerNode int           : Max students per remote node

Output:
  - *RemoteClient: Initialized client

Lines: ~15
*/
func NewRemoteClient(addrs []string, timeoutSec, studentsPerNode int) *RemoteClient {
	infra.FnTrace(fmt.Sprintf("entering addrs=%v timeout=%ds perNode=%d", addrs, timeoutSec, studentsPerNode))
	defer infra.FnTrace("OK")
	return &RemoteClient{
		addrs:           addrs,
		timeout:         time.Duration(timeoutSec) * time.Second,
		studentsPerNode: studentsPerNode,
	}
}

/*
Function: IsEnabled
Description:
  Returns true if any remote schools are configured.

Input:
  - none

Output:
  - bool: True if remote training is available

Lines: ~5
*/
func (rc *RemoteClient) IsEnabled() bool {
	return len(rc.addrs) > 0
}

/*
Function: NodeCount
Description:
  Returns number of configured remote nodes.

Input:
  - none

Output:
  - int: Remote node count

Lines: ~5
*/
func (rc *RemoteClient) NodeCount() int {
	return len(rc.addrs)
}

/*
Function: DistributeTraining
Description:
  Splits models across remote school nodes, sends training commands,
  and collects fitness results. Remaining models (beyond remote capacity)
  stay for local training.

Input:
  - models []*ModelMetadata : Models to distribute

Output:
  - []TrainingResult : Results from remote schools
  - []*ModelMetadata  : Models left for local training

Lines: ~45
*/
func (rc *RemoteClient) DistributeTraining(models []*ModelMetadata) ([]TrainingResult, []*ModelMetadata) {
	infra.FnTrace(fmt.Sprintf("entering nModels=%d nRemote=%d", len(models), len(rc.addrs)))
	if !rc.IsEnabled() {
		infra.FnTrace("no remote schools configured — all local")
		return nil, models
	}

	// Calculate how many models each remote can handle
	capacity := len(rc.addrs) * rc.studentsPerNode
	remoteModels := models
	localModels := []*ModelMetadata(nil)
	if len(models) > capacity {
		remoteModels = models[:capacity]
		localModels = models[capacity:]
	}

	// Split models across remote nodes
	type batch struct {
		addr   string
		models []*ModelMetadata
	}
	batches := make([]batch, len(rc.addrs))
	for i, m := range remoteModels {
		idx := i / rc.studentsPerNode
		if idx >= len(rc.addrs) {
			idx = len(rc.addrs) - 1
		}
		batches[idx].addr = rc.addrs[idx]
		batches[idx].models = append(batches[idx].models, m)
	}

	// Send to all remotes concurrently and collect results
	var wg sync.WaitGroup
	resultsCh := make(chan []TrainingResult, len(batches))

	for _, b := range batches {
		if b.addr == "" || len(b.models) == 0 {
			continue
		}
		wg.Add(1)
		go func(b batch) {
			defer wg.Done()
			resultsCh <- rc.sendToRemote(b.addr, b.models)
		}(b)
	}
	wg.Wait()
	close(resultsCh)

	var allResults []TrainingResult
	for r := range resultsCh {
		allResults = append(allResults, r...)
	}
	infra.FnTrace(fmt.Sprintf("done remote=%d local=%d results=%d", len(allResults), len(localModels), len(allResults)))
	return allResults, localModels
}

/*
Function: sendToRemote
Description:
  Sends a training batch to a single remote school node via UDP
  and parses the results.

Input:
  - addr   string            : Remote address (ip:port)
  - models []*ModelMetadata  : Models to train

Output:
  - []TrainingResult: Fitness results (one per model)

Lines: ~50
*/
func (rc *RemoteClient) sendToRemote(addr string, models []*ModelMetadata) []TrainingResult {
	infra.FnTrace(fmt.Sprintf("sending %d models to %s", len(models), addr))
	var results []TrainingResult

	// Resolve remote address
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		for _, m := range models {
			results = append(results, TrainingResult{
				ModelName: m.Name, RemoteAddr: addr,
				Err: fmt.Sprintf("resolve failed: %v", err),
			})
		}
		infra.Error(fmt.Sprintf("Remote school %s: resolve failed: %v", addr, err))
		return results
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		for _, m := range models {
			results = append(results, TrainingResult{ModelName: m.Name, RemoteAddr: addr, Err: err.Error()})
		}
		return results
	}
	defer conn.Close()

	// Build training command: "train:model1,model2,..."
	names := make([]string, len(models))
	for i, m := range models {
		names[i] = m.Name
	}
	cmd := "train:" + strings.Join(names, ",")

	_, err = conn.Write([]byte(cmd))
	if err != nil {
		for _, m := range models {
			results = append(results, TrainingResult{ModelName: m.Name, RemoteAddr: addr, Err: err.Error()})
		}
		return results
	}

	// Read responses (one per model, or aggregated)
	conn.SetReadDeadline(time.Now().Add(rc.timeout))
	buf := make([]byte, 4096)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		for _, m := range models {
			results = append(results, TrainingResult{ModelName: m.Name, RemoteAddr: addr, Err: err.Error()})
		}
		return results
	}

	// Parse: "result:model:sharpe:sortino:profit:drawdown:accuracy:consistency:efficiency"
	response := string(buf[:n])
	results = append(results, parseTrainingResults(response, addr)...)

	return results
}

// parseTrainingResults splits a multi-result response into TrainingResult structs.
func parseTrainingResults(raw, addr string) []TrainingResult {
	var results []TrainingResult
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "result:") {
			continue
		}
		parts := strings.SplitN(line, ":", 9)
		if len(parts) < 9 {
			continue
		}
		r := TrainingResult{ModelName: parts[1], RemoteAddr: addr}
		fmt.Sscanf(parts[2], "%f", &r.Sharpe)
		fmt.Sscanf(parts[3], "%f", &r.Sortino)
		fmt.Sscanf(parts[4], "%f", &r.Profit)
		fmt.Sscanf(parts[5], "%f", &r.Drawdown)
		fmt.Sscanf(parts[6], "%f", &r.Accuracy)
		fmt.Sscanf(parts[7], "%f", &r.Consistency)
		fmt.Sscanf(parts[8], "%f", &r.Efficiency)
		results = append(results, r)
	}
	return results
}
