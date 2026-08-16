#!/usr/bin/env python3
# ##############################################################################
# File Name        : inspect_checkpoint.py
# File Path        : apps/school/inspect_checkpoint.py
# Description      : Tabular checkpoint inspection tool with horizontal layout separators.
# ##############################################################################

import os
import sys
import json
import glob

def find_checkpoint_file():
    """Locates the active lstm_ga_checkpoint.json file across known paths."""
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

    print("═" * 110)
    print(f"🔍 [CHECKPOINT INSPECTION] Source: {ckpt_path}")
    print("═" * 110)

    try:
        with open(ckpt_path, "r") as f:
            data = json.load(f)

        run_id = data.get("run_id", "UNKNOWN")
        gen = data.get("generation", 1)
        chromosomes = data.get("chromosome_population", [])

        evaluated = [c for c in chromosomes if c.get("fitness_evaluated", False)]
        total_pop = max(1, len(chromosomes))
        eval_ratio = (len(evaluated) / total_pop) * 100.0

        print(f"🆔 Run ID            : {run_id}")
        print(f"🧬 Active Generation : Generation {gen}")
        print(f"📊 Total Population  : {len(chromosomes)} models")
        print(f"✅ Evaluated Models  : {len(evaluated)} / {total_pop} ({eval_ratio:.1f}%)")
        print("─" * 110)

        # Tabular Header Output
        header = f"│ {'MODEL ID':<10} │ {'STATUS':<14} │ {'SKILL DA (%)':<14} │ {'SHARPE RATIO':<14} │ {'MAX DD (%)':<14} │ {'VAL RMSE':<14} │"
        
        print("┌" + "─" * 108 + "┐")
        print(header)
        print("├" + "─" * 12 + "┼" + "─" * 16 + "┼" + "─" * 16 + "┼" + "─" * 16 + "┼" + "─" * 16 + "┼" + "─" * 16 + "┤")

        for chrom in chromosomes:
            c_id = chrom.get("id", "N/A")
            is_eval = chrom.get("fitness_evaluated", False)
            eval_status = "✅ EVALUATED" if is_eval else "⏳ PENDING"
            perf = chrom.get("perf_vector", [0.0, 0.0, 0.0, 0.0])

            if is_eval and len(perf) >= 4:
                da_str = f"{perf[0]*100:+.2f}%"
                sharpe_str = f"{perf[1]:.2f}"
                maxdd_str = f"{perf[2]*100:.2f}%"
                rmse_str = f"{perf[3]:.6f}"
            else:
                da_str = "N/A"
                sharpe_str = "N/A"
                maxdd_str = "N/A"
                rmse_str = "N/A"

            row_str = f"│ {c_id:<10} │ {eval_status:<14} │ {da_str:<14} │ {sharpe_str:<14} │ {maxdd_str:<14} │ {rmse_str:<14} │"
            print(row_str)

        print("└" + "─" * 108 + "┘")
        print("═" * 110)

    except Exception as e:
        print(f"💥 [ERROR] Failed reading checkpoint: {e}")

if __name__ == "__main__":
    main()
