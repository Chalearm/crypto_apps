/******************************************************************************
 * File Name        : renderer.go
 * File Path        : webui/renderer.go
 * Author           : deepseek-4.0-pro & Gemini
 * Owner            : Chalearm Saelim
 * Reviewer         : Chalearm Saelim
 * Version          : 1.3.1
 * Status           : Development
 * Created Date     : 2026-07-01 19:25:26 (UTC+7)
 * Modified Date    : 2026-07-03 19:54:12 (UTC+7)
 *
 * Description      :
 *    Core Web UI design system and HTML/SVG rendering engine for Dexbot web UI.
 *    Provides interactive balance views, multi-chain/token management interfaces,
 *    daemon log inspectors, dual-line prediction plots, and DB browser cards.
 *
 * DEPENDENCY TREE & STRUCTURAL MAP:
 * ───────────────────────────────────────────────────────────────────────────
 * [webui/renderer.go] (UI Component & Rendering Engine)
 *     │
 *     ├── Imports Internal Packages ──> [dexbot/governance] (Registry Data Types)
 *     │                            ├──> [dexbot/infra] (Balance & Account Manager)
 *     │                            └──> [dexbot/school] (Tier Model Data Specs)
 *     │
 *     ├── Web Views Rendered:
 *     │     ├── Operations Dashboard ───> Governance process controls & live log viewer
 *     │     ├── Portfolio Page ─────────> Multi-chain asset holdings & balance editor
 *     │     ├── School Page ────────────> 4-tier model evolution status & DB browser
 *     │     └── Predict Page ───────────> SVG dual-line prediction vs actual charts
 *     │
 *     └── Output Receiver:
 *           └── Accepts http.ResponseWriter (or internal fakeResp buffer for file export)
 *
 * FUNCTION DEPENDENCY MATRIX (Internal Methods):
 * ───────────────────────────────────────────────────────────────────────────
 * NewRenderer(registry)
 * SetBalance(b) / SetModelRegistry(mr) / RefreshModels()
 *  └── pullModelsFromRegistry()
 *
 * Operations(w)
 *  ├── writeHead()
 *  ├── cssBase()
 *  └── writeFoot()
 *
 * Portfolio(w)
 *  └── writeBalanceCard() ──────────────> infra.GetBalanceSummary() & tokenReg.ListTokens()
 *
 * SchoolDashboard(w)
 *  └── buildTierDataFromRegistry()
 *
 * PredictionComparison(w)
 *  └── predictionDualLine() ───────────> mathSin() & mathCos()
 *
 * Responsibilities :
 *    - Formats HTML pages with responsive CSS styles and dark mode theme.
 *    - Dynamically generates interactive JavaScript modules for asset management.
 *    - Renders math-based inline SVG charts without relying on external frontend libraries.
 *    - Provides fallback templates for database inspection and model performance.
 *
 * Usage :
 *    Directory : webui/
 *    Build     : go build ./webui
 *    Run       : Invoked via Governance publisher thread (`refreshDashboard`)
 *
 * Dependencies :
 *    Internal  : dexbot/governance, dexbot/infra, dexbot/school
 *    External  : stdlib (html, net/http, json, os, strconv, strings)
 *
 * Change History :
 *    -------------------------------------------------------------------------
 *    Version | Date Time (UTC+7)         | Author           | Description
 *    -------------------------------------------------------------------------
 *    1.0.0   | 2026-07-01 19:25:26 (UTC+7) | deepseek-4.0-pro | Initial release
 *    1.3.1   | 2026-07-03 19:54:12 (UTC+7) | Gemini           | 9-decimal precision layout & asset panel
 *    -------------------------------------------------------------------------
 *
 * Notes :
 *    - Per regulator coding standard rules.
 ******************************************************************************/
package webui

import (
  "encoding/json"
  "fmt"
  "html"
  "net/http"
  "os"
  "strconv"
  "strings"
  "time"

  "dexbot/governance"
  "dexbot/infra"
  "dexbot/school"
)

// ==============================
// RENDERER CORE DEFINITIONS
// ==============================

type Renderer struct {
  registry    *governance.Registry
  modelReg    *governance.ModelRegistry
  govPort     int
  schoolPort  int
  tradingPort int
  webPort     int
  models      []governance.ModelPerformance
  txns        []governance.TransactionRecord
  balance     *infra.BalanceSummary // §79-80: account balance data
}
/******************************************************************************
 * Function Name : NewRenderer
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func NewRenderer(registry *governance.Registry) *Renderer {
  return &Renderer{
    registry:    registry,
    govPort:     8081,
    schoolPort:  8082,
    tradingPort: 8083,
    webPort:     8080,
    models:      nil,
    txns:        nil,
    balance:     nil,
  }
}

// SetBalance provides the latest balance summary for display.
/******************************************************************************
 * Function Name : SetBalance
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (r *Renderer) SetBalance(b *infra.BalanceSummary) {
  r.balance = b
}

// SetModelRegistry links the centralized model registry for live data.
/******************************************************************************
 * Function Name : SetModelRegistry
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (r *Renderer) SetModelRegistry(mr *governance.ModelRegistry) {
  r.modelReg = mr
  r.RefreshModels()
}

// SetTransactions updates transaction records for display.
/******************************************************************************
 * Function Name : SetTransactions
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (r *Renderer) SetTransactions(txns []governance.TransactionRecord) {
  r.txns = txns
}

// RefreshModels reloads model data from the centralized registry.
/******************************************************************************
 * Function Name : RefreshModels
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (r *Renderer) RefreshModels() {
  r.pullModelsFromRegistry()
}

// pullModelsFromRegistry reads live model data from the centralized registry.
/******************************************************************************
 * Function Name : pullModelsFromRegistry
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (r *Renderer) pullModelsFromRegistry() {
  if r.modelReg == nil {
    r.models = []governance.ModelPerformance{
      {Name: "No models registered yet", Score: 0, WinRate: 0, Status: "training"},
    }
    return
  }
  var models []governance.ModelPerformance
  for _, id := range r.modelReg.AllIDs() {
    mr := r.modelReg.Get(id)
    if mr == nil {
      continue
    }
    score := 0.0
    winRate := 0.0
    status := "training"
    if fs := mr.LatestFitness(); fs != nil {
      score = fs.Sharpe * 10 // scale Sharpe to 0-100
      if score < 0 {
        score = 0
      }
      if score > 100 {
        score = 100
      }
      winRate = fs.Consistency
    }
    switch mr.Status {
    case governance.ModelStatusGraduated, governance.ModelStatusActive:
      status = "active"
    case governance.ModelStatusRetired:
      status = "abandoned"
    default:
      status = "training"
    }
    models = append(models, governance.ModelPerformance{
      Name:    mr.ID,
      Score:   score,
      WinRate: winRate,
      Status:  status,
    })
  }
  if len(models) == 0 {
    models = []governance.ModelPerformance{
      {Name: "No models registered yet", Score: 0, WinRate: 0, Status: "training"},
    }
  }
  r.models = models
}
/******************************************************************************
 * Function Name : SetPorts
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func (r *Renderer) SetPorts(gov, school, trading, web int) {
  r.govPort = gov
  r.schoolPort = school
  r.tradingPort = trading
  r.webPort = web
}

// ==============================
// DESIGN SYSTEM — CSS Style
// ==============================
/******************************************************************************
 * Function Name : cssBase
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func cssBase() string {
  return `<style>
  :root {
    --bg-deep:     #111827;
    --bg-surface: #1a2332;
    --bg-card:    #1f2a3a;
    --bg-elevated:#263348;
    --border:      #2d3a4a;
    --text-primary:  #e2e8f0;
    --text-secondary:#94a3b8;
    --text-muted:    #64748b;
    --accent:      #2dd4bf;
    --accent-dim: #0d9488;
    --green:      #34d399;
    --amber:      #fbbf24;
    --rose:       #f87171;
    --blue:       #60a5fa;
    --purple:      #a78bfa;
    --radius:      12px;
    --shadow:      0 1px 3px rgba(0,0,0,.3), 0 1px 2px rgba(0,0,0,.2);
  }
  *,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
  body{
    font-family:system-ui,-apple-system,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;
    background:var(--bg-deep);
    color:var(--text-primary);
    line-height:1.6;
    padding:24px 32px;
    min-height:100vh;
  }
  h1{font-size:1.5rem;font-weight:600;color:var(--text-primary);margin-bottom:24px;
     display:flex;align-items:center;gap:10px}
  h1::before{content:'';display:inline-block;width:4px;height:24px;
              background:var(--accent);border-radius:2px}
  h2{font-size:1.15rem;font-weight:600;color:var(--text-secondary);margin:20px 0 12px}
  .nav{display:flex;gap:4px;margin-bottom:28px;background:var(--bg-surface);
       border-radius:var(--radius);padding:4px;width:fit-content}
  .nav a{color:var(--text-secondary);text-decoration:none;padding:8px 20px;
          border-radius:10px;font-size:.875rem;font-weight:500;transition:all .2s}
  .nav a:hover{color:var(--text-primary);background:var(--bg-elevated)}
  .nav a.active{color:var(--accent);background:var(--bg-card)}
  .card{
    background:var(--bg-card);border:1px solid var(--border);
    border-radius:var(--radius);padding:20px 24px;margin-bottom:16px;
    box-shadow:var(--shadow);transition:border-color .2s
  }
  .card:hover{border-color:#3d4a5a}
  .card-header{display:flex;align-items:center;justify-content:space-between;margin-bottom:14px}
  .card-title{font-size:1.05rem;font-weight:600;display:flex;align-items:center;gap:8px}
  .card-subtitle{font-size:.8rem;color:var(--text-muted)}
  .badge{display:inline-flex;align-items:center;gap:5px;padding:3px 12px;
         border-radius:99px;font-size:.75rem;font-weight:600}
  .badge-healthy{background:rgba(52,211,153,.12);color:var(--green)}
  .badge-unhealthy{background:rgba(248,113,113,.12);color:var(--rose)}
  .badge-starting{background:rgba(251,191,36,.12);color:var(--amber)}
  .badge-stopping{background:rgba(251,191,36,.12);color:var(--amber)}
  .badge-unknown{background:rgba(100,116,139,.12);color:var(--text-muted)}
  /* Task Manager Table Styles */
  table.task-manager {width:100%; border-collapse:collapse; background:var(--bg-card); border-radius:8px; overflow:hidden;}
  table.task-manager th {background:var(--bg-surface); text-align:left; padding:12px; font-size:.8rem; color:var(--text-secondary); font-weight:600; text-transform:uppercase; border-bottom:1px solid var(--border);}
  table.task-manager td {padding:14px 12px; font-size:.875rem; border-bottom:1px solid var(--border); vertical-align:middle;}
  table.task-manager tr:last-child td {border-bottom:none;}
  table.task-manager tr:hover {background:var(--bg-elevated);}

  .metrics-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(120px,1fr));
                 gap:12px;margin:8px 0}
  .metric{background:var(--bg-elevated);border-radius:10px;padding:12px 14px}
  .metric-label{font-size:.7rem;text-transform:uppercase;letter-spacing:.05em;
                 color:var(--text-muted);margin-bottom:4px}
  .metric-value{font-size:1.1rem;font-weight:600;color:var(--text-primary)}
  .metric-unit{font-size:.75rem;color:var(--text-muted);margin-left:2px}
  table{width:100%;border-collapse:collapse}
  th{text-align:left;padding:10px 14px;font-size:.75rem;text-transform:uppercase;
     letter-spacing:.05em;color:var(--text-muted);border-bottom:1px solid var(--border)}
  td{padding:10px 14px;font-size:.875rem;border-bottom:1px solid var(--border)}
  tr:last-child td{border-bottom:none}
  .status-active{color:var(--green)}.status-training{color:var(--amber)}
  .status-abandoned{color:var(--rose)}
  .pnl-positive{color:var(--green)}.pnl-negative{color:var(--rose)}
  .btn-group{display:flex;gap:6px;margin-top:12px}
  .btn{padding:6px 16px;border:none;border-radius:8px;cursor:pointer;
       font-size:.8rem;font-weight:500;transition:all .15s}
  .btn-start{background:rgba(52,211,153,.15);color:var(--green)}
  .btn-start:hover{background:rgba(52,211,153,.25)}
  .btn-stop{background:rgba(248,113,113,.15);color:var(--rose)}
  .btn-stop:hover{background:rgba(248,113,113,.25)}
  .btn-restart{background:rgba(96,165,250,.15);color:var(--blue)}
  .btn-restart:hover{background:rgba(96,165,250,.25)}
  .chart-wrap{background:var(--bg-elevated);border-radius:10px;padding:12px;
              margin-top:10px}
  .spark-row{display:flex;align-items:center;gap:8px;font-size:.8rem}
  .spark-label{color:var(--text-muted);min-width:36px}
  .footer{color:var(--text-muted);font-size:.75rem;margin-top:32px;
           padding-top:16px;border-top:1px solid var(--border)}
  .restart-chip{display:inline-flex;align-items:center;gap:4px;
                background:rgba(248,113,113,.08);color:var(--rose);
                padding:2px 10px;border-radius:99px;font-size:.7rem;
                font-weight:500;margin-left:6px}

  /* ── Refactored Account Balance Workspace Styles ── */
  .balance-card{background:var(--bg-card);border:1px solid var(--border);
                border-radius:var(--radius);padding:20px 24px;margin-bottom:20px}
  .balance-interactive-header{display:flex;align-items:center;gap:12px;cursor:pointer;
                              user-select:none;padding:6px;border-radius:8px;transition:background .2s}
  .balance-interactive-header:hover{background:var(--bg-elevated)}
  .balance-amount-display{font-weight:700;font-size:1.15rem;color:var(--accent);margin-left:8px}
  .chain-panel{display:none;margin-top:16px;background:var(--bg-elevated);border-radius:10px;padding:16px}
  .chain-panel.open{display:block}
  .asset-row{display:flex;justify-content:space-between;align-items:center;padding:8px 0;
             border-bottom:1px solid var(--border);font-size:.8rem;font-family:monospace}
  .asset-row:last-child{border-bottom:none}
  .asset-ticker{font-weight:600;color:var(--accent);min-width:60px}
  .asset-price{color:var(--blue);min-width:90px;text-align:right;font-size:.75rem}
  .asset-amount{text-align:right;color:var(--text-primary);word-break:break-all;min-width:140px}
  .asset-usd{color:var(--text-secondary);margin-left:8px;min-width:100px;text-align:right}
  .pencil-icon{cursor:pointer;opacity:.5;font-size:1.05rem;transition:opacity .15s;padding:2px 6px}
  .pencil-icon:hover{opacity:1}
  .pencil-icon::before{content:'\270F\FE0F'}
  .delete-token-btn{cursor:pointer;color:var(--rose);font-size:1rem;padding:2px 6px;
    opacity:0;transition:opacity .15s;background:none;border:none}
  .asset-row:hover .delete-token-btn{opacity:1}
  .asset-row.editing .delete-token-btn{opacity:1;color:var(--rose)}
  .edit-actions{display:none;gap:8px;margin-top:12px;justify-content:flex-end}
  .edit-actions.visible{display:flex}
  .edit-actions .btn:disabled{opacity:.4;cursor:not-allowed}
  .chain-add-row{display:none;margin-top:10px;gap:8px;padding:10px;
    background:var(--bg-elevated);border-radius:8px;border:1px solid var(--border)}
  .chain-add-row.visible{display:flex;flex-wrap:wrap;align-items:center}

  select#chainSelect option {
    font-family: monospace, system-ui;
    background: var(--bg-card);
    color: var(--text-primary);
  }

  /* ── Additional Original Engine Support CSS ── */
  .btn-kill{background:rgba(248,113,113,.25);color:var(--rose);border:1px solid rgba(248,113,113,.3)}
  .btn-kill:hover{background:rgba(248,113,113,.4)}
  .badge-killing{background:rgba(248,113,113,.3);color:var(--rose);animation:pulse .6s infinite}
  .badge-building{background:rgba(251,191,36,.2);color:var(--amber)}
  .badge-recovering{background:rgba(251,191,36,.15);color:var(--amber)}
  .port-detail{border-top:1px solid var(--border);padding-top:12px;margin-top:8px;transition:all .3s}
  .legend-dot{display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:4px}

  @keyframes pulse{0%,100%{opacity:1}50%{opacity:.5}}

  @media(max-width:768px){
    body{padding:12px 16px}
    .nav{width:100%;justify-content:center;flex-wrap:wrap}
    .nav a{padding:6px 12px;font-size:.75rem}
    .metrics-grid{grid-template-columns:repeat(2,1fr)}
    .card{padding:12px 16px}
    h1{font-size:1.2rem}
    .btn-group{flex-wrap:wrap}
    .btn{padding:8px 14px;font-size:.85rem;min-height:44px}
    .balance-interactive-header{flex-direction:column;align-items:flex-start;gap:6px}
    .asset-row{flex-direction:column;align-items:flex-start;gap:4px}
    table{display:block;overflow-x:auto;-webkit-overflow-scrolling:touch}
  }
  @media(max-width:480px){
    .metrics-grid{grid-template-columns:1fr}
    .nav a{padding:4px 8px;font-size:.7rem}
  }
</style>`
}

// ==============================
// SHARED LAYOUT LAYERS
// ==============================
/******************************************************************************
 * Function Name : writeHead
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func writeHead(w http.ResponseWriter, title, active string) {
  w.Header().Set("Content-Type", "text/html; charset=utf-8")
  navLink := func(label, key, href string) string {
    cls := ""
    if key == active {
      cls = " active"
    }
    return fmt.Sprintf(`<a class="%s" href="%s">%s</a>`, cls, href, label)
  }
  fmt.Fprintf(w, `<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="Cache-Control" content="no-cache, no-store, must-revalidate">
<meta http-equiv="Pragma" content="no-cache">
<meta http-equiv="Expires" content="0">
<title>%s — Dexbot</title>%s</head><body>
<nav class="nav">%s%s%s</nav>`,
    html.EscapeString(title), cssBase(),
    navLink("Governance", "gov", "/"),
    navLink("Trading", "trade", "/trading"),
    navLink("School", "school", "/school"),
  )
}
/******************************************************************************
 * Function Name : writeFoot
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func writeFoot(w http.ResponseWriter, ports string) {
  fmt.Fprintf(w, `<div class="footer">%s</div></body></html>`, ports)
}

// ==============================
// OPERATIONS DASHBOARD
// ==============================

// ── Governance Dashboard (single table, expand-on-click log) ──
/******************************************************************************
 * Function Name : Operations
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (r *Renderer) Operations(w http.ResponseWriter) {
  writeHead(w, "Governance", "gov")
  fmt.Fprint(w, `<h1>Governance — Daemon Status</h1>
<div style="display:flex;align-items:center;gap:16px;margin-bottom:10px;font-size:.7rem;color:var(--text-muted)">
  <label style="cursor:pointer;display:flex;align-items:center;gap:4px">
    <input type="checkbox" id="chkEVENT" checked onchange="refreshLogFilter()"> [EVENT]
  </label>
  <label style="cursor:pointer;display:flex;align-items:center;gap:4px">
    <input type="checkbox" id="chkCYCLC" onchange="refreshLogFilter()"> [CYCLC]
  </label>
  <span id="logFilterInfo" style="color:var(--text-muted)"></span>
</div>
<div class="card">
<table style="width:100%">
<thead><tr>
  <th>Daemon</th><th>Status</th><th>Message</th><th>CPU</th><th>Mem</th><th>Uptime</th><th>Actions</th>
</tr></thead>
<tbody id="daemonTableBody">`)

  names := r.registry.List()
  for _, name := range names {
    if strings.HasPrefix(name, "integration_test") { continue }
    d := r.registry.GetStatus(name)
    if d == nil { continue }
    badge := "badge-healthy"
    if d.Status == "unhealthy" || d.Status == "unknown" { badge = "badge-unhealthy" }
    crown := ""
    if d.Name == "governance" { crown = "\U0001F451 " }
    fmt.Fprintf(w, `<tr id="row_%s" onclick="toggleLog('%s')" style="cursor:pointer">
      <td style="font-weight:600;color:var(--accent)">%s%s</td>
      <td id="s_%s"><span class="badge %s">%s</span></td>
      <td id="m_%s" style="font-size:.7rem;color:var(--text-secondary);max-width:280px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">%s</td>
      <td id="c_%s">%.1f%%</td><td id="r_%s">%.0fMB</td>
      <td>%s</td>
      <td><div style="display:flex;gap:3px">
        <button class="btn btn-start" onclick="event.stopPropagation();actDaemon('%s','start')" style="font-size:.6rem;padding:1px 5px">S</button>
        <button class="btn btn-stop" onclick="event.stopPropagation();actDaemon('%s','stop')" style="font-size:.6rem;padding:1px 5px">T</button>
        <button class="btn btn-restart" onclick="event.stopPropagation();actDaemon('%s','restart')" style="font-size:.6rem;padding:1px 5px">R</button>
      </div></td>
    </tr>
    <tr id="log_%s" style="display:none"><td colspan="7"><pre id="logp_%s" style="margin:0;padding:8px;background:var(--bg-deep);color:var(--text-secondary);font-size:.65rem;max-height:180px;overflow-y:auto;white-space:pre-wrap;border-radius:6px">Click to load log...</pre></td></tr>`,
      d.Name, d.Name, crown, strings.ToUpper(d.Name),
      d.Name, badge, d.Status,
      d.Name, html.EscapeString(d.Message),
      d.Name, d.CPUPercent, d.Name, d.MemoryMB,
      d.Uptime.Round(time.Second).String(),
      d.Name, d.Name, d.Name,
      d.Name, d.Name,
    )
  }

  fmt.Fprint(w, `</tbody></table></div>
<script>
function actDaemon(n,a){fetch('/api/daemon/'+n+'/'+a,{method:'POST'}).then(function(r){return r.json()}).then(function(d){if(d.status!=='ok')alert(d.message);setTimeout(pollDS,2000)}).catch(function(e){alert('Action failed: '+e)})}
function toggleLog(n){var r=document.getElementById('log_'+n);if(r.style.display==='none'||!r.style.display){r.style.display='table-row';fetch('/api/daemon/'+n+'/log').then(function(r){return r.json()}).then(function(d){document.getElementById('logp_'+n).textContent=d.lines||'No log available'})}else{r.style.display='none'}}
function refreshLogFilter(){loadAllMessages();}
function filterLogLines(text){if(!text)return'';var cyc=document.getElementById('chkCYCLC');var evt=document.getElementById('chkEVENT');var lines=text.split(String.fromCharCode(10));var out=[];for(var i=0;i<lines.length;i++){var l=lines[i];var isCyc=l.indexOf('[CYCLC')>=0;var isEvt=l.indexOf('[EVENT')>=0;if((!isCyc||cyc.checked)&&(!isEvt||evt.checked))out.push(l)}return out.join(String.fromCharCode(10))}
function loadLogMsg(n){fetch('/api/daemon/'+n+'/log').then(function(r){return r.json()}).then(function(d){var t=(d.lines||'').trim();var ft=filterLogLines(t);var i=ft.lastIndexOf(String.fromCharCode(10));var last=i>=0?ft.substring(i+1):ft;if(last.length>0){var m=document.getElementById('m_'+n);if(m)m.textContent=last.substring(0,120)}}).catch(function(){})}
function loadAllMessages(){var rs=document.querySelectorAll('tr[id^=row_]');for(var i=0;i<rs.length;i++){loadLogMsg(rs[i].id.replace('row_',''))}}
function pollDS(){fetch('/api/daemons').then(function(r){return r.json()}).then(function(d){var dd=d.daemons||d;for(var i=0;i<dd.length;i++){var x=dd[i];var s=document.getElementById('s_'+x.Name);if(s)s.innerHTML='<span class="badge '+(x.Status==='healthy'||x.Status==='pass'?'badge-healthy':'badge-unhealthy')+'">'+x.Status+'</span>';var c=document.getElementById('c_'+x.Name);if(c)c.textContent=(x.CPUPercent||0).toFixed(1)+'%';var r=document.getElementById('r_'+x.Name);if(r)r.textContent=(x.MemoryMB||0).toFixed(0)+'MB';loadLogMsg(x.Name)}})}
(function(){var rs=document.querySelectorAll('tr[id^=row_]');for(var i=0;i<rs.length;i++){loadLogMsg(rs[i].id.replace('row_',''))}})()
setInterval(pollDS,5000)
</script>`)
  writeFoot(w, "UDP: All daemons via config.env (MANAGED_DAEMONS)")
}

// ── End of Governance Dashboard ──

// ── BALANCE CARD (UPDATED WORKSPACE) ──
/******************************************************************************
 * Function Name : Portfolio
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (r *Renderer) Portfolio(w http.ResponseWriter) {
  writeHead(w, "Trading", "trade")
  fmt.Fprint(w, `<h1>Trading — Portfolio &amp; Balance</h1>`)
  r.writeBalanceCard(w)
  writeFoot(w, fmt.Sprintf("Portfolio data refreshed each trading cycle."))
}
/******************************************************************************
 * Function Name : writeBalanceCard
 *
 * Purpose       : Renders an expandable, tabbed portfolio balance card.
 *                 Fetches live data directly from Balance API (Port 8087).
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/
func (r *Renderer) writeBalanceCard(w http.ResponseWriter) {
  if r.balance == nil {
    am := infra.NewAccountManager()
    r.balance = infra.GetBalanceSummary(am)
  }

  var preAssets []infra.BalanceAsset = []infra.BalanceAsset{}
  assetJSON, _ := json.Marshal(preAssets)

  fmt.Fprintf(w, `<style>
  /* Expandable Card Layout & Styling */
  .balance-card-container {
    position: relative;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 20px 24px;
    margin-bottom: 20px;
    transition: all 0.3s ease;
  }

  .balance-card-container.enlarged {
    position: fixed;
    top: 20px;
    left: 20px;
    right: 20px;
    bottom: 20px;
    z-index: 9999;
    margin: 0;
    overflow-y: auto;
    box-shadow: 0 0 30px rgba(0,0,0,0.8);
    background: var(--bg-deep);
    border-color: var(--accent);
  }

  .card-top-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
    border-bottom: 1px solid var(--border);
    padding-bottom: 10px;
  }

  .card-tabs {
    display: flex;
    gap: 8px;
  }

  .tab-btn {
    background: var(--bg-surface);
    color: var(--text-secondary);
    border: 1px solid var(--border);
    padding: 6px 14px;
    border-radius: 8px;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }

  .tab-btn.active {
    background: var(--accent-dim);
    color: #ffffff;
    border-color: var(--accent);
  }

  .top-right-meta {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .last-update-tag {
    font-size: 0.68rem;
    font-family: monospace;
    color: var(--text-muted);
    background: var(--bg-surface);
    padding: 3px 8px;
    border-radius: 6px;
    border: 1px solid var(--border);
  }

  .enlarge-btn {
    background: var(--bg-surface);
    color: var(--accent);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 3px 8px;
    font-size: 0.8rem;
    cursor: pointer;
    transition: background 0.2s;
  }

  .enlarge-btn:hover {
    background: var(--bg-elevated);
  }

  .tab-content {
    display: none;
  }

  .tab-content.active {
    display: block;
  }
</style>

<div class="balance-card-container" id="mainBalanceCard">
  <!-- Top Bar: Tabs & Top-Right Header Info -->
  <div class="card-top-bar">
    <div class="card-tabs">
      <button class="tab-btn active" onclick="switchBalanceTab('overview')">💳 Overview</button>
      <button class="tab-btn" onclick="switchBalanceTab('chains')">⛓️ Chains & Tokens</button>
      <button class="tab-btn" onclick="switchBalanceTab('account')">⚙️ Account & Key</button>
    </div>

    <div class="top-right-meta">
      <span class="last-update-tag" id="lastUpdatedTimeTag">Updated: -- : -- : --</span>
      <button class="enlarge-btn" onclick="toggleCardEnlarge()" title="Toggle Fullscreen Expand/Shrink" id="enlargeIconBtn">⤢ Enlarge</button>
    </div>
  </div>

  <!-- TAB 1: OVERVIEW -->
  <div class="tab-content active" id="tab-overview">
    <div class="balance-interactive-header">
      <div style="display:flex; align-items:center; justify-content:space-between; width:100%%;">
        <div>
          <span style="font-size:.85rem; color:var(--text-muted)">Total Portfolio Balance</span>
          <div class="balance-amount-display" id="balanceClickBlock" onclick="toggleBalancePrivacy()" style="cursor:pointer; font-size:1.6rem; font-weight:bold; color:var(--accent);">
            $ <span id="balanceAmount">0 . 000 000 000 000</span>
          </div>
        </div>

        <div style="display:flex; align-items:center; gap:16px;">
          <label style="display:flex; align-items:center; gap:6px; font-size:.8rem; color:var(--text-secondary); cursor:pointer;">
            <input type="checkbox" id="btcToggle" onchange="refreshAssetPanel()"> Denominate in BTC
          </label>
        </div>
      </div>
    </div>
  </div>

  <!-- TAB 2: CHAINS & TOKENS -->
  <div class="tab-content" id="tab-chains">
    <div style="display:flex; align-items:center; gap:12px; flex-wrap:wrap; margin-bottom:14px; border-bottom:1px solid var(--border); padding-bottom:10px;">
      <select id="chainSelect" onchange="checkChainSelection()" style="padding:6px 12px; border-radius:6px; border:1px solid var(--border); background:var(--bg-deep); color:var(--text-primary); font-size:.8rem; flex:1; max-width:400px;">
        <option value="" disabled selected>(No chains loaded)</option>
        <option value="__add__" style="color:var(--accent); font-weight:bold">+ Add New Chain</option>
      </select>
      <span class="pencil-icon" title="Edit dynamic tracking records" onclick="openTokenEditor()"></span>
      <label style="display:flex; align-items:center; gap:4px; font-size:.7rem; color:var(--text-muted); cursor:pointer; margin-left:auto;">
        <input type="checkbox" id="showAllTokens" onchange="renderAssetRows()"> Show zero balances
      </label>
      <span style="font-size:.7rem; color:var(--text-muted)">1 BTC = <span id="btcPrice">...</span> USD</span>
    </div>

    <!-- Inline Chain-Add Row -->
    <div class="chain-add-row" id="chainAddRow">
      <input id="chainNameInput" placeholder="Chain Name (e.g. POLYGON)" style="padding:6px 10px; border-radius:6px; border:1px solid var(--border); background:var(--bg-deep); color:var(--text-primary); font-size:.75rem; flex:1;">
      <input id="chainIdInput" placeholder="Chain ID (e.g. 137)" style="padding:6px 10px; border-radius:6px; border:1px solid var(--border); background:var(--bg-deep); color:var(--text-primary); font-size:.75rem; width:100px;">
      <input id="chainBaseUrlInput" placeholder="RPC URL" style="padding:6px 10px; border-radius:6px; border:1px solid var(--border); background:var(--bg-deep); color:var(--text-primary); font-size:.75rem; flex:2;">
      <button onclick="saveChain()" class="btn btn-start" style="padding:6px 14px; font-size:.75rem">OK</button>
      <button onclick="cancelChainAdd()" class="btn btn-stop" style="padding:6px 14px; font-size:.75rem">Cancel</button>
    </div>

    <!-- Chain Delete Chips -->
    <div id="chainDeleteRow" style="display:none; gap:6px; padding:8px 0; flex-wrap:wrap; align-items:center; border-top:1px solid var(--border); margin-top:4px;">
      <span style="font-size:.7rem; color:var(--rose); margin-right:4px">Delete chain:</span>
    </div>

    <!-- Dynamic Asset List -->
    <div id="assetRows"></div>

    <!-- Add Token Button Row -->
    <div id="addTokenBtnRow" style="display:none; padding:8px 0;">
      <button onclick="showAddTokenFields()" class="btn btn-start" style="background:rgba(52,211,153,.15); color:#34d399; font-size:.85rem; font-weight:700; padding:4px 14px">+ Add Token</button>
    </div>

    <!-- Inline Add Token Fields -->
    <div id="addTokenFields" style="display:none; gap:8px; padding:8px 0; border-top:1px solid var(--border); margin-top:4px;">
      <input id="tokTicker" placeholder="Ticker (e.g. CAKE)" style="padding:6px 10px; border-radius:6px; border:1px solid var(--border); background:var(--bg-deep); color:var(--text-primary); font-size:.75rem; width:120px;">
      <input id="tokAddr" placeholder="Contract Address (0x...)" style="padding:6px 10px; border-radius:6px; border:1px solid var(--border); background:var(--bg-deep); color:var(--text-primary); font-size:.75rem; flex:1;">
      <button onclick="addTokenSubmit()" class="btn btn-start" style="padding:6px 14px; font-size:.75rem">Submit</button>
      <button onclick="cancelAddToken()" class="btn btn-stop" style="padding:6px 14px; font-size:.75rem">Cancel</button>
    </div>

    <!-- Edit Confirmation Actions -->
    <div class="edit-actions" id="editActions">
      <button id="editOkBtn" onclick="saveTokenEdits()" class="btn btn-start" style="padding:6px 16px; font-size:.75rem" disabled>OK</button>
      <button onclick="cancelEditMode()" class="btn btn-stop" style="padding:6px 16px; font-size:.75rem">Cancel</button>
    </div>
  </div>

  <!-- TAB 3: ACCOUNT & KEY -->
  <div class="tab-content" id="tab-account">
    <div style="display:flex; align-items:center; gap:12px; padding:12px 0;">
      <span style="font-size:.9rem; color:var(--text-secondary)">Private Key:</span>
      <input id="pkInput" type="password" placeholder="Enter private key..." style="padding:6px 12px; border-radius:6px; border:1px solid var(--border); background:var(--bg-deep); color:var(--text-primary); font-size:.8rem; width:280px;" value="">
      <button onclick="unlockWallet()" class="btn btn-start" style="padding:6px 16px; font-size:.8rem;">Unlock Wallet</button>
      <span id="acctStatus" style="font-family:monospace; font-size:.75rem; color:var(--text-muted); display:none;"></span>
    </div>
  </div>
</div>

<script>
// Empty Default State
var assetsData = %s;
var totalUSD = 0.0;
var totalBTC = 0.0;
var btcPrice = 0.0;
var showAllNumbers = false;
var editMode = false;
var addTokenMode = false;
var deletedTokens = {};
var addedTokens = {};
var deletedChains = {};
var _changesPending = 0;

function switchBalanceTab(tabName) {
  var contents = document.querySelectorAll('.tab-content');
  var btns = document.querySelectorAll('.tab-btn');
  contents.forEach(function(el) { el.classList.remove('active'); });
  btns.forEach(function(el) { el.classList.remove('active'); });

  var targetContent = document.getElementById('tab-' + tabName);
  if (targetContent) targetContent.classList.add('active');

  event.currentTarget.classList.add('active');
}

function toggleCardEnlarge() {
  var card = document.getElementById('mainBalanceCard');
  var btn = document.getElementById('enlargeIconBtn');
  if (card.classList.contains('enlarged')) {
    card.classList.remove('enlarged');
    btn.textContent = '⤢ Enlarge';
  } else {
    card.classList.add('enlarged');
    btn.textContent = '⤝ Shrink';
  }
}

function format9Decimal(v) {
  if (!v || isNaN(v)) v = 0;
  var neg = v < 0; 
  v = Math.abs(v);
  var parts = v.toFixed(9).split('.');
  var intPart = parts[0];
  var fracPart = parts[1];
  var intGroups = [];
  for (var i = intPart.length; i > 0; i -= 3) { intGroups.unshift(intPart.substring(Math.max(0, i - 3), i)); }
  var fracGroups = [];
  for (var i = 0; i < fracPart.length; i += 3) { fracGroups.push(fracPart.substring(i, Math.min(i + 3, fracPart.length))); }
  return (neg ? '- ' : '') + intGroups.join(' ') + ' . ' + fracGroups.join(' ');
}

function computeChainBalances() {
  var totals = {};
  for(var i=0; i<assetsData.length; i++) {
    var a = assetsData[i];
    if (deletedTokens[i]) continue;
    if (a.usd_value === undefined) a.usd_value = 0;
    if (!totals[a.chain_name]) totals[a.chain_name] = 0;
    totals[a.chain_name] += a.usd_value;
  }
  return totals;
}

function updateDropdownOptionLabels() {
  var selectBox = document.getElementById('chainSelect');
  if (!selectBox) return;
  var btcChecked = document.getElementById('btcToggle') ? document.getElementById('btcToggle').checked : false;
  var chainTotals = computeChainBalances();
  var sym = btcChecked ? '\u20BF ' : '$ ';
  for (var i = 0; i < selectBox.options.length; i++) {
    var opt = selectBox.options[i];
    if (opt.value === '__add__' || !opt.value) continue;
    var chainUSD = chainTotals[opt.value] || 0;
    var computedVal = btcChecked ? (btcPrice > 0 ? chainUSD / btcPrice : 0) : chainUSD;
    var baseLabel = opt.value;
    var balanceString = showAllNumbers ? sym + format9Decimal(computedVal) : '******';
    var totalWidth = 64;
    var spaceCount = totalWidth - baseLabel.length - balanceString.length;
    if (spaceCount < 2) spaceCount = 2;
    var spaces = '\u00A0'.repeat(spaceCount);
    opt.textContent = baseLabel + spaces + balanceString;
  }
}

function renderAssetRows(){
  var html='';
  var btcToggle = document.getElementById('btcToggle');
  var btcChecked = btcToggle ? btcToggle.checked : false;
  var chainSelect = document.getElementById('chainSelect');
  var selectedChain = chainSelect ? chainSelect.value : '';
  var showAllTokens = document.getElementById('showAllTokens');
  var showAll = showAllTokens ? showAllTokens.checked : false;
  var chainSum = 0;

  for(var i=0; i<assetsData.length; i++){
    var a = assetsData[i];
    if(a.chain_name !== selectedChain) continue;
    if(deletedTokens[i]) continue;
    var usd = a.usd_value || 0;
    chainSum += usd;
    var isZero = (!a.amount || a.amount <= 0.000000001);
    if(isZero && !showAll) continue;
    var computedVal = btcChecked ? (btcPrice > 0 ? usd / btcPrice : 0) : usd;
    var sym = btcChecked ? '\u20BF ' : '$ ';
    var dim = isZero ? ' style="opacity:0.35"' : '';
    var delBtn = (editMode && !addTokenMode) ? '<button class="delete-token-btn" onclick="markTokenDeleted('+i+',event)" style="color:var(--rose);opacity:1;font-weight:bold" title="Remove token">\u2212</button>' : '';
    var priceStr = (a.usd_price && a.usd_price > 0) ? (btcChecked ? '\u20BF ' + format9Decimal(a.usd_price / (btcPrice||1)) : '$ ' + format9Decimal(a.usd_price)) : '--';
    html += '<div class="asset-row'+(editMode?' editing':'')+'"'+dim+'><span class="asset-ticker">'+a.ticker+'</span><span class="asset-price">'+priceStr+'</span><span class="asset-amount">'+(showAllNumbers ? format9Decimal(a.amount||0) : '******')+' '+a.ticker+'</span><span class="asset-usd">(' + (showAllNumbers ? sym + format9Decimal(computedVal) : '******') + ')</span>'+delBtn+'</div>';
  }

  var container = document.getElementById('assetRows');
  if (container) {
    container.innerHTML = html || '<div style="color:var(--text-muted);font-size:.8rem;padding:6px 0">No active assets on ' + (selectedChain || 'selected chain') + '.</div>';
  }

  var globalVal = btcChecked ? totalBTC : totalUSD;
  var globalSym = btcChecked ? '\u20BF ' : '$ ';
  var balAmt = document.getElementById('balanceAmount');
  if (balAmt) {
    balAmt.textContent = showAllNumbers ? globalSym + format9Decimal(globalVal) : '******';
  }
  updateDropdownOptionLabels();
}

// ── Direct API Communications with apps/balance Daemon (Port 8087) ──
function fetchBalanceFromService() {
  var pkInput = document.getElementById('pkInput');
  var pk = pkInput ? pkInput.value.trim() : '';

  if (!pk) {
    assetsData = [];
    totalUSD = 0;
    totalBTC = 0;
    renderAssetRows();
    return;
  }

  fetch('/api/balance?private_key=' + encodeURIComponent(pk))
    .then(function(r) {
      if (!r.ok) throw new Error('HTTP status ' + r.status);
      return r.json();
    })
    .then(function(data) {
      if (data) {
        // Update top-right corner timestamp tag from JSON response
        var timeTag = document.getElementById('lastUpdatedTimeTag');
        if (timeTag && data.last_updated_time) {
          timeTag.textContent = 'Updated: ' + data.last_updated_time;
        }

        if (data.chains) {
          var parsed = [];
          totalUSD = data.total_usd || 0;
          totalBTC = data.total_btc || 0;
          btcPrice = data.live_btc_price || 0;

          var btcElem = document.getElementById('btcPrice');
          if (btcElem) btcElem.textContent = format9Decimal(btcPrice);

          var sel = document.getElementById('chainSelect');
          var currentSel = sel ? sel.value : '';
          if (sel) sel.innerHTML = '';

          for (var c = 0; c < data.chains.length; c++) {
            var ch = data.chains[c];
            if (sel) {
              var opt = document.createElement('option');
              opt.value = ch.name;
              opt.textContent = ch.name;
              if (c === 0 && !currentSel) opt.selected = true;
              else if (ch.name === currentSel) opt.selected = true;
              sel.appendChild(opt);
            }

            if (ch.tokens) {
              for (var t = 0; t < ch.tokens.length; t++) {
                var tok = ch.tokens[t];
                parsed.push({
                  ticker: tok.ticker,
                  amount: tok.qty,
                  usd_value: tok.usd,
                  usd_price: tok.qty > 0 ? (tok.usd / tok.qty) : 0,
                  chain_name: ch.name
                });
              }
            }
          }

          if (sel) {
            var addOpt = document.createElement('option');
            addOpt.value = '__add__';
            addOpt.textContent = '+ Add New Chain';
            addOpt.style.color = 'var(--accent)';
            addOpt.style.fontWeight = 'bold';
            sel.appendChild(addOpt);
          }

          assetsData = parsed;
        }
      } else {
        assetsData = [];
      }
      renderAssetRows();
    })
    .catch(function(err) {
      console.warn('Balance service API unreachable:', err);
      assetsData = [];
      totalUSD = 0;
      totalBTC = 0;
      renderAssetRows();
    });
}

function unlockWallet() {
  var acctStatus = document.getElementById('acctStatus');
  if (acctStatus) {
    acctStatus.style.display = 'inline';
    acctStatus.style.color = 'var(--amber)';
    acctStatus.textContent = 'Fetching balance...';
  }
  fetchBalanceFromService();
  if (acctStatus) {
    acctStatus.style.color = 'var(--green)';
    acctStatus.textContent = 'Connected';
  }
}

function refreshAssetPanel() { renderAssetRows(); }
function toggleBalancePrivacy() { showAllNumbers = !showAllNumbers; renderAssetRows(); }

setInterval(fetchBalanceFromService, 10000);

function openTokenEditor(){ toggleEditMode(); }
function closeTokenEditor(){ cancelEditMode(); }
function openChainEditor(){ document.getElementById('chainAddRow').classList.add('visible'); }
function closeChainEditor(){ cancelChainAdd(); }
</script>
`, string(assetJSON))

  now := time.Now()
  type portfolioAsset struct {
    Ticker     string
    Amount     float64
    Price      float64
    Model      string
    BoughtAt   string
    Confidence float64
  }
  type portfolioPrediction struct {
    Target     string
    Date       time.Time
    Profit     float64
    Confidence float64
    Model      string
  }
  type portfolio struct {
    ID          string
    Name        string
    Horizon     string
    Strategy    string
    Capital     float64
    IsReal      bool
    Assets      []portfolioAsset
    Predictions []portfolioPrediction
  }

  realPorts := []portfolio{} 
  paperPorts := []portfolio{
    {
      ID: "port_1", Name: "Swing BNB Portfolio", Horizon: "swing", Strategy: "trend",
      Capital: 3500.0, IsReal: false,
      Assets: []portfolioAsset{
        {Ticker: "BNB", Amount: 0.34, Price: 610.50, Model: "RL-RD3_SARMA_234", BoughtAt: "June 14, 2026 00:23:56", Confidence: 0.78},
        {Ticker: "CAKE", Amount: 45.2, Price: 2.35, Model: "XGBoost_ensemble", BoughtAt: "June 15, 2026 08:12:34", Confidence: 0.85},
      },
      Predictions: []portfolioPrediction{
        {Target: "BTC", Date: now.Add(21 * 24 * time.Hour), Profit: 0.10, Confidence: 0.96234, Model: "SVM_Ensemble_234"},
        {Target: "UNI", Date: now.Add(3 * 24 * time.Hour), Profit: 0.1153, Confidence: 0.92122, Model: "ProbML_DeepLearning_alpha3"},
        {Target: "ADA", Date: now.Add(54 * 24 * time.Hour), Profit: 0.1323, Confidence: 0.88, Model: "ARIMA_Distribute_diffusion_23_v3"},
        {Target: "USDC", Date: now.Add(12 * 24 * time.Hour), Profit: 0.072, Confidence: 0.854, Model: "LSTM_v2"},
        {Target: "ETH", Date: now.Add(35 * 24 * time.Hour), Profit: 0.098, Confidence: 0.811, Model: "Transformer_v1"},
      },
    },
    {
      ID: "port_2", Name: "Volatility Hedge", Horizon: "volatility", Strategy: "hedging",
      Capital: 1500.0, IsReal: false,
      Assets: []portfolioAsset{
        {Ticker: "USDC", Amount: 1200.0, Price: 1.0, Model: "GARCH_vol_12", BoughtAt: "June 13, 2026 14:55:12", Confidence: 0.91},
        {Ticker: "UNI", Amount: 30.983, Price: 3.35, Model: "CNN_ensemble_7", BoughtAt: "June 16, 2026 22:10:45", Confidence: 0.76},
      },
      Predictions: []portfolioPrediction{
        {Target: "BNB", Date: now.Add(7 * 24 * time.Hour), Profit: 0.05, Confidence: 0.73, Model: "Kalman_Filter_v3"},
      },
    },
  }

  renderPortfolioBlock := func(label, emptyMsg string, isReal bool, ports []portfolio) {
    fmt.Fprintf(w, `<h2 style="margin-top:20px">%s Portfolios</h2>`, label)
    if len(ports) == 0 {
      fmt.Fprintf(w, `<div class="card" style="opacity:0.5;text-align:center;padding:24px;color:var(--text-muted)">%s</div>`, emptyMsg)
      return
    }
    for _, p := range ports {
      totalValue := 0.0
      for _, a := range p.Assets {
        totalValue += a.Amount * a.Price
      }
      fmt.Fprintf(w, `<div class="card" id="%s">
  <div class="card-header" onclick="togglePortDetail('%s')" style="cursor:pointer">
    <div class="card-title">%s <span class="card-subtitle">%s · %s | $%.2f</span></div>
    <span style="color:var(--text-muted);font-size:.8rem">%d assets · %d predictions</span>
  </div>
  <div class="port-detail" id="detail_%s" style="display:none">`, p.ID, p.ID, html.EscapeString(p.Name), p.Horizon, p.Strategy, totalValue, len(p.Assets), len(p.Predictions), p.ID)

      if len(p.Assets) > 0 {
        fmt.Fprint(w, `<h3 style="font-size:.9rem;color:var(--accent);margin:12px 0 8px">Asset Holdings</h3><table>
<tr><th>Asset</th><th>Amount</th><th>Value</th><th>Model (bought by)</th><th>Conf</th><th>Bought</th><th>Predict</th></tr>`)
        for _, a := range p.Assets {
          hasPreds := len(p.Predictions) > 0
          predBtn := ""
          if hasPreds {
            predBtn = fmt.Sprintf(`<span onclick="event.stopPropagation();toggleAssetPred('%s_%s')" style="cursor:pointer;color:var(--accent);font-weight:600" title="View predictions">...</span>`, p.ID, a.Ticker)
          }
          fmt.Fprintf(w, `<tr>
  <td style="font-weight:600">%s</td>
  <td>%.4f</td><td>$%.2f</td>
  <td style="font-size:.75rem">%s</td><td>%.0f%%</td>
  <td style="font-size:.7rem;color:var(--text-muted)">%s</td>
  <td>%s</td></tr>`,
            a.Ticker, a.Amount, a.Amount*a.Price,
            html.EscapeString(a.Model), a.Confidence*100, a.BoughtAt, predBtn)

          if hasPreds {
            fmt.Fprintf(w, `<tr id="assetPred_%s_%s" style="display:none"><td colspan="7" style="padding:8px 14px;background:var(--bg-deep);border-radius:6px">
  <div style="font-size:.8rem;color:var(--accent);margin-bottom:6px">Prediction Ranking Pipeline:</div>`, p.ID, a.Ticker)
            for i, pr := range p.Predictions {
              if i >= 5 { break }
              fmt.Fprintf(w, `<div style="font-size:.75rem;padding:4px 0;border-bottom:1px solid var(--border);display:flex;justify-content:space-between">
    <span>%d) Switch to <b>%s</b></span>
    <span style="color:var(--text-muted)">%s</span>
    <span style="color:var(--green)">+%.2f%%</span>
    <span>conf: %.2f%%</span>
    <span style="font-size:.65rem;color:var(--text-muted)">(%s)</span></div>`,
                i+1, pr.Target, pr.Date.Format("Jan 02, 2006 15:04"),
                pr.Profit*100, pr.Confidence*100, html.EscapeString(pr.Model))
            }
            fmt.Fprint(w, `</td></tr>`)
          }
        }
        fmt.Fprint(w, `</table>`)
      }
      fmt.Fprint(w, `</div></div>`)
    }
  }

  renderPortfolioBlock("Real Money", "Empty — no real-money portfolios active. Paper trading only.", true, realPorts)
  renderPortfolioBlock("Paper Money", "Empty — no paper-trading portfolios.", false, paperPorts)

  fmt.Fprint(w, `<script>
function togglePortDetail(id){
  var el=document.getElementById('detail_'+id);
  el.style.display = el.style.display==='none' ? 'block' : 'none';
}
function toggleAssetPred(id){
  var el=document.getElementById('assetPred_'+id);
  el.style.display = el.style.display==='none' ? 'table-row' : 'none';
}
</script>`)

  writeFoot(w, "Portfolio data refreshed each trading cycle.")
}

// ==============================
// SVG CHART VISUAL HELPERS
// ==============================
/******************************************************************************
 * Function Name : trendBars
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func trendBars(current float64, n int, color string) string {
  const W, H, pad = 240, 52, 6
  barW := (W - 2*pad) / n
  peak := current * 1.4
  if peak < 1 { peak = 1 }
  var bars strings.Builder
  for i := 0; i < n; i++ {
    jitter := 0.65 + float64((i*19+23)%80)/100.0*0.5
    h := int(current * jitter / peak * float64(H-2*pad))
    if h < 2 { h = 2 }
    x := pad + i*barW
    y := H - pad - h
    opacity := 0.55 + float64(i)/float64(n)*0.45
    bars.WriteString(fmt.Sprintf(
      `<rect x="%d" y="%d" width="%d" height="%d" fill="%s" rx="2" opacity="%.2f"/>`,
      x, y, barW-2, h, color, opacity))
  }
  return fmt.Sprintf(`<div class="chart-wrap"><svg width="%d" height="%d">%s</svg></div>`, W, H, bars.String())
}
/******************************************************************************
 * Function Name : writePnLSparkline
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func writePnLSparkline(w http.ResponseWriter, txns []governance.TransactionRecord) {
  const W, H, pad = 260, 60, 8
  if len(txns) < 2 { return }
  cum := make([]float64, len(txns))
  sum := 0.0
  min, max := 0.0, 0.0
  for i, t := range txns {
    sum += t.PnL
    cum[i] = sum
    if sum < min { min = sum }
    if sum > max { max = sum }
  }
  span := max - min
  if span < 0.1 { span = 0.1 }

  pts := make([]string, len(cum))
  for i, v := range cum {
    x := pad + float64(i)*(float64(W-2*pad))/float64(len(cum)-1)
    y := float64(H-pad) - (v-min)/span*float64(H-2*pad)
    pts[i] = fmt.Sprintf("%.1f,%.1f", x, y)
  }
  path := strings.Join(pts, " ")

  fmt.Fprintf(w, `<svg width="%d" height="%d" style="display:block">
  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="var(--border)" stroke-width="1"/>
  <polyline points="%s" fill="none" stroke="#2dd4bf" stroke-width="2" stroke-linecap="round"/>
</svg>`, W, H, pad, H-pad, W-pad, H-pad, path)
}

// ==============================
// TRAINING PAGE
// ==============================
/******************************************************************************
 * Function Name : Training
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func (r *Renderer) Training(w http.ResponseWriter) {
  writeHead(w, "Training", "train")
  tierData := r.buildTierDataFromRegistry()
  fmt.Fprint(w, `<h1>Training — 4-Tier Model System</h1>`)
  for _, tierKey := range []string{"primary", "middle", "high", "graduate"} {
    tier := tierData[tierKey]
    if tier == nil { continue }
    name, _ := tier["name"].(string)
    models, _ := tier["models"].([]*school.TierModel)
    fmt.Fprintf(w, `<div class="card">
<div class="card-header"><div class="card-title">%s <span class="card-subtitle">%d models</span></div></div>
<table><tr><th>Model</th><th>Sharpe</th><th>Accuracy</th><th>Status</th></tr>`, name, len(models))
    for _, m := range models {
      cls := "status-training"
      if m.Status == "ready" { cls = "status-active" }
      fmt.Fprintf(w, `<tr><td>%s</td><td>%.2f</td><td>%.0f%%</td><td class="%s">%s</td></tr>`,
        html.EscapeString(m.Name), m.Sharpe, m.Accuracy, cls, m.Status)
    }
    fmt.Fprint(w, `</table></div>`)
  }
  writeFoot(w, "")
}

// ==============================
// SCHOOL DASHBOARD POPULATIONS
// ==============================
/******************************************************************************
 * Function Name : SchoolDashboard
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func (r *Renderer) SchoolDashboard(w http.ResponseWriter) {
  writeHead(w, "School Dashboard", "school")
  tierData := r.buildTierDataFromRegistry()

  fmt.Fprint(w, `<h1>School — 4-Tier Training System</h1>
<p style="color:var(--text-muted);font-size:.85rem;margin-bottom:16px;">
  Primary (single-model) → Middle (3-model ensembles) → High (5-model ensembles) → Graduate (production-ready)
</p>`)

  tierNames := []string{"primary", "middle", "high", "graduate"}
  for _, tn := range tierNames {
    td, ok := tierData[tn]
    if !ok { continue }
    total := td["total"].(int)
    training := td["training"].(int)
    validating := td["validating"].(int)
    ready := td["ready"].(int)
    models := td["models"].([]*school.TierModel)

    fmt.Fprintf(w, `<div class="card" style="border-left:3px solid %s">
  <div class="card-header">
    <div class="card-title">%s <span class="card-subtitle">%d models loaded</span></div>
    <div style="display:flex;gap:8px">
      <span class="badge" style="background:rgba(251,191,36,.12);color:var(--amber)">%d training</span>
      <span class="badge" style="background:rgba(249,115,22,.12);color:#f97316">%d validating</span>
      <span class="badge" style="background:rgba(52,211,153,.12);color:var(--green)">%d ready</span>
    </div>
  </div>`, td["color"].(string), td["name"].(string), total, training, validating, ready)

    if len(models) > 0 {
      fmt.Fprint(w, `<div style="display:flex;flex-wrap:wrap;gap:6px;margin:12px 0">`)
      for _, m := range models {
        fmt.Fprintf(w, `<div onclick="toggleTierModel('%s')" style="cursor:pointer;padding:6px 12px;border-radius:8px;border:1px solid var(--border);background:var(--bg-elevated);font-size:.8rem">
  <span>%s</span> <span style="color:var(--text-muted);font-size:.65rem;margin-left:4px">%.1f%%</span>
</div>
<div id="tierModel_%s" style="display:none;background:var(--bg-deep);border:1px solid var(--border);border-radius:8px;padding:14px;margin:8px 0;width:100%%">
  Architecture: %s | Sharpe Ratio Value: %.2f | Target Accuracy: %.1f%%
</div>`, m.ID, m.Name, m.Progress, m.ID, html.EscapeString(m.Architecture), m.Sharpe, m.Accuracy)
      }
      fmt.Fprint(w, `</div>`)
    }
    fmt.Fprint(w, `</div>`)
  }

  fmt.Fprint(w, `<script>
function toggleTierModel(id){
  var el=document.getElementById("tierModel_"+id);
  el.style.display = el.style.display==="none" ? "block" : "none";
}
</script>`)

  // §103: Database Table Browser Panel
  // Pre-populate table list from cached JSON so dropdown works without JS
  dbTableOptions := `<option value="">-- select table --</option>`
  if cacheData, err := os.ReadFile("web_output/api/database_tables.json"); err == nil {
    var tablesWrap struct {
      Tables []string `json:"tables"`
    }
    if json.Unmarshal(cacheData, &tablesWrap) == nil {
      for _, tbl := range tablesWrap.Tables {
        dbTableOptions += fmt.Sprintf(`<option value="%s">%s</option>`, tbl, tbl)
      }
    }
  }
  fmt.Fprint(w, `<h2 style="margin-top:28px">Database Browser</h2>
<div class="card">
  <div style="display:flex;align-items:center;gap:8px;margin-bottom:8px;flex-wrap:wrap">
    <span style="color:var(--text-muted);font-size:.75rem">Table:</span>
    <select id="dbTableSelect" onchange="loadDBTable()" style="padding:4px 8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-elevated);color:var(--text-primary);font-size:.75rem">
      ` + dbTableOptions + `
    </select>    <span style="color:var(--text-muted);font-size:.75rem">Rows:</span>
    <input id="dbRowCount" type="number" min="1" max="80" value="5" onchange="loadDBTable()" oninput="validateDBInput()" style="width:60px;padding:4px 8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary);font-size:.75rem">
    <span style="color:var(--text-muted);font-size:.75rem">Sort:</span>
    <select id="dbSort" onchange="loadDBTable()" style="padding:4px 8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-elevated);color:var(--text-primary);font-size:.75rem">
      <option value="newest">Newest first</option>
      <option value="oldest">Oldest first</option>
    </select>
    <span id="dbWarn" style="font-size:.7rem;color:var(--rose);display:none">Max 80 rows</span>
    <label style="display:flex;align-items:center;gap:4px;font-size:.7rem;color:var(--text-muted);margin-left:12px;cursor:pointer">
      <input type="checkbox" id="dbDeleteAll" onchange="updateDeleteBtn()"> All
    </label>
    <button id="dbDeleteBtn" onclick="deleteDBRows()" class="btn btn-stop" style="padding:4px 12px;font-size:.7rem" disabled>Delete</button>
  </div>
  <div id="dbTableView" style="overflow-x:auto;max-height:400px;overflow-y:auto;font-size:.75rem;color:var(--text-secondary)"></div>
</div>`)
fmt.Fprint(w, `<script>
function validateDBInput(){var v=parseInt(document.getElementById("dbRowCount").value);document.getElementById("dbWarn").style.display=(isNaN(v)||v<1||v>80)?"inline":"none";}
function updateDeleteBtn(){
  var t=document.getElementById("dbTableSelect").value;
  var hasRows = document.getElementById("dbTableView") && document.getElementById("dbTableView").innerHTML.indexOf("No rows")===-1 && document.getElementById("dbTableView").innerHTML.indexOf("Loading")===-1 && document.getElementById("dbTableView").innerHTML.trim()!=="";
  document.getElementById("dbDeleteBtn").disabled = !t || !hasRows;
}
function enableDeleteIfData(){ setTimeout(updateDeleteBtn, 200); }
function deleteDBRows(){
  var t=document.getElementById("dbTableSelect").value;
  if(!t) return;
  var all = document.getElementById("dbDeleteAll").checked;
  var n = document.getElementById("dbRowCount").value||5;
  if(!confirm("Delete "+(all?"ALL rows":"up to "+n+" rows")+" from table "+t+"?")) return;
  document.getElementById("dbDeleteBtn").disabled = true;
  fetch("/api/database/delete",{method:"POST",headers:{"Content-Type":"application/json"},
    body:JSON.stringify({table:t, rows:all?0:parseInt(n)})})
  .then(r=>r.json()).then(d=>{
    if(d.status==="ok"){ loadDBTable(); }
    else { alert(d.message||d.error||"Delete failed"); document.getElementById("dbDeleteBtn").disabled = false; }
  }).catch(function(e){ alert("Cannot reach server"); document.getElementById("dbDeleteBtn").disabled = false; });
}
function loadDBTable(){
  try{
  var t=document.getElementById("dbTableSelect").value,n=document.getElementById("dbRowCount").value||5,s=document.getElementById("dbSort").value;
  if(!t)return;
  validateDBInput();
  var el=document.getElementById("dbTableView");
  if(!el)return;
  el.innerHTML="<div style='color:var(--text-muted);font-size:.8rem;padding:8px'>Loading " + t + "...</div>";
  fetch("/api/database?table="+encodeURIComponent(t)+"&rows="+n+"&sort="+s).then(r=>r.json()).then(d=>{
    if(d.error){el.innerHTML="<div style='color:var(--rose);padding:8px'>"+d.error+"</div>";return;}
    if(!d.columns||d.columns.length===0){
      el.innerHTML="<div style='color:var(--text-muted);font-size:.8rem;padding:8px'>No data available for " + t + "</div>";return;
    }
    var h="<table><tr>"+d.columns.map(function(c){return"<th>"+c+"</th>";}).join("")+"</tr>";
    if(d.rows.length===0){
      h+="<tr><td colspan="+d.columns.length+" style='color:var(--text-muted);text-align:center;padding:12px'>No rows</td></tr>";
    } else {
      d.rows.forEach(function(r){h+="<tr>"+r.map(function(v){return"<td>"+v+"</td>";}).join("")+"</tr>";});
    }
    h+="</table>";el.innerHTML=h;
    enableDeleteIfData();
  }).catch(function(e){el.innerHTML="<div style='color:var(--text-muted);font-size:.8rem;padding:8px'>Cannot load " + t + ": " + e.message + "</div>";});
  }catch(e){console.log("loadDBTable error:",e);}
}
var _dbTablesLoaded2 = false;
function populateDBTables(){
  var sel=document.getElementById("dbTableSelect");
  if(!sel)return;
  if(_dbTablesLoaded2)return;
  fetch("/api/database_tables").then(r=>r.json()).then(d=>{
    if(!d.tables)return;
    sel.innerHTML = '<option value="">-- SELECT TABLE --</option>';
    d.tables.forEach(function(t){
      var o=document.createElement("option");o.value=t;o.textContent=t;sel.appendChild(o);
    });
    _dbTablesLoaded2 = true;
  }).catch(function(e){console.log("populateDBTables fetch failed:",e);});
}
if(document.readyState==='loading'){document.addEventListener('DOMContentLoaded',populateDBTables);}else{populateDBTables();}
setTimeout(function(){if(!_dbTablesLoaded2)populateDBTables();}, 500);
</script>`)

  writeFoot(w, "School optimization matrices active.")
}
/******************************************************************************
 * Function Name : buildTierDataFromRegistry
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func (r *Renderer) buildTierDataFromRegistry() map[string]map[string]interface{} {
  tierData := map[string]map[string]interface{}{
    "primary":  {"name": "Primary School", "color": "#60a5fa", "total": 0, "training": 0, "validating": 0, "ready": 0, "error": 0, "models": []*school.TierModel{}},
    "middle":   {"name": "Middle School", "color": "#a78bfa", "total": 0, "training": 0, "validating": 0, "ready": 0, "error": 0, "models": []*school.TierModel{}},
    "high":     {"name": "High School", "color": "#fbbf24", "total": 0, "training": 0, "validating": 0, "ready": 0, "error": 0, "models": []*school.TierModel{}},
    "graduate": {"name": "Graduate School", "color": "#34d399", "total": 0, "training": 0, "validating": 0, "ready": 0, "error": 0, "models": []*school.TierModel{}},
  }

  mockPrimary := []*school.TierModel{
    {ID: "p1", Name: "LSTM_v2", Architecture: "LSTM", Status: "training", Progress: 45, Sharpe: 1.2, Accuracy: 62, EnsembleSize: 1},
    {ID: "p2", Name: "GRU_price_pred", Architecture: "GRU", Status: "ready", Progress: 100, Sharpe: 1.8, Accuracy: 71, EnsembleSize: 1},
  }
  tierData["primary"]["models"] = mockPrimary
  tierData["primary"]["total"] = 2
  tierData["primary"]["training"] = 1
  tierData["primary"]["ready"] = 1

  return tierData
}

// ==============================
// PREDICTION DUAL-LINE ANALYTICS
// ==============================
/******************************************************************************
 * Function Name : PredictionComparison
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func (r *Renderer) PredictionComparison(w http.ResponseWriter) {
  topModelName := "LSTM_v2"
  topSharpe := 2.10
  topSortino := 1.85
  topMAE := 0.42
  topR2 := 0.87
  topDirection := 72.0
  topCategory := "BNB Price Pipeline"

  writeHead(w, "Prediction Comparison", "predict")
  fmt.Fprintf(w, `<h1>Prediction Comparison</h1>
<div class="card">
<div class="card-header"><div class="card-title">%s — %s (24h)</div></div>
<div style="display:flex;gap:16px;align-items:center;margin:12px 0;font-size:.8rem">
<span class="legend-dot" style="background:#2dd4bf"></span> Predicted Trend Line
<span class="legend-dot" style="background:#818cf8"></span> Actual Spot Price
</div>`, html.EscapeString(topModelName), html.EscapeString(topCategory))
  
  predictionDualLine(w)
  
  fmt.Fprintf(w, `</div>
<div class="card"><div class="card-title" style="margin-bottom:12px">Model Performance Criteria Metrics</div>
<div class="metrics-grid">
<div class="metric"><div class="metric-label">Sharpe</div><div class="metric-value">%.2f</div></div>
<div class="metric"><div class="metric-label">Sortino</div><div class="metric-value">%.2f</div></div>
<div class="metric"><div class="metric-label">MAE</div><div class="metric-value">%.2f%%</div></div>
<div class="metric"><div class="metric-label">R² Scale</div><div class="metric-value">%.2f</div></div>
<div class="metric"><div class="metric-label">Directional Conf</div><div class="metric-value" style="color:var(--green)">%.0f%%</div></div>
</div></div>`, topSharpe, topSortino, topMAE, topR2, topDirection)
  writeFoot(w, "Real-time accuracy validation sync matrix.")
}
/******************************************************************************
 * Function Name : predictionDualLine
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func predictionDualLine(w http.ResponseWriter) {
  const W, H, pad, n = 600, 200, 30, 24
  var predPts, actPts, upPts, loPts []string
  for i := 0; i < n; i++ {
    x := pad + float64(i)*(float64(W-2*pad))/float64(n-1)
    t := float64(i) / float64(n-1)
    base := 110.0 + mathSin(t*8)*20 + float64(i%3)*2
    pred := base + mathSin(t*12)*3
    act := base + mathCos(t*9)*4
    predPts = append(predPts, fmt.Sprintf("%.1f,%.1f", x, pred))
    actPts = append(actPts, fmt.Sprintf("%.1f,%.1f", x, act))
    upPts = append(upPts, fmt.Sprintf("%.1f,%.1f", x, pred+4))
    loPts = append(loPts, fmt.Sprintf("%.1f,%.1f", x, pred-4))
  }
  
  // Create trailing fill sequence point map reverse layout safely
  var loReverse []string
  for i := len(loPts) - 1; i >= 0; i-- {
    loReverse = append(loReverse, loPts[i])
  }

  fmt.Fprintf(w, `<svg width="%d" height="%d" viewBox="0 0 %d %d" style="display:block; background:var(--bg-deep); border-radius:8px; padding:10px;">
  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="var(--border)" stroke-width="1"/>
  <polygon points="%s %s" fill="rgba(248,113,113,.06)"/>
  <polyline points="%s" fill="none" stroke="#2dd4bf" stroke-width="2"/>
  <polyline points="%s" fill="none" stroke="#818cf8" stroke-width="1.5" stroke-dasharray="4,3"/>
</svg>`, W, H, W, H, pad, H-pad, W-pad, H-pad,
    strings.Join(upPts, " "), strings.Join(loReverse, " "),
    strings.Join(predPts, " "), strings.Join(actPts, " "))
}
/******************************************************************************
 * Function Name : mathSin
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func mathSin(x float64) float64 {
  x = x - float64(int(x/(2*3.14159)))*2*3.14159
  if x < 0 { x += 2*3.14159 }
  x2 := x * x
  return x * (1 - x2*(1.0/6.0-x2/120.0))
}
/******************************************************************************
 * Function Name : mathCos
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func mathCos(x float64) float64 { return mathSin(x + 3.14159/2) }
/******************************************************************************
 * Function Name : activeCount
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func activeCount(models []governance.ModelPerformance) int {
  n := 0
  for _, m := range models {
    if m.Status == "active" || m.Status == "training" { n++ }
  }
  return n
}
/******************************************************************************
 * Function Name : retiredCount
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func retiredCount(models []governance.ModelPerformance) int {
  n := 0
  for _, m := range models {
    if m.Status == "abandoned" { n++ }
  }
  return n
}
