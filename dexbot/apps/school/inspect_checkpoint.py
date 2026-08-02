#!/usr/bin/env python3
# ##############################################################################
# File Name        : inspect_checkpoint.py
# File Path        : apps/school/inspect_checkpoint.py
# Description      : Multi-path search checkpoint inspection tool.
# ##############################################################################

import os
import sys
import json
import glob

def find_checkpoint_file():
    script_dir = os.path.dirname(os.path.abspath(__file__))
    cwd = os.getcwd()

    candidates = [
        os.path.join(script_dir, "lstm_ga_checkpoint.json"),
        os.path.join(cwd, "lstm_ga_checkpoint.json"),
        "/workspace/crypto_apps/dexbot/apps/school/lstm_ga_checkpoint.json",
    ]

    for c in candidates:
        if os.path.exists(c):
            return c

    search_patterns = [
        os.path.join(script_dir, "logs", "**", "lstm_ga_checkpoint.json"),
        os.path.join(cwd, "logs", "**", "lstm_ga_checkpoint.json"),
        "/workspace/crypto_apps/dexbot/apps/school/logs/**/lstm_ga_checkpoint.json",
    ]

    found_files = []
    for pattern in search_patterns:
        found_files.extend(glob.glob(pattern, recursive=True))

    if found_files:
        found_files.sort(key=os.path.getmtime, reverse=True)
        return found_files[0]
    return None

def main():
    ckpt_path = find_checkpoint_file()

    if not ckpt_path:
        print("❌ Error: No active checkpoint file found in root or logs/* directories.")
        sys.exit(1)

    print("=" * 80)
    print(f"🔍 [CHECKPOINT INSPECTION] Source: {ckpt_path}")
    print("=" * 80)

    try:
        with open(ckpt_path, "r") as f:
            data = json.load(f)

        run_id = data.get("run_id", "UNKNOWN")
        gen = data.get("generation", 1)
        chromosomes = data.get("chromosome_population", [])

        evaluated = [c for c in chromosomes if c.get("fitness_evaluated", False)]

        print(f"🆔 Run ID            : {run_id}")
        print(f"🧬 Generation        : {gen}")
        print(f"📊 Total Population  : {len(chromosomes)}")
        print(f"✅ Evaluated Models  : {len(evaluated)} / {max(1, len(chromosomes))} ({len(evaluated)/max(1, len(chromosomes))*100:.1f}%)")
        print("-" * 80)

        for chrom in chromosomes:
            c_id = chrom.get("id", "N/A")
            eval_status = "✅ EVALUATED" if chrom.get("fitness_evaluated", False) else "⏳ PENDING"
            perf = chrom.get("perf_vector", [0.0, 0.0, 0.0, 0.0])
            
            print(f"  ID: {c_id:<7} | Status: {eval_status}")
            if chrom.get("fitness_evaluated", False) and len(perf) >= 4:
                print(f"      ├── Skill DA : {perf[0]*100:+.2f}%")
                print(f"      ├── Sharpe   : {perf[1]:.2f}")
                print(f"      ├── MaxDD    : {perf[2]*100:.2f}%")
                print(f"      └── RMSE     : {perf[3]:.6f}")

        print("=" * 80)

    except Exception as e:
        print(f"💥 [ERROR] Failed reading checkpoint: {e}")

if __name__ == "__main__":
    main()