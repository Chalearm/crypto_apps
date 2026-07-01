/******************************************************************************
 * File Name       : ga.go
 * File Path       : school/ga.go
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
 *   Genetic Algorithm engine for the School daemon. Evolves model populations through selection, crossover, mutation, and fitness evaluation. All parameters externalized via config.SchoolConfig (myreq2.tx
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
	"math/rand"
	"sort"
	"time"
)

// ==============================
// GA ENGINE
// ==============================

/*
Struct: GAConfig
Description:
  GA parameters loaded from config.env / config.SchoolConfig.
  All measurable values externalized — no hardcoded magic numbers.

Fields:
  - PopulationSize      int     : Total models in population
  - TopSurvivors        int     : Number selected for reproduction
  - MutationRate        float64 : Probability of mutating a gene (0-1)
  - CrossoverRate       float64 : Probability of crossover vs clone (0-1)
  - GenerationsPerCycle int     : Generations per evolution cycle
  - GraduateTopN        int     : Top N promoted to graduate status
  - RetireBottomN       int     : Bottom N retired
  - GraduationThreshold float64 : Minimum composite fitness to graduate
  - RetirementThreshold float64 : Maximum fitness before forced retirement

Lines: ~8
*/
type GAConfig struct {
	PopulationSize      int
	TopSurvivors        int
	MutationRate        float64
	CrossoverRate       float64
	GenerationsPerCycle int
	GraduateTopN        int
	RetireBottomN       int
	GraduationThreshold float64
	RetirementThreshold float64
}

/*
Struct: GAEngine
Description:
  Runs genetic evolution over a ModelPopulation. Uses tournament selection,
  weighted crossover, and Gaussian mutation.

Fields:
  - cfg  GAConfig           : Evolution parameters
  - pop  *ModelPopulation   : Model population to evolve
  - rng  *rand.Rand         : Deterministic random source

Lines: ~5
*/
type GAEngine struct {
	cfg GAConfig
	pop *ModelPopulation
	rng *rand.Rand
}

/*
Function: NewGA
Description:
  Creates a new GA engine for a model population.

Input:
  - cfg GAConfig          : Evolution parameters
  - pop *ModelPopulation  : Seed population (must be non-empty)
  - rng *rand.Rand        : Random source

Output:
  - *GAEngine: Initialized engine

Lines: ~10
*/
func NewGA(cfg GAConfig, pop *ModelPopulation, rng *rand.Rand) *GAEngine {
	return &GAEngine{cfg: cfg, pop: pop, rng: rng}
}

/*
Function: Evolve
Description:
  Runs one full evolution cycle: ranks models, retires bottom N,
  promotes top N to graduate, then runs N generations of selection,
  crossover, and mutation to replenish the population.

Input:
  - weights FitnessWeights : Scoring weights

Output:
  - int    : Number of new graduates this cycle
  - int    : Number retired this cycle
  - string : Summary log line

Lines: ~35
*/
func (ga *GAEngine) Evolve(weights FitnessWeights) (int, int, string) {
	// 1. Evaluate fitness for all models
	ranked := ga.rankByFitness(weights)
	if len(ranked) == 0 {
		return 0, 0, "empty population"
	}

	// 2. Retire bottom N (below threshold)
	retired := 0
	for i := len(ranked) - 1; i >= 0 && retired < ga.cfg.RetireBottomN; i-- {
		m := ranked[i]
		if m.Fitness == nil {
			continue
		}
		score, _ := CompositeFitness(m.Fitness, weights)
		if score < ga.cfg.RetirementThreshold {
			ga.pop.Retire(m.Name)
			retired++
		}
	}

	// 3. Graduate top N (above threshold)
	graduated := 0
	for i := 0; i < len(ranked) && graduated < ga.cfg.GraduateTopN; i++ {
		m := ranked[i]
		if m.Status != StatusActive && m.Status != StatusTraining {
			continue
		}
		if m.Fitness == nil {
			continue
		}
		score, _ := CompositeFitness(m.Fitness, weights)
		if score >= ga.cfg.GraduationThreshold {
			ga.pop.Graduate(m.Name)
			graduated++
		}
	}

	// 4. Run generations of evolution
	for gen := 0; gen < ga.cfg.GenerationsPerCycle; gen++ {
		ga.runGeneration(weights)
	}

	summary := fmt.Sprintf("gen=%d graduated=%d retired=%d pop=%d",
		ga.cfg.GenerationsPerCycle, graduated, retired, ga.pop.ActiveCount())
	return graduated, retired, summary
}

// rankByFitness returns models sorted by composite fitness (descending).
func (ga *GAEngine) rankByFitness(weights FitnessWeights) []*ModelMetadata {
	var active []*ModelMetadata
	// Collect all non-retired models across all categories
	categories := []string{
		CategoryOptions, CategoryRisk, CategoryIntraday, CategorySwing,
		CategoryLongTerm, CategoryVolatility, CategoryLiquidity, CategoryPortfolio,
	}
	seen := make(map[string]bool)
	for _, cat := range categories {
		for _, m := range ga.pop.ListByCategory(cat) {
			if m.Status != StatusRetired && !seen[m.Name] {
				seen[m.Name] = true
				active = append(active, m)
			}
		}
	}

	sort.Slice(active, func(i, j int) bool {
		si, _ := CompositeFitness(active[i].Fitness, weights)
		sj, _ := CompositeFitness(active[j].Fitness, weights)
		return si > sj
	})
	return active
}

// runGeneration performs one generation of selection, crossover, mutation.
func (ga *GAEngine) runGeneration(weights FitnessWeights) {
	survivors := ga.tournamentSelect(weights)
	if len(survivors) < 2 {
		return
	}

	// Create offspring to maintain population size
	needed := ga.cfg.PopulationSize - ga.pop.ActiveCount()
	if needed <= 0 {
		return
	}

	for i := 0; i < needed; i++ {
		p1 := survivors[ga.rng.Intn(len(survivors))]
		p2 := survivors[ga.rng.Intn(len(survivors))]
		child := ga.crossover(p1, p2)
		ga.mutate(child)
		ga.pop.AddModel(child)
	}
}

// tournamentSelect picks top N by fitness.
func (ga *GAEngine) tournamentSelect(weights FitnessWeights) []*ModelMetadata {
	ranked := ga.rankByFitness(weights)
	n := ga.cfg.TopSurvivors
	if n > len(ranked) {
		n = len(ranked)
	}
	return ranked[:n]
}

// crossover blends two parent models into an offspring.
func (ga *GAEngine) crossover(a, b *ModelMetadata) *ModelMetadata {
	gen := a.Generation
	if b.Generation > gen {
		gen = b.Generation
	}

	child := &ModelMetadata{
		Name:       fmt.Sprintf("gen%d_%d", gen+1, ga.rng.Intn(9999)),
		Version:    fmt.Sprintf("v%d.%d", gen+1, ga.rng.Intn(99)),
		Generation: gen + 1,
		Category:   a.Category,
		Status:     StatusTraining,
	}

	// Crossover hyperparameters and ensemble weights
	if ga.rng.Float64() < ga.cfg.CrossoverRate && b != nil {
		child.Hyperparameters = blendMap(a.Hyperparameters, b.Hyperparameters, ga.rng)
		child.EnsembleComposition = blendWeights(a.EnsembleComposition, b.EnsembleComposition, ga.rng)
		child.Architecture = pickOne(a.Architecture, b.Architecture, ga.rng)
		child.FeatureSet = pickSlice(a.FeatureSet, b.FeatureSet, ga.rng)
	} else {
		child.Hyperparameters = copyMap(a.Hyperparameters)
		child.EnsembleComposition = copyWeights(a.EnsembleComposition)
		child.Architecture = a.Architecture
		child.FeatureSet = append([]string{}, a.FeatureSet...)
	}

	child.CreatedAt = time.Now()
	return child
}

// mutate applies random changes to an offspring.
func (ga *GAEngine) mutate(m *ModelMetadata) {
	if m.Hyperparameters == nil {
		m.Hyperparameters = make(map[string]string)
	}
	for k, v := range m.Hyperparameters {
		if ga.rng.Float64() < ga.cfg.MutationRate {
			// Gaussian perturbation on numeric values
			if f, err := parseFloat(v); err == nil {
				f += ga.rng.NormFloat64() * 0.1
				m.Hyperparameters[k] = fmt.Sprintf("%.4f", f)
			}
		}
	}
	if ga.rng.Float64() < ga.cfg.MutationRate && len(m.EnsembleComposition) > 0 {
		// Randomly shift one weight up and normalize
		keys := make([]string, 0, len(m.EnsembleComposition))
		for k := range m.EnsembleComposition {
			keys = append(keys, k)
		}
		k := keys[ga.rng.Intn(len(keys))]
		m.EnsembleComposition[k] += ga.rng.NormFloat64() * 0.05
		if m.EnsembleComposition[k] < 0 {
			m.EnsembleComposition[k] = 0
		}
		normalizeWeights(m.EnsembleComposition)
	}
}

// ── helpers ──

func blendMap(a, b map[string]string, rng *rand.Rand) map[string]string {
	out := make(map[string]string)
	for k, v := range a {
		if rng.Float64() < 0.5 {
			out[k] = v
		}
	}
	for k, v := range b {
		if _, ok := out[k]; !ok {
			out[k] = v
		}
	}
	return out
}

func blendWeights(a, b map[string]float64, rng *rand.Rand) map[string]float64 {
	out := make(map[string]float64)
	seen := make(map[string]bool)
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}
	for k := range seen {
		wa := a[k]
		wb := b[k]
		out[k] = wa*rng.Float64() + wb*(1-rng.Float64())
	}
	normalizeWeights(out)
	return out
}

func normalizeWeights(w map[string]float64) {
	sum := 0.0
	for _, v := range w {
		sum += v
	}
	if sum > 0 {
		for k := range w {
			w[k] /= sum
		}
	}
}

func copyMap(src map[string]string) map[string]string {
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func copyWeights(src map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func pickOne(a, b string, rng *rand.Rand) string {
	if rng.Float64() < 0.5 {
		return a
	}
	return b
}

func pickSlice(a, b []string, rng *rand.Rand) []string {
	if rng.Float64() < 0.5 {
		return append([]string{}, a...)
	}
	return append([]string{}, b...)
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
