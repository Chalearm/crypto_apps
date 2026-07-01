/******************************************************************************
 * File Name       : agent.go
 * File Path       : trading/agent.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 2.0.0
 * Created Date    : 2026-06-27 14:30 (UTC+7)
 * Modified Date   : 2026-06-29 17:10 (UTC+7)
 *
 * Description     :
 *   Portfolio agent types for the Trading daemon v2.0. Expanded to support:
 *   - Configurable portfolio counts (tens to thousands) (§50)
 *   - Multiple time horizons (§51)
 *   - Holdings, risk profiles, KPI histories, lifecycle states (§52)
 *   - Agent lifecycle: replicate, evolve, merge, retire, promote (§59)
 *
 * Change History :
 *   v1.0.0 | 2026-06-27 | deepseek-4.0-pro | Initial version
 *   v2.0.0 | 2026-06-29 | deepseek-4.0-pro | Added horizons, holdings,
 *            risk profiles, KPI history, lifecycle states, scale support
 ******************************************************************************/

package trading

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// ==============================
// CATEGORIES & HORIZONS & STATES
// ==============================

const (
	CapitalSmall  = "small"
	CapitalMedium = "medium"
	CapitalLarge  = "large"
)

const (
	StrategyHedging     = "hedging"
	StrategyTrend       = "trend"
	StrategyOptions     = "options"
	StrategyVolatility  = "volatility"
	StrategyLiquidity   = "liquidity"
	StrategyArbitrage   = "arbitrage"
	StrategyDiversified = "diversified"
)

// TradingHorizon per myreq3.txt §51.
const (
	Horizon15Min     = "15min"
	HorizonHourly    = "hourly"
	HorizonIntraday  = "intraday"
	HorizonMultiDay  = "multi-day"
	HorizonSwing     = "swing"
	HorizonOptions   = "options"
	HorizonVolatility = "volatility"
	HorizonLongTerm  = "long-term"
)

// AgentLifecycle per myreq3.txt §52, §59.
const (
	LifecycleCreated  = "created"
	LifecycleActive   = "active"
	LifecycleRetiring = "retiring"
	LifecycleRetired  = "retired"
	LifecycleArchived = "archived"
	LifecyclePromoted = "promoted"
)

// AllHorizons returns all trading horizon constants.
func AllHorizons() []string {
	return []string{Horizon15Min, HorizonHourly, HorizonIntraday, HorizonMultiDay,
		HorizonSwing, HorizonOptions, HorizonVolatility, HorizonLongTerm}
}

// ==============================
// AGENT DNA
// ==============================

type AgentDNA struct {
	ModelAssignments    []string
	RiskPreference      float64
	HedgingPolicy       string
	PositionSizingRule  string
	RebalancingSchedule string
	StopLossRule        string
	LeverageConstraint  float64
	AssetSelection      []string
	Horizon             string // §51: trading horizon
}

// ==============================
// HOLDINGS
// ==============================

// Holding represents a single asset position.
type Holding struct {
	Asset    string  `json:"asset"`
	Quantity float64 `json:"quantity"`
	AvgPrice float64 `json:"avg_price"`
}

// ==============================
// RISK PROFILE
// ==============================

// RiskProfile per myreq3.txt §52.
type RiskProfile struct {
	VaR95     float64 `json:"var_95"`     // 95% Value-at-Risk
	CVaR95    float64 `json:"cvar_95"`    // 95% Conditional VaR
	Beta      float64 `json:"beta"`       // Market beta
	Volatility float64 `json:"volatility"` // Annualized volatility
	MaxDrawdown float64 `json:"max_drawdown"`
}

// ==============================
// KPI HISTORY
// ==============================

// KPIEntry is a single historical performance snapshot.
type KPIEntry struct {
	Timestamp  time.Time
	PnL        float64
	Sharpe     float64
	Sortino    float64
	Drawdown   float64
	WinRate    float64
	NumTrades  int
}

// ==============================
// AGENT PERFORMANCE
// ==============================

type AgentPerformance struct {
	PnL         float64
	Sharpe      float64
	Drawdown    float64
	WinRate     float64
	ExecQuality float64
	Trades      int
}

// ==============================
// PORTFOLIO AGENT v2.0
// ==============================

type PortfolioAgent struct {
	ID          string
	Name        string
	Category    string
	Strategy    string
	DNA         *AgentDNA
	Capital     float64
	Performance *AgentPerformance
	CreatedAt   time.Time
	Active      bool

	// §51-52 new fields
	Horizon     string                   // Trading time horizon
	Holdings    map[string]*Holding      // Asset → position
	Risk        *RiskProfile             // Current risk profile
	KPIHistory  []KPIEntry               // Full historical KPI timeline
	Lifecycle   string                   // Lifecycle status (created/active/retiring/retired/archived/promoted)
	HedgingRules map[string]float64      // Asset → hedge ratio
	ParentID    string                   // If this agent was replicated/evolved from another
	Generation  int                      // Replication generation counter
}

// ==============================
// AGENT POOL v2.0
// ==============================

type AgentPool struct {
	mu      sync.RWMutex
	agents  map[string]*PortfolioAgent
	counter int
}

// NewAgentPool creates a new empty agent pool.
func NewAgentPool() *AgentPool {
	return &AgentPool{
		agents: make(map[string]*PortfolioAgent),
	}
}

/******************************************************************************
 * Function Name : CreateAgent
 *
 * Purpose :
 *   Creates a new portfolio agent with full Horizon, Lifecycle, Holdings,
 *   and Risk fields. Supports configurable time horizons per §51.
 *
 * Inputs :
 *   category  string — CapitalSmall/Medium/Large
 *   strategy  string — Strategy type constant
 *   horizon   string — Trading horizon (Horizon15Min, etc.)
 *
 * Return :
 *   Type        : *PortfolioAgent
 *   Description : Initialized agent with Active lifecycle.
 *
 * Number Of Lines : 25
 ******************************************************************************/
func (p *AgentPool) CreateAgent(category, strategy, horizon string) *PortfolioAgent {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.counter++
	id := fmt.Sprintf("agent_%d", p.counter)

	agent := &PortfolioAgent{
		ID:        id,
		Name:      fmt.Sprintf("%s-%s-%s-%d", strategy, category, horizon, p.counter),
		Category:  category,
		Strategy:  strategy,
		Horizon:   horizon,
		Lifecycle: LifecycleActive,
		DNA: &AgentDNA{
			RiskPreference:     0.5,
			LeverageConstraint: 1.0,
			Horizon:            horizon,
		},
		Holdings:     make(map[string]*Holding),
		Risk:         &RiskProfile{},
		KPIHistory:   make([]KPIEntry, 0),
		HedgingRules: make(map[string]float64),
		Generation:   0,
		Performance:  &AgentPerformance{},
		CreatedAt:    time.Now(),
		Active:       true,
	}
	p.agents[id] = agent
	return agent
}

// Legacy CreateAgent for backward compatibility (creates with Swing horizon).
func (p *AgentPool) CreateAgentSimple(category, strategy string) *PortfolioAgent {
	return p.CreateAgent(category, strategy, HorizonSwing)
}

/******************************************************************************
 * Function Name : ReplicateAgent
 *
 * Purpose :
 *   Clones a successful agent with a small DNA mutation (§59).
 *
 * Inputs :
 *   parentID  string — Source agent ID
 *
 * Return :
 *   Type        : *PortfolioAgent
 *   Description : New agent with slightly mutated DNA.
 *
 * Error Cases :
 *   - Parent not found : returns nil
 *
 * Number Of Lines : 25
 ******************************************************************************/
func (p *AgentPool) ReplicateAgent(parentID string) *PortfolioAgent {
	p.mu.Lock()
	defer p.mu.Unlock()

	parent, ok := p.agents[parentID]
	if !ok {
		return nil
	}

	p.counter++
	id := fmt.Sprintf("agent_%d", p.counter)

	// Clone DNA with small mutation
	dna := *parent.DNA
	dna.RiskPreference += (rand.Float64() - 0.5) * 0.1
	if dna.RiskPreference < 0 {
		dna.RiskPreference = 0
	}
	if dna.RiskPreference > 1.0 {
		dna.RiskPreference = 1.0
	}
	dna.LeverageConstraint += (rand.Float64() - 0.5) * 0.2
	if dna.LeverageConstraint < 0.5 {
		dna.LeverageConstraint = 0.5
	}
	if dna.LeverageConstraint > 5.0 {
		dna.LeverageConstraint = 5.0
	}

	child := &PortfolioAgent{
		ID:           id,
		Name:         fmt.Sprintf("%s_r%d", parent.Name, parent.Generation+1),
		Category:     parent.Category,
		Strategy:     parent.Strategy,
		Horizon:      parent.Horizon,
		Lifecycle:    LifecycleActive,
		DNA:          &dna,
		Holdings:     make(map[string]*Holding),
		Risk:         &RiskProfile{},
		KPIHistory:   make([]KPIEntry, 0),
		HedgingRules: make(map[string]float64),
		ParentID:     parentID,
		Generation:   parent.Generation + 1,
		Performance:  &AgentPerformance{},
		CreatedAt:    time.Now(),
		Active:       true,
	}
	p.agents[id] = child
	return child
}

/******************************************************************************
 * Function Name : EvolveAgent
 *
 * Purpose :
 *   Adjusts an agent's DNA based on its KPI history (§59).
 *   Higher Sharpe → increase risk preference. High drawdown → reduce leverage.
 *
 * Inputs :
 *   id  string — Agent ID to evolve
 *
 * Return :
 *   Type        : bool
 *   Description : true if agent was found and evolved.
 *
 * Number Of Lines : 20
 ******************************************************************************/
func (p *AgentPool) EvolveAgent(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	a, ok := p.agents[id]
	if !ok {
		return false
	}

	// Compute average recent Shar pe (last 10 entries)
	avgSharpe := 0.0
	count := 0
	start := len(a.KPIHistory) - 10
	if start < 0 {
		start = 0
	}
	for i := start; i < len(a.KPIHistory); i++ {
		avgSharpe += a.KPIHistory[i].Sharpe
		count++
	}
	if count > 0 {
		avgSharpe /= float64(count)
	}

	// Adjust risk preference
	if avgSharpe > 1.0 {
		a.DNA.RiskPreference += 0.02
	} else if avgSharpe < 0 {
		a.DNA.RiskPreference -= 0.02
	}
	if a.DNA.RiskPreference < 0 {
		a.DNA.RiskPreference = 0
	}
	if a.DNA.RiskPreference > 1.0 {
		a.DNA.RiskPreference = 1.0
	}

	// Reduce leverage if drawdown is high
	if a.Performance.Drawdown > 0.2 {
		a.DNA.LeverageConstraint *= 0.95
	}

	return true
}

/******************************************************************************
 * Function Name : MergeAgents
 *
 * Purpose :
 *   Merges two agents by averaging their DNA and capital (§59).
 *   Both parents are retired. A new child agent is created.
 *
 * Inputs :
 *   idA, idB  string — Agent IDs to merge
 *
 * Return :
 *   Type        : *PortfolioAgent
 *   Description : Merged child agent.
 *
 * Number Of Lines : 30
 ******************************************************************************/
func (p *AgentPool) MergeAgents(idA, idB string) *PortfolioAgent {
	p.mu.Lock()
	defer p.mu.Unlock()

	a, okA := p.agents[idA]
	b, okB := p.agents[idB]
	if !okA || !okB {
		return nil
	}

	p.counter++
	id := fmt.Sprintf("agent_%d", p.counter)

	// Average DNA
	mergedDNA := AgentDNA{
		RiskPreference:     (a.DNA.RiskPreference + b.DNA.RiskPreference) / 2.0,
		LeverageConstraint: (a.DNA.LeverageConstraint + b.DNA.LeverageConstraint) / 2.0,
		Horizon:            a.Horizon, // keep parent A's horizon
	}

	mergedCapital := (a.Capital + b.Capital) / 2.0

	child := &PortfolioAgent{
		ID:           id,
		Name:         fmt.Sprintf("%s_%s_merged", a.Name, b.Name),
		Category:     a.Category,
		Strategy:     a.Strategy,
		Horizon:      a.Horizon,
		Lifecycle:    LifecycleActive,
		DNA:          &mergedDNA,
		Capital:      mergedCapital,
		Holdings:     make(map[string]*Holding),
		Risk:         &RiskProfile{},
		KPIHistory:   make([]KPIEntry, 0),
		HedgingRules: make(map[string]float64),
		ParentID:     idA,
		Generation:   maxGen(a.Generation, b.Generation) + 1,
		Performance:  &AgentPerformance{},
		CreatedAt:    time.Now(),
		Active:       true,
	}

	// Retire parents
	a.Lifecycle = LifecycleRetired
	a.Active = false
	b.Lifecycle = LifecycleRetired
	b.Active = false

	p.agents[id] = child
	return child
}

/******************************************************************************
 * Function Name : PromoteAgent
 *
 * Purpose :
 *   Promotes an agent (increase capital allocation tier) (§59).
 *   Small → Medium → Large.
 *
 * Inputs :
 *   id  string — Agent ID
 *
 * Return :
 *   Type        : string
 *   Description : New capital tier.
 *
 * Number Of Lines : 15
 ******************************************************************************/
func (p *AgentPool) PromoteAgent(id string) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	a, ok := p.agents[id]
	if !ok {
		return ""
	}
	switch a.Category {
	case CapitalSmall:
		a.Category = CapitalMedium
	case CapitalMedium:
		a.Category = CapitalLarge
	}
	a.Lifecycle = LifecyclePromoted
	return a.Category
}

/******************************************************************************
 * Function Name : RecordKPI
 *
 * Purpose :
 *   Appends a KPI snapshot to the agent's history (§52).
 *
 * Inputs :
 *   id  string  — Agent ID
 *   kpi KPIEntry — Performance snapshot
 *
 * Number Of Lines : 10
 ******************************************************************************/
func (p *AgentPool) RecordKPI(id string, kpi KPIEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if a, ok := p.agents[id]; ok {
		kpi.Timestamp = time.Now()
		a.KPIHistory = append(a.KPIHistory, kpi)
		// Cap at 1000 entries for memory
		if len(a.KPIHistory) > 1000 {
			a.KPIHistory = a.KPIHistory[len(a.KPIHistory)-1000:]
		}
	}
}

/******************************************************************************
 * Function Name : UpdateHoldings
 *
 * Purpose :
 *   Updates an agent's asset holdings (§52).
 *
 * Inputs :
 *   id       string             — Agent ID
 *   holdings map[string]*Holding — New holdings map
 *
 * Number Of Lines : 8
 ******************************************************************************/
func (p *AgentPool) UpdateHoldings(id string, holdings map[string]*Holding) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if a, ok := p.agents[id]; ok {
		a.Holdings = holdings
	}
}

/******************************************************************************
 * Function Name : UpdateRisk
 *
 * Purpose :
 *   Updates an agent's risk profile (§52).
 *
 * Number Of Lines : 8
 ******************************************************************************/
func (p *AgentPool) UpdateRisk(id string, risk *RiskProfile) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if a, ok := p.agents[id]; ok {
		a.Risk = risk
	}
}

// RetireAgent marks an agent as retired.
func (p *AgentPool) RetireAgent(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	a, ok := p.agents[id]
	if !ok {
		return false
	}
	a.Active = false
	a.Lifecycle = LifecycleRetired
	return true
}

// GetAgent returns an agent by ID.
func (p *AgentPool) GetAgent(id string) *PortfolioAgent {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.agents[id]
}

// ActiveAgents returns all active agents.
func (p *AgentPool) ActiveAgents() []*PortfolioAgent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []*PortfolioAgent
	for _, a := range p.agents {
		if a.Active {
			result = append(result, a)
		}
	}
	// Sort by ID for deterministic ordering
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// Count returns total agent count.
func (p *AgentPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.agents)
}

// ActiveCount returns active agent count.
func (p *AgentPool) ActiveCount() int {
	return len(p.ActiveAgents())
}

/******************************************************************************
 * Function Name : ListByHorizon
 *
 * Purpose :
 *   Returns all active agents for a given trading horizon (§51).
 *
 * Inputs :
 *   horizon  string — Horizon15Min, HorizonHourly, etc.
 *
 * Return :
 *   Type        : []*PortfolioAgent
 *   Description : Agents matching the horizon.
 *
 * Number Of Lines : 14
 ******************************************************************************/
func (p *AgentPool) ListByHorizon(horizon string) []*PortfolioAgent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []*PortfolioAgent
	for _, a := range p.agents {
		if a.Active && a.Horizon == horizon {
			result = append(result, a)
		}
	}
	return result
}

// ListByLifecycle returns agents in the given lifecycle stage.
func (p *AgentPool) ListByLifecycle(stage string) []*PortfolioAgent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []*PortfolioAgent
	for _, a := range p.agents {
		if a.Lifecycle == stage {
			result = append(result, a)
		}
	}
	return result
}

// RebalanceCapital distributes total capital equally among active agents.
func (p *AgentPool) RebalanceCapital(total float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	active := 0
	for _, a := range p.agents {
		if a.Active {
			active++
		}
	}
	if active == 0 {
		return
	}

	share := total / float64(active)
	for _, a := range p.agents {
		if a.Active {
			a.Capital = share
		}
	}
}

// TotalAllocated returns sum of capital across active agents.
func (p *AgentPool) TotalAllocated() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var total float64
	for _, a := range p.agents {
		if a.Active {
			total += a.Capital
		}
	}
	return total
}

// TopBySharpe returns top N agents by Sharpe ratio.
func (p *AgentPool) TopBySharpe(n int) []*PortfolioAgent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	type pair struct {
		a *PortfolioAgent
		s float64
	}
	var pairs []pair
	for _, a := range p.agents {
		if a.Active {
			pairs = append(pairs, pair{a, a.Performance.Sharpe})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].s > pairs[j].s })

	result := make([]*PortfolioAgent, 0, n)
	for i := 0; i < n && i < len(pairs); i++ {
		result = append(result, pairs[i].a)
	}
	return result
}

// WorstBySharpe returns bottom N agents by Sharpe.
func (p *AgentPool) WorstBySharpe(n int) []*PortfolioAgent {
	p.mu.RLock()
	defer p.mu.RUnlock()

	type pair struct {
		a *PortfolioAgent
		s float64
	}
	var pairs []pair
	for _, a := range p.agents {
		if a.Active {
			pairs = append(pairs, pair{a, a.Performance.Sharpe})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].s < pairs[j].s })

	result := make([]*PortfolioAgent, 0, n)
	for i := 0; i < n && i < len(pairs); i++ {
		result = append(result, pairs[i].a)
	}
	return result
}

// ==============================
// HELPERS
// ==============================

func maxGen(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ComputeRiskProfile calculates VaR, CVaR, Beta from a returns series.
func ComputeRiskProfile(returns []float64) *RiskProfile {
	if len(returns) == 0 {
		return &RiskProfile{}
	}
	n := len(returns)

	// Mean and std
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(n)

	variance := 0.0
	for _, r := range returns {
		d := r - mean
		variance += d * d
	}
	variance /= float64(n)
	std := math.Sqrt(variance)

	// VaR 95%: Gaussian assumption
	// z-score for 95% confidence = 1.645
	z95 := 1.645
	var95 := mean - z95*std

	// CVaR: average of returns below VaR
	cvarSum := 0.0
	cvarCount := 0
	for _, r := range returns {
		if r < var95 {
			cvarSum += r
			cvarCount++
		}
	}
	cvar95 := 0.0
	if cvarCount > 0 {
		cvar95 = cvarSum / float64(cvarCount)
	}

	// Max drawdown
	cum := 0.0
	peak := 0.0
	maxDD := 0.0
	for _, r := range returns {
		cum += r
		if cum > peak {
			peak = cum
		}
		dd := peak - cum
		if dd > maxDD {
			maxDD = dd
		}
	}

	return &RiskProfile{
		VaR95:       var95,
		CVaR95:      cvar95,
		Beta:        1.0, // placeholder — need market benchmark
		Volatility:  std * math.Sqrt(252),
		MaxDrawdown: maxDD,
	}
}
