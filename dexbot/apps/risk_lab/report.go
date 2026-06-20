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

func generateHTML(data map[string][]float64) {

    names := sortedNames(data)
    length := len(data[names[0]])

    jsonData, _ := json.Marshal(data)

    // portfolio
    matrix := covarianceMatrix(data)
    weights := optimizeWeights(data, names)
    rets := expectedReturns(data, names)

    pReturn := portfolioReturn(weights, rets)
    pRisk := portfolioRisk(weights, matrix)

    jsonWeights, _ := json.Marshal(weights)
    jsonLabels, _ := json.Marshal(names)

    html := ""

    html += buildHeader(buildSummary(data))
    html += buildLineChart(string(jsonData), length)

    html += buildPriceTable(data, names, length)
    html += buildReturnsTable(data, names, length)
    html += buildStatsTable(data, names)

    html += buildCovarianceHeatmap(matrix, names)

    html += buildPortfolioSection(names, weights, pReturn, pRisk)
    html += buildPieChart(string(jsonWeights), string(jsonLabels))

    html += "</body></html>"

    os.WriteFile("report.html", []byte(html), 0644)
}

//////////////////////////
// ✅ HEADER
//////////////////////////

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