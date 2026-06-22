/*
Filename: apps/risk_lab/report.go

Author: M365 Copilot (GPT-5)
Version: v7.0 (Clean + Stable Dashboard)
Owner: Chalearm Saelim
Date: 2026-06-21 06:02 ICT

Description:
Complete risk dashboard generator.

Includes:
✅ Price chart
✅ Price table
✅ Returns table
✅ Statistics (Mean / Std / Var / MDD)
✅ Covariance heatmap
✅ Risk summary
✅ Portfolio (MPT baseline)
✅ Pie chart allocation

Run:
    go run .
    open report.html
*/

package main

import (
    "encoding/json"
    "fmt"
    "os"
    "sort"
)

//////////////////////////
// ✅ MAIN ENTRY
//////////////////////////
//////////////////////////
// ✅ MAIN HTML GENERATOR
//////////////////////////

func generateHTML(data map[string][]float64) {

    names := sortedNames(data)
    length := len(data[names[0]])

    // ✅ JSON for JS
    jsonData, _ := json.Marshal(data)

    // ✅ options
    options := generateOptions(data)

    // ✅ covariance
    matrix := covarianceMatrix(data)

    // ✅ PHASE 1 portfolio (before options)
    w1 := optimizeSharpe(data, names)
    ret1 := portfolioReturn(w1, expectedReturns(data, names))
    risk1 := portfolioRisk(w1, matrix)

    // ✅ PHASE 2 portfolio (after options / reopt)
    w2 := optimizeSharpe(data, names)
    ret2 := portfolioReturn(w2, expectedReturns(data, names))
    risk2 := portfolioRisk(w2, matrix)

    jsonW1, _ := json.Marshal(w1)
    jsonW2, _ := json.Marshal(w2)
    jsonLabels, _ := json.Marshal(names)

    html := ""

    // ✅ HEADER
    html += buildHeader(buildSummary(data))

    // ✅ PRICE CHART
    html += buildLineChart(string(jsonData), length)

    // ✅ TABLES
    html += buildPriceTable(data, names, length)
    html += buildReturnsTable(data, names, length)
    html += buildStatsTable(data, names)
    html += buildGreeksTable(data)
    html += buildOptionTimelineTable(data)
    html += buildOptionDecisionTable(data)
    // ✅ HEATMAP
    html += buildCovarianceHeatmap(matrix, names)

    // ✅ OPTIONS TABLE
    html += buildOptionTable(options)

    // ✅ PHASE 1 PORTFOLIO
    html += "<div class='card'><h2>🧠 Portfolio Phase 1 (Before Options)</h2>"
    html += fmt.Sprintf("<p>Return: %.4f | Risk: %.6f</p>", ret1, risk1)
    html += "<table><tr><th>Asset</th><th>Weight</th></tr>"

    for i, n := range names {
        html += fmt.Sprintf("<tr><td>%s</td><td>%.2f</td></tr>", n, w1[i])
    }

    html += "</table></div>"

    // ✅ PIE PHASE 1
    html += buildPieChartPhase("🥧 Portfolio Phase 1", string(jsonW1), string(jsonLabels), "pie1")
    // ✅ PHASE 2 PORTFOLIO
    html += "<div class='card'><h2>🧠 Portfolio Phase 2 (After Options + Reopt)</h2>"
    html += fmt.Sprintf("<p>Return: %.4f | Risk: %.6f</p>", ret2, risk2)
    html += "<table><tr><th>Asset</th><th>Weight</th></tr>"

    for i, n := range names {
        html += fmt.Sprintf("<tr><td>%s</td><td>%.2f</td></tr>", n, w2[i])
    }

    html += "</table></div>"

    // ✅ PIE PHASE 2
    html += buildPieChartPhase("🥧 Portfolio Phase 2", string(jsonW2), string(jsonLabels), "pie2")
    // ✅ CLOSE HTML
    html += `
</body>
</html>
`

    os.WriteFile("report.html", []byte(html), 0644)
}
func buildGreeksTable(data map[string][]float64) string {

    html := "<div class='card'><h2>📐 Greeks (Delta / Gamma)</h2><table>"
    html += "<tr><th>Asset</th><th>Delta(Call)</th><th>Gamma</th></tr>"

    for k, v := range data {

        S := v[len(v)-1]
        K := S * 1.05

        d := deltaCall(S, K, 0.1, 0.01, 0.3)
        g := gamma(S, K, 0.1, 0.01, 0.3)

        html += fmt.Sprintf(
            "<tr><td>%s</td><td>%.3f</td><td>%.5f</td></tr>",
            k, d, g,
        )
    }

    html += "</table></div>"

    return html
}
//////////////////////////
// ✅ HEADER
//////////////////////////
func buildOptionDecisionTable(data map[string][]float64) string {

    html := "<div class='card'><h2>🧠 Option Decision</h2><table>"
    html += "<tr><th>Asset</th><th>Decision</th></tr>"

    for k, v := range data {

        decision := decideOptionAction(k, v)

        html += "<tr><td>" + k + "</td><td>" + decision + "</td></tr>"
    }

    html += "</table></div>"

    return html
}
func buildHeader(summary string) string {
    return fmt.Sprintf(`
<html>
<head>
<title>Risk Dashboard</title>

<script src="https://cdn.jsdelivr.net/npm/chart.js"></script>

<style>
body { font-family: Arial; background:#0f172a; color:#e2e8f0; padding:20px; }
h1 { color:#38bdf8; }

.card { background:#1e293b; padding:15px; border-radius:10px; margin-bottom:20px; }

table { border-collapse:collapse; margin-top:10px; }
td,th { border:1px solid #334155; padding:6px; text-align:center; }
th { background:#1e293b; }
</style>
</head>

<body>

<h1>🚀 Risk Dashboard</h1>

<div class="card">
<h2>🧠 Risk Summary</h2>
%s
</div>
`, summary)
}

//////////////////////////
// ✅ LINE CHART
//////////////////////////
func buildLineChart(jsonData string, length int) string {
    return fmt.Sprintf(`
<div class="card">
<h2>📈 Price Chart</h2>

<!-- ✅ FIX SIZE -->
<div style="width:800px; margin:auto;">
<canvas id="line"></canvas>
</div>

</div>

<script>
window.onload = function() {

    const dataObj = %s;

    const labels = Array.from({length:%d}, (_,i)=>i+1);

    const colors = ["#38bdf8","#22c55e","#eab308","#f43f5e","#a78bfa"];

    const datasets = Object.keys(dataObj).map((k,i)=>({
        label:k,
        data:dataObj[k],
        borderColor:colors[i%%5],
        fill:false,
        tension:0.2
    }));

    new Chart(document.getElementById("line"),{
        type:"line",
        data:{labels,datasets},
        options:{
            responsive:true,
            maintainAspectRatio:true // ✅ FIX SCALE
        }
    });
};
</script>
`, jsonData, length)
}

func buildOptionTimelineTable(data map[string][]float64) string {

    html := "<div class='card'><h2>⏱ Option Timeline</h2><table>"
    html += "<tr><th>Day</th><th>Asset</th><th>Price</th><th>Decision</th></tr>"

    for asset, prices := range data {

        decisions := decideOptionTimeline(asset, prices)

        for i := 20; i < len(prices); i++ {

            html += fmt.Sprintf(
                "<tr><td>%d</td><td>%s</td><td>%.2f</td><td>%s</td></tr>",
                i, asset, prices[i], decisions[i],
            )
        }
    }

    html += "</table></div>"

    return html
}

//////////////////////////
// ✅ PRICE TABLE
//////////////////////////

func buildPriceTable(data map[string][]float64, names []string, length int) string {

    html := "<div class='card'><h2>📊 Price Table</h2><table><tr><th>Day</th>"

    for _, n := range names {
        html += "<th>" + n + "</th>"
    }

    html += "</tr>"

    for i := 0; i < length; i++ {
        html += fmt.Sprintf("<tr><td>%d</td>", i+1)
        for _, n := range names {
            html += fmt.Sprintf("<td>%.2f</td>", data[n][i])
        }
        html += "</tr>"
    }

    return html + "</table></div>"
}

//////////////////////////
// ✅ RETURNS TABLE
//////////////////////////

func buildReturnsTable(data map[string][]float64, names []string, length int) string {

    html := "<div class='card'><h2>📉 Returns Table</h2><table><tr><th>Day</th>"

    for _, n := range names {
        html += "<th>" + n + "</th>"
    }

    html += "</tr>"

    for i := 0; i < length-1; i++ {
        html += fmt.Sprintf("<tr><td>%d</td>", i+1)
        for _, n := range names {
            r := returns(data[n])
            html += fmt.Sprintf("<td>%.4f</td>", r[i])
        }
        html += "</tr>"
    }

    return html + "</table></div>"
}
func buildOptionTable(options []OptionResult) string {

    html := "<div class='card'><h2>📉 Options Table</h2><table>"
    html += "<tr><th>Day</th><th>Asset</th><th>Call</th><th>Put</th></tr>"

    // limit display (avoid huge UI)
    for i, o := range options {

        if i > 50 {
            break
        }

        html += fmt.Sprintf(
            "<tr><td>%d</td><td>%s</td><td>%.2f</td><td>%.2f</td></tr>",
            o.Day, o.Asset, o.Call, o.Put,
        )
    }

    html += "</table></div>"

    return html
}
//////////////////////////
// ✅ STATS
//////////////////////////

func buildStatsTable(data map[string][]float64, names []string) string {

    html := "<div class='card'><h2>📊 Statistics</h2><table>"
    html += "<tr><th>Asset</th><th>Mean</th><th>StdDev</th><th>Variance</th><th>MDD</th></tr>"

    for _, n := range names {
        r := returns(data[n])
        html += fmt.Sprintf(
            "<tr><td>%s</td><td>%.4f</td><td>%.4f</td><td>%.6f</td><td>%.2f%%</td></tr>",
            n, mean(r), stddev(r), variance(r), maxDrawdown(data[n])*100,
        )
    }

    return html + "</table></div>"
}

//////////////////////////
// ✅ HEATMAP
//////////////////////////

func buildCovarianceHeatmap(matrix [][]float64, names []string) string {

    html := "<div class='card'><h2>🔥 Covariance Heatmap</h2><table><tr><th></th>"

    for _, n := range names {
        html += "<th>" + n + "</th>"
    }

    html += "</tr>"

    for i := range matrix {

        html += "<tr><td>" + names[i] + "</td>"

        for j := range matrix[i] {

            val := matrix[i][j]

            alpha := val * 40
            if alpha > 1 {
                alpha = 1
            }

            color := fmt.Sprintf("rgba(255,0,0,%.2f)", alpha)

            html += fmt.Sprintf("<td style='background:%s'>%.5f</td>", color, val)
        }

        html += "</tr>"
    }

    return html + "</table></div>"
}

//////////////////////////
// ✅ PORTFOLIO
//////////////////////////

func buildPortfolioSection(names []string, weights []float64, ret float64, risk float64) string {

    html := "<div class='card'><h2>🧠 Portfolio</h2>"

    html += fmt.Sprintf("<p>Expected Return: %.4f</p>", ret)
    html += fmt.Sprintf("<p>Risk (Variance): %.6f</p>", risk)

    html += "<table><tr><th>Asset</th><th>Weight</th></tr>"

    for i, n := range names {
        html += fmt.Sprintf("<tr><td>%s</td><td>%.2f</td></tr>", n, weights[i])
    }

    return html + "</table></div>"
}

//////////////////////////
// ✅ PIE CHART
//////////////////////////

func buildPieChart(jsonWeights string, jsonLabels string) string {
    return fmt.Sprintf(`
<div class="card">
<h2>🥧 Portfolio Allocation</h2>

<div style="width:300px; height:300px; margin:auto;">
<canvas id="pie"></canvas>
</div>

</div>

<script>
window.addEventListener("load", function() {

    const labels = %s;
    const weights = %s;

    const colors = ["#38bdf8","#22c55e","#eab308","#f43f5e","#a78bfa"];

    new Chart(document.getElementById("pie"), {
        type:"pie",
        data:{
            labels:labels,
            datasets:[{data:weights, backgroundColor:colors}]
        },
        options:{
            responsive:true,
            maintainAspectRatio:false
        }
    });

});
</script>
`, jsonLabels, jsonWeights)
}

//////////////////////////
// ✅ SUMMARY
//////////////////////////

func buildSummary(data map[string][]float64) string {

    type row struct {
        name string
        risk float64
    }

    var rows []row

    for k, v := range data {
        rows = append(rows, row{k, stddev(returns(v))})
    }

    sort.Slice(rows, func(i, j int) bool {
        return rows[i].risk < rows[j].risk
    })

    html := "<ul>"

    for i, r := range rows {

        tag := "Medium"

        if i == 0 {
            tag = "✅ Low"
        } else if i == len(rows)-1 {
            tag = "🚨 High"
        }

        html += fmt.Sprintf("<li>%s = %.4f (%s)</li>", r.name, r.risk, tag)
    }

    html += "</ul>"

    return html
}
func buildPieChartPhase(title string, jsonWeights string, jsonLabels string, canvasID string) string {

    return fmt.Sprintf(`
<div class="card">
<h2>%s</h2>

<div style="width:300px; height:300px; margin:auto;">
<canvas id="%s"></canvas>
</div>

</div>

<script>

window.addEventListener("load", function() {

    const labels = %s;
    const weights = %s;

    const colors = ["#38bdf8","#22c55e","#eab308","#f43f5e","#a78bfa"];

    new Chart(document.getElementById("%s"), {
        type:"pie",
        data:{
            labels:labels,
            datasets:[{
                data:weights,
                backgroundColor:colors
            }]
        },
        options:{
            responsive:true,
            maintainAspectRatio:false
        }
    });

});

</script>
`, title, canvasID, jsonLabels, jsonWeights, canvasID)
}
func buildOptionSection(data map[string][]float64) string {

    html := "<div class='card'><h2>📉 Options Analysis</h2>"

    prices := data["ADA"]

    for day := 20; day < len(prices); day++ {

        call := blackScholesCall(prices[day], prices[day]*1.05, 0.1, 0.01, 0.3)

        put := blackScholesPut(prices[day], prices[day]*1.05, 0.1, 0.01, 0.3)

        html += fmt.Sprintf(
            "<p>Day %d → Call %.2f | Put %.2f</p>",
            day, call, put,
        )
    }

    return html + "</div>"
}
//////////////////////////
// ✅ UTIL
//////////////////////////

func sortedNames(data map[string][]float64) []string {
    names := []string{}
    for k := range data {
        names = append(names, k)
    }
    sort.Strings(names)
    return names
}