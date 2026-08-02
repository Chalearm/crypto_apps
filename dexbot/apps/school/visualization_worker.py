#!/usr/bin/env python3
# ##############################################################################
# File Name        : visualization_worker.py
# File Path        : apps/school/visualization_worker.py
#
# Author           : Chalearm Saelim & Gemini
# Owner            : Chalearm Saelim
# Reviewer         : Chalearm Saelim
#
# Version          : 1.1.0
# Status           : Development
# Created Date     : 2026-07-26 08:00:00 (UTC+7)
# Modified Date    : 2026-07-31 15:00:00 (UTC+7)
#
# Description      :
#    Offloaded worker node engine that reconstructs datasets, trains full-horizon
#    Keras LSTM models for top Pareto front candidate chromosomes, renders multi-step
#    direct sequence-to-sequence validation overlays and future forecasting plots via
#    Matplotlib, and serializes deployed model binaries, MinMaxScaler pickles, and JSON metadata.
#
#    DEPENDENCY TREE & STRUCTURAL MAP:
#    ───────────────────────────────────────────────────────────────────────────
#    [celery_tasks.py] (Celery Task Router)
#        └── Calls ──> [visualization_worker.py] (Offloaded Renderer & Serializer)
#                        │
#                        ├── Imports ──> [utils.py] (resolve_target_directories)
#                        ├── Reconstructs Pandas DataFrames from JSON strings
#                        ├── Dynamically extracts dynamic closing prices from datasets
#                        ├── Binds MinMaxScaler & Column Classifications
#                        ├── Re-trains Keras LSTM on Full Sequences
#                        ├── Generates Direct Sequence Multi-Step Forecasts
#                        │    ├── Validation Overlay Plots  ──> [prediction_result/<run_id>/<model_id>/prc/ & vol/]
#                        │    └── True Future Projections   ──> [prediction_result/<run_id>/<model_id>/prc/ & vol/]
#                        │
#                        └── Serializes .keras Models, Scalers, & JSON Metadata 
#                            ──> [deployed_models/<run_id>/]
#
#    FUNCTION DEPENDENCY MATRIX (Internal Sub-Routines):
#    ───────────────────────────────────────────────────────────────────────────
#    generate_pareto_graphs_and_exports(...)
#     ├── utils.resolve_target_directories(run_id)
#     ├── _extract_asset_name(target_col)
#     ├── _get_dynamic_base_price(df, target_col)
#     ├── _split_features(df)
#     ├── _prepare_lstm_tensors(chromosome, df)
#     ├── _build_and_train_full_model(chromosome, X, y)
#     ├── _plot_validation_overlay(chromosome, model, scaler, master_df, val_df, plot_dir)
#     ├── _plot_future_projection(chromosome, model, scaler, combined_df, plot_dir)
#     └── _export_candidate_model(chromosome, model, scaler, master_df, export_dir, rank)
#
# Responsibilities :
#    - Reconstructs DataFrame structures and fits MinMaxScalers from serialized task payloads.
#    - Dynamically derives actual starting asset prices from raw dataset columns.
#    - Fits candidate LSTM models across complete training sequences.
#    - Executes direct sequence-to-sequence multi-step prediction loops.
#    - Renders Matplotlib validation overlays and true future projection graphs with date X-axis references.
#    - Exports Keras model binaries (.keras), scaler pickles (.pkl), and JSON metadata.
#
# Usage :
#    Directory : apps/school/
#
#    Build :
#      N/A (Interpreted Python Script)
#
#    Run :
#      Executed asynchronously by Celery worker pool via tasks.export_and_plot_task
#
# Dependencies :
#    Internal :
#      - utils (resolve_target_directories)
#
#    External :
#      - numpy, pandas, matplotlib, scikit-learn, tensorflow
#
# Change History :
#    -------------------------------------------------------------------------
#    Version | Date Time (UTC+7)         | Author          | Description
#    -------------------------------------------------------------------------
#    1.0.0   | 2026-07-26 08:00:00       | Chalearm Saelim | Initial release
#    1.1.0   | 2026-07-31 15:00:00       | Chalearm Saelim | Added dynamic price extraction & date ticks
#    -------------------------------------------------------------------------
# ##############################################################################

import io
import os
import sys
import gc
import re
import pickle
import json
import logging
import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
from sklearn.preprocessing import MinMaxScaler 
import tensorflow as tf
from tensorflow.keras.models import Sequential
from tensorflow.keras.layers import LSTM, Dense, Dropout, Reshape
from utils import resolve_target_directories

# Mute low-level TensorFlow warnings
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'
os.environ['CUDA_VISIBLE_DEVICES'] = '-1'

# Logger Configuration
log_formatter = logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger("VisualizationWorker")
logger.setLevel(logging.INFO)

if not logger.handlers:
    c_handler = logging.StreamHandler(sys.stdout)
    c_handler.setFormatter(log_formatter)
    logger.addHandler(c_handler)


# ==============================================================================
# HELPER UTILITIES: DYNAMIC PRICE RESOLUTION & DATE FORMATTING
# ==============================================================================

def _extract_asset_name(target_col: str) -> str:
    """Extracts a clean asset symbol/name from column strings like 'price_log_return_bitcoin' -> 'bitcoin'."""
    clean = target_col.lower()
    clean = re.sub(r'^(price_|log_return_|close_|adj_close_|volume_log_change_)', '', clean)
    clean = re.sub(r'_(log_return|price|close|usd)$', '', clean)
    return clean if clean else target_col


def _get_dynamic_base_price(df: pd.DataFrame, target_col: str) -> float:
    """
    Dynamically retrieves the starting dollar price from the dataset file.
    First checks for direct raw close/price columns. If only log returns exist,
    reconstructs or retrieves the latest valid numeric price.
    """
    asset_name = _extract_asset_name(target_col)

    # 1. Look for explicit close/price columns matching the asset name in DataFrame
    price_candidates = [
        col for col in df.columns 
        if asset_name in col.lower() and any(p in col.lower() for p in ["close", "price", "usd", "val"]) and "return" not in col.lower()
    ]

    if price_candidates:
        latest_price = df[price_candidates[0]].dropna().iloc[-1]
        if latest_price > 0:
            return float(latest_price)

    # 2. Fallback: Search for any non-return price column in the dataset
    for col in df.columns:
        if ("close" in col.lower() or "price" in col.lower()) and "return" not in col.lower():
            val = df[col].dropna().iloc[-1]
            if val > 0:
                return float(val)

    # 3. Default fallback if dataset strictly contains normalized/stationary features
    return 100.0


def _reconstruct_price_series(log_returns: np.ndarray, base_price: float) -> np.ndarray:
    """Compounds log returns into actual cumulative price series: P_t = P_0 * exp(cumsum(r_t))"""
    return base_price * np.exp(np.cumsum(log_returns))


def _format_xaxis_dates(ax, dates_list: list, n_hist: int, n_horizon: int):
    """Formats X-axis ticks with key reference points: 0 (Start), Mid-Hist, Hist-End, Horizon-End."""
    n_total = n_hist + n_horizon
    
    tick_indices = [
        0,
        int(n_hist * 0.5),
        n_hist - 1,
        n_total - 1
    ]

    start_date = dates_list[0] if len(dates_list) > 0 else "Start"
    mid_date = dates_list[int(len(dates_list)*0.5)] if len(dates_list) > int(len(dates_list)*0.5) else "Mid"
    end_hist_date = dates_list[min(n_hist - 1, len(dates_list)-1)] if len(dates_list) > 0 else "End Hist"
    
    tick_labels = [
        f"0\n({start_date})",
        f"{int(n_hist * 0.5)}\n({mid_date})",
        f"{n_hist}\n({end_hist_date})",
        f"{n_total}\n(+{n_horizon}d)"
    ]

    ax.set_xticks(tick_indices)
    ax.set_xticklabels(tick_labels, fontsize=8)


# ==============================================================================
# TENSOR PREPARATION & MODEL COMPILATION
# ==============================================================================

def _compile_lstm_architecture(chromosome: dict, input_shape: tuple, num_targets: int, forecast_horizon: int = 52) -> tf.keras.Model:
    units = int(chromosome.get('lstm_units', 64))
    dropout_rate = float(chromosome.get('dropout_rate', 0.2))
    num_layers = int(chromosome.get('num_layers', 2))

    model = Sequential()
    
    # Input / First LSTM Layer
    model.add(LSTM(
        units=units, 
        return_sequences=(num_layers > 1), 
        input_shape=input_shape
    ))
    model.add(Dropout(dropout_rate))
    
    # Hidden LSTM Layers
    for i in range(1, num_layers):
        model.add(LSTM(
            units=units, 
            return_sequences=(i < num_layers - 1)
        ))
        model.add(Dropout(dropout_rate))
        
    # Direct Sequence-to-Sequence Output Layer predicting (horizon * num_targets)
    model.add(Dense(forecast_horizon * num_targets))
    model.add(Reshape((forecast_horizon, num_targets)))
    
    return model


def _split_features(df: pd.DataFrame):
    temporal_patterns = ['day_wk_sin', 'day_wk_cos', 'day_yr_sin', 'day_yr_cos', 'hour_sin', 'hour_cos', 'min_sin', 'min_cos']
    time_cols = [c for c in df.columns if any(p in c for p in temporal_patterns)]
    base_asset_cols = [c for c in df.columns if c not in time_cols and not c.startswith('close_')]
    
    USER_EXCLUDE_FEATURES = ['volume_log_change_fed']
    banned_lower = [banned.lower() for banned in USER_EXCLUDE_FEATURES]
    asset_cols = [c for c in base_asset_cols if c.lower() not in banned_lower]

    return time_cols, asset_cols


def _prepare_lstm_tensors(chromosome: dict, df: pd.DataFrame):
    time_cols, asset_cols = _split_features(df)
    mask = np.array(chromosome.get('feature_mask', []))

    if len(mask) != len(asset_cols):
        mask = np.ones(len(asset_cols), dtype=int)

    asset_values = df[asset_cols].values[:, mask == 1]
    time_values = df[time_cols].values if time_cols else np.zeros((len(df), 0))
    combined_data = np.hstack([asset_values, time_values]) if time_cols else asset_values

    lookback = int(chromosome.get('lookback_window', 60))
    forecast = int(chromosome.get('forecast_horizon', 30))

    num_samples = len(combined_data) - lookback - forecast
    if num_samples > 0:
        X, y = [], []
        for i in range(num_samples):
            X.append(combined_data[i : (i + lookback)])
            y.append(asset_values[i + lookback : i + lookback + forecast])
        return np.array(X, dtype=np.float32), np.array(y, dtype=np.float32), mask

    return np.array([]), np.array([]), mask


def _build_and_train_full_model(chromosome: dict, X_train: np.ndarray, y_train: np.ndarray):
    X_train = np.nan_to_num(X_train, nan=0.0, posinf=1.0, neginf=-1.0)
    y_train = np.nan_to_num(y_train, nan=0.0, posinf=1.0, neginf=-1.0)

    num_targets = y_train.shape[-1] if len(y_train.shape) > 1 else 1
    forecast_horizon = int(chromosome.get('forecast_horizon', 52))

    model = _compile_lstm_architecture(
        chromosome, 
        input_shape=(X_train.shape[1], X_train.shape[2]), 
        num_targets=num_targets,
        forecast_horizon=forecast_horizon
    )

    lr = min(float(chromosome.get('learning_rate', 0.001)), 0.001)
    optimizer = tf.keras.optimizers.Adam(learning_rate=lr, clipvalue=0.5)
    model.compile(optimizer=optimizer, loss='mse')

    epochs = int(chromosome.get('epochs', 25))
    batch_size = int(chromosome.get('batch_size', 32))

    model.fit(
        X_train, y_train,
        epochs=epochs,
        batch_size=batch_size,
        verbose=0
    )

    return model


# ==============================================================================
# SUB-ROUTINES: GRAPH RENDERING & MODEL EXPORTS
# ==============================================================================

def _plot_validation_overlay(chromosome: dict, model, scaler: MinMaxScaler, master_df: pd.DataFrame, val_df: pd.DataFrame, plot_dir: str):
    horizon = int(chromosome.get('forecast_horizon', 30))
    lookback = int(chromosome.get('lookback_window', 60))
    
    raw_combined = pd.concat([master_df, val_df], ignore_index=True) if val_df is not None else master_df.copy()
    raw_combined = raw_combined.ffill().bfill().fillna(0.0)

    dates_master = pd.to_datetime(master_df["date"]).dt.strftime('%Y-%m-%d').tolist() if "date" in master_df.columns else []
    dates_val = pd.to_datetime(val_df["date"]).dt.strftime('%Y-%m-%d').tolist() if val_df is not None and "date" in val_df.columns else []
    all_dates = dates_master + dates_val

    scaled_combined = raw_combined.copy()
    feature_cols = [c for c in master_df.columns if hasattr(scaler, 'feature_names_in_') and c in scaler.feature_names_in_]
    if not feature_cols:
        feature_cols = master_df.columns[:len(scaler.min_)]
    
    if len(feature_cols) > 0:
        scaled_combined[feature_cols] = scaler.transform(raw_combined[feature_cols])

    X_full, _, mask = _prepare_lstm_tensors(chromosome, scaled_combined)
    if len(X_full) == 0:
        print("❌ [VALIDATION ERROR] X_full array generation returned empty tensor!")
        return

    val_start_idx = max(0, len(master_df) - lookback)
    curr_window = X_full[min(val_start_idx, len(X_full) - 1): min(val_start_idx, len(X_full) - 1) + 1].copy()
    curr_window = np.nan_to_num(curr_window, nan=0.0)

    _, asset_cols = _split_features(master_df)
    selected_features = [
        asset_cols[i] for i, val in enumerate(mask) 
        if val == 1 and i < len(asset_cols)
    ]

    seq_pred = model.predict(curr_window, verbose=0)[0]
    if np.isnan(seq_pred).any() or np.isinf(seq_pred).any():
        seq_pred = np.nan_to_num(seq_pred, nan=0.0)

    val_preds_matrix = np.clip(seq_pred, -0.05, 0.05)
    model_base_dir = os.path.join(plot_dir, chromosome.get('id', 'model'))

    for f_idx, feat_name in enumerate(selected_features):
        if feat_name not in master_df.columns or f_idx >= val_preds_matrix.shape[1]:
            continue

        fig, ax = plt.subplots(figsize=(10, 4.5), dpi=150)
        pred_returns = val_preds_matrix[:, f_idx]

        asset_base = _extract_asset_name(feat_name)
        is_price = 'price' in feat_name.lower() or 'close' in feat_name.lower()
        sub_folder = "prc" if is_price else "vol"
        target_dir = os.path.join(model_base_dir, sub_folder)
        os.makedirs(target_dir, exist_ok=True)

        base_price = _get_dynamic_base_price(master_df, feat_name)

        if is_price:
            pred_plot = _reconstruct_price_series(pred_returns, base_price)
            hist_returns = master_df[feat_name].values[-100:]
            history_plot = _reconstruct_price_series(hist_returns, base_price)
            
            if val_df is not None and feat_name in val_df.columns:
                val_returns = val_df[feat_name].values[:horizon]
                actual_val_plot = _reconstruct_price_series(val_returns, history_plot[-1])
            else:
                actual_val_plot = np.full(horizon, history_plot[-1])

            y_label = f"Price (USD) - {asset_base.upper()}"
            title = f"Validation Overlay: {asset_base.upper()} Price Projection"
        else:
            pred_plot = pred_returns
            actual_val_plot = val_df[feat_name].values[:horizon] if val_df is not None and feat_name in val_df.columns else np.zeros(horizon)
            history_plot = master_df[feat_name].values[-100:]
            y_label = "Log Return / Change"
            title = f"Validation Target: {feat_name}"

        history_plot = np.nan_to_num(np.array(history_plot).flatten(), nan=1.0)
        actual_val_plot = np.nan_to_num(np.array(actual_val_plot).flatten(), nan=1.0)
        pred_plot = np.nan_to_num(np.array(pred_plot).flatten(), nan=1.0)

        last_hist_val = history_plot[-1] if len(history_plot) > 0 else 1.0
        connected_actual = np.insert(actual_val_plot, 0, last_hist_val)
        connected_pred = np.insert(pred_plot, 0, last_hist_val)

        n_hist = len(history_plot)
        x_hist = np.arange(n_hist)
        x_val_actual = np.arange(n_hist - 1, n_hist - 1 + len(connected_actual))
        x_val_pred = np.arange(n_hist - 1, n_hist - 1 + len(connected_pred))

        ax.plot(x_hist, history_plot, label='Training History', color='blue', linewidth=1.5)
        ax.plot(x_val_actual, connected_actual, label='Actual Validation Data', color='green', linewidth=1.5)
        ax.plot(x_val_pred, connected_pred, label=f'LSTM Forecast ({len(pred_plot)}d)', color='red', linestyle='--', linewidth=1.5)

        ax.axvline(x=n_hist - 1, color='gold', linestyle=':')
        ax.axvspan(n_hist - 1, n_hist - 1 + len(connected_pred), color='yellow', alpha=0.1)

        ax.set_title(title, fontweight='bold')
        ax.set_ylabel(y_label)
        _format_xaxis_dates(ax, all_dates, n_hist, horizon)
        ax.legend(loc='upper left')
        ax.grid(True, alpha=0.3)

        out_file = os.path.join(target_dir, f"val_overlay_{feat_name}.png")
        plt.savefig(out_file, bbox_inches="tight")
        plt.close()
        print(f"   ├── 📈 [PLOT SAVED] Validation Overlay: {out_file}")


def _plot_future_projection(chromosome: dict, model, scaler: MinMaxScaler, combined_df: pd.DataFrame, plot_dir: str):
    horizon = int(chromosome.get('forecast_horizon', 30))
    lookback = int(chromosome.get('lookback_window', 60))

    raw_combined = combined_df.ffill().bfill().fillna(0.0)
    dates = pd.to_datetime(combined_df["date"]).dt.strftime('%Y-%m-%d').tolist() if "date" in combined_df.columns else []

    scaled_combined = raw_combined.copy()
    feature_cols = [c for c in combined_df.columns if hasattr(scaler, 'feature_names_in_') and c in scaler.feature_names_in_]
    if not feature_cols:
        feature_cols = combined_df.columns[:len(scaler.min_)]

    if len(feature_cols) > 0:
        scaled_combined[feature_cols] = scaler.transform(raw_combined[feature_cols])

    X_full, _, mask = _prepare_lstm_tensors(chromosome, scaled_combined)
    if len(X_full) == 0:
        print("❌ [FUTURE ERROR] X_full array generation returned empty tensor!")
        return

    curr_future_win = X_full[-1:].copy()
    curr_future_win = np.nan_to_num(curr_future_win, nan=0.0)

    _, asset_cols = _split_features(combined_df)
    selected_features = [
        asset_cols[i] for i, val in enumerate(mask) 
        if val == 1 and i < len(asset_cols)
    ]

    seq_pred = model.predict(curr_future_win, verbose=0)[0]
    if np.isnan(seq_pred).any() or np.isinf(seq_pred).any():
        seq_pred = np.nan_to_num(seq_pred, nan=0.0)

    fut_preds_matrix = np.clip(seq_pred, -0.05, 0.05)
    model_base_dir = os.path.join(plot_dir, chromosome.get('id', 'model'))

    for f_idx, feat_name in enumerate(selected_features):
        if feat_name not in combined_df.columns or f_idx >= fut_preds_matrix.shape[1]:
            continue

        fig, ax = plt.subplots(figsize=(10, 4.5), dpi=150)
        fut_returns = fut_preds_matrix[:, f_idx]

        asset_base = _extract_asset_name(feat_name)
        is_price = 'price' in feat_name.lower() or 'close' in feat_name.lower()
        sub_folder = "prc" if is_price else "vol"
        target_dir = os.path.join(model_base_dir, sub_folder)
        os.makedirs(target_dir, exist_ok=True)

        base_price = _get_dynamic_base_price(combined_df, feat_name)

        if is_price:
            hist_returns = combined_df[feat_name].values[-100:]
            history_plot = _reconstruct_price_series(hist_returns, base_price)
            future_plot = _reconstruct_price_series(fut_returns, history_plot[-1])

            y_label = f"Price (USD) - {asset_base.upper()}"
            title = f"True Future Projection: {asset_base.upper()} Price"
        else:
            future_plot = fut_returns
            history_plot = combined_df[feat_name].values[-100:]
            y_label = "Log Return / Change"
            title = f"True Future Projection: {feat_name}"

        history_plot = np.nan_to_num(np.array(history_plot).flatten(), nan=1.0)
        future_plot = np.nan_to_num(np.array(future_plot).flatten(), nan=1.0)

        n_hist = len(history_plot)
        last_hist_val = history_plot[-1] if len(history_plot) > 0 else 1.0
        connected_future = np.insert(future_plot, 0, last_hist_val)

        x_hist = np.arange(n_hist)
        x_future = np.arange(n_hist - 1, n_hist - 1 + len(connected_future))

        ax.plot(x_hist, history_plot, label='Known Market Data', color='blue', linewidth=1.5)
        ax.plot(x_future, connected_future, label=f'True Future Forecast ({len(future_plot)}d)', color='magenta', linestyle='--', linewidth=1.5)

        ax.axvline(x=n_hist - 1, color='magenta', linestyle=':')
        ax.axvspan(n_hist - 1, n_hist - 1 + len(connected_future), color='purple', alpha=0.1)

        ax.set_title(title, fontweight='bold')
        ax.set_ylabel(y_label)
        _format_xaxis_dates(ax, dates, n_hist, horizon)
        ax.legend(loc='upper left')
        ax.grid(True, alpha=0.3)

        out_file = os.path.join(target_dir, f"future_forecast_{feat_name}.png")
        plt.savefig(out_file, bbox_inches="tight")
        plt.close()
        print(f"   └── 🔮 [PLOT SAVED] Future Projection: {out_file}")


def _export_candidate_model(chromosome: dict, model, scaler: MinMaxScaler, master_df: pd.DataFrame, export_dir: str, rank: int):
    model_id = chromosome['id']
    prefix = f"rank_{rank}_" if rank is not None else "final_"

    # 1. Save Keras Binary
    model_save_path = os.path.join(export_dir, f"{prefix}lstm_model.keras")
    model.save(model_save_path)
    tf.keras.backend.clear_session()

    # 2. Save MinMaxScaler Pickle
    with open(os.path.join(export_dir, f"{prefix}scaler.pkl"), 'wb') as f:
        pickle.dump(scaler, f)

    # 3. Save Structured Metadata JSON
    _, asset_cols = _split_features(master_df)
    selected_features = [
        col for col, mask_val in zip(asset_cols, chromosome.get('feature_mask', [])) if mask_val == 1
    ]

    metadata = {
        "chromosome_id": model_id,
        "rank": rank,
        "lookback_window": chromosome['lookback_window'],
        "forecast_horizon": chromosome['forecast_horizon'],
        "batch_size": chromosome.get('batch_size', 32),
        "learning_rate": chromosome.get('learning_rate', 0.001),
        "dropout_rate": chromosome.get('dropout_rate', 0.2),
        "selected_features": selected_features,
        "perf_vector": chromosome.get('perf_vector', [])
    }

    with open(os.path.join(export_dir, f"{prefix}metadata.json"), 'w') as f:
        json.dump(metadata, f, indent=4)


# ==============================================================================
# MAIN ENTRY POINT FOR VISUALIZATION WORKER
# ==============================================================================

def generate_pareto_graphs_and_exports(payload: dict) -> dict:
    """
    Celery task execution entry point. Reconstructs dataset DataFrames from
    payloads, fits candidate models, renders plot graphics, and packages deployment
    artifacts into prc/ and vol/ directories.
    """
    run_id = payload.get("run_id", "DEFAULT_RUN")
    gen_idx = payload.get("gen_idx", 1)
    
    log_dir, export_dir, plot_dir = resolve_target_directories(run_id)

    print("\n" + "═" * 80)
    print(f"📊 [VISUALIZATION WORKER] Generating Dynamic Price Plots & Exports for Run: {run_id} | Gen: {gen_idx}")
    print("═" * 80)

    # 1. Load Data Matrices
    if "master_data" in payload and payload["master_data"]:
        master_df = pd.read_json(io.StringIO(payload["master_data"])) if isinstance(payload["master_data"], str) else pd.DataFrame(payload["master_data"])
    else:
        master_df = pd.DataFrame()

    if "val_data" in payload and payload["val_data"]:
        val_df = pd.read_json(io.StringIO(payload["val_data"])) if isinstance(payload["val_data"], str) else pd.DataFrame(payload["val_data"])
    else:
        val_df = None

    scaler = MinMaxScaler(feature_range=(-1, 1))
    numeric_cols = master_df.select_dtypes(include=[np.number]).columns
    scaler.fit(master_df[numeric_cols])

    combined_df = pd.concat([master_df, val_df], ignore_index=True) if val_df is not None else master_df

    top_chromosomes = payload.get("top_chromosomes", [])

    # 2. Iterate Candidate Models
    for rank_idx, chromosome in enumerate(top_chromosomes):
        rank = rank_idx + 1
        c_id = chromosome.get("id", f"G{gen_idx}-M{rank_idx}")
        
        print(f"\n⚙️ [PARETO STEP] Processing Candidate Rank {rank}/{len(top_chromosomes)} (Model: {c_id})...")

        X_full, y_full, _ = _prepare_lstm_tensors(chromosome, combined_df)
        if len(X_full) == 0:
            print(f"⚠️ [PARETO SKIP] Candidate {c_id} yielded empty input tensors.")
            continue

        model = _build_and_train_full_model(chromosome, X_full, y_full)

        _plot_validation_overlay(chromosome, model, scaler, master_df, val_df, plot_dir)
        _plot_future_projection(chromosome, model, scaler, combined_df, plot_dir)

        _export_candidate_model(chromosome, model, scaler, master_df, export_dir, rank)

        gc.collect()

    print("\n" + "🎉" * 40)
    print(f"🎉 [WORKER COMPLETE] Visualization & Export tasks finished for Generation {gen_idx} (Run ID: {run_id})!")
    print("🎉" * 40 + "\n")

    return {"status": "success", "gen_idx": gen_idx, "run_id": run_id}