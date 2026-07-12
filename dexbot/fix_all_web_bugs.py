#!/usr/bin/env python3
"""Apply all web UI bug fixes to renderer.go"""
import os

fpath = '/workspace/crypto_apps/dexbot/webui/renderer.go'
with open(fpath, 'r') as f:
    content = f.read()

changes = 0

# FIX 1: DB dropdown max rows 25 -> 80
for old, new in [
    ('max="25" value="5" onchange="loadDBTable()" oninput="validateDBInput()"',
     'max="80" value="5" onchange="loadDBTable()" oninput="validateDBInput()"'),
    ('Max 25 rows', 'Max 80 rows'),
    ('isNaN(v)||v<1||v>25', 'isNaN(v)||v<1||v>80'),
]:
    if old in content:
        content = content.replace(old, new)
        changes += 1
        print(f"FIX: {old[:50]}")

# FIX 2: Chain total display next to chain row
old2 = '<span style="font-size:.7rem;color:var(--text-muted);margin-left:12px">1 BTC = <span id="btcPrice">...</span> USD</span>'
new2 = '<span id="chainTotalDisplay" style="font-family:monospace;font-size:.8rem;color:var(--accent);margin-left:auto">$ ******</span><span style="font-size:.7rem;color:var(--text-muted);margin-left:12px">1 BTC = <span id="btcPrice">...</span> USD</span>'
if old2 in content:
    content = content.replace(old2, new2)
    changes += 1
    print("FIX: chainTotalDisplay added")

# FIX 3: Update refreshAssetPanel to set chainTotalDisplay
old3 = 'function refreshAssetPanel(){\n  renderAssetRows();\n  updateDropdownOptionLabels();\n  var btcChk2'
new3 = 'function refreshAssetPanel(){\n  renderAssetRows();\n  updateDropdownOptionLabels();\n  var csel=document.getElementById("chainSelect");var cv=csel?csel.value:"BSC";var btcChk=document.getElementById("btcToggle").checked;var sym=btcChk?"BTC ":"$ ";var ct=computeChainBalances();var chainVal=ct[cv]||0;var disp=btcChk?(chainVal/btcPrice):chainVal;document.getElementById("chainTotalDisplay").textContent=showAllNumbers?sym+format9Decimal(disp):"******";\n  var btcChk2'
if old3 in content:
    content = content.replace(old3, new3)
    changes += 1
    print("FIX: chainTotalDisplay update in refreshAssetPanel")

# FIX 4: saveToken POST to API
old4 = """function saveToken() {
  var ticker = document.getElementById('tokTicker').value.trim().toUpperCase();
  var addr = document.getElementById('tokAddr').value.trim();
  var targetChain = document.getElementById('chainEditSelect').value;
  if(!ticker || !addr) { alert('Parameters missing.'); return; }
  
  assetsData.push({
    Ticker: ticker,
    Amount: 1250.000000000,
    USDPrice: 1.00,
    USDValue: 1250.00,
    ChainID: "56",
    ChainName: targetChain
  });
  
  hideAddTokenFields();
  renderExistingTokens();
  renderAssetRows();
}"""
new4 = """function saveToken() {
  var ticker = document.getElementById('tokTicker').value.trim().toUpperCase();
  var addr = document.getElementById('tokAddr').value.trim();
  var targetChain = document.getElementById('chainEditSelect').value;
  if(!ticker || !addr) { alert('Parameters missing.'); return; }
  fetch('/api/verify/token/add',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({ticker:ticker,address:addr,chain_name:targetChain,chain_id:targetChain==='BSC'?'56':targetChain==='POLYGON'?'137':'1'})})
  .then(r=>r.json()).then(d=>{
    if(d.status==='ok'){
      assetsData.push({Ticker:ticker,Amount:0,USDPrice:0,USDValue:0,ChainID:targetChain==='BSC'?'56':'137',ChainName:targetChain,bsc_addr:addr});
      hideAddTokenFields();
      renderExistingTokens();
      renderAssetRows();
    } else { alert(d.message||'Save failed'); }
  }).catch(function(e){alert('Cannot reach server');});
}"""
if old4 in content:
    content = content.replace(old4, new4)
    changes += 1
    print("FIX: saveToken wired to API")

# FIX 5: saveChain POST to API
old5 = """function saveChain() {
  var name = document.getElementById('chainNameInput').value.trim().toUpperCase();
  var id = document.getElementById('chainIdInput').value.trim();
  if(!name || !id) { alert('Dynamic fields required.'); return; }
  
  var selectBox = document.getElementById('chainSelect');
  var editSelectBox = document.getElementById('chainEditSelect');
  
  var newOption = document.createElement('option');
  newOption.value = name;
  newOption.textContent = name;
  selectBox.insertBefore(newOption, selectBox.lastChild);
  
  var newEditOption = document.createElement('option');
  newEditOption.value = name;
  newEditOption.textContent = name;
  editSelectBox.appendChild(newEditOption);
  
  selectBox.value = name;
  closeChainEditor();
  refreshAssetPanel();
}"""
new5 = """function saveChain() {
  var name = document.getElementById('chainNameInput').value.trim().toUpperCase();
  var id = document.getElementById('chainIdInput').value.trim();
  if(!name || !id) { alert('Dynamic fields required.'); return; }
  fetch('/api/verify/chain/add',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({name:name,id:id,base_url:''})})
  .then(r=>r.json()).then(d=>{
    if(d.status==='ok'){
      var selectBox = document.getElementById('chainSelect');
      var editSelectBox = document.getElementById('chainEditSelect');
      var newOption = document.createElement('option');
      newOption.value = name;
      newOption.textContent = name;
      selectBox.insertBefore(newOption, selectBox.lastChild);
      var newEditOption = document.createElement('option');
      newEditOption.value = name;
      newEditOption.textContent = name;
      editSelectBox.appendChild(newEditOption);
      selectBox.value = name;
      closeChainEditor();
      refreshAssetPanel();
    } else { alert(d.message||'Chain add failed'); }
  }).catch(function(e){alert('Cannot reach server');});
}"""
if old5 in content:
    content = content.replace(old5, new5)
    changes += 1
    print("FIX: saveChain wired to API")

# FIX 6: BTC price use real value from AJAX, not initial embedded
old6 = "document.getElementById('btcPrice').textContent = format9Decimal(btcPrice);"
new6 = "document.getElementById('btcPrice').textContent = format9Decimal(btcPrice);\n  var btcEl2=document.getElementById('btcPrice');if(btcEl2)btcEl2.textContent=format9Decimal(btcPrice);"
# Actually the BTC price IS already using real AJAX data. Clean.

# FIX 7: Add base_url input to chain editor
old7 = '<input id="chainIdInput" placeholder="Chain ID Value Integer (e.g. 137)" style="padding:8px; border-radius:6px; border:1px solid var(--border); background:var(--bg-deep); color:var(--text-primary); margin-bottom:14px; width:100%%; font-size:0.8rem;">'
new7 = '<input id="chainIdInput" placeholder="Chain ID Value Integer (e.g. 137)" style="padding:8px; border-radius:6px; border:1px solid var(--border); background:var(--bg-deep); color:var(--text-primary); margin-bottom:10px; width:100%%; font-size:0.8rem;">\n    <input id="chainBaseUrlInput" placeholder="RPC Base URL (e.g. https://polygon-rpc.com)" style="padding:8px; border-radius:6px; border:1px solid var(--border); background:var(--bg-deep); color:var(--text-primary); margin-bottom:14px; width:100%%; font-size:0.8rem;">'
if old7 in content:
    content = content.replace(old7, new7)
    changes += 1
    print("FIX: base_url input added to chain editor")

# Update saveChain to read base_url
old7b = "var id = document.getElementById('chainIdInput').value.trim()"
new7b = "var id = document.getElementById('chainIdInput').value.trim();\n  var baseUrl = document.getElementById('chainBaseUrlInput').value.trim()"
if old7b in content:
    content = content.replace(old7b, new7b)
    changes += 1
    print("FIX: saveChain reads baseUrl")

old7c = "body:JSON.stringify({name:name,id:id,base_url:''})"
new7c = "body:JSON.stringify({name:name,id:id,base_url:baseUrl})"
if old7c in content:
    content = content.replace(old7c, new7c)
    changes += 1
    print("FIX: saveChain passes baseUrl to API")

with open(fpath, 'w') as f:
    f.write(content)
print(f"\nTotal changes: {changes}")
