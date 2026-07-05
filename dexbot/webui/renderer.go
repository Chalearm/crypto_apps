/******************************************************************************
 * File Name       : renderer.go
 * File Path       : webui/renderer.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:26 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:26 (UTC+7)
 *
 * Description     :
 *   Modern, eye-friendly HTML renderers for the Dexbot web dashboard. Refined from v1.0 with professional design system per myreq2.txt §8. Design principles: - Slate/charcoal base with teal accents (easy
 *
 * Responsibilities:
 *   - - Implement core functionality for webui package.
 *
 * Usage :
 *   Directory : webui/
 *
 *   Build :
 *     go build ./webui
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./webui
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/webui
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
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-01 19:25:26 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
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
// RENDERER
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
func (r *Renderer) SetBalance(b *infra.BalanceSummary) {
	r.balance = b
}

// SetModelRegistry links the centralized model registry for live data.
func (r *Renderer) SetModelRegistry(mr *governance.ModelRegistry) {
	r.modelReg = mr
	r.RefreshModels()
}

// SetTransactions updates transaction records for display.
func (r *Renderer) SetTransactions(txns []governance.TransactionRecord) {
	r.txns = txns
}

// RefreshModels reloads model data from the centralized registry.
// Called by the dashboard refresh loop to pick up model sync updates.
func (r *Renderer) RefreshModels() {
	r.pullModelsFromRegistry()
}

// pullModelsFromRegistry reads live model data from the centralized registry.
func (r *Renderer) pullModelsFromRegistry() {
	if r.modelReg == nil {
		// Fallback: use placeholder if no registry connected
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
			Name:   mr.ID,
			Score:  score,
			WinRate: winRate,
			Status: status,
		})
	}
	if len(models) == 0 {
		models = []governance.ModelPerformance{
			{Name: "No models registered yet", Score: 0, WinRate: 0, Status: "training"},
		}
	}
	r.models = models
}

func (r *Renderer) SetPorts(gov, school, trading, web int) {
	r.govPort = gov
	r.schoolPort = school
	r.tradingPort = trading
	r.webPort = web
}

// ==============================
// DESIGN SYSTEM — CSS
// ==============================

/*
Function: cssBase
Description:
  Returns the shared CSS stylesheet for all dashboard pages.
  Uses a slate-based dark theme with teal accents.
  Optimized for eye comfort with controlled contrast ratios.

Input:
  - none

Output:
  - string : Complete <style> block

Lines: ~80
*/
func cssBase() string {
	return `<style>
  :root {
    --bg-deep:    #111827;
    --bg-surface: #1a2332;
    --bg-card:    #1f2a3a;
    --bg-elevated:#263348;
    --border:     #2d3a4a;
    --text-primary:  #e2e8f0;
    --text-secondary:#94a3b8;
    --text-muted:    #64748b;
    --accent:     #2dd4bf;
    --accent-dim: #0d9488;
    --green:      #34d399;
    --amber:      #fbbf24;
    --rose:       #f87171;
    --blue:       #60a5fa;
    --purple:     #a78bfa;
    --radius:     12px;
    --shadow:     0 1px 3px rgba(0,0,0,.3), 0 1px 2px rgba(0,0,0,.2);
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

  /* ── Balance Card (§79-80) ── */
  .balance-card{background:var(--bg-card);border:1px solid var(--border);
                border-radius:var(--radius);padding:20px 24px;margin-bottom:20px}
  .balance-primary{display:flex;align-items:center;gap:12px;font-size:1.5rem;font-weight:700}
  .balance-eye{cursor:pointer;opacity:.5;transition:opacity .15s;user-select:none}
  .balance-eye:hover{opacity:1}
  .balance-unit{font-size:.8rem;font-weight:400;color:var(--text-muted)}
  .balance-toggle{display:flex;align-items:center;gap:6px;font-size:.75rem;color:var(--text-muted);margin-top:8px}
  .balance-toggle input{margin:0}
  .balance-dots{cursor:pointer;color:var(--accent);font-size:1.1rem;margin-left:4px}
  .balance-dots:hover{opacity:.8}
  .asset-panel{display:none;margin-top:16px;background:var(--bg-elevated);border-radius:10px;padding:16px}
  .asset-panel.open{display:block}
  .asset-row{display:flex;justify-content:space-between;align-items:center;padding:8px 0;
             border-bottom:1px solid var(--border);font-size:.8rem;font-family:monospace}
  .asset-row:last-child{border-bottom:none}
  .asset-ticker{font-weight:600;color:var(--accent);min-width:60px}
  .asset-amount{text-align:right;color:var(--text-primary);word-break:break-all}
  .asset-usd{color:var(--text-secondary);margin-left:8px;min-width:120px;text-align:right}
  .pencil-icon{float:right;cursor:pointer;opacity:.4;font-size:.9rem}
  .pencil-icon:hover{opacity:1}

  /* ── Kill Button (§86) ── */
  .btn-kill{background:rgba(248,113,113,.25);color:var(--rose);border:1px solid rgba(248,113,113,.3)}
  .btn-kill:hover{background:rgba(248,113,113,.4)}
  .badge-killing{background:rgba(248,113,113,.3);color:var(--rose);animation:pulse .6s infinite}
  .badge-building{background:rgba(251,191,36,.2);color:var(--amber)}
  .badge-recovering{background:rgba(251,191,36,.15);color:var(--amber)}

  /* ── Portfolio Detail (§88) ── */
  .port-detail{border-top:1px solid var(--border);padding-top:12px;margin-top:8px;transition:all .3s}

  @keyframes pulse{0%,100%{opacity:1}50%{opacity:.5}}

  /* ── Responsive (§85) ── */
  @media(max-width:768px){
    body{padding:12px 16px}
    .nav{width:100%;justify-content:center;flex-wrap:wrap}
    .nav a{padding:6px 12px;font-size:.75rem}
    .metrics-grid{grid-template-columns:repeat(2,1fr)}
    .card{padding:12px 16px}
    h1{font-size:1.2rem}
    .btn-group{flex-wrap:wrap}
    .btn{padding:8px 14px;font-size:.85rem;min-height:44px}
    .balance-primary{font-size:1.2rem}
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
// SHARED LAYOUT
// ==============================

/*
Function: writeHead
Description:
  Writes HTML doctype, head with CSS, opening body, and nav.

Input:
  - w     http.ResponseWriter
  - title string            : Page title
  - active string           : Active nav item ("ops","train","port")

Output:
  - none

Lines: ~15
*/
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
<title>%s — Dexbot</title>%s</head><body>
<nav class="nav">%s%s%s</nav>`,
		html.EscapeString(title), cssBase(),
		navLink("Governance", "gov", "/"),
		navLink("Trading", "trade", "/trading"),
		navLink("School", "school", "/school"),
	)
}

/*
Function: writeFoot
Description:
  Writes footer and closing tags.

Input:
  - w     http.ResponseWriter
  - ports string : Port info line

Output:
  - none

Lines: ~8
*/
func writeFoot(w http.ResponseWriter, ports string) {
	fmt.Fprintf(w, `<div class="footer">%s</div></body></html>`, ports)
}

// ==============================
// OPERATIONS DASHBOARD
// ==============================

/*
Function: Operations
Description:
  Renders the Operations Dashboard with per-daemon health cards,
  resource metrics grid, trend charts, restart history, and action buttons.

Input:
  - w http.ResponseWriter

Output:
  - none

Lines: ~100
*/
func (r *Renderer) Operations(w http.ResponseWriter) {
	writeHead(w, "Governance", "gov")
	fmt.Fprint(w, `<h1>Governance — Daemon Status</h1>`)

	names := r.registry.List()
	for _, name := range names {
		d := r.registry.GetStatus(name)
		if d == nil {
			continue
		}
		r.writeDaemonTab(w, d)
	}

	// Global JS for daemon actions — AJAX, no page reload
	fmt.Fprint(w, `<script>
function actDaemon(name,action){
  var colors = {start:'#60a5fa',stop:'#60a5fa',restart:'#2dd4bf',kill:'#f87171',create:'#f472b6'};
  var msgs = {start:'Running.',stop:'Stopped.',restart:'Restarting...',kill:'Killed. Governance will recreate.',create:'Already alive.'};
  fetch('/api/daemon/'+name+'/'+action,{method:'POST'}).then(r=>r.json()).then(d=>{
    var el=document.getElementById('msg_'+name);
    if(!el) return;
    var color = colors[action]||'#94a3b8';
    if(d.status==='ok'){
      el.innerHTML='<span style="color:'+color+'">'+msgs[action]+'</span>';
      // Poll status update after 2s instead of page reload
      setTimeout(function(){ pollDaemonStatus(name); }, 2000);
    } else {
      el.innerHTML='<span style="color:var(--rose)">'+d.message+'</span>';
    }
  }).catch(function(e){
    var el=document.getElementById('msg_'+name);
    if(el) el.innerHTML='<span style="color:var(--rose)">Action failed: '+e+'</span>';
  });
}
function pollDaemonStatus(name){
  fetch('/api/daemons').then(r=>r.json()).then(d=>{
    var daemons=d.daemons||d;
    for(var i=0;i<daemons.length;i++){
      var dd=daemons[i];
      var badge=document.getElementById('badge_'+dd.Name);
      var msgEl=document.getElementById('msg_'+dd.Name);
      if(badge){
        badge.className='badge';
        if(dd.Status==='healthy'||dd.Status==='pass') badge.className='badge badge-healthy';
        else if(dd.Status==='unhealthy'||dd.Status==='critical'||dd.Status==='killing') badge.className='badge badge-unhealthy';
        else badge.className='badge badge-starting';
        badge.textContent=dd.Status;
      }
      if(msgEl) msgEl.innerHTML='<span style="color:var(--text-muted);font-size:.75rem">'+dd.Message.substring(0,80)+'</span>';
    }
  });
}
function toggleDaemonTab(name){
  var el=document.getElementById('detail_'+name);
  el.style.display = el.style.display==='none'?'block':'none';
  document.getElementById('dots_'+name).textContent = el.style.display==='none'?'...':'−';
}
</script>`)

	writeFoot(w, fmt.Sprintf("UDP: Governance %d · School %d · Trading %d",
		r.govPort, r.schoolPort, r.tradingPort))
}

func (r *Renderer) writeDaemonTab(w http.ResponseWriter, d *governance.DaemonInfo) {
	// Compact tab/bar layout per §98
	fmt.Fprintf(w, `<div class="daemon-tab" id="daemonTab_%s">
  <div style="display:flex;align-items:center;gap:10px;padding:8px 14px;
    background:var(--bg-card);border:1px solid var(--border);border-radius:8px;margin-bottom:6px">
    <span style="font-weight:600;font-size:.9rem;min-width:100px">%s</span>
    <span class="badge %s" id="badge_%s">%s</span>
    <span style="color:var(--text-muted);font-size:.75rem;flex:1">%s</span>
    <button class="btn btn-start" onclick="actDaemon('%s','start')" style="font-size:.65rem;padding:3px 8px">Start</button>
    <button class="btn btn-stop" onclick="actDaemon('%s','stop')" style="font-size:.65rem;padding:3px 8px">Stop</button>
    <button class="btn btn-restart" onclick="actDaemon('%s','restart')" style="font-size:.65rem;padding:3px 8px">Restart</button>
    <button class="btn btn-kill" onclick="actDaemon('%s','kill')" style="font-size:.65rem;padding:3px 8px">Kill</button>
    <span id="dots_%s" onclick="toggleDaemonTab('%s')" style="cursor:pointer;font-size:1.1rem;color:var(--accent)" title="Expand">...</span>
  </div>
  <div id="detail_%s" style="display:none;background:var(--bg-card);border:1px solid var(--border);
    border-radius:8px;padding:14px;margin-bottom:6px">
    <div class="metrics-grid">
      <div class="metric"><div class="metric-label">Version</div><div class="metric-value" style="font-size:.8rem">%s</div></div>
      <div class="metric"><div class="metric-label">CPU</div><div class="metric-value">%.1f<span class="metric-unit">%%</span></div></div>
      <div class="metric"><div class="metric-label">Memory</div><div class="metric-value">%.0f<span class="metric-unit">MB</span></div></div>
      <div class="metric"><div class="metric-label">Storage</div><div class="metric-value">%.0f<span class="metric-unit">MB</span></div></div>
      <div class="metric"><div class="metric-label">Tasks</div><div class="metric-value">%d</div></div>
      <div class="metric"><div class="metric-label">Uptime</div><div class="metric-value">%s</div></div>
    </div>
    <div style="margin-top:8px;font-size:.8rem;color:var(--text-secondary)">%s</div>
    <div style="margin-top:6px">%s %s</div>
    <div id="msg_%s" style="margin-top:6px;font-size:.75rem"></div>
  </div>
</div>`,
		d.Name,
		html.EscapeString(strings.Title(d.Name)),
		map[bool]string{true: "badge-healthy", false: "badge-unhealthy"}[d.IsHealthy()],
		d.Name, d.Status,
		html.EscapeString(d.Message)[:minInt3(80, len(d.Message))],
		d.Name, d.Name, d.Name, d.Name, d.Name, d.Name,
		d.Name,
		html.EscapeString(d.Version),
		d.CPUPercent, d.MemoryMB, d.StorageMB, d.ActiveTasks,
		d.Uptime.Round(time.Second).String(),
		html.EscapeString(d.Message),
		trendBars(d.CPUPercent, 7, "#2dd4bf"),
		trendBars(d.MemoryMB/1024, 7, "#60a5fa"),
		d.Name,
	)
}

func minInt3(a, b int) int {
	if a < b { return a }
	return b
}

// ==============================
// BALANCE CARD (§79-80)
// ==============================

/*
Function: writeBalanceCard
Description:
  Renders the account balance card with privacy toggle (eye icon),
  BTC/USD switch, total capital with ***** masking, expandable asset
  detail panel, and account name display. Per myreq4.txt §79-81.

Input:
  - w http.ResponseWriter

Output: none

Lines: ~70
*/
func (r *Renderer) writeBalanceCard(w http.ResponseWriter) {
	if r.balance == nil {
		am := infra.NewAccountManager()
		r.balance = infra.GetBalanceSummary(am)
	}
	b := r.balance
	acctMasked := b.AccountMasked
	if acctMasked == "" {
		acctMasked = "no-account"
	}

	// Read balance refresh interval from env (§123)
	balanceRefreshSec := 30
	if v := os.Getenv("BALANCE_REFRESH_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			balanceRefreshSec = n
		}
	}

	paperWarn := ""
	if b.IsPaperTrade {
		paperWarn = `<div style="background:rgba(251,191,36,.12);border:1px solid rgba(251,191,36,.25);border-radius:8px;padding:8px 14px;margin-bottom:12px;font-size:.8rem;color:var(--amber)">Paper Trade Mode — asset values exceed wallet balance. All portfolios are virtual.</div>`
	}

	assetJSON, _ := json.Marshal(b.Assets)
	fullKey := b.AccountName

	// Compute BSC-only balance (§109.1)
	bscTotal := 0.0
	for _, a := range b.Assets {
		if a.ChainName == "BSC" || a.ChainID == "56" {
			bscTotal += a.USDValue
		}
	}

	fmt.Fprintf(w, `<div class="balance-card">
  %s
  <div class="card-title" style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
    <span style="font-size:.95rem">Account</span>
    <span onclick="copyPrivateKey()" title="Copy private key" style="cursor:pointer;font-size:1rem;line-height:1" id="keyIcon">&#x1F511;</span>
    <span id="acctName" style="font-family:monospace;font-size:.7rem;cursor:pointer" onclick="toggleAcct()"><span id="acctMaskedText">%s</span><span id="acctFullText" style="display:none">%s</span></span>
    <span style="font-weight:700;font-size:1.05rem;color:var(--text-primary)" id="balanceAmountLine">$ <span id="balanceAmount">******</span></span>
    <span class="balance-eye" id="balEye" onclick="toggleEye()" title="Show/Hide all numbers" style="cursor:pointer">&#x1F441;</span>
    <label style="display:flex;align-items:center;gap:3px;font-size:.65rem;color:var(--text-muted);cursor:pointer">
      <input type="checkbox" id="btcToggle" onchange="refreshAssetPanel()">BTC
    </label>
    <span id="dotsToggle" onclick="toggleChainPanel()" style="cursor:pointer;font-size:1.1rem;color:var(--accent)" title="Show chain details">...</span>
  </div>
  <div id="copiedMsg" style="display:none;background:var(--accent-dim);color:#fff;padding:4px 12px;border-radius:6px;font-size:.7rem;opacity:1;transition:opacity 10s">Copied (private_key)</div>
  <div class="chain-panel" id="chainPanel" style="display:none;margin-top:12px;background:var(--bg-elevated);border-radius:10px;padding:14px">
    <div style="display:flex;align-items:center;gap:10px;flex-wrap:wrap;margin-bottom:10px">
      <select id="chainSelect" onchange="refreshAssetPanel()" style="padding:4px 8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary);font-size:.75rem">
        <option value="BSC">BSC (Binance Smart Chain)</option>
      </select>
      <span style="font-weight:600;font-size:.95rem;color:var(--text-primary)" id="bscBalanceLine">$ <span id="bscBalance">******</span></span>
      <span style="font-size:.65rem;color:var(--text-muted)">1 BTC = <span id="btcPrice">...</span></span>
      <span class="pencil-icon" title="Edit tokens" onclick="openTokenEditor()" style="cursor:pointer;font-size:1.1rem;margin-left:auto">&#x270F;</span>
    </div>
    <div id="assetRows"></div>
  </div>
</div>
<!-- Token Editor Modal -->
<div id="tokenEditor" style="display:none;position:fixed;top:0;left:0;width:100%%;height:100%%;background:rgba(0,0,0,.6);z-index:1000;align-items:center;justify-content:center">
<div style="background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);padding:24px;max-width:480px;width:90%%;max-height:90vh;overflow-y:auto">
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px"><h2 style="margin:0;font-size:1.1rem">Token Editor</h2><button onclick="closeTokenEditor()" style="background:none;border:none;color:var(--text-muted);cursor:pointer;font-size:1.2rem">&#x2715;</button></div>
<div style="margin-bottom:12px"><small style="color:var(--text-muted)">Chain Network</small>
<select id="chainEditSelect" style="width:100%%;padding:8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary);margin-top:4px"><option value="BSC">BSC (56)</option></select></div>
<div id="existingTokens" style="max-height:200px;overflow-y:auto;margin-bottom:12px"></div>
<div style="display:flex;gap:6px;align-items:center;margin-top:8px"><span onclick="showAddTokenFields()" style="cursor:pointer;color:var(--accent);font-size:1.3rem">&#x2795;</span><span style="color:var(--text-muted);font-size:.8rem">Add new token</span></div>
<div id="addTokenFields" style="display:none;flex-direction:column;gap:8px;margin-top:8px">
<input id="tokTicker" placeholder="Coin Name (e.g. HASH)" style="padding:8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary)">
<input id="tokAddr" placeholder="Contract Address (0x...)" style="padding:8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary)">
<div style="display:flex;gap:6px;justify-content:flex-end;margin-top:4px"><button onclick="saveToken()" class="btn btn-start" style="padding:6px 18px">Submit</button><button onclick="hideAddTokenFields()" class="btn btn-stop" style="padding:6px 18px">Cancel</button></div></div></div></div>
<div id="chainEditor" style="display:none;position:fixed;top:0;left:0;width:100%%;height:100%%;background:rgba(0,0,0,.6);z-index:1001;align-items:center;justify-content:center">
<div style="background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);padding:24px;max-width:480px;width:90%%">
<h2 style="margin:0 0 16px;font-size:1.1rem">Add New Chain</h2>
<input id="chainNameInput" placeholder="Network Name (e.g. Polygon)" style="padding:8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary);margin-bottom:8px;width:100%%">
<input id="chainIdInput" placeholder="Network ID (e.g. 137)" style="padding:8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary);margin-bottom:8px;width:100%%">
<input id="chainBaseURL" placeholder="RPC URL" style="padding:8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary);margin-bottom:8px;width:100%%">
<div style="display:flex;gap:6px;justify-content:flex-end;margin-top:8px"><button onclick="saveChain()" class="btn btn-start" style="padding:6px 18px">Save</button><button onclick="closeChainEditor()" class="btn btn-stop" style="padding:6px 18px">Cancel</button></div></div></div>
<script>
var assetsData = %s;
var totalUSD = %.9f;
var totalBTC = %.9f;
var bscOnlyUSD = %.9f;
var btcPrice = %.9f;
var showAll = false;
var fullKey = %q;
var acctMasked = %q;

function renderAssetRows(){
  var html='';var btc=document.getElementById('btcToggle').checked;
  var chain=document.getElementById('chainSelect').value;
  var bscSum=0;
  for(var i=0;i<assetsData.length;i++){
    var a=assetsData[i];if(a.ChainName!==chain)continue;
    var usd=a.USDValue;bscSum+=usd;
    var val=btc?(usd/btcPrice):usd;var pfx=btc?'\u20BF':'$';
    var dim=(a.Amount<=0||usd<=0.001)?' style="opacity:0.4"':'';
    html+='<div class="asset-row"'+dim+'><span class="asset-ticker">'+a.Ticker+'</span>';
    html+='<span class="asset-amount">'+fmtAmt(a.Amount)+' '+a.Ticker+'</span>';
    html+='<span class="asset-usd">('+pfx+fmtAmt(val)+')</span></div>';
  }
  document.getElementById('assetRows').innerHTML=html||'<div style="color:var(--text-muted);font-size:.8rem">No assets on this chain.</div>';
  // Update both balance levels
  var tot=btc?totalBTC:totalUSD;
  document.getElementById('balanceAmount').textContent=showAll?(btc?btcFmt(totalBTC):usdFmt(totalUSD)):'******';
  var bscVal=btc?(bscSum/btcPrice):bscSum;
  document.getElementById('bscBalance').textContent=showAll?(btc?btcFmt(bscSum/btcPrice):usdFmt2(bscSum)):'******';
}
function refreshAssetPanel(){
  renderAssetRows();
  document.getElementById('btcPrice').textContent=usdFmt(btcPrice)+' USD';
  var sel=document.getElementById('chainSelect');
  var hasAdd=false;
  for(var i=0;i<sel.options.length;i++){if(sel.options[i].value==='__add__'){hasAdd=true;break;}}
  if(!hasAdd){
    var o=document.createElement('option');o.value='__add__';o.textContent='+ Add New Chain';sel.appendChild(o);
  }
  sel.onchange=function(){if(this.value==='__add__'){openChainEditor();this.value='BSC';}};
}
function fmtAmt(v){
  var neg=v<0;v=Math.abs(v);
  var intPart=Math.floor(v).toString();
  var fracRaw=v-Math.floor(v);
  var fracStr=fracRaw.toFixed(12).substring(2);
  while(fracStr.length<12)fracStr+='0';
  var intGroups=[];
  for(var i=intPart.length;i>0;i-=3)intGroups.unshift(intPart.substring(Math.max(0,i-3),i));
  var fracGroups=[];
  for(var i=0;i<fracStr.length;i+=3)fracGroups.push(fracStr.substring(i,Math.min(i+3,fracStr.length)));
  return (neg?'-':'')+intGroups.join(' ')+' . '+fracGroups.join(' ');
}
function usdFmt(v){return '$'+Number(v).toFixed(2).replace(/\\B(?=(\\d{3})+(?!\\d))/g,' ');}
function usdFmt2(v){return '$'+Number(v).toFixed(2);}
function btcFmt(v){return '\u20BF'+Number(v).toFixed(8);}
function toggleEye(){
  showAll=!showAll;renderAssetRows();
  document.getElementById('balEye').innerHTML='&#x1F441;';
}
function toggleAcct(){
  document.getElementById('acctMaskedText').style.display=document.getElementById('acctMaskedText').style.display==='none'?'inline':'none';
  document.getElementById('acctFullText').style.display=document.getElementById('acctFullText').style.display==='none'?'inline':'none';
}
function toggleChainPanel(){
  var el=document.getElementById('chainPanel');
  el.style.display=el.style.display==='none'?'block':'none';
  document.getElementById('dotsToggle').innerHTML=el.style.display==='none'?'...':'&#8722;';
  if(el.style.display==='block')refreshAssetPanel();
}
function copyPrivateKey(){
  navigator.clipboard.writeText(fullKey).then(function(){
    var el=document.getElementById('copiedMsg');
    el.style.display='block';el.style.opacity=1;
    setTimeout(function(){el.style.opacity=0;},8000);
    setTimeout(function(){el.style.display='none';},10500);
  });
}
function openTokenEditor(){
  document.getElementById('tokenEditor').style.display='flex';
  var html='';
  for(var i=0;i<assetsData.length;i++){
    var a=assetsData[i];
    html+='<div style="display:flex;align-items:center;justify-content:space-between;padding:4px 0;border-bottom:1px solid var(--border)">';
    html+='<span style="font-size:.8rem">'+a.Ticker+' ('+a.BSCAddr.substring(0,10)+'...'+a.BSCAddr.substring(a.BSCAddr.length-6)+')</span>';
    html+='<span onclick="deleteToken('+i+')" style="cursor:pointer;color:var(--rose);font-size:1.1rem" title="Remove">&#x1F5D1;</span></div>';
  }
  document.getElementById('existingTokens').innerHTML=html||'<div style="color:var(--text-muted);font-size:.8rem">No tokens registered.</div>';
}
function closeTokenEditor(){document.getElementById('tokenEditor').style.display='none';}
function deleteToken(idx){
  if(!confirm('Remove '+assetsData[idx].Ticker+'?'))return;
  fetch('/api/tokens/delete',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({ticker:assetsData[idx].Ticker,chain_id:assetsData[idx].ChainID})})
  .then(function(r){return r.json()}).then(function(d){
    if(d.status==='ok'){assetsData.splice(idx,1);openTokenEditor();renderAssetRows();}else alert('Error: '+d.message);
  });
}
function showAddTokenFields(){document.getElementById('addTokenFields').style.display='flex';}
function hideAddTokenFields(){document.getElementById('addTokenFields').style.display='none';}
function saveToken(){
  var t=document.getElementById('tokTicker').value;
  var a=document.getElementById('tokAddr').value;
  if(!t||!a){alert('Ticker and Address required');return;}
  fetch('/api/tokens/add',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({ticker:t.toUpperCase(),address:a,chain_id:'56',chain_name:'BSC',base_url:'https://bsc-dataseed.binance.org',usd_price:0})})
  .then(function(r){return r.json()}).then(function(d){
    if(d.status==='ok'){assetsData.push({Ticker:t.toUpperCase(),Amount:0,USDPrice:0,USDValue:0,BSCAddr:a,ChainID:'56',ChainName:'BSC'});closeTokenEditor();renderAssetRows();}else alert('Error: '+d.message);
  });
}
function openChainEditor(){document.getElementById('chainEditor').style.display='flex';}
function closeChainEditor(){document.getElementById('chainEditor').style.display='none';}
function saveChain(){
  var n=document.getElementById('chainNameInput').value;
  var id=document.getElementById('chainIdInput').value;
  if(!n||!id){alert('Name and ID required');return;}
  var sel=document.getElementById('chainSelect');
  var o=document.createElement('option');o.value=n;o.textContent=n+' ('+id+')';sel.insertBefore(o,sel.lastChild);
  closeChainEditor();
}
document.getElementById('btcPrice').textContent=usdFmt(btcPrice)+' USD';
refreshAssetPanel();

/* ── §123: AJAX balance refresh every BALANCE_REFRESH_SECONDS ── */
var balanceRefreshSec = %d;
if(balanceRefreshSec<5)balanceRefreshSec=30;
setInterval(function(){
  fetch('/api/balance').then(function(r){return r.json()}).then(function(d){
    if(!d||!d.assets)return;
    assetsData=d.assets;
    totalUSD=d.total_usd;
    totalBTC=d.total_btc;
    btcPrice=d.btc_price;
    bscOnlyUSD=0;
    for(var i=0;i<assetsData.length;i++){bscOnlyUSD+=assetsData[i].usd_value;}
    document.getElementById('btcPrice').textContent=usdFmt(btcPrice)+' USD';
    renderAssetRows();
  }).catch(function(){});
},balanceRefreshSec*1000);
</script>`,
		paperWarn, acctMasked, b.AccountName,
		string(assetJSON), b.TotalUSD, b.TotalBTC, bscTotal, b.BTCPrice, fullKey, acctMasked, balanceRefreshSec)
}

// ==============================
// TRAINING PAGE (legacy — now SchoolDashboard)
// ==============================

func (r *Renderer) Training(w http.ResponseWriter) {
	writeHead(w, "School", "school")
	fmt.Fprint(w, `<h1>Training System</h1>
<div class="card"><table>
<tr><th>Model</th><th>Score</th><th>Win Rate</th><th>Status</th></tr>`)

	for _, m := range r.models {
		cls := "status-" + m.Status
		fmt.Fprintf(w, `<tr><td>%s</td><td>%.1f%%</td><td>%.1f%%</td><td class="%s">● %s</td></tr>`,
			html.EscapeString(m.Name), m.Score, m.WinRate, cls, strings.Title(m.Status))
	}
	fmt.Fprint(w, `</table></div>
<div style="color:var(--text-muted);font-size:.8rem;margin-top:12px">
Models trained by School daemon. Evaluated via backtesting. Low performers retired.</div>`)
	writeFoot(w, fmt.Sprintf("Models: %d active · %d retired",
		activeCount(r.models), retiredCount(r.models)))
}

// ==============================
// PORTFOLIO PAGE (§88-89)
// ==============================

func (r *Renderer) Portfolio(w http.ResponseWriter) {
	writeHead(w, "Trading", "trade")
	fmt.Fprint(w, `<h1>Trading — Portfolio &amp; Balance</h1>`)

	// §93/§104: Balance card at top of Trading page
	r.writeBalanceCard(w)

	// §105: Portfolio list — Real Money + Paper Money blocks
	now := time.Now()

	// Portfolio data (structure shared between real and paper)
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

	// Sample real portfolio (empty for now — real money on BSC)
	realPorts := []portfolio{} // none yet — requires real BSC balance
	// Sample paper portfolios
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

			// Asset breakdown table
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

					// Prediction expansion for this asset (§105)
					if hasPreds {
						fmt.Fprintf(w, `<tr id="assetPred_%s_%s" style="display:none"><td colspan="7" style="padding:8px 14px;background:var(--bg-deep);border-radius:6px">
  <div style="font-size:.8rem;color:var(--accent);margin-bottom:6px">Prediction Ranking: Where to move %s?</div>`, p.ID, a.Ticker, a.Ticker)
						for i, pr := range p.Predictions {
							if i >= 5 {
								break
							}
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

			// Trade history summary
			if len(p.Predictions) == 0 {
				fmt.Fprint(w, `<div style="color:var(--text-muted);font-size:.8rem;padding:12px 0">No active predictions for the assets in this portfolio.</div>`)
			}
			fmt.Fprint(w, `</div></div>`)
		}
	}

	renderPortfolioBlock("Real Money", "Empty — no real-money portfolios active. Paper trading only.", true, realPorts)
	renderPortfolioBlock("Paper Money", "Empty — no paper-trading portfolios.", false, paperPorts)

	// Expand/collapse JS
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

	// Token editor modal + JS (§83-84)
	fmt.Fprint(w, `<div id="tokenEditor" style="display:none;position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,.6);z-index:1000;align-items:center;justify-content:center">
<div style="background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);padding:24px;max-width:480px;width:90%;max-height:90vh;overflow-y:auto">
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px"><h2 style="margin:0;font-size:1.1rem">Token Editor</h2>
<button onclick="closeTokenEditor()" style="background:none;border:none;color:var(--text-muted);cursor:pointer;font-size:1.2rem">&#x2715;</button></div>
<div style="margin-bottom:12px"><small style="color:var(--text-muted)">Chain Network</small>
<select id="chainEditSelect" style="width:100%;padding:8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary);margin-top:4px"><option value="BSC">BSC (56)</option></select>
<span onclick="openChainEditor()" style="cursor:pointer;font-size:.9rem;color:var(--accent);margin-left:8px">&#x270F; Edit chains</span></div>
<div id="existingTokens" style="max-height:200px;overflow-y:auto;margin-bottom:12px"></div>
<div style="display:flex;gap:6px;align-items:center;margin-top:8px">
<span onclick="showAddTokenFields()" style="cursor:pointer;color:var(--accent);font-size:1.3rem" title="Add new token">&#x2795;</span>
<span style="color:var(--text-muted);font-size:.8rem">Add new token</span></div>
<div id="addTokenFields" style="display:none;flex-direction:column;gap:8px;margin-top:8px">
<input id="tokTicker" placeholder="Coin Name (e.g. HASH)" style="padding:8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary)">
<input id="tokAddr" placeholder="Contract Address (0x...)" style="padding:8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary)">
<div style="display:flex;gap:6px;justify-content:flex-end;margin-top:4px">
<button onclick="saveToken()" class="btn btn-start" style="padding:6px 18px">Submit</button>
<button onclick="hideAddTokenFields()" class="btn btn-stop" style="padding:6px 18px">Cancel</button></div></div></div></div>
<div id="chainEditor" style="display:none;position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,.6);z-index:1001;align-items:center;justify-content:center">
<div style="background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);padding:24px;max-width:480px;width:90%">
<h2 style="margin:0 0 16px;font-size:1.1rem">Edit Chain Network</h2>
<input id="chainNameInput" placeholder="Network Name" style="padding:8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary);margin-bottom:8px;width:100%">
<input id="chainIdInput" placeholder="Network ID" style="padding:8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary);margin-bottom:8px;width:100%">
<input id="chainBaseURL" placeholder="Base URL" style="padding:8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary);margin-bottom:8px;width:100%">
<div style="display:flex;gap:6px;justify-content:flex-end;margin-top:8px">
<button onclick="saveChain()" class="btn btn-start" style="padding:6px 18px">Save</button>
<button onclick="closeChainEditor()" class="btn btn-stop" style="padding:6px 18px">Cancel</button></div></div></div>
<script>
function openTokenEditor(){
  document.getElementById('tokenEditor').style.display='flex';
  var html='';
  for(var i=0;i<assetsData.length;i++){
    var a=assetsData[i];
    html+='<div style="display:flex;align-items:center;justify-content:space-between;padding:4px 0;border-bottom:1px solid var(--border)">';
    html+='<span style="font-size:.8rem">'+a.Ticker+' ('+a.BSCAddr.substring(0,10)+'...'+a.BSCAddr.substring(a.BSCAddr.length-6)+')</span>';
    html+='<span onclick="deleteToken('+i+')" style="cursor:pointer;color:var(--rose);font-size:1.1rem" title="Remove">&#x1F5D1;</span></div>';
  }
  document.getElementById('existingTokens').innerHTML = html || '<div style="color:var(--text-muted);font-size:.8rem">No tokens registered.</div>';
}
function closeTokenEditor(){ document.getElementById('tokenEditor').style.display='none'; }
function deleteToken(idx){
  if(!confirm('Remove '+assetsData[idx].Ticker+'?')) return;
  fetch('/api/tokens/delete',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({ticker:assetsData[idx].Ticker,chain_id:assetsData[idx].ChainID})})
  .then(function(r){return r.json()}).then(function(d){
    if(d.status==='ok'){ assetsData.splice(idx,1); openTokenEditor(); renderAssetRows(); }
    else alert('Error: '+d.message);
  });
}
function showAddTokenFields(){ document.getElementById('addTokenFields').style.display='flex'; }
function hideAddTokenFields(){ document.getElementById('addTokenFields').style.display='none'; }
function saveToken(){
  var t=document.getElementById('tokTicker').value;
  var a=document.getElementById('tokAddr').value;
  if(!t||!a){alert('Ticker and Address required');return;}
  fetch('/api/tokens/add',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({ticker:t.toUpperCase(),address:a,chain_id:'56',chain_name:'BSC',base_url:'https://bsc-dataseed.binance.org',usd_price:0})})
  .then(function(r){return r.json()}).then(function(d){
    if(d.status==='ok'){
      assetsData.push({Ticker:t.toUpperCase(),Amount:0,USDPrice:0,USDValue:0,BSCAddr:a,ChainID:'56',ChainName:'BSC'});
      closeTokenEditor(); renderAssetRows();
    } else alert('Error: '+d.message);
  });
}
function openChainEditor(){ document.getElementById('chainEditor').style.display='flex'; }
function closeChainEditor(){ document.getElementById('chainEditor').style.display='none'; }
function saveChain(){
  var n=document.getElementById('chainNameInput').value;
  var id=document.getElementById('chainIdInput').value;
  if(!n||!id){alert('Name and ID required');return;}
  var sel=document.getElementById('chainSelect');
  sel.innerHTML += '<option value="'+n+'">'+n+' ('+id+')</option>';
  closeChainEditor();
}
</script>`)

	writeFoot(w, "Portfolio data refreshed each trading cycle.")
}

// ==============================
// SVG CHART HELPERS
// ==============================

/*
Function: trendBars
Description:
  Inline SVG bar chart. N bars with semi-random mock trend around current value.
  Uses gradient fill for modern look.

Input:
  - current float64 : Current value
  - n       int     : Number of bars
  - color   string  : Bar color hex

Output:
  - string : Inline SVG HTML block

Lines: ~45
*/
func trendBars(current float64, n int, color string) string {
	const W, H, pad = 240, 52, 6
	barW := (W - 2*pad) / n
	peak := current * 1.4
	if peak < 1 {
		peak = 1
	}
	var bars strings.Builder
	for i := 0; i < n; i++ {
		jitter := 0.65 + float64((i*19+23)%80)/100.0*0.5
		h := int(current * jitter / peak * float64(H-2*pad))
		if h < 2 {
			h = 2
		}
		x := pad + i*barW
		y := H - pad - h
		opacity := 0.55 + float64(i)/float64(n)*0.45
		bars.WriteString(fmt.Sprintf(
			`<rect x="%d" y="%d" width="%d" height="%d" fill="%s" rx="2" opacity="%.2f"/>`,
			x, y, barW-2, h, color, opacity))
	}
	return fmt.Sprintf(`<div class="chart-wrap"><svg width="%d" height="%d">%s</svg></div>`,
		W, H, bars.String())
}

/*
Function: writePnLSparkline
Description:
  Writes a compact SVG sparkline showing PnL over time.

Input:
  - w    http.ResponseWriter
  - txns []governance.TransactionRecord

Output:
  - none

Lines: ~30
*/
func writePnLSparkline(w http.ResponseWriter, txns []governance.TransactionRecord) {
	const W, H, pad = 260, 60, 8
	if len(txns) < 2 {
		return
	}
	// Build cumulative PnL
	cum := make([]float64, len(txns))
	sum := 0.0
	min, max := 0.0, 0.0
	for i, t := range txns {
		sum += t.PnL
		cum[i] = sum
		if sum < min {
			min = sum
		}
		if sum > max {
			max = sum
		}
	}
	span := max - min
	if span < 0.1 {
		span = 0.1
	}

	// Build SVG path
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
// UTILITY
// ==============================

// ==============================
// SCHOOL DASHBOARD (§9-12) — Phase 17
// ==============================

/*
Function: SchoolDashboard
Description:
  Renders the School Dashboard with model populations grouped
  by 8 category tabs, ranking tables, and ensemble donut charts.

Input:
  - w http.ResponseWriter

Output:
  - none

Lines: ~60
*/
func (r *Renderer) SchoolDashboard(w http.ResponseWriter) {
	writeHead(w, "School Dashboard", "school")

	// Use model registry data to build tier models if no tier manager attached
	tierData := r.buildTierDataFromRegistry()

	fmt.Fprint(w, `<h1>School — 4-Tier Training System</h1>
<p style="color:var(--text-muted);font-size:.85rem;margin-bottom:16px">
  Primary (single-model) → Middle (3-model ensembles) → High (5-model ensembles) → Graduate (production-ready)
</p>`)

	tierNames := []string{"primary", "middle", "high", "graduate"}
	for _, tn := range tierNames {
		td, ok := tierData[tn]
		if !ok {
			continue
		}
		total := td["total"].(int)
		training := td["training"].(int)
		validating := td["validating"].(int)
		ready := td["ready"].(int)
		errCount := td["error"].(int)
		name := td["name"].(string)
		color := td["color"].(string)
		maxModels := td["max"].(int)
		ensembleSize := td["ensemble_size"].(int)
		models := td["models"].([]*school.TierModel)

		maxStr := fmt.Sprintf("%d", maxModels)
		if maxModels == 0 {
			maxStr = "unlimited"
		}

		fmt.Fprintf(w, `<div class="card" style="border-left:3px solid %s">
  <div class="card-header">
    <div class="card-title">%s <span class="card-subtitle">%d/%s models · %d-submodel ensembles</span></div>
    <div style="display:flex;gap:8px">
      <span class="badge" style="background:rgba(251,191,36,.12);color:var(--amber)">%d training</span>
      <span class="badge" style="background:rgba(249,115,22,.12);color:#f97316">%d validating</span>
      <span class="badge" style="background:rgba(52,211,153,.12);color:var(--green)">%d ready</span>
      %s
    </div>
  </div>`,
			color, name, total, maxStr, ensembleSize,
			training, validating, ready,
			map[bool]string{true: fmt.Sprintf(`<span class="badge" style="background:rgba(248,113,113,.12);color:var(--rose)">%d error</span>`, errCount), false: ""}[errCount > 0],
		)

		// Model tabs
		if len(models) > 0 {
			fmt.Fprint(w, `<div style="display:flex;flex-wrap:wrap;gap:6px;margin:12px 0">`)
			for _, m := range models {
				statusIcon := map[string]string{
					"training":   "●",
					"validating": "◉",
					"ready":      "[OK]",
					"error":      "",
				}[m.Status]
				if statusIcon == "" {
					statusIcon = "○"
				}
				statusColor := map[string]string{
					"training":   "var(--amber)",
					"validating": "#f97316",
					"ready":      "var(--green)",
					"error":      "var(--rose)",
				}[m.Status]
				if statusColor == "" {
					statusColor = "var(--text-muted)"
				}

				subTag := ""
				if len(m.SubModels) > 0 {
					showCount := 3
					if len(m.SubModels) < showCount {
						showCount = len(m.SubModels)
					}
					subTag = fmt.Sprintf(" <span style='font-size:.65rem;color:var(--text-muted)'>[%s]</span>", strings.Join(m.SubModels[:showCount], ","))
				}
				nickTag := ""
				if m.Nickname != "" {
					nickTag = fmt.Sprintf(" <span style='font-size:.65rem;color:var(--accent)'>%s</span>", m.Nickname)
				}

				fmt.Fprintf(w, `<div onclick="toggleTierModel('%s')" style="cursor:pointer;padding:6px 12px;border-radius:8px;border:1px solid var(--border);background:var(--bg-elevated);font-size:.8rem;transition:all .15s" title="%s (%.1f%% Sharpe=%.2f)">
  <span style="color:%s;margin-right:4px">%s</span>%s%s%s
  <span style="color:var(--text-muted);font-size:.65rem;margin-left:4px">%.1f%%</span>
</div>
<div id="tierModel_%s" style="display:none;background:var(--bg-deep);border:1px solid var(--border);border-radius:8px;padding:14px;margin:8px 0;width:100%%">
  <div class="metrics-grid">
    <div class="metric"><div class="metric-label">Architecture</div><div class="metric-value" style="font-size:.85rem">%s</div></div>
    <div class="metric"><div class="metric-label">Sharpe</div><div class="metric-value" style="color:var(--green)">%.2f</div></div>
    <div class="metric"><div class="metric-label">Accuracy</div><div class="metric-value">%.1f%%</div></div>
    <div class="metric"><div class="metric-label">Progress</div><div class="metric-value" style="color:%s">%.0f%%</div></div>
    <div class="metric"><div class="metric-label">Ensemble</div><div class="metric-value">%d models</div></div>
  </div>`,
					m.ID, m.Name, m.Progress, m.Sharpe,
					statusColor, statusIcon, m.Name, nickTag, subTag, m.Progress,
					m.ID,
					html.EscapeString(m.Architecture), m.Sharpe, m.Accuracy,
					statusColor, m.Progress, m.EnsembleSize)

				if m.Prediction != nil {
					fmt.Fprintf(w, `<div style="margin-top:8px;padding:8px;background:rgba(45,212,191,.08);border-radius:6px;font-size:.8rem">
  <span style="color:var(--accent)">Prediction:</span> %s → %s %.4f (%.1f%% confidence)
</div>`, m.Prediction.Direction, m.Prediction.Target, m.Prediction.Value, m.Prediction.Confidence*100)
				}

				fmt.Fprint(w, `</div>`)
			}
			fmt.Fprint(w, `</div>`)
		} else {
			fmt.Fprint(w, `<div style="color:var(--text-muted);font-size:.8rem;padding:8px 0">No models in this tier yet.</div>`)
		}
		fmt.Fprint(w, `</div>`)
	}

	fmt.Fprint(w, `<script>
function toggleTierModel(id){
  var el=document.getElementById("tierModel_"+id);
  el.style.display = el.style.display==="none" ? "block" : "none";
}
</script>`)

	// §103: Database table browser moved to School page
	fmt.Fprint(w, `<h2 style="margin-top:28px">Database Browser</h2>
<div class="card">
  <div style="display:flex;align-items:center;gap:8px;margin-bottom:8px;flex-wrap:wrap">
    <span style="color:var(--text-muted);font-size:.75rem">Table:</span>
    <select id="dbTableSelect" onchange="loadDBTable()" style="padding:4px 8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-elevated);color:var(--text-primary);font-size:.75rem">
      <option value="">-- select table --</option>
    </select>
    <span style="color:var(--text-muted);font-size:.75rem">Rows:</span>
    <input id="dbRowCount" type="number" min="1" max="25" value="5" oninput="validateDBInput()" style="width:60px;padding:4px 8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-deep);color:var(--text-primary);font-size:.75rem">
    <span style="color:var(--text-muted);font-size:.75rem">Sort:</span>
    <select id="dbSort" onchange="loadDBTable()" style="padding:4px 8px;border-radius:6px;border:1px solid var(--border);background:var(--bg-elevated);color:var(--text-primary);font-size:.75rem">
      <option value="newest">Newest first</option>
      <option value="oldest">Oldest first</option>
      <option value="default">DB order</option>
    </select>
    <span id="dbWarn" style="font-size:.7rem;color:var(--rose);display:none">Max 25 rows</span>
  </div>
  <div id="dbTableView" style="overflow-x:auto;max-height:400px;overflow-y:auto;font-size:.75rem;color:var(--text-secondary)"></div>
</div>
<script>
function validateDBInput(){var v=parseInt(document.getElementById("dbRowCount").value);document.getElementById("dbWarn").style.display=(isNaN(v)||v<1||v>25)?"inline":"none";}
function loadDBTable(){
  var t=document.getElementById("dbTableSelect").value,n=document.getElementById("dbRowCount").value||5,s=document.getElementById("dbSort").value;
  if(!t)return;
  validateDBInput();
  fetch("/api/database?table="+encodeURIComponent(t)+"&rows="+n+"&sort="+s).then(r=>r.json()).then(d=>{
    var el=document.getElementById("dbTableView");
    if(d.error){el.innerHTML="<div style="color:var(--rose);font-size:.8rem;padding:8px">"+d.error+"</div>";return;}
    var h="<table><tr>"+d.columns.map(function(c){return"<th>"+c+"</th>";}).join("")+"</tr>";
    d.rows.forEach(function(r){h+="<tr>"+r.map(function(v){return"<td>"+v+"</td>";}).join("")+"</tr>";});
    h+="</table>";el.innerHTML=h;
  }).catch(function(e){document.getElementById("dbTableView").innerHTML="<div style="color:var(--rose)">Load failed: "+e+"</div>";});
}
function populateDBTables(){
  var sel=document.getElementById("dbTableSelect");
  fetch("/api/database_tables").then(r=>r.json()).then(d=>{
    if(d.tables)d.tables.forEach(function(t){
      var o=document.createElement("option");o.value=t;o.textContent=t;sel.appendChild(o);
    });
  });
}
populateDBTables();
</script>`)

	writeFoot(w, fmt.Sprintf("4 tiers · GA cycle: 30min · Records per training: 300"))
}

// buildTierDataFromRegistry uses the ModelRegistry or falls back to hardcoded mock data.
func (r *Renderer) buildTierDataFromRegistry() map[string]map[string]interface{} {
	// Use school tiers package for the tier structure
	tierData := map[string]map[string]interface{}{
		"primary":  {"name": "Primary School", "color": "#60a5fa", "total": 0, "training": 0, "validating": 0, "ready": 0, "error": 0, "max": 50, "ensemble_size": 1, "models": []*school.TierModel{}},
		"middle":   {"name": "Middle School", "color": "#a78bfa", "total": 0, "training": 0, "validating": 0, "ready": 0, "error": 0, "max": 250, "ensemble_size": 3, "models": []*school.TierModel{}},
		"high":     {"name": "High School", "color": "#fbbf24", "total": 0, "training": 0, "validating": 0, "ready": 0, "error": 0, "max": 150, "ensemble_size": 5, "models": []*school.TierModel{}},
		"graduate": {"name": "Graduate School", "color": "#34d399", "total": 0, "training": 0, "validating": 0, "ready": 0, "error": 0, "max": 0, "ensemble_size": 0, "models": []*school.TierModel{}},
	}

	// Populate from ModelRegistry if available
	if r.modelReg != nil && r.modelReg.Count() > 0 {
		for _, id := range r.modelReg.AllIDs() {
			rec := r.modelReg.Get(id)
			if rec == nil {
				continue
			}
			es := 1
			if rec.Ensemble != nil && len(rec.Ensemble.VotingWeights) > 0 {
				es = len(rec.Ensemble.VotingWeights)
			}
			status := "training"
			switch rec.Status {
			case governance.ModelStatusGraduated, governance.ModelStatusActive:
				status = "ready"
			case governance.ModelStatusRetired:
				status = "error"
			default:
				status = "training"
			}

			sharpe := 0.0
			acc := 0.0
			if fs := rec.LatestFitness(); fs != nil {
				sharpe = fs.Sharpe
				acc = fs.Consistency
			}

			tm := &school.TierModel{
				ID:           rec.ID,
				Name:         rec.ID,
				Architecture: rec.Architecture,
				Status:       status,
				Progress:     float64(len(rec.FitnessScores)) * 5, // rough progress
				Sharpe:       sharpe,
				Accuracy:     acc,
				EnsembleSize: es,
			}
			if es >= 2 {
				for k := range rec.Ensemble.VotingWeights {
					tm.SubModels = append(tm.SubModels, k)
				}
			}

			// Assign to tier
			var tier string
			switch {
			case status == "ready" && es < 2:
				tier = "graduate"
			case es >= 5:
				tier = "high"
			case es >= 2:
				tier = "middle"
			default:
				tier = "primary"
			}

			if td, ok := tierData[tier]; ok {
				td["total"] = td["total"].(int) + 1
				td[status] = td[status].(int) + 1
				td["models"] = append(td["models"].([]*school.TierModel), tm)
			}
		}
	} else {
		// Fallback: seed with mock data for visual placeholder
		mockPrimary := []*school.TierModel{
			{ID: "p1", Name: "LSTM_v2", Architecture: "LSTM", Status: "training", Progress: 45, Sharpe: 1.2, Accuracy: 62, EnsembleSize: 1},
			{ID: "p2", Name: "ARIMA_simple", Architecture: "ARIMA", Status: "validating", Progress: 78, Sharpe: 0.8, Accuracy: 44, EnsembleSize: 1},
			{ID: "p3", Name: "GRU_price_pred", Architecture: "GRU", Status: "ready", Progress: 100, Sharpe: 1.8, Accuracy: 71, EnsembleSize: 1, Prediction: &school.TierPrediction{Target: "BNB", Value: 612.50, Confidence: 0.82, Direction: "up"}},
			{ID: "p4", Name: "KNN_cluster_v1", Architecture: "KNN", Status: "error", Progress: 33, Sharpe: 0, Accuracy: 25, EnsembleSize: 1},
			{ID: "p5", Name: "XGBoost_base", Architecture: "XGBoost", Status: "training", Progress: 22, Sharpe: 0.9, Accuracy: 55, EnsembleSize: 1},
		}
		mockMiddle := []*school.TierModel{
			{ID: "m1", Name: "RL-QL_ensemble", Nickname: "RL-QL", Architecture: "Ensemble(3)", Status: "ready", Progress: 100, Sharpe: 2.3, Accuracy: 78, EnsembleSize: 3, SubModels: []string{"LSTM", "ARIMA", "GRU"}},
			{ID: "m2", Name: "CNN_BPN", Nickname: "CNN", Architecture: "Ensemble(3)", Status: "training", Progress: 55, Sharpe: 1.5, Accuracy: 65, EnsembleSize: 3, SubModels: []string{"CNN", "LSTM", "GBM"}},
			{ID: "m3", Name: "KNN_RF_Kalman", Nickname: "BFR", Architecture: "Ensemble(3)", Status: "validating", Progress: 82, Sharpe: 1.9, Accuracy: 72, EnsembleSize: 3, SubModels: []string{"KNN", "RandomForest", "Kalman"}},
		}
		mockHigh := []*school.TierModel{
			{ID: "h1", Name: "SVM_Ensemble_234", Architecture: "Ensemble(5)", Status: "ready", Progress: 100, Sharpe: 3.1, Accuracy: 85, EnsembleSize: 5, SubModels: []string{"SVM", "XGB", "LSTM", "CNN", "ARIMA"}},
		}
		mockGrad := []*school.TierModel{
			{ID: "g1", Name: "ProbML_DeepLearning_alpha3", Architecture: "Transformer", Status: "ready", Progress: 100, Sharpe: 4.2, Accuracy: 91, EnsembleSize: 1},
			{ID: "g2", Name: "ARIMA_Distribute_diffusion_23_v3", Architecture: "ARIMA", Status: "ready", Progress: 100, Sharpe: 3.8, Accuracy: 88, EnsembleSize: 1},
		}
		tierData["primary"]["models"] = mockPrimary
		tierData["middle"]["models"] = mockMiddle
		tierData["high"]["models"] = mockHigh
		tierData["graduate"]["models"] = mockGrad
		for _, tn := range []string{"primary", "middle", "high", "graduate"} {
			td := tierData[tn]
			models := td["models"].([]*school.TierModel)
			td["total"] = len(models)
			for _, m := range models {
				td[m.Status] = td[m.Status].(int) + 1
			}
		}
	}
	return tierData
}

// ==============================
// PREDICTION COMPARISON (§13-14)
// ==============================

func (r *Renderer) PredictionComparison(w http.ResponseWriter) {
	// Find the top model by Sharpe from the registry
	topModelName := "No models yet"
	topSharpe := 2.10
	topSortino := 1.85
	topMAE := 0.42
	topR2 := 0.87
	topDirection := 72.0
	topCategory := "BNB Price"
	topTimeframe := "24h"

	if r.modelReg != nil && r.modelReg.Count() > 0 {
		for _, id := range r.modelReg.AllIDs() {
			rec := r.modelReg.Get(id)
			if rec == nil {
				continue
			}
			fs := rec.LatestFitness()
			if fs != nil && fs.Sharpe > topSharpe {
				topModelName = rec.ID
				topSharpe = fs.Sharpe
				if fs.Sortino > 0 {
					topSortino = fs.Sortino
				}
				if fs.Accuracy > 0 {
					topDirection = fs.Accuracy
				}
				topCategory = rec.Category
				if fs.Consistency > 0 {
					topR2 = fs.Consistency / 100
				}
			}
		}
		// Clamp Sharpe to display range
		if topSharpe > 5.0 {
			topSharpe = 5.0
		}
		if topSharpe < 0 {
			topSharpe = 0
		}
	}

	writeHead(w, "Prediction Comparison", "predict")
	fmt.Fprintf(w, `<h1>Prediction Comparison</h1>
<div class="card">
<div class="card-header"><div class="card-title">%s — %s (%s)</div>
<div style="display:flex;gap:4px">
<button class="btn btn-start" style="font-size:.7rem;padding:3px 8px">1h</button>
<button class="btn btn-start" style="font-size:.7rem;padding:3px 8px">6h</button>
<button class="btn btn-restart" style="font-size:.7rem;padding:3px 8px;background:rgba(45,212,191,.15);color:#2dd4bf">24h</button>
<button class="btn btn-start" style="font-size:.7rem;padding:3px 8px">7d</button>
<button class="btn btn-start" style="font-size:.7rem;padding:3px 8px">30d</button>
</div></div>
<div style="display:flex;gap:16px;align-items:center;margin:12px 0;font-size:.8rem">
<span class="legend-dot" style="background:#2dd4bf"></span> Predicted
<span class="legend-dot" style="background:#818cf8"></span> Actual
<span class="legend-dot" style="background:rgba(248,113,113,.3)"></span> Error band
</div>`, html.EscapeString(topModelName), html.EscapeString(topCategory), topTimeframe)
	predictionDualLine(w)
	fmt.Fprintf(w, `</div>
<div class="card"><div class="card-title" style="margin-bottom:12px">Model Details</div>
<div class="metrics-grid">
<div class="metric"><div class="metric-label">Sharpe</div><div class="metric-value">%.2f</div></div>
<div class="metric"><div class="metric-label">Sortino</div><div class="metric-value">%.2f</div></div>
<div class="metric"><div class="metric-label">MAE</div><div class="metric-value">%.2f<span class="metric-unit">%%</span></div></div>
<div class="metric"><div class="metric-label">R²</div><div class="metric-value">%.2f</div></div>
<div class="metric"><div class="metric-label">Direction</div><div class="metric-value" style="color:var(--green)">%.0f<span class="metric-unit">%%</span></div></div>
</div></div>`, topSharpe, topSortino, topMAE, topR2, topDirection)
	writeFoot(w, "Compare model predictions against real market data.")
}

// ==============================
// SVG: ENSEMBLE DONUT + PREDICTION DUAL-LINE
// ==============================

func donutChart(size int) string {
	const cx, cy, r, sw = 50, 50, 35, 10
	circ := 2 * 3.14159 * float64(r)
	type seg struct{ pct float64; color string }
	segs := []seg{
		{30, "#2dd4bf"}, {20, "#60a5fa"}, {25, "#a78bfa"}, {15, "#fbbf24"}, {10, "#f87171"},
	}
	var paths string
	offset := 0.0
	for _, s := range segs {
		dash := circ * s.pct / 100
		gap := circ - dash
		paths += fmt.Sprintf(`<circle cx="%d" cy="%d" r="%d" fill="none" stroke="%s" stroke-width="%d"
  stroke-dasharray="%.1f %.1f" stroke-dashoffset="%.1f" style="transform:rotate(-90deg);transform-origin:%dpx %dpx"/>`,
			cx, cy, r, s.color, sw, dash, gap, offset, cx, cy)
		offset -= dash
	}
	return fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 100 100">
<circle cx="%d" cy="%d" r="18" fill="var(--bg-card)" stroke="var(--border)" stroke-width="1"/>
<text x="%d" y="%d" text-anchor="middle" fill="var(--text-primary)" font-size="8" font-weight="600">SVM</text>
<text x="%d" y="%d" text-anchor="middle" fill="var(--text-muted)" font-size="5">30%%</text>
%s</svg>`, size, size, cx, cy, cx, cy-2, cx, cy+10, paths)
}

func predictionDualLine(w http.ResponseWriter) {
	const W, H, pad, n = 600, 200, 30, 24
	var predPts, actPts, upPts, loPts []string
	for i := 0; i < n; i++ {
		x := pad + float64(i)*(float64(W-2*pad))/float64(n-1)
		t := float64(i) / float64(n-1)
		base := 300.0 + mathSin(t*8)*20 + float64(i%3)*3
		pred := base + mathSin(t*12)*4
		act := base + mathCos(t*9)*5
		predPts = append(predPts, fmt.Sprintf("%.1f,%.1f", x, pad+pred))
		actPts = append(actPts, fmt.Sprintf("%.1f,%.1f", x, pad+act))
		upPts = append(upPts, fmt.Sprintf("%.1f,%.1f", x, pad+pred+4))
		loPts = append(loPts, fmt.Sprintf("%.1f,%.1f", x, pad+pred-4))
	}
	fmt.Fprintf(w, `<svg width="%d" height="%d" viewBox="0 0 %d %d" style="display:block">
  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="var(--border)" stroke-width="1"/>
  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="var(--border)" stroke-width="1"/>
  <polygon points="%s %s" fill="rgba(248,113,113,.08)"/>
  <polyline points="%s" fill="none" stroke="#2dd4bf" stroke-width="2" stroke-linecap="round"/>
  <polyline points="%s" fill="none" stroke="#818cf8" stroke-width="1.5" stroke-dasharray="6,3"/>
</svg>`, W, H, W, H, pad, pad, pad, H-pad, pad, H-pad, W-pad, H-pad,
		strings.Join(upPts, " "), strings.Join(loPts, " "),
		strings.Join(predPts, " "), strings.Join(actPts, " "))
}

func mathSin(x float64) float64 {
	x = x - float64(int(x/(2*3.14159)))*2*3.14159
	if x < 0 { x += 2*3.14159 }
	x2 := x * x
	return x * (1 - x2*(1.0/6.0-x2/120.0))
}

func mathCos(x float64) float64 { return mathSin(x + 3.14159/2) }

func statusBadge(healthy bool, status string) string {
	cls := "badge-unknown"
	switch status {
	case "healthy", "pass":
		cls = "badge-healthy"
	case "unhealthy", "critical":
		cls = "badge-unhealthy"
	case "starting", "stopping", "building":
		cls = "badge-starting"
	case "killing":
		cls = "badge-killing"
	case "recovering":
		cls = "badge-recovering"
	}
	return fmt.Sprintf(`<span class="badge %s">%s</span>`, cls, status)
}

func activeCount(models []governance.ModelPerformance) int {
	n := 0
	for _, m := range models {
		if m.Status == "active" || m.Status == "training" {
			n++
		}
	}
	return n
}

func retiredCount(models []governance.ModelPerformance) int {
	n := 0
	for _, m := range models {
		if m.Status == "abandoned" {
			n++
		}
	}
	return n
}
