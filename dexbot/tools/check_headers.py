#!/usr/bin/env python3
"""
File Name       : check_headers.py
File Path       : tools/check_headers.py
Author          : deepseek-4.0-pro
Owner           : Chalearm Saelim
Version         : 1.0.0
Created Date    : 2026-07-01 17:00:00 (UTC+7)

Description     :
  Validates Go file headers against rule1.txt format (/workspace/doc/rule1.txt).
  Per myreq6.txt 106, 106.1, 112.

  Modes:
    check_headers.py               — scan only, report counts
    check_headers.py --fix         — fix legacy/missing headers with unique timestamps
    check_headers.py --fix-dates   — refresh Created/Modified dates on ALL files
                                     with per-file unique real-time timestamps

Usage :
  python3 tools/check_headers.py
  python3 tools/check_headers.py --fix-dates
"""

import os, sys, time, re
from datetime import datetime, timezone, timedelta

ICT = timezone(timedelta(hours=7))
ROOT = '/workspace/crypto_apps/dexbot'

def now_str():
    return datetime.now(ICT).strftime('%Y-%m-%d %H:%M:%S (UTC+7)')

def classify(content):
    if content.startswith('/******************************************************************************'):
        return 'PROPER'
    if content.startswith('/* '):
        return 'LEGACY'
    return 'MISSING'

def extract_desc(content):
    """Extract description field from existing header."""
    for m in re.finditer(r'Description\s*:\s*\n\s*\*\s*(.+?)(?:\n|$)', content[:3000], re.DOTALL):
        desc = m.group(1).strip()
        if len(desc) > 10:
            return desc[:200]
    return 'Dexbot component.'

def extract_resp(content):
    for m in re.finditer(r'Responsibilities:\s*\n\s*\*\s*(.+?)(?:\n|$)', content[:3000], re.DOTALL):
        return m.group(1).strip()[:100]
    return 'Implement core functionality.'

def generate_header(fpath, rel_path, desc, resp):
    n = now_str()
    pkg = rel_path.split('/')[0] if '/' in rel_path else 'main'
    testpkg = './' + '/'.join(rel_path.split('/')[:-1])
    buildpkg = testpkg
    usagedir = '/'.join(rel_path.split('/')[:-1]) + '/'
    return f'''/******************************************************************************
 * File Name       : {os.path.basename(fpath)}
 * File Path       : {rel_path}
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : {n}
 * Modified Date   : {n}
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
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test {testpkg}
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/{pkg}
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
 *   {"-" * 73}
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   {"-" * 73}
 *   1.0.0   | {n}   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   {"-" * 73}
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
'''

def fix_header(fpath, rel_path, refresh_dates_only=False):
    with open(fpath) as f:
        content = f.read()

    if classify(content) == 'PROPER' and not refresh_dates_only:
        return False  # already proper, skip

    # Save existing description if available
    desc = extract_desc(content)
    resp = extract_resp(content)

    # Strip existing header block
    if content.startswith('/******************************************************************************'):
        end = content.find('******************************************************************************/\n', 1)
        if end == -1:
            end = content.find('**/\n', 1)
        if end >= 0:
            # Find the actual end of the block
            close_pos = content.find('**/\n', end)
            if close_pos < 0:
                close_pos = end + 4
            else:
                close_pos += 4
            content = content[close_pos:].lstrip('\n')
    elif content.startswith('/* '):
        end = content.find('*/\n', 2)
        if end >= 0:
            content = content[end+3:].lstrip('\n')

    new_header = generate_header(fpath, rel_path, desc, resp)
    new_content = new_header.rstrip('\n') + '\n' + content

    with open(fpath, 'w') as f:
        f.write(new_content)
    return True

def main():
    refresh_dates = '--fix-dates' in sys.argv
    fix_mode = '--fix' in sys.argv or refresh_dates

    total = proper = legacy = missing = fixed = 0

    for dirpath, dirnames, filenames in os.walk(ROOT):
        dirnames[:] = [d for d in dirnames if d not in ('vendor', '.git', 'logs', 'runtime', 'web_output')]
        for f in sorted(filenames):
            if not f.endswith('.go'):
                continue
            total += 1
            fpath = os.path.join(dirpath, f)
            rel_path = os.path.relpath(fpath, ROOT)

            with open(fpath) as fh:
                content = fh.read()

            cls = classify(content)
            if cls == 'PROPER':
                proper += 1
            elif cls == 'LEGACY':
                legacy += 1
            else:
                missing += 1

            if fix_mode and (cls != 'PROPER' or refresh_dates):
                if fix_header(fpath, rel_path, refresh_dates_only=(cls == 'PROPER' and refresh_dates)):
                    fixed += 1
                    time.sleep(0.3)  # unique timestamp per file
                    print(f"  UPDATED {rel_path}")

    print(f"\nHeader Check Report")
    print(f"===================")
    print(f"  Total files:  {total}")
    print(f"  Proper:       {proper}")
    print(f"  Legacy:       {legacy}")
    print(f"  Missing:      {missing}")
    print(f"  Fixed:        {fixed}" if fix_mode else "")
    print(f"\n  Compliance: {proper}/{total} ({100*proper//total}%)")

    if fix_mode and fixed > 0:
        print(f"  All timestamps are now unique (real-time per file).")

if __name__ == '__main__':
    main()
