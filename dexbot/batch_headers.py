#!/usr/bin/env python3
"""
batch_headers.py — Auto-generates rule1.txt compliant headers for all Go files
in the dexbot project. Reads existing legacy header for description hints,
generates proper /***** format headers per /workspace/doc/rule1.txt.

Usage:
  cd /workspace/crypto_apps/dexbot
  python3 batch_headers.py          # dry-run (shows changes)
  python3 batch_headers.py --apply  # actually write files
"""
import os, sys, re, time, argparse
from datetime import datetime, timezone, timedelta

ICT = timezone(timedelta(hours=7))
NOW = datetime.now(ICT).strftime("%Y-%m-%d %H:%M:%S (UTC+7)")

HEADER = '''/******************************************************************************
 * File Name       : {fname}
 * File Path       : {fpath}
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : {created}
 * Modified Date   : {modified}
 *
 * Description     :
 *   {desc}
 *
 * Responsibilities:
 *   - {resp}
 *
 * Usage :
 *   Directory : {usagedir}
 *
 *   Build :
 *     go build {buildpkg}
 *
 *   Run :
 *     {runcmd}
 *
 *   Test :
 *     go test {testpkg}
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/{pkg_parent}
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
 *   {newparts}
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | {created}   | deepseek-4.0-pro | Initial version — rule1.txt header batch
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
'''

def get_pkg(fpath):
    """Extract package information from file path"""
    rel = fpath.replace('/workspace/crypto_apps/dexbot/', '')
    parts = rel.split('/')
    pkg = parts[0] if len(parts) > 1 else 'main'
    if parts[-1].endswith('_test.go'):
        testpkg = './' + '/'.join(parts[:-1])
        buildpkg = ''
    else:
        testpkg = './' + '/'.join(parts[:-1])
        buildpkg = './' + '/'.join(parts[:-1])
    
    return {
        'rel': rel,
        'pkg': pkg,
        'testpkg': testpkg,
        'buildpkg': buildpkg if buildpkg else testpkg,
        'usagedir': '/'.join(parts[:-1]) + '/',
        'runcmd': 'go run .  (from dexbot root)',
        'pkg_parent': parts[0] if len(parts) > 1 else 'infra',
    }

def extract_legacy_desc(content):
    """Extract description from old legacy header style"""
    lines = content.split('\n')
    for i, line in enumerate(lines[:30]):
        stripped = line.strip()
        # Skip the "Description:" label line
        if 'Description:' in stripped or 'Description :' in stripped:
            desc_lines = []
            for j in range(i, min(i+15, len(lines))):
                l = lines[j].strip()
                if l.startswith('*/') or l.startswith('package ') or l.startswith('import '):
                    break
                if l.startswith('//') or l.startswith('*'):
                    continue
                if any(l.startswith(x) for x in ['Author:', 'Version:', 'Date:', 'Owner:', 
                    'Filename:', 'Usage:', 'Updated:', 'New:', 'Features:', 'Environment',
                    'Description:', 'Description :']):
                    continue
                if l and not l.startswith('/*'):
                    desc_lines.append(l)
            if desc_lines:
                raw = ' '.join(desc_lines)
                # Clean up common prefixes
                for prefix in ['Description: ', 'Description : ', '  ']:
                    if raw.startswith(prefix):
                        raw = raw[len(prefix):]
                return raw[:200]
    return 'Dexbot component — auto-documented per rule1.txt.'

def has_proper_header(content):
    """Check if file already has a proper /**** rule1 header"""
    return content.startswith('/****')

def process_file(fpath, apply=False):
    """Process a single Go file, adding/updating header"""
    with open(fpath, 'r') as f:
        content = f.read()
    
    # Skip files that already have proper rule1 headers
    if has_proper_header(content):
        return False, fpath + " (SKIP - already proper)"
    
    # Extract existing info
    desc = extract_legacy_desc(content)
    meta = get_pkg(fpath)
    fname = os.path.basename(fpath)
    
    # Remove existing legacy header (everything between first /* and */)
    if content.startswith('/*'):
        end = content.find('*/', 2)
        if end >= 0:
            content = content[end+3:].lstrip('\n')
    
    # Strip leading blank lines
    while content.startswith('\n') or content.startswith('\r'):
        content = content[1:]
    
    # Determine new parts description based on file contents
    newparts = '[Functions] All exported functions in this file'
    if 'func Test' in content:
        newparts = '[Test Functions] Test suite: ' + ', '.join(
            re.findall(r'func (Test\w+)', content)[:4])
    if 'type ' in content and 'struct' in content:
        newparts += '\n *   [Types] Struct definitions in this file'
    
    # Generate header
    header = HEADER.format(
        fname=fname,
        fpath=meta['rel'],
        created=NOW,
        modified=NOW,
        desc=desc.replace('\n','\n *   '),
        resp='Implement core functionality for ' + meta['pkg'] + ' package.',
        usagedir=meta['usagedir'],
        buildpkg=meta['buildpkg'],
        runcmd=meta['runcmd'],
        testpkg=meta['testpkg'],
        pkg_parent=meta['pkg_parent'],
        newparts=newparts,
    )
    
    new_content = header.rstrip('\n') + '\n' + content
    
    if apply:
        with open(fpath, 'w') as f:
            f.write(new_content)
        return True, fpath + " (UPDATED)"
    else:
        return True, fpath + " (DRY-RUN)"

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--apply', action='store_true', help='Actually write files')
    args = parser.parse_args()
    
    root = '/workspace/crypto_apps/dexbot'
    count = 0
    updated = 0
    skipped = 0
    
    for dirpath, dirnames, filenames in os.walk(root):
        # Skip vendor/build directories
        dirnames[:] = [d for d in dirnames if d not in ('vendor', '.git', 'logs', 'runtime', 'web_output')]
        
        for f in sorted(filenames):
            if not f.endswith('.go'):
                continue
            fpath = os.path.join(dirpath, f)
            changed, msg = process_file(fpath, apply=args.apply)
            count += 1
            if changed:
                updated += 1
            else:
                skipped += 1
            if 'UPDATED' in msg or 'DRY-RUN' in msg:
                print(msg)
    
    print(f"\n=== SUMMARY ===")
    print(f"Total .go files: {count}")
    print(f"Updated: {updated}")
    print(f"Skipped (already proper): {skipped}")
    print(f"Mode: {'APPLY' if args.apply else 'DRY RUN'}")

if __name__ == '__main__':
    main()
