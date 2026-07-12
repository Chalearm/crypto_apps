#!/usr/bin/env python3
"""Fix orphaned HTML in renderer.go — wrap in fmt.Fprint"""
fpath = '/workspace/crypto_apps/dexbot/webui/renderer.go'
with open(fpath, 'r') as f:
    lines = f.readlines()

# Find orphaned </div> near line 1202 (after DB browser Fprint ended)
fixed = 0
for i in range(1200, min(1260, len(lines))):
    stripped = lines[i].rstrip()
    prev = lines[i-1].rstrip() if i > 0 else ''
    if stripped == '</div>' and not prev.endswith('`') and 'Fprint' not in prev and 'Fprintf' not in prev:
        lines[i] = '\tfmt.Fprint(w, `</div>\n'
        fixed += 1
        break

if fixed == 0:
    # Try a different approach: find line that is just '</div>' with no fmt prefix
    for i in range(1180, min(1300, len(lines))):
        if lines[i].strip() == '</div>' and '`' not in lines[i-1][-5:]:
            lines[i] = '\tfmt.Fprint(w, `</div>\n'
            fixed += 1
            break

# Find closing </script> in the DB section and add closing backtick+paren
for i in range(1220, min(1310, len(lines))):
    if '</script>' in lines[i] and 'fmt.Fprint' not in lines[i]:
        # Check if already has closing paren
        if '`)' not in lines[i]:
            lines[i] = lines[i].replace('</script>', '</script>`)\n')
            fixed += 1
            break

if fixed > 0:
    with open(fpath, 'w') as f:
        f.writelines(lines)
    print(f'Fixed {fixed} lines')
else:
    print('No fix needed — file may already be correct')
