#!/usr/bin/env python3
"""Fix 2: remove orphaned text between Fprint closing paren and script tag"""
import os

fpath = os.path.join(os.path.dirname(__file__) or '.', 'webui', 'renderer.go')
with open(fpath, 'r') as f:
    content = f.read()

# Find the Fprint line for the DB browser
idx = content.find('fmt.Fprint(w, `<h2 style="margin-top:28px">Database Browser</h2>')
if idx < 0:
    idx = content.find('fmt.Fprintf(w, `<h2 style="margin-top:28px">Database Browser</h2>')
if idx < 0:
    print("ERROR: cannot find DB browser Fprint/Fprintf")
    exit(1)

# Find the script block after it
script_pos = content.find('</div>\n<script>\nfunction validateDBInput', idx)
if script_pos < 0:
    print("ERROR: cannot find script after Fprint")
    exit(1)

# Count parentheses from Fprint( to find the closing )
block = content[idx:script_pos]
depth = 0
closing_pos = None
for i, ch in enumerate(block):
    if ch == '(':
        depth += 1
    elif ch == ')':
        depth -= 1
        if depth == 0:
            closing_pos = idx + i
            break

if closing_pos is None:
    print("ERROR: could not find closing paren")
    exit(1)

# Remove orphaned text between closing paren and script
orphan = content[closing_pos+1:script_pos]
orphan_stripped = orphan.strip()
if orphan_stripped:
    print(f"Removing orphan ({len(orphan_stripped)} chars): {repr(orphan_stripped[:100])}")
    content = content[:closing_pos+1] + '\n' + content[script_pos:]
    with open(fpath, 'w') as f:
        f.write(content)
    print("FIXED: orphaned text removed")
else:
    print("No orphaned text found — file may already be correct")
