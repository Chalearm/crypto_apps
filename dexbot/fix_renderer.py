#!/usr/bin/env python3
"""Fix renderer.go: dbTableOptions syntax error"""
import os

fpath = os.path.join(os.path.dirname(__file__) or '.', 'webui', 'renderer.go')
with open(fpath, 'r') as f:
    content = f.read()

# Find and fix the broken area
# The issue: Fprintf with %s was changed but dbTableOptions was left as , arg
# Fix: change Fprintf to Fprint with string concat
old_marker = 'dbTableOptions)'
idx = content.find(old_marker)
if idx >= 0:
    # Find the fmt.Fprintf line before it
    block_start = content.rfind('fmt.Fprintf', 0, idx)
    if block_start >= 0:
        # Change fmt.Fprintf to fmt.Fprint
        content = content[:block_start] + 'fmt.Fprint' + content[block_start+11:]
        # Replace %s in the backtick with string concat
        # Find the %s inside the select
        pct_s = content.find('      %s\n    </select>', block_start, idx)
        if pct_s >= 0:
            # Replace '%s' with '` + dbTableOptions + `'
            content = content[:pct_s] + '      ` + dbTableOptions + `\n    </select>' + content[pct_s+23:]
            # Remove the , dbTableOptions) argument from Fprintf (now Fprint)
            comma_idx = content.find(', dbTableOptions)', block_start)
            if comma_idx >= 0:
                content = content[:comma_idx] + content[comma_idx+len(', dbTableOptions)'):]
            print("FIXED: dbTableOptions converted to string concatenation")
        else:
            print("pct_s not found")
    else:
        print("fmt.Fprintf not found before dbTableOptions")
else:
    print("dbTableOptions) not found")

with open(fpath, 'w') as f:
    f.write(content)
