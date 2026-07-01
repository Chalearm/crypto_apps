/******************************************************************************
 * File Name       : risk_test.go
 * File Path       : apps/risk_lab/risk_test.go
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
 *   Updated tests for return-based financial calculations. Covers: ✅ returns calculation ✅ mean of returns ✅ variance (non-zero) ✅ covariance (non-zero) ✅ beta (logical check) Ensures: - correctness of fi
 *
 * Responsibilities:
 *   - Implement core functionality for apps package.
 *
 * Usage :
 *   Directory : apps/risk_lab/
 *
 *   Build :
 *     go build ./apps/risk_lab
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./apps/risk_lab
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
 *   [Test Functions] Test suite: TestReturns_Length, TestBlackScholes, TestMDD, TestMaxDrawdown_Flat
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
package main

import "testing"
// ✅ TEST RETURNS TABLE (important)
func TestReturns_Length(t *testing.T) {

    data := []float64{100, 110, 121}

    r := returns(data)

    if len(r) != 2 {
        t.Error("returns length incorrect")
    }
}

func TestBlackScholes(t *testing.T) {

    call := blackScholesCall(100, 100, 1, 0.05, 0.2)

    if call <= 0 {
        t.Error("invalid call price")
    }
}

func TestMDD(t *testing.T) {

    data := []float64{100, 120, 80}

    if maxDrawdown(data) < 0.3 {
        t.Error("MDD wrong")
    }
}

// ✅ constant price
func TestMaxDrawdown_Flat(t *testing.T) {

    data := []float64{100, 100, 100}

    mdd := maxDrawdown(data)

    if mdd != 0 {
        t.Error("MDD flat case failed")
    }
} 
// ✅ MDD basic test
func TestMaxDrawdown(t *testing.T) {

    data := []float64{100, 120, 80}

    // peak 120 → drop to 80 = 33%
    mdd := maxDrawdown(data)

    if mdd < 0.3 || mdd > 0.4 {
        t.Error("MDD calculation incorrect")
    }
}

func TestReturns_Value(t *testing.T) {

    data := []float64{100, 110}

    r := returns(data)

    if r[0] != 0.1 {
        t.Error("returns value incorrect")
    }
}

func TestOptionDecision(t *testing.T) {

    data := []float64{100, 120, 60} // big drop

    action := decideOptionAction("TEST", data)

    if action == "" {
        t.Error("should decide action")
    }
}

func TestDeltaGamma(t *testing.T) {

    d := deltaCall(100, 100, 1, 0.01, 0.2)
    g := gamma(100, 100, 1, 0.01, 0.2)

    if d <= 0 || g <= 0 {
        t.Error("invalid greek values")
    }
}

// ✅ TEST RETURNS FUNCTION
func TestReturns(t *testing.T) {

    data := []float64{100, 110}

    r := returns(data)

    if len(r) != 1 {
        t.Error("returns length incorrect")
    }

    if r[0] != 0.1 {
        t.Error("returns calculation incorrect")
    }
}

// ✅ TEST MEAN (ON RETURNS)
func TestMean_Returns(t *testing.T) {

    data := []float64{100, 110, 121}

    r := returns(data)

    m := mean(r)

    // returns = [0.1, 0.1]
    if m != 0.1 {
        t.Error("mean of returns incorrect")
    }
}

// ✅ TEST VARIANCE
func TestVariance_Returns(t *testing.T) {

    data := []float64{100, 110, 100}

    r := returns(data)

    v := variance(r)

    if v == 0 {
        t.Error("variance should not be zero")
    }
}

// ✅ TEST COVARIANCE
func TestCovariance(t *testing.T) {

    a := []float64{100, 110, 120}
    b := []float64{200, 210, 220}

    ra := returns(a)
    rb := returns(b)

    c := covariance(ra, rb)

    if c == 0 {
        t.Error("covariance should not be zero")
    }
}

// ✅ TEST BETA
func TestBeta(t *testing.T) {

    a := []float64{100, 110, 120}
    b := []float64{200, 210, 220}

    betaVal := beta(a, b)

    if betaVal == 0 {
        t.Error("beta calculation failed")
    }
}