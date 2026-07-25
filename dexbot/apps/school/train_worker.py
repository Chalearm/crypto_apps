#!/usr/bin/env python3
#/******************************************************************************
#* File Name        : train_worker.py
#* Path             : apps/school/train_worker.py
#* Author           : Chalearm Saelim & Gemini
#* System Role      : Distributed Fold Training Engine / Worker Node
#* Architecture     : Distributed Client-Server / Master-Worker (Celery + Redis)
#* 
#* DEPENDENCY TREE & STRUCTURAL MAP:
#* ─────────────────────────────────────────────────────────────────────────────
#* [celery_tasks.py] (Celery Task Router)
#*    └── Calls ──> [train_worker.py] (Isolated Training Engine)
#*                   │
#*                   ├── Imports ──> [utils.py] (resolve_target_directories)
#*                   ├── Deserializes JSON arrays to NumPy (X_train, y_train, X_val, y_val)
#*                   ├── Builds Dynamic TensorFlow/Keras LSTM Architecture
#*                   ├── Trains Model with EarlyStopping & Adam Optimizer
#*                   ├── Evaluates Validation Inference & Metrics:
#*                   │    ├── Directional Accuracy (Baseline DA, Model DA, Skill DA)
#*                   │    └── Portfolio Backtest (Sharpe, MaxDD, RMSE, CAGR, Profit Factor)
#*                   │
#*                   ├── Logs Fold Lifecycle ──> [logs/<run_id>/folds_lifecycle.log]
#*                   └── Returns JSON Result Dictionary to Redis Master Queue
#*
#* FUNCTION DEPENDENCY MATRIX (Internal Methods):
#* ─────────────────────────────────────────────────────────────────────────────
#* execute_fold_training(payload)
#*  ├── utils.resolve_target_directories(run_id)
#*  ├── _build_lstm_model(num_timesteps, num_features, num_targets, chromosome)
#*  ├── _evaluate_directional_accuracy(y_val, predictions, target_cols)
#*  ├── _backtest_portfolio_strategy(y_val, predictions, target_cols, horizon)
#*  └── _log_fold_audit_metrics(logger_obj, ...)
#******************************************************************************/

import os
import sys
import gc
import time
import socket
import logging
import numpy as np
import tensorflow as tf

from utils import resolve_target_directories
# 1. Force CPU-only mode (stops CUDA cuInit attempts entirely)
os.environ["CUDA_VISIBLE_DEVICES"] = "-1"

# 2. Suppress TensorFlow C++ log output (0=ALL, 1=NO_INFO, 2=NO_WARNING, 3=NO_ERROR)
os.environ["TF_CPP_MIN_LOG_LEVEL"] = "3"
os.environ["TF_ENABLE_ONEDNN_OPTS"] = "0"

# Now import TensorFlow safely
import tensorflow as tf
tf.get_logger().setLevel('ERROR')


# ==============================================================================
# HELPER SUB-ROUTINES FOR TRAINING & METRICS EVALUATION
# ==============================================================================

def _build_lstm_model(num_timesteps: int, num_features: int, num_targets: int, chromosome: dict):
    """
    Constructs and compiles a Sequential Keras LSTM network using chromosome genes.
    """
    import tensorflow as tf

    model = tf.keras.Sequential()
    model.add(tf.keras.Input(shape=(num_timesteps, num_features)))

    num_layers = chromosome['lstm_layers']
    nodes = chromosome['nodes_per_layer']
    dropout_rate = chromosome['dropout_rate']

    for i in range(num_layers):
        is_last = (i == num_layers - 1)
        node_count = nodes[i] if i < len(nodes) else nodes[0]
        model.add(tf.keras.layers.LSTM(node_count, return_sequences=not is_last))
        model.add(tf.keras.layers.Dropout(dropout_rate))

    model.add(tf.keras.layers.Dense(num_targets))

    optimizer = tf.keras.optimizers.Adam(
        learning_rate=chromosome['learning_rate'],
        clipnorm=1.0  # Clip norm applied to prevent exploding gradients / NaN loss
    )
    model.compile(optimizer=optimizer, loss='mse')
    return model


def _evaluate_directional_accuracy(y_val: np.ndarray, predictions: np.ndarray, target_cols: list):
    """
    Calculates Baseline DA, Model DA, and Skill DA (edge over naive baseline) per asset target channel.
    """
    asset_skills = {}
    fold_assets_skill_list = []

    for target_idx, raw_col in enumerate(target_cols):
        if raw_col.startswith('price_log_return_'):
            asset_label = f"{raw_col.replace('price_log_return_', '').upper()} [PRICE]"
        elif raw_col.startswith('volume_log_change_'):
            asset_label = f"{raw_col.replace('volume_log_change_', '').upper()} [VOL]"
        else:
            asset_label = raw_col.upper()

        asset_targets = y_val[:, target_idx]
        tot_count = len(asset_targets)
        if tot_count == 0:
            continue

        pos_count = np.sum(asset_targets > 0)
        neg_count = np.sum(asset_targets < 0)
        baseline_da = max(pos_count / tot_count, neg_count / tot_count)

        asset_actual_s = np.sign(asset_targets)
        asset_pred_s = np.sign(predictions[:, target_idx])
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


def _backtest_portfolio_strategy(y_val: np.ndarray, predictions: np.ndarray, target_cols: list, horizon: int):
    """
    Simulates a long/short portfolio backtest accounting for trading fees, 
    computing Sharpe ratio, Max Drawdown, CAGR, Profit Factor, and Calmar ratio.
    """
    pred_signs = np.sign(predictions)
    price_indices = [idx for idx, col in enumerate(target_cols) if 'price_log_return' in col]
    if not price_indices:
        price_indices = [0]

    clean_pred_signs = pred_signs[:, price_indices]
    clean_y_val = y_val[:, price_indices]

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

    equity_curve = np.exp(np.cumsum(portfolio_returns))
    running_max = np.maximum.accumulate(equity_curve)
    drawdowns = (equity_curve - running_max) / running_max
    max_dd = float(abs(np.min(drawdowns))) if len(drawdowns) > 0 else 1.0

    fractional_years = max(1, horizon) / 365.0
    ending_wealth = float(equity_curve[-1])
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
    # Fix win_ratio formatting below:
    fold_logger.info(f"    📦 Day Counts -> Wins: {p_metrics['winning_days']} | Losses: {p_metrics['losing_days']} | Flats: {p_metrics['flat_days']} [Win Rate: {p_metrics['win_ratio']:.2f}%]")
    fold_logger.info(f"    📉 Bounds     -> Worst Day: {p_metrics['worst_return']:.6f} | Best Day: {p_metrics['best_return']:.6f}")
    fold_logger.info(f"    ⚡ Risk Specs -> Annualized Sharpe: {p_metrics['sharpe']:.2f} | Max Drawdown: {p_metrics['max_dd']*100:.2f}% | RMSE: {rmse:.4f}")
    fold_logger.info(f"    ⏱️ Execution  -> Fold Training Duration: {execution_duration:.2f} seconds")
    fold_logger.info("=" * 65)

# ==============================================================================
# MAIN WORKER EXECUTION ENTRY POINT
# ==============================================================================

def execute_fold_training(payload: dict) -> dict:
    """
    Main worker execution function triggered by Celery tasks. Deserializes data tensors,
    trains Keras LSTM model, evaluates metrics, logs diagnostics, and returns JSON payload.
    """
    import gc
    import time
    import socket
    import logging
    import numpy as np
    import tensorflow as tf
    
    tf.get_logger().setLevel('ERROR')
    from sklearn.metrics import root_mean_squared_error

    start_time = time.perf_counter()
    worker_hostname = socket.gethostname()
    worker_pid = os.getpid()
    node_info = f"{worker_hostname} (PID: {worker_pid})"

    # 1. Extract metadata & resolve sub-directories
    run_id = payload.get("run_id", None)
    chrom_id = payload.get('chrom_id', 'UNKNOWN')
    fold_idx = payload.get('fold_idx', 0)
    num_folds = payload.get('num_folds', 7)

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
        # 3. DESERIALIZE TENSORS
        cache_file = payload['cache_file']
        train_start, train_end = payload['train_slice']
        val_start, val_end = payload['val_slice']

        # Load cached arrays from disk
        with np.load(cache_file) as data:
            X_full = data['X']
            y_full = data['y']

        X_train, y_train = X_full[train_start:train_end], y_full[train_start:train_end]
        X_val, y_val = X_full[val_start:val_end], y_full[val_start:val_end]

        horizon = payload['horizon']
        chromosome = payload['chromosome']
        target_cols = payload['target_cols']

        num_timesteps, num_features = X_train.shape[1], X_train.shape[2]
        num_targets = y_train.shape[1]

        # 4. BUILD & TRAIN MODEL
        model = _build_lstm_model(num_timesteps, num_features, num_targets, chromosome)

        early_stop = tf.keras.callbacks.EarlyStopping(
            monitor='loss',
            patience=5,
            restore_best_weights=True
        )

        history = model.fit(
            X_train, y_train,
            epochs=40,
            batch_size=chromosome['batch_size'],
            verbose=0,
            callbacks=[early_stop]
        )

        duration = time.perf_counter() - start_time
        final_loss = float(history.history['loss'][-1])

        # 5. INFERENCE & EVALUATION
        predictions = model.predict(X_val, verbose=0)
        rmse = float(root_mean_squared_error(y_val, predictions))

        # Directional Accuracy & Skill DA
        asset_skills, avg_skill = _evaluate_directional_accuracy(y_val, predictions, target_cols)

        # Portfolio Backtest Metrics
        p_metrics = _backtest_portfolio_strategy(y_val, predictions, target_cols, horizon)

        # 6. LOG AUDIT METRICS TO DISK
        _log_fold_audit_metrics(
            fold_logger=fold_logger, 
            chrom_id=chrom_id, 
            fold_idx=fold_idx, 
            num_folds=num_folds, 
            asset_skills=asset_skills, 
            p_metrics=p_metrics, 
            rmse=rmse, 
            node_info=node_info,
            execution_duration=duration
        )
        fold_logger.info(f"✅ [TRAIN COMPLETE] Model {chrom_id} | Fold {fold_idx}/{num_folds} finished in {duration:.2f}s")

        # 7. RETURN JSON-SERIALIZABLE PAYLOAD TO MASTER
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
            "rmse": rmse,
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
        fold_logger.error(f"❌ [WORKER CRASH] Model {chrom_id} | Fold {fold_idx}/{num_folds} failed on Node {node_info}: {e}")

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
        # Guarantee session clearance & OS memory reclaim on every execution
        try:
            tf.keras.backend.clear_session()
        except Exception:
            pass
        gc.collect()