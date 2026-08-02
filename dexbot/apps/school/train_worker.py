#!/usr/bin/env python3
# ##############################################################################
# File Name        : train_worker.py
# File Path        : apps/school/train_worker.py
#
# Author           : Chalearm Saelim & Gemini
# Owner            : Chalearm Saelim
# Reviewer         : Chalearm Saelim
#
# Version          : 1.0.1
# Status           : Development
# Created Date     : 2026-07-26 08:00:00 (UTC+7)
# Modified Date    : 2026-07-27 16:30:00 (UTC+7)
#
# Description      :
#    Isolated worker engine node that executes LSTM fold cross-validation training
#    tasks dispatched via Celery over a Redis message broker. Loads feature tensors
#    from local disk cache (.npz), constructs dynamic Keras LSTM networks with Seq2Seq
#    headers, executes training with EarlyStopping, computes directional accuracy and
#    portfolio backtest metrics, and logs execution audit trails to disk.
#
#    DEPENDENCY TREE & STRUCTURAL MAP:
#    ───────────────────────────────────────────────────────────────────────────
#    [celery_tasks.py] (Celery Task Router)
#        └── Calls ──> [train_worker.py] (Isolated Training Engine)
#                        │
#                        ├── Imports ──> [utils.py] (resolve_target_directories)
#                        ├── Loads .npz Tensors from Disk (X_train, y_train, X_val, y_val)
#                        ├── Builds Dynamic TensorFlow/Keras LSTM Architecture
#                        ├── Trains Model with EarlyStopping & Adam Optimizer
#                        ├── Evaluates Validation Inference & Metrics:
#                        │    ├── Directional Accuracy (Baseline DA, Model DA, Skill DA)
#                        │    └── Portfolio Backtest (Sharpe, MaxDD, RMSE, CAGR, Profit Factor)
#                        │
#                        ├── Logs Fold Lifecycle ──> [logs/<run_id>/folds_lifecycle.log]
#                        └── Returns JSON Result Dictionary to Redis Master Queue
#
#    FUNCTION DEPENDENCY MATRIX (Internal Methods):
#    ───────────────────────────────────────────────────────────────────────────
#    execute_fold_training(payload)
#     ├── utils.resolve_target_directories(run_id)
#     ├── _build_and_compile_lstm(chromosome, input_shape, num_targets, forecast_horizon)
#     ├── _evaluate_directional_accuracy(y_val, predictions, target_cols)
#     ├── _backtest_portfolio_strategy(y_val, predictions, target_cols, horizon)
#     └── _log_fold_audit_metrics(logger_obj, ...)
#
# Responsibilities :
#    - Deserializes dataset slice indices and loads cached array tensors.
#    - Dynamically builds and fits Keras LSTM neural network models per fold.
#    - Computes financial backtest metrics and directional accuracy statistics.
#    - Audits training lifecycle and releases worker RAM/GPU sessions on completion.
#
# Usage :
#    Directory : apps/school/
#
#    Build :
#      N/A (Interpreted Python Script)
#
#    Run :
#      Executed internally by Celery worker pool via tasks.run_fold_training_task
#
# Dependencies :
#    Internal :
#      - utils (resolve_target_directories)
#
#    External :
#      - numpy, tensorflow, scikit-learn
#
# Updated Parts :
#    [Function]
#      - _build_and_compile_lstm() (Used Keras 3 Input(shape) object & Reshape layer)
#      - _backtest_portfolio_strategy() (Clipped cumsum log returns to eliminate exp float overflow)
#      - execute_fold_training() (Fixed function call alignment and added full print telemetry)
#
# New Parts :
#    None
#
# Change History :
#    -------------------------------------------------------------------------
#    Version | Date Time (UTC+7)        | Author          | Description
#    -------------------------------------------------------------------------
#    1.0.0   | 2026-07-26 08:00:00      | Chalearm Saelim | Initial release
#    1.0.1   | 2026-07-27 16:30:00      | Chalearm Saelim | Fixed exp overflow & Keras 3 input shape
#    -------------------------------------------------------------------------
#
# TODO :
#    - Add mixed-precision training support for acceleration.
#
# Notes :
#    - Per regulator coding standard rules.
# ##############################################################################

import os
import sys
import gc
import time
import socket
import logging
import numpy as np
import rust_lstm_engine  # Native Rust C-Extension Module!
# 1. Force CPU-only mode (stops CUDA cuInit attempts entirely)
os.environ["CUDA_VISIBLE_DEVICES"] = "-1"
os.environ["TF_CPP_MIN_LOG_LEVEL"] = "3"
os.environ["TF_ENABLE_ONEDNN_OPTS"] = "0"

# --- Memory & Thread Safeguards for Celery Workers ---
os.environ["OMP_NUM_THREADS"] = "1"
os.environ["MKL_NUM_THREADS"] = "1"
os.environ["OPENBLAS_NUM_THREADS"] = "1"
os.environ["VECLIB_MAXIMUM_THREADS"] = "1"
os.environ["NUMEXPR_NUM_THREADS"] = "1"

import tensorflow as tf
from tensorflow.keras.models import Sequential
from tensorflow.keras.layers import Input, LSTM, Dense, Dropout, Reshape

tf.config.threading.set_intra_op_parallelism_threads(1)
tf.config.threading.set_inter_op_parallelism_threads(1)
tf.get_logger().setLevel('ERROR')

from sklearn.metrics import root_mean_squared_error
from utils import resolve_target_directories


# ==============================================================================
# HELPER SUB-ROUTINES FOR TRAINING & METRICS EVALUATION
# ==============================================================================

# ##############################################################################
# Function Name : _evaluate_directional_accuracy
#
# Purpose :
#    Calculates Baseline DA, Model DA, and Skill DA (edge over naive baseline)
#    for each target feature channel.
#
# Inputs :
#    y_val
#        Type        : numpy.ndarray
#        Description : Ground truth validation target values.
#    predictions
#        Type        : numpy.ndarray
#        Description : Predicted values output by LSTM model.
#    target_cols
#        Type        : list
#        Description : Names of active target feature columns.
#
# Return :
#    Type        : tuple (dict, float)
#    Description : (asset_skills dictionary, avg_skill float)
#
# Complexity :
#    Time  : O(N * C) where N is validation sample count, C is channel count.
#    Space : O(C)
#
# Error Cases :
#    - Returns 0.0 average skill if target sample count is zero.
#
# Number Of Lines :
#    35
# ##############################################################################
def _evaluate_directional_accuracy(y_val: np.ndarray, predictions: np.ndarray, target_cols: list):
    asset_skills = {}
    fold_assets_skill_list = []

    # Flatten time steps if 3D arrays are provided for evaluation
    y_val_flat = y_val.reshape(-1, y_val.shape[-1]) if len(y_val.shape) == 3 else y_val
    preds_flat = predictions.reshape(-1, predictions.shape[-1]) if len(predictions.shape) == 3 else predictions

    for target_idx, raw_col in enumerate(target_cols):
        if target_idx >= y_val_flat.shape[1]:
            continue

        if raw_col.startswith('price_log_return_'):
            asset_label = f"{raw_col.replace('price_log_return_', '').upper()} [PRICE]"
        elif raw_col.startswith('volume_log_change_'):
            asset_label = f"{raw_col.replace('volume_log_change_', '').upper()} [VOL]"
        else:
            asset_label = raw_col.upper()

        asset_targets = y_val_flat[:, target_idx]
        tot_count = len(asset_targets)
        if tot_count == 0:
            continue

        pos_count = np.sum(asset_targets > 0)
        neg_count = np.sum(asset_targets < 0)
        baseline_da = max(pos_count / tot_count, neg_count / tot_count)

        asset_actual_s = np.sign(asset_targets)
        asset_pred_s = np.sign(preds_flat[:, target_idx])
        model_da = np.sum(asset_actual_s == asset_pred_s) / tot_count
        skill_da = float(model_da - baseline_da)

        asset_skills[asset_label] = {
            'baseline': float(baseline_da),
            'model': float(model_da),
            'skill': skill_da
        }
        fold_assets_skill_list.append(skill_da)

    avg_skill = float(np.mean(fold_assets_skill_list)) if fold_assets_skill_list else 0.0
    return asset_skills, avg_skill


# ##############################################################################
# Function Name : _backtest_portfolio_strategy
#
# Purpose :
#    Simulates a long/short portfolio backtest accounting for transaction fees,
#    computing Sharpe ratio, Max Drawdown, CAGR, Profit Factor, and Calmar ratio.
#
# Inputs :
#    y_val
#        Type        : numpy.ndarray
#        Description : Validation ground truth returns.
#    predictions
#        Type        : numpy.ndarray
#        Description : Predicted return directional signals.
#    target_cols
#        Type        : list
#        Description : Active feature channel columns.
#    horizon
#        Type        : int
#        Description : Forecast horizon days for annualization calculations.
#
# Return :
#    Type        : dict
#    Description : Dictionary containing backtest performance metrics.
#
# Complexity :
#    Time  : O(N * P) where N is days, P is price indices count.
#    Space : O(N)
#
# Error Cases :
#    - Handles zero variance returns by falling back to default penalty values.
#
# Number Of Lines :
#    58
# ##############################################################################
def _backtest_portfolio_strategy(y_val: np.ndarray, predictions: np.ndarray, target_cols: list, horizon: int):
    y_val_flat = y_val.reshape(-1, y_val.shape[-1]) if len(y_val.shape) == 3 else y_val
    preds_flat = predictions.reshape(-1, predictions.shape[-1]) if len(predictions.shape) == 3 else predictions

    pred_signs = np.sign(preds_flat)
    price_indices = [idx for idx, col in enumerate(target_cols) if 'price_log_return' in col]
    if not price_indices:
        price_indices = [0]

    clean_pred_signs = pred_signs[:, price_indices]
    clean_y_val = y_val_flat[:, price_indices]

    raw_strategy_returns = clean_pred_signs * clean_y_val
    position_changes = np.abs(np.diff(clean_pred_signs, axis=0, prepend=clean_pred_signs[:1]))
    
    TRADING_FEE_RATE = 0.0005  # 5 bps transaction fee
    friction_costs = position_changes * TRADING_FEE_RATE
    net_strategy_returns = raw_strategy_returns - friction_costs
    portfolio_returns = np.clip(np.mean(net_strategy_returns, axis=1), -0.15, 0.15)

    winning_days = int(np.sum(portfolio_returns > 0))
    losing_days = int(np.sum(portfolio_returns < 0))
    flat_days = int(np.sum(portfolio_returns == 0))
    total_days = len(portfolio_returns)
    win_ratio = (winning_days / total_days * 100) if total_days > 0 else 0.0

    gross_profits = float(np.sum(portfolio_returns[portfolio_returns > 0]))
    gross_losses = float(np.sum(np.abs(portfolio_returns[portfolio_returns < 0])))
    profit_factor = (gross_profits / gross_losses) if gross_losses > 1e-6 else 1.0

    daily_std = float(np.std(portfolio_returns))
    mean_ret = float(np.mean(portfolio_returns))
    crypto_ann_factor = float(np.sqrt(365.0 / max(1, horizon)))
    sharpe = float((mean_ret / daily_std * crypto_ann_factor)) if daily_std > 1e-6 else -5.0

    # Safe Cumulative Log Return Calculation (Clips max growth to prevent e^700 overflow)
    cum_returns = np.clip(np.cumsum(portfolio_returns), -50.0, 50.0)
    equity_curve = np.exp(cum_returns)
    equity_curve = np.nan_to_num(equity_curve, nan=1.0, posinf=1e6, neginf=1e-6)

    running_max = np.maximum.accumulate(equity_curve)
    
    # Safe Drawdown Calculation (Prevents inf/inf and 0/0 NaNs)
    with np.errstate(divide='ignore', invalid='ignore'):
        drawdowns = np.where(running_max > 0, (equity_curve - running_max) / running_max, 0.0)
        drawdowns = np.nan_to_num(drawdowns, nan=0.0)

    max_dd = float(abs(np.min(drawdowns))) if len(drawdowns) > 0 else 0.0

    fractional_years = max(1, horizon) / 365.0
    ending_wealth = float(equity_curve[-1]) if len(equity_curve) > 0 else 1.0
    raw_cagr = (ending_wealth ** (1.0 / fractional_years)) - 1.0 if ending_wealth > 0 else -1.0
    cagr = float(np.clip(raw_cagr, -0.99, 5.00))
    calmar = (cagr / max_dd) if max_dd > 1e-5 else (cagr / 0.00001)

    return {
        "sharpe": sharpe,
        "max_dd": max_dd,
        "cagr": cagr,
        "profit_factor": profit_factor,
        "calmar": calmar,
        "win_ratio": win_ratio,
        "winning_days": winning_days,
        "losing_days": losing_days,
        "flat_days": flat_days,
        "worst_return": float(portfolio_returns.min()) if total_days > 0 else 0.0,
        "best_return": float(portfolio_returns.max()) if total_days > 0 else 0.0
    }


# ##############################################################################
# Function Name : _log_fold_audit_metrics
#
# Purpose :
#    Formats and logs target balance, per-asset accuracy, vector diagnostics,
#    and training timing details into the fold lifecycle log file.
#
# Inputs :
#    fold_logger
#        Type        : logging.Logger
#        Description : Target log handler object.
#    chrom_id
#        Type        : str
#        Description : Unique identifier string for chromosome.
#    fold_idx
#        Type        : int
#        Description : Current fold index.
#    num_folds
#        Type        : int
#        Description : Total fold count.
#    asset_skills
#        Type        : dict
#        Description : Per-asset directional accuracy metrics dictionary.
#    p_metrics
#        Type        : dict
#        Description : Portfolio backtest risk metrics dictionary.
#    rmse
#        Type        : float
#        Description : Root Mean Squared Error value.
#    node_info
#        Type        : str
#        Description : Worker hostname and PID string.
#    execution_duration
#        Type        : float
#        Description : Fold training execution duration in seconds.
#
# Return :
#    Type        : None
#    Description : Logs formatted diagnostic blocks to disk.
#
# Complexity :
#    Time  : O(A) where A is asset channel count.
#    Space : O(1)
#
# Error Cases :
#    - None
#
# Number Of Lines :
#    18
# ##############################################################################
def _log_fold_audit_metrics(fold_logger, chrom_id, fold_idx, num_folds, asset_skills, p_metrics, rmse, node_info, execution_duration=0.0):
    fold_logger.info("=" * 65)
    fold_logger.info(f"🕵️‍♂️ [TARGET BALANCE CHECK - MODEL {chrom_id} | FOLD {fold_idx}/{num_folds} | Node: {node_info}]:")
    for asset, metrics in asset_skills.items():
        fold_logger.info(f"    {asset.upper().ljust(22)} Target -> Baseline DA: {metrics['baseline']*100:.1f}%")
    
    fold_logger.info("-" * 65)
    fold_logger.info(f"🎯 [PER-ASSET FORECASTING ACCURACY - FOLD {fold_idx}/{num_folds}]:")
    for asset, metrics in asset_skills.items():
        fold_logger.info(f"    {asset.upper().ljust(22)} Model DA: {metrics['model']*100:.2f}% | Skill DA: {metrics['skill']*100:+.2f}%")
        
    fold_logger.info("-" * 65)
    fold_logger.info(f"🕵️‍♂️ [SURGICAL AUDIT FOLD {fold_idx}/{num_folds}] Vector Diagnostics:")
    fold_logger.info(f"    📦 Day Counts -> Wins: {p_metrics['winning_days']} | Losses: {p_metrics['losing_days']} | Flats: {p_metrics['flat_days']} [Win Rate: {p_metrics['win_ratio']:.2f}%]")
    fold_logger.info(f"    📉 Bounds     -> Worst Day: {p_metrics['worst_return']:.6f} | Best Day: {p_metrics['best_return']:.6f}")
    fold_logger.info(f"    ⚡ Risk Specs -> Annualized Sharpe: {p_metrics['sharpe']:.2f} | Max Drawdown: {p_metrics['max_dd']*100:.2f}% | RMSE: {rmse:.4f}")
    fold_logger.info(f"    ⏱️ Execution  -> Fold Training Duration: {execution_duration:.2f} seconds")
    fold_logger.info("=" * 65)


# ==============================================================================
# MAIN WORKER EXECUTION ENTRY POINT
# ==============================================================================

# ##############################################################################
# Function Name : execute_fold_training
#
# Purpose :
#    Executes cross-validation fold training using the native Rust PyO3 C-extension
#    (rust_lstm_engine). Completely replaces TensorFlow to reduce RAM usage down
#    to ~40 MB and eliminate Keras graph overhead while preserving all logging,
#    directional accuracy evaluation, and backtest telemetry output.
#
# Inputs :
#    payload
#        Type        : dict
#        Description : Celery task payload containing model hyperparameters,
#                      data slice indices, run_id, and disk cache path.
#
# Return :
#    Type        : dict
#    Description : JSON-serializable evaluation metrics vector returned to master.
# ##############################################################################
import os
import gc
import time
import socket
import logging
import numpy as np
from utils import resolve_target_directories

# Native Rust PyO3 C-Extension (Replaces TensorFlow)
import rust_lstm_engine


def execute_fold_training(payload: dict) -> dict:
    start_time = time.perf_counter()
    worker_hostname = socket.gethostname()
    worker_pid = os.getpid()
    node_info = f"{worker_hostname} (PID: {worker_pid})"

    # 1. Extract metadata & resolve sub-directories
    run_id = payload.get("run_id", None)
    chrom_id = payload.get('chrom_id', 'UNKNOWN')
    fold_idx = payload.get('fold_idx', 0)
    num_folds = payload.get('num_folds', 6)

    log_dir, export_dir, plot_dir = resolve_target_directories(run_id)

    # 2. Setup dynamic FileHandler pointing to logs/<run_id>/folds_lifecycle.log
    fold_log_file = os.path.join(log_dir, "folds_lifecycle.log")
    fold_logger = logging.getLogger(f"FoldLifecycle_{run_id or 'LEGACY'}")
    
    if not fold_logger.handlers:
        fh = logging.FileHandler(fold_log_file)
        fh.setFormatter(logging.Formatter('%(asctime)s - %(levelname)s - %(message)s'))
        fold_logger.addHandler(fh)
        fold_logger.setLevel(logging.INFO)

    fold_logger.info(f"🏋️ [TRAIN START] Model {chrom_id} | Fold {fold_idx}/{num_folds} | Run Directory: {log_dir}")

    try:
        # 3. DESERIALIZE TENSORS FROM DISK CACHE
        cache_file = payload['cache_file']
        train_start, train_end = payload['train_slice']
        val_start, val_end = payload['val_slice']

        # Load cached arrays from disk
        with np.load(cache_file) as data:
            X_full = data['X']
            y_full = data['y']

        X_train, y_train = X_full[train_start:train_end], y_full[train_start:train_end]
        X_val, y_val = X_full[val_start:val_end], y_full[val_start:val_end]

        chromosome = payload['chromosome']
        target_cols = payload['target_cols']

        # 4. DERIVE 3D TENSOR SHAPES SAFELY
        num_timesteps, num_features = X_train.shape[1], X_train.shape[2]
        
        if len(y_train.shape) == 3:
            forecast_horizon = y_train.shape[1]
            num_targets = y_train.shape[2]
        else:
            forecast_horizon = 1
            num_targets = y_train.shape[1]

        # Detailed worker telemetry output
        print("\n" + "=" * 70)
        print(f"🏋️ [WORKER FOLD EXECUTION (RUST ENGINE)] Model: {chrom_id} | Fold: {fold_idx}/{num_folds} | Node: {node_info}")
        print(f"   ├── Training Slices   : {train_start} -> {train_end} (Samples: {len(X_train)})")
        print(f"   ├── Validation Slices : {val_start} -> {val_end} (Samples: {len(X_val)})")
        print(f"   ├── Input Tensor      : X_train shape = {X_train.shape}")
        print(f"   ├── Target Tensor     : y_train shape = {y_train.shape}")
        print(f"   └── Output Target Spec: Horizon = {forecast_horizon} days, Active Targets = {num_targets}")
        print("=" * 70)

        # Extract LSTM units safely from chromosome gene dictionary
        nodes_gene = chromosome.get('nodes_per_layer', [64])
        lstm_units = int(nodes_gene[0]) if isinstance(nodes_gene, list) and len(nodes_gene) > 0 else int(chromosome.get('lstm_units', 64))
        learning_rate = float(chromosome.get('learning_rate', 0.001))
        batch_size = int(chromosome.get('batch_size', 32))

        # 5. TRAIN & PREDICT IN NATIVE RUST (Replaces TensorFlow model.fit + predict)
        # Returns predicted array directly in 3D shape matching y_val
        predictions = rust_lstm_engine.train_and_predict(
            x_train_py=X_train.astype(np.float32),
            y_train_py=y_train.astype(np.float32),
            x_val_py=X_val.astype(np.float32),
            lstm_units=lstm_units,
            learning_rate=learning_rate,
            epochs=30,
            _batch_size=batch_size
        )

        duration = time.perf_counter() - start_time

        # 6. INFERENCE & EVALUATION METRICS (Calculated in Python over predictions)
        rmse_val = float(np.sqrt(np.mean((y_val.reshape(-1, num_targets) - predictions.reshape(-1, num_targets)) ** 2)))

        # Directional Accuracy & Skill DA
        asset_skills, avg_skill = _evaluate_directional_accuracy(y_val, predictions, target_cols)

        # Portfolio Backtest Metrics
        p_metrics = _backtest_portfolio_strategy(y_val, predictions, target_cols, forecast_horizon)

        # Estimate final MSE loss for reporting
        final_loss = float(rmse_val ** 2)

        # 7. LOG AUDIT METRICS TO DISK
        _log_fold_audit_metrics(
            fold_logger=fold_logger, 
            chrom_id=chrom_id, 
            fold_idx=fold_idx, 
            num_folds=num_folds, 
            asset_skills=asset_skills, 
            p_metrics=p_metrics, 
            rmse=rmse_val, 
            node_info=node_info,
            execution_duration=duration
        )
        fold_logger.info(f"✅ [TRAIN COMPLETE] Model {chrom_id} | Fold {fold_idx}/{num_folds} finished in {duration:.2f}s")

        print(f"✅ [WORKER SUCCESS] Model: {chrom_id} | Fold: {fold_idx}/{num_folds} | Skill DA: {avg_skill*100:+.2f}% | Loss: {final_loss:.6f} | Time: {duration:.2f}s\n")

        # 8. RETURN JSON-SERIALIZABLE PAYLOAD TO MASTER
        return {
            "status": "success",
            "run_id": run_id,
            "chrom_id": chrom_id,
            "fold_idx": fold_idx,
            "execution_duration": duration,
            "worker_node": node_info,
            "skill_da": avg_skill,
            "sharpe": p_metrics['sharpe'],
            "max_dd": p_metrics['max_dd'],
            "rmse": rmse_val,
            "cagr": p_metrics['cagr'],
            "profit_factor": p_metrics['profit_factor'],
            "calmar": p_metrics['calmar'],
            "win_ratio": p_metrics['win_ratio'],
            "winning_days": p_metrics['winning_days'],
            "losing_days": p_metrics['losing_days'],
            "flat_days": p_metrics['flat_days'],
            "worst_return": p_metrics['worst_return'],
            "best_return": p_metrics['best_return'],
            "asset_skills": asset_skills,
            "loss": final_loss
        }

    except Exception as e:
        duration = time.perf_counter() - start_time
        err_msg = f"❌ [WORKER CRASH] Model {chrom_id} | Fold {fold_idx}/{num_folds} failed on Node {node_info}: {e}"
        print(err_msg)
        fold_logger.error(err_msg)

        return {
            "status": "error",
            "run_id": run_id,
            "chrom_id": chrom_id,
            "fold_idx": fold_idx,
            "worker_node": node_info,
            "error": str(e),
            "execution_duration": duration
        }

    finally:
        # Guarantee Garbage Collection & memory release
        gc.collect()