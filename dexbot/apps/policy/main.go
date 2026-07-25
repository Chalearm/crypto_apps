#!/usr/bin/env python3
"""
policy_scan.py — Scans all Go files in dexbot/ for:
  1. Missing or malformed file header comment (per rule1.txt)
  2. Missing or malformed function header comment (per rule1.txt §5)
  3. Lines containing private key fragments
  4. Prints a summary report.

Usage: python3 policy_scan.py [path_to_dexbot]
"""
import os, re, sys
from datetime import datetime

target = sys.argv[1] if len(sys.argv) > 1 else "."
FILE_HEADER_REQUIRED = ["File Name", "File Path", "Author", "Owner", "Version", "Created Date", "Description", "Usage"]
FUNC_HEADER_REQUIRED = ["Function Name", "Purpose", "Inputs", "Outputs", "Return", "Error Cases"]

def scan_file_header(content, filepath):
    """Check that the file starts with a proper block comment header."""
    issues = []
    if not content.strip().startswith("/****"):
        issues.append("Missing file header block comment")
        return issues
    # Check required fields
    for field in FILE_HEADER_REQUIRED:
        if field not in content.split("****")[0] if "****" in content else content[:2000]:
            pass  # rough check — we look for "field :"
        pattern = re.escape(field) + r"\s*:"
        if not re.search(pattern, content[:3000]):
            issues.append(f"File header missing field: {field}")
    return issues

def scan_func_headers(content, filepath):
    """Check that each function has a proper /*** header block before it."""
    issues = []
    funcs = re.finditer(r'^func\s+(?:\(.*?\)\s+)?(\w+)', content, re.MULTILINE)
    for m in funcs:
        fn_name = m.group(1)
        if fn_name in ("init", "main"):
            continue
        start = m.start()
        # Look at the 40 lines before the function for a /*** block
        before = content[:start]
        lines_before = before.split("\n")[-40:] if before else []
        before_text = "\n".join(lines_before)
        if "Function Name" not in before_text:
            issues.append(f"Function '{fn_name}' missing header comment block")
            continue
        # Check required fields
        for field in FUNC_HEADER_REQUIRED:
            if field not in before_text:
                issues.append(f"Function '{fn_name}' header missing field: {field}")
    return issues

def scan_pk_fragments(content, filepath):
    """Check for any 6+ consecutive hex chars that could be a PK fragment."""
    issues = []
    if "config.env" in filepath or "pk_helper" in filepath:
        return issues
    # Look for patterns that look like partial keys (8+ hex chars in a string literal)
    hex_patterns = re.findall(r'"([0-9a-fA-F]{8,})"', content)
    for p in hex_patterns:
        # Skip if it's an obvious address (0x prefix in nearby context)
        # Skip SHA256 hashes (64 chars)
        if len(p) >= 60:
            continue
        issues.append(f"Potential PK fragment: '{p}'")
    return issues

def main():
    issues = {"file_header": [], "func_header": [], "pk": []}
    for root, dirs, files in os.walk(target):
        dirs[:] = [d for d in dirs if d not in (".git", "node_modules", "__pycache__", "vendor", "runtime", "logs", "web_output", "data")]
        for f in files:
            if not f.endswith(".go"):
                continue
            filepath = os.path.join(root, f)
            try:
                with open(filepath, "r", encoding="utf-8") as fh:
                    content = fh.read()
            except:
                continue
            fh_issues = scan_file_header(content, filepath)
            fh_issues2 = scan_func_headers(content, filepath)
            fh_issues3 = scan_pk_fragments(content, filepath)
            if fh_issues:
                issues["file_header"].extend([f"{filepath}: {i}" for i in fh_issues])
            if fh_issues2:
                issues["func_header"].extend([f"{filepath}: {i}" for i in fh_issues2])
            if fh_issues3:
                issues["pk"].extend([f"{filepath}: {i}" for i in fh_issues3])

    print("=" * 70)
    print(f"POLICY COMPLIANCE SCAN — {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 70)
    print(f"\nFile header issues: {len(issues['file_header'])}")
    for i in issues["file_header"][:20]:
        print(f"  - {i}")
    if len(issues["file_header"]) > 20:
        print(f"  ... and {len(issues['file_header'])-20} more")

    print(f"\nFunction header issues: {len(issues['func_header'])}")
    for i in issues["func_header"][:20]:
        print(f"  - {i}")
    if len(issues["func_header"]) > 20:
        print(f"  ... and {len(issues['func_header'])-20} more")

    print(f"\nPK fragment issues: {len(issues['pk'])}")
    for i in issues["pk"]:
        print(f"  - {i}")

    total = sum(len(v) for v in issues.values())
    print(f"\nTOTAL ISSUES: {total}")
    print("=" * 70)

if __name__ == "__main__":
    main()
