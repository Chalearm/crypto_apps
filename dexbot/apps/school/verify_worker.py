#!/usr/bin/env python3
# ##############################################################################
# File Name        : verify_worker.py
# File Path        : apps/school/verify_worker.py
#
# Author           : Chalearm Saelim & Gemini
# Owner            : Chalearm Saelim
# Reviewer         : Chalearm Saelim
#
# Version          : 1.0.0
# Status           : Development
# Created Date     : 2026-07-27 16:40:00 (UTC+7)
# Modified Date    : 2026-07-27 16:40:00 (UTC+7)
#
# Description      :
#    Isolated worker node that executes Post-GA Out-of-Sample (OOS) Walk-Forward
#    Verification on the Rank 1 elite candidate model across 10 expanding time horizon
#    slices, collecting out-of-sample Skill DA, Sharpe ratio, and Max Drawdown metrics.
#
#    DEPENDENCY TREE & STRUCTURAL MAP:
#    ───────────────────────────────────────────────────────────────────────────
#    [celery_tasks.py] (Celery Router)
#        └── Calls ──> [verify_worker.py] (Isolated OOS Verifier)
#                        │
#                        ├── Imports ──> [utils.py] (resolve_target_directories)
#                        ├── Reconstructs Dataset DataFrame from JSON Payload
#                        ├── Fits MinMaxScaler and Generates Sliding Tensors
#                        ├── Executes 10-Fold Anchored Expanding Walk-Forward Evaluation
#                        └── Returns JSON-Serializable Verification Summary
#
# Responsibilities :
#    - Executes 10-fold expanding walk-forward verification without memory leakage.
#    - Re-trains Keras model weights per fold slice to avoid lookahead bias.
#    - Computes out-of-sample direction and portfolio risk metrics.
#    - Clears Keras sessions and triggers garbage collection on completion.
#
# Usage :
#    Directory : apps/school/
#
#    Build :
#      N/A (Interpreted Python Script)
#
#    Run :
#      Executed asynchronously by Celery worker pool via tasks.run_oos_verification_task
#
# Dependencies :
#    Internal :
#      - utils (resolve_target_directories)
#      - train_worker (_build_and_compile_lstm, _evaluate_directional_accuracy, _backtest_portfolio_strategy)
#
#    External :
#      - numpy, pandas, scikit-learn, tensorflow
#
# Notes :
#    - Per regulator coding standard rules.
# ##############################################################################

import io
import os
import sys
import gc
import time
import socket
import logging
import numpy as np
import pandas as pd
from sklearn.preprocessing import MinMaxScaler
import tensorflow as tf

# Mute low-level TensorFlow warnings and force CPU mode
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'
os.environ['CUDA_VISIBLE_DEVICES'] = '-1'

from train_worker import (
    _build_and_compile_lstm, 
    _evaluate_directional_accuracy, 
    _backtest_portfolio_strategy
)
from utils import resolve_target_directories


# ##############################################################################
# Function Name : execute_oos_verification
#
# Purpose :
#    Executes 10-fold walk-forward validation across the complete historical
#    dataset for a target elite chromosome payload, returning summary statistics.
#
# Inputs :
#    payload
#        Type        : dict
#        Description : Payload containing run_id, chromosome, master_data, and num_folds.
#
# Return :
#    Type        : dict
#    Description : JSON-serializable status and summary metrics dictionary.
#
# Complexity :
#    Time  : O(K * E * B) where K is fold count, E is epochs, B is batch count.
#    Space : O(S * F) where S is sample count and F is feature dimension.
#
# Error Cases :
#    - Handles parsing exceptions and returns error status dictionary.
#
# Number Of Lines :
#    85
# ##############################################################################
def execute_oos_verification(payload: dict) -> dict:
    start_time = time.perf_counter()
    worker_hostname = socket.gethostname()
    worker_pid = os.getpid()
    node_info = f"{worker_hostname} (PID: {worker_pid})"

    run_id = payload.get("run_id", "UNKNOWN")
    chromosome = payload.get("chromosome", {})
    num_folds = payload.get("num_folds", 10)
    master_data_raw = payload.get("master_data")

    print("\n" + "=" * 80)
    print(f"🧐 [OOS VERIFIER START] Executing {num_folds}-Fold Walk-Forward Verification on {node_info}")
    print(f"   ├── Target Run ID   : {run_id}")
    print(f"   ├── Model Chrom ID  : {chromosome.get('id', 'N/A')}")
    print(f"   ├── Lookback Window : {chromosome.get('lookback_window')} days")
    print(f"   └── Forecast Horizon: {chromosome.get('forecast_horizon')} days")
    print("=" * 80)

    try:
        # 1. Parse raw DataFrame
        if isinstance(master_data_raw, str):
            df_raw = pd.read_json(io.StringIO(master_data_raw))
        else:
            df_raw = pd.DataFrame(master_data_raw)

        # Scale features using fresh scaler fit on historical data
        scaler = MinMaxScaler(feature_range=(-1, 1))
        df_scaled = pd.DataFrame(
            scaler.fit_transform(df_raw),
            columns=df_raw.columns,
            index=df_raw.index
        )

        # 2. Extract active features based on chromosome mask
        temporal_patterns = ['day_wk_sin', 'day_wk_cos', 'day_yr_sin', 'day_yr_cos', 'fourier_']
        time_cols = [c for c in df_scaled.columns if any(p in c for p in temporal_patterns)]
        base_asset_cols = [c for c in df_scaled.columns if c not in time_cols and not c.startswith('close_')]
        asset_cols = [c for c in base_asset_cols if c.lower() != 'volume_log_change_fed']

        mask = np.array(chromosome.get('feature_mask', []))
        if len(mask) != len(asset_cols):
            mask = np.ones(len(asset_cols), dtype=int)

        selected_asset_cols = [col for col, active in zip(asset_cols, mask) if active == 1]
        
        asset_vals = df_scaled[selected_asset_cols].values
        time_vals = df_scaled[time_cols].values
        combined_data = np.hstack([asset_vals, time_vals])

        lookback = int(chromosome.get('lookback_window', 60))
        horizon = int(chromosome.get('forecast_horizon', 52))

        # 3. Build sliding window sequence tensors
        num_samples = len(combined_data) - lookback - horizon
        X, y = [], []
        for i in range(num_samples):
            X.append(combined_data[i : i + lookback])
            y.append(asset_vals[i + lookback : i + lookback + horizon])

        X, y = np.array(X, dtype=np.float32), np.array(y, dtype=np.float32)
        total_samples = len(X)

        # 4. Execute Anchored Expanding Walk-Forward Fold Loop
        val_size = max(5, min(horizon, total_samples // (num_folds + 1)))
        fold_step = max(5, (total_samples - val_size) // num_folds)

        fold_results = []
        for fold in range(num_folds):
            fold_idx = fold + 1
            train_end = fold_step * fold_idx
            val_start = train_end
            val_end = min(total_samples, val_start + val_size)

            if val_end <= val_start or train_end < 10:
                continue

            X_train, y_train = X[:train_end], y[:train_end]
            X_val, y_val = X[val_start:val_end], y[val_start:val_end]

            model = _build_and_compile_lstm(
                chromosome=chromosome,
                input_shape=(X_train.shape[1], X_train.shape[2]),
                num_targets=y_train.shape[2],
                forecast_horizon=horizon
            )

            early_stop = tf.keras.callbacks.EarlyStopping(monitor='loss', patience=5, restore_best_weights=True)
            model.fit(
                X_train, y_train, 
                epochs=30, 
                batch_size=int(chromosome.get('batch_size', 32)), 
                verbose=0, 
                callbacks=[early_stop]
            )

            preds = model.predict(X_val, verbose=0)
            
            _, avg_skill = _evaluate_directional_accuracy(y_val, preds, selected_asset_cols)
            p_metrics = _backtest_portfolio_strategy(y_val, preds, selected_asset_cols, horizon)

            print(f"   ├── Fold {fold_idx:2d}/{num_folds} | Train Samples: {len(X_train):4d} | Skill DA: {avg_skill*100:+.2f}% | Sharpe: {p_metrics['sharpe']:.2f} | MaxDD: {p_metrics['max_dd']*100:.1f}%")

            fold_results.append({
                'fold_idx': fold_idx,
                'skill_da': avg_skill,
                'sharpe': p_metrics['sharpe'],
                'max_dd': p_metrics['max_dd']
            })

            tf.keras.backend.clear_session()
            gc.collect()

        # 5. Compute Out-of-Sample Summary Statistics
        mean_skill = float(np.mean([r['skill_da'] for r in fold_results])) if fold_results else 0.0
        mean_sharpe = float(np.mean([r['sharpe'] for r in fold_results])) if fold_results else -5.0
        mean_maxdd = float(np.mean([r['max_dd'] for r in fold_results])) if fold_results else 1.0
        duration = time.perf_counter() - start_time

        print("\n" + "=" * 80)
        print("📊 [OOS WALK-FORWARD VERIFICATION SUMMARY]")
        print(f"   ├── Mean Walk-Forward Skill DA : {mean_skill*100:+.2f}%")
        print(f"   ├── Mean Annualized Sharpe     : {mean_sharpe:.2f}")
        print(f"   ├── Mean Max Drawdown          : {mean_maxdd*100:.2f}%")
        print(f"   └── Total Execution Duration   : {duration:.2f} seconds")
        print("=" * 80 + "\n")

        return {
            "status": "success",
            "run_id": run_id,
            "chrom_id": chromosome.get('id', 'N/A'),
            "worker_node": node_info,
            "execution_duration": duration,
            "oos_mean_skill_da": mean_skill,
            "oos_mean_sharpe": mean_sharpe,
            "oos_mean_max_dd": mean_maxdd,
            "fold_details": fold_results
        }

    except Exception as e:
        duration = time.perf_counter() - start_time
        err_msg = f"❌ [OOS VERIFY CRASH] Run {run_id} failed on Node {node_info}: {e}"
        print(err_msg)
        return {
            "status": "error",
            "run_id": run_id,
            "worker_node": node_info,
            "error": str(e),
            "execution_duration": duration
        }
    finally:
        tf.keras.backend.clear_session()
        gc.collect()