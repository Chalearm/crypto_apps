#!/usr/bin/env python3
# ##############################################################################
# File Name        : generate_test_checkpoint.py
# File Path        : apps/school/generate_test_checkpoint.py
# Description      : Generates realistic mock GA-LSTM population data and saves
#                    it atomically to both the root and deployed_models paths.
# ##############################################################################

import os
import json
import time
import random
import uuid

# Configuration constants matching real GA settings
RUN_ID = uuid.uuid4().hex[:8].upper()
APP_ROOT = "/workspace/crypto_apps/dexbot/apps/school"
TOTAL_GENERATIONS = 3
MODELS_PER_GEN = 2

FEATURE_POOL = [
    'price_log_return_uniswap', 'volume_log_change_uniswap', 
    'price_log_return_solana', 'volume_log_change_solana', 
    'price_log_return_spy', 'volume_log_change_spy', 'volume_raw_spy', 
    'price_log_return_ethereum', 'volume_log_change_ethereum', 'volume_raw_ethereum', 
    'price_log_return_bitcoin', 'volume_log_change_bitcoin', 
    'price_log_return_binance', 'volume_log_change_binance', 'volume_raw_binance', 
    'price_log_return_fed', 'volume_raw_fed', 
    'price_log_return_oil', 'volume_log_change_oil', 'volume_raw_oil', 
    'price_log_return_gold', 'volume_log_change_gold', 'volume_raw_gold'
]

def generate_mock_chromosome(gen_idx, model_idx, is_evaluated=False):
    """Creates a single chromosome matching real GA-LSTM JSON schema."""
    num_layers = random.randint(1, 3)
    nodes = [random.choice([32, 64, 128, 256]) for _ in range(num_layers)]
    
    # Feature mask selection
    num_selected = random.randint(5, len(FEATURE_POOL))
    mask = [1] * num_selected + [0] * (len(FEATURE_POOL) - num_selected)
    random.shuffle(mask)

    if is_evaluated:
        skill_da = round(random.uniform(-0.05, 0.05), 4)
        sharpe = round(random.uniform(2.0, 25.0), 2)
        max_dd = round(random.uniform(0.0, 0.15), 4)
        rmse = round(random.uniform(0.12, 0.22), 6)
        cagr = round(random.uniform(1.5, 5.0), 2)
        win_ratio = round(random.uniform(85.0, 100.0), 2)
        perf_vector = [skill_da, sharpe, max_dd, rmse, cagr, win_ratio]
    else:
        perf_vector = [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]

    return {
        "id": f"G{gen_idx}-M{model_idx}",
        "lstm_layers": num_layers,
        "nodes_per_layer": nodes,
        "lookback_window": random.randint(60, 150),
        "forecast_horizon": random.randint(14, 60),
        "learning_rate": round(random.uniform(0.0001, 0.005), 5),
        "dropout_rate": round(random.uniform(0.1, 0.4), 2),
        "batch_size": random.choice([16, 32, 64]),
        "feature_mask": mask,
        "fitness_evaluated": is_evaluated,
        "perf_vector": perf_vector
    }

def main():
    population = []
    
    print(f"🎲 Generating synthetic checkpoint for Run ID: {RUN_ID}")
    
    # Generate G1 - G3 population (Mix of evaluated and pending models)
    for gen in range(1, TOTAL_GENERATIONS + 1):
        for m in range(MODELS_PER_GEN):
            # Gen 1 evaluated, Gen 2 partially evaluated, Gen 3 pending
            if gen == 1:
                evaluated = True
            elif gen == 2:
                evaluated = (m == 0)  # First model of Gen 2 evaluated
            else:
                evaluated = False

            chrom = generate_mock_chromosome(gen, m, is_evaluated=evaluated)
            population.append(chrom)

    checkpoint_payload = {
        "run_id": RUN_ID,
        "generation": 2,
        "chromosome_population": population,
        "chromosomes": population,  # Dual-key for Python and Go CLI compatibility
        "timestamp": time.time()
    }

    # Prepare target file locations
    root_ckpt_path = os.path.join(APP_ROOT, "lstm_ga_checkpoint.json")
    models_dir = os.path.join(APP_ROOT, "deployed_models", RUN_ID)
    os.makedirs(models_dir, exist_ok=True)
    deployed_ckpt_path = os.path.join(models_dir, "checkpoint.json")

    target_paths = [root_ckpt_path, deployed_ckpt_path]

    # Perform atomic write with OS buffer flush
    for path in target_paths:
        tmp_path = f"{path}.tmp"
        with open(tmp_path, "w") as f:
            json.dump(checkpoint_payload, f, indent=2)
            f.flush()
            os.fsync(f.fileno())
        os.replace(tmp_path, path)
        print(f"✅ Saved synthetic checkpoint to: {path}")

    print("\n🎉 Test dataset ready! Run the inspection command to verify.")

if __name__ == "__main__":
    main()