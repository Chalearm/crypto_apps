/******************************************************************************
 * File Name       : school_tiers_test.go
 * File Path       : school/school_tiers_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-06-30 05:10:00 (UTC+7)
 * Modified Date   : 2026-06-30 05:10:00 (UTC+7)
 *
 * Description     :
 *   Tests for school tier management (§90). 12 positive + 4 negative.
 *
 * Change History :
 *   1.0.0 | 2026-06-30 05:10 | deepseek-4.0-pro | Initial version
 ******************************************************************************/

package school

import (
	"testing"
)

// P1: All 4 tiers created by manager
func TestSchoolTierManagerCreatesAllTiers(t *testing.T) {
	sm := NewSchoolTierManager()
	if len(sm.Tiers) != 4 {
		t.Errorf("expected 4 tiers, got %d", len(sm.Tiers))
	}
	for _, tier := range AllTiers() {
		if sm.Get(tier) == nil {
			t.Errorf("tier %s should exist", tier)
		}
	}
}

// P2: Tier names and colors
func TestTierNamesAndColors(t *testing.T) {
	if TierName(TierPrimary) != "Primary School" {
		t.Error("wrong primary name")
	}
	if TierName(TierMiddle) != "Middle School" {
		t.Error("wrong middle name")
	}
	if TierColor(TierGraduate) != "#34d399" {
		t.Error("wrong graduate color")
	}
}

// P3: Add model to tier
func TestTierAddModel(t *testing.T) {
	st := NewSchoolTier(TierPrimary)
	st.AddModel(&TierModel{ID: "m1", Name: "test_model", Status: "training"})
	if st.Count() != 1 {
		t.Errorf("expected 1, got %d", st.Count())
	}
}

// P4: Respect MaxModels cap
func TestTierMaxModelsCap(t *testing.T) {
	st := NewSchoolTier(TierMiddle)
	st.Config.MaxModels = 3
	for i := 0; i < 10; i++ {
		st.AddModel(&TierModel{ID: "m" + string(rune('a'+i)), Name: "x", Status: "training"})
	}
	if st.Count() != 3 {
		t.Errorf("expected cap at 3, got %d", st.Count())
	}
}

// P5: Remove model
func TestTierRemoveModel(t *testing.T) {
	st := NewSchoolTier(TierPrimary)
	st.AddModel(&TierModel{ID: "to_remove", Name: "x", Status: "training"})
	st.AddModel(&TierModel{ID: "keep", Name: "y", Status: "ready"})
	if !st.RemoveModel("to_remove") {
		t.Error("expected remove to succeed")
	}
	if st.Count() != 1 {
		t.Errorf("expected 1 after remove, got %d", st.Count())
	}
}

// P6: CountByStatus
func TestTierCountByStatus(t *testing.T) {
	st := NewSchoolTier(TierPrimary)
	st.AddModel(&TierModel{ID: "a", Status: "training"})
	st.AddModel(&TierModel{ID: "b", Status: "ready"})
	st.AddModel(&TierModel{ID: "c", Status: "training"})
	if st.CountByStatus("training") != 2 {
		t.Errorf("expected 2 training, got %d", st.CountByStatus("training"))
	}
}

// P7: GetModels returns thread-safe copy
func TestTierGetModels(t *testing.T) {
	st := NewSchoolTier(TierPrimary)
	st.AddModel(&TierModel{ID: "a"})
	models := st.GetModels()
	if len(models) != 1 {
		t.Error("expected 1")
	}
	models[0] = nil // mutate copy — shouldn't affect origin
	if st.Count() != 1 {
		t.Error("copy mutation affected original")
	}
}

// P8: Manager Summary returns all tiers
func TestManagerSummary(t *testing.T) {
	sm := NewSchoolTierManager()
	sm.Get(TierPrimary).AddModel(&TierModel{ID: "p1", Status: "training"})
	sm.Get(TierPrimary).AddModel(&TierModel{ID: "p2", Status: "ready"})
	sm.Get(TierGraduate).AddModel(&TierModel{ID: "g1", Status: "ready"})

	sum := sm.Summary()
	prim := sum[TierPrimary].(map[string]interface{})
	if prim["total"].(int) != 2 {
		t.Errorf("expected 2 total in primary, got %v", prim["total"])
	}
	grad := sum[TierGraduate].(map[string]interface{})
	if grad["total"].(int) != 1 {
		t.Errorf("expected 1 in graduate, got %v", grad["total"])
	}
}

// P9: SeedFromPopulation distributes models across tiers
func TestSeedFromPopulation(t *testing.T) {
	pop := NewModelPopulation()
	// Add models with different ensemble sizes
	pop.AddModel(&ModelMetadata{Name: "single", Category: CategorySwing, EnsembleComposition: map[string]float64{"A": 1.0}, Architecture: "LSTM", Status: StatusTraining})
	pop.AddModel(&ModelMetadata{Name: "trio", Category: CategoryIntraday, EnsembleComposition: map[string]float64{"A": 0.5, "B": 0.3, "C": 0.2}, Architecture: "Ensemble", Status: StatusTraining})
	pop.AddModel(&ModelMetadata{Name: "penta", Category: CategoryPortfolio, EnsembleComposition: map[string]float64{"A": 0.3, "B": 0.2, "C": 0.2, "D": 0.15, "E": 0.15}, Architecture: "Advanced", Status: StatusTraining})
	pop.AddModel(&ModelMetadata{Name: "grad", Category: CategoryOptions, EnsembleComposition: map[string]float64{"X": 1.0}, Architecture: "Final", Status: StatusGraduate})

	sm := NewSchoolTierManager()
	sm.SeedFromPopulation(pop)

	if sm.Get(TierPrimary).Count() < 1 {
		t.Error("primary should have models")
	}
	if sm.Get(TierMiddle).Count() < 1 {
		t.Error("middle should have models")
	}
	if sm.Get(TierHigh).Count() < 1 {
		t.Error("high should have models")
	}
	if sm.Get(TierGraduate).Count() < 1 {
		t.Error("graduate should have models")
	}
}

// P10: AllModels flattens across tiers
func TestAllModelsFlatten(t *testing.T) {
	sm := NewSchoolTierManager()
	sm.Get(TierPrimary).AddModel(&TierModel{ID: "a"})
	sm.Get(TierHigh).AddModel(&TierModel{ID: "b"})
	sm.Get(TierGraduate).AddModel(&TierModel{ID: "c"})
	all := sm.AllModels()
	if len(all) != 3 {
		t.Errorf("expected 3 flat models, got %d", len(all))
	}
}

// P11: Default config values
func TestTierDefaultConfigs(t *testing.T) {
	if NewTierConfig(TierPrimary).MaxModels != 50 {
		t.Error("primary max should be 50")
	}
	if NewTierConfig(TierMiddle).EnsembleSize != 3 {
		t.Error("middle ensemble should be 3")
	}
	if NewTierConfig(TierHigh).EnsembleSize != 5 {
		t.Error("high ensemble should be 5")
	}
	if NewTierConfig(TierGraduate).MaxModels != 0 {
		t.Error("graduate max should be 0 (unlimited)")
	}
}

// P12: Empty tier summary works
func TestEmptyTierSummary(t *testing.T) {
	sm := NewSchoolTierManager()
	sum := sm.Summary()
	if len(sum) != 4 {
		t.Errorf("expected 4 tiers in summary, got %d", len(sum))
	}
}

// N1: Remove nonexistent model returns false
func TestRemoveNonexistentModel(t *testing.T) {
	st := NewSchoolTier(TierPrimary)
	if st.RemoveModel("ghost") {
		t.Error("remove nonexistent should return false")
	}
}

// N2: Get nonexistent tier returns nil gracefully
func TestGetNonexistentTier(t *testing.T) {
	sm := NewSchoolTierManager()
	st := sm.Get("bogus_tier")
	if st != nil {
		t.Error("expected nil for bogus tier")
	}
}

// N3: Remove model from empty tier returns false
func TestRemoveFromEmptyTier(t *testing.T) {
	st := NewSchoolTier(TierHigh)
	if st.RemoveModel("anything") {
		t.Error("remove from empty should return false")
	}
}

// N4: SeedFromPopulation with empty population
func TestSeedFromEmptyPopulation(t *testing.T) {
	pop := NewModelPopulation()
	sm := NewSchoolTierManager()
	sm.SeedFromPopulation(pop) // should not panic
	if sm.Get(TierPrimary).Count() != 0 {
		t.Error("expected 0 after seeding from empty")
	}
}
