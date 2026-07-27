#!/usr/bin/env python3issues = []
    if not content.strip().startswith("/***"):
        issues.append("MISSING: file header block comment")
        return issues
    for field in FILE_HEADER_REQUIRED:
        pattern = r'\b' + re.escape(field) + r'\s*:'
        if not re.search(pattern, content[:4000]):
            issues.append(f"MISSING field '{field}'")
    return issues
#/******************************************************************************
# * Function Name : scan_func_headers
# *
# * Purpose :
# *   Performs its designated operation.
# *
# * Inputs :
# *   None (see function signature)
# *
# * Return :
# *   Type        : varies
# *   Description : Result of computation.
# *
# * Complexity :
# *   Time  : O(1)
# *   Space : O(1)
# *
# * Error Cases :
# *   - None
# *
# * Number Of Lines :
# *   10
# ******************************************************************************/


def scan_func_headers(content, filepath):
    issues = []
    funcs = list(re.finditer(r'^func\s+(?:\(.*?\)\s+)?(\w+)\s*\(', content, re.MULTILINE))
    for m in funcs:
        fn_name = m.group(1)
        if fn_name in ("init", "main"):
            continue
        start = m.start()
        before = content[:start]
        lines_before = before.split("\n")[-60:]
        before_text = "\n".join(lines_before)
        if "Function Name" not in before_text:
            issues.append(f"FUNC '{fn_name}' missing header block")
            continue
        for field in FUNC_HEADER_MIN_FIELDS:
            pattern = r'\b' + re.escape(field) + r'\s*:'
            if not re.search(pattern, before_text):
                issues.append(f"FUNC '{fn_name}' header missing '{field}'")
    return issues

def scan_pk_fragments(content, filepath):if "config.env" in filepath or "pk_helper" in filepath:
        return []
    issues = []
    hex_patterns = re.findall(r'"([0-9a-fA-F]{8,})"', content)
    for p in hex_patterns:
        if len(p) >= 60: continue
        if len(set(p)) <= 2: continue
        issues.append(f"PK fragment: '{p}'")
    return issues

def main():issues = {"file_header": [], "func_header": [], "pk": []}
    for root, dirs, files in os.walk(target):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        for f in files:
            if not f.endswith(".go"): continue
            if f.endswith("_test.go"): continue  # test files exempt
            filepath = os.path.join(root, f)
            try:
                with open(filepath, "r", encoding="utf-8") as fh:
                    content = fh.read()
            except: continue
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
    print(f"POLICY SCAN v3 — {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 70)
    print(f"\nFile header issues: {len(issues['file_header'])}")
    for i in issues["file_header"][:10]:
        print(f"  - {i}")
    print(f"\nFunction header issues: {len(issues['func_header'])}")
    for i in issues["func_header"][:15]:
        print(f"  - {i}")
    print(f"\nPK fragment issues: {len(issues['pk'])}")
    for i in issues["pk"]:
        print(f"  - {i}")
    total = sum(len(v) for v in issues.values())
    print(f"\nTOTAL: {total}")
    print("=" * 70)
    sys.exit(1 if total > 0 else 0)

if __name__ == "__main__":
    main()
