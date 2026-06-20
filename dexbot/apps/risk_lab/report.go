/*
Filename: apps/risk_lab/report.go

Author: M365 Copilot (GPT-5)
Version: v5.0 (Complete Risk Dashboard)
Owner: Chalearm Saelim
Date: 2026-06-21 06:02 ICT (UTC+7)

Description:
Full financial risk dashboard generator.

Features:
✅ Price Table
✅ Returns Table
✅ Statistics (mean, std, variance)
✅ Max Drawdown (MDD)
✅ Covariance Matrix
✅ Risk Summary (human-readable)
✅ Chart visualization (Chart.js)
✅ Dark UI with layout panels

Usage:
    cd dexbot/apps/risk_lab
    go run .
    open report.html

Goal:
Transform raw quant data into readable dashboard.
*/

package main

import (
    "encoding/json"
    "fmt"
    "os"
    "sort"
)

// ✅ HUMAN READABLE SUMMARY
func buildSummary(data map[string][]float64) string {

    type row struct {
        name string
        risk float64
        mdd  float64
    }

    var rows []row

    for k, v := range data {
        rows = append(rows, row{
            name: k,
            risk: stddev(returns(v)),
            mdd:  maxDrawdown(v),
        })
    }

    sort.Slice(rows, func(i, j int) bool {
        return rows[i].risk < rows[j].risk
    })

    html := "<ul>"

    for i, r := range rows {

        tag := "Medium Risk"

        if i == 0 {
            tag = "✅ Low Risk"
        } else if i == len(rows)-1 {
            tag = "🚨 High Risk"
        }

        html += fmt.Sprintf(
            "<li><b>%s</b>: Vol=%.4f | MDD=%.2f%% (%s)</li>",
            r.name,
            r.risk,
            r.mdd*100,
            tag,
        )
    }

    html += "</ul>"

    html += `
<p>
<b>Explanation:</b><br>
Volatility = price fluctuation risk.<br>
MDD = worst loss from peak.<br>
Low risk = stable + small drawdown.<br>
High risk = volatile + big drops.<br>
</p>`

    return html
}

// ✅ MAIN HTML GENERATOR
func generateHTML(data map[string][]float64) {

    names := []string{}
    for k := range data {
        names = append(names, k)
    }
    sort.Strings(names)

    jsonData, _ := json.Marshal(data)
    summary := buildSummary(data)

    length := len(data[names[0]])

    html := fmt.Sprintf(`
<html>
<head>
<title>Risk Dashboard</title>

<script src="https://cdn.jsdelivr.net/npm/chart.js"></script>

<style>
body { font-family: Arial; background:#0f172a; color:#e2e8f0; padding:20px; }
h1 { color:#38bdf8; }
.card { background:#1e293b; padding:15px; border-radius:10px; margin-bottom:20px; }

table { border-collapse:collapse; margin-top:10px; }
td, th { border:1px solid #334155; padding:6px; text-align:center; }
th { background:#1e293b; }
</style>

</head>

<body>

<h1>🚀 Risk Dashboard</h1>

<div style="display:flex; gap:20px;">

<!-- LEFT -->
<div class="card" style="width:30%%">
<h2>🧠 Risk Summary</h2>
%s
</div>

<!-- RIGHT -->
<div class="card" style="width:70%%">
<h2>📈 Price Chart</h2>
<canvas id="chart"></canvas>
</div>

</div>
`, summary)

    // ✅ PRICE TABLE
    html += "<div class='card'><h2>📊 Price Table</h2><table><tr><th>Day</th>"
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
    html += "</table></div>"

    // ✅ RETURNS TABLE
    html += "<div class='card'><h2>📉 Returns Table</h2><table><tr><th>Day</th>"
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
    html += "</table></div>"

    // ✅ STATISTICS
    html += "<div class='card'><h2>📊 Statistics</h2><table>"
    html += "<tr><th>Asset</th><th>Mean</th><th>StdDev</th><th>Variance</th><th>MDD</th></tr>"

    for _, n := range names {
        r := returns(data[n])
        html += fmt.Sprintf(
            "<tr><td>%s</td><td>%.4f</td><td>%.4f</td><td>%.6f</td><td>%.2f%%</td></tr>",
            n,
            mean(r),
            stddev(r),
            variance(r),
            maxDrawdown(data[n])*100,
        )
    }
    html += "</table></div>"

    // ✅ COVARIANCE MATRIX
    matrix := covarianceMatrix(data)

    html += "<div class='card'><h2>📊 Covariance Matrix</h2><table><tr><th></th>"
    for _, n := range names {
        html += "<th>" + n + "</th>"
    }
    html += "</tr>"

    for i := range matrix {
        html += "<tr><td>" + names[i] + "</td>"
        for j := range matrix[i] {
            html += fmt.Sprintf("<td>%.6f</td>", matrix[i][j])
        }
        html += "</tr>"
    }
    html += "</table></div>"

    // ✅ CHART SCRIPT
    html += fmt.Sprintf(`
<script>

const dataObj = %s;

const labels = Array.from({length: %d}, (_, i) => i+1);

const colors = ["#38bdf8","#22c55e","#eab308","#f43f5e","#a78bfa"];

const datasets = Object.keys(dataObj).map((k,i) => ({
    label:k,
    data:dataObj[k],
    borderColor: colors[i %% 5],
    fill:false
}));

new Chart(document.getElementById("chart"), {
    type:"line",
    data:{labels:labels, datasets:datasets}
});

</script>

</body>
</html>
`, string(jsonData), length)

    os.WriteFile("report.html", []byte(html), 0644)
}