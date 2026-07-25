#!/usr/bin/env python3
#/******************************************************************************
#* File Name        : visualization_worker.py
#* Path             : apps/school/visualization_worker.py
#* Author           : Chalearm Saelim & Gemini
#* System Role      : Offloaded Visualization & Model Packaging Engine
#* Architecture     : Distributed Client-Server / Master-Worker (Celery + Redis)
#* 
#* DEPENDENCY TREE & STRUCTURAL MAP:
#* ─────────────────────────────────────────────────────────────────────────────
#* [celery_tasks.py] (Celery Task Router)
#*    └── Calls ──> [visualization_worker.py] (Offloaded Renderer & Serializer)
#*                   │
#*                   ├── Imports ──> [utils.py] (resolve_target_directories)
#*                   ├── Reconstructs Pandas DataFrames from JSON strings
#*                   ├── Binds MinMaxScaler & Column Classifications
#*                   ├── Re-trains Keras LSTM on Full Sequences
#*                   ├── Generates Dynamic Autoregressive Multi-Step Forecasts
#*                   │    ├── Validation Overlay Plots  ──> [prediction_result/<run_id>/]
#*                   │    └── True Future Projections   ──> [prediction_result/<run_id>/]
#*                   │
#*                   └── Serializes .keras Models, Scalers, & JSON Metadata 
#*                        ──> [deployed_models/<run_id>/]
#*
#* FUNCTION DEPENDENCY MATRIX (Internal Sub-Routines):
#* ─────────────────────────────────────────────────────────────────────────────
#* generate_pareto_graphs_and_exports(...)
#*  ├── utils.resolve_target_directories(run_id)
#*  ├── _reconstruct_dataset(master_data_json, val_data_json)
#*  ├── _build_and_train_full_model(chromosome, X, y)
#*  ├── _plot_validation_overlay(chromosome, model, scaler, master_df, val_df, plot_dir)
#*  ├── _plot_future_projection(chromosome, model, scaler, combined_df, plot_dir)
#*  └── _export_candidate_model(chromosome, model, scaler, master_df, export_dir, rank)
#******************************************************************************/

import os
import sys
import gc
import pickle
import json
import logging
import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
from sklearn.preprocessing import MinMaxScaler

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
# HELPER UTILITIES: DATA RECONSTRUCTION & TENSOR SPLITTING
# ==============================================================================

def _split_features(df: pd.DataFrame):
    """
    Separates global cyclical temporal channels from GA optimization asset columns.
    """
    temporal_patterns = ['day_wk_sin', 'day_wk_cos', 'day_yr_sin', 'day_yr_cos', 'hour_sin', 'hour_cos', 'min_sin', 'min_cos']
    time_cols = [c for c in df.columns if any(p in c for p in temporal_patterns)]
    base_asset_cols = [c for c in df.columns if c not in time_cols and not c.startswith('close_')]
    
    USER_EXCLUDE_FEATURES = ['volume_log_change_fed']
    banned_lower = [banned.lower() for banned in USER_EXCLUDE_FEATURES]
    asset_cols = [c for c in base_asset_cols if c.lower() not in banned_lower]

    return time_cols, asset_cols


def _prepare_lstm_tensors(chromosome: dict, df: pd.DataFrame):
    """
    Converts input Pandas DataFrame into 3D sliding-window tensor arrays (X, y).
    """
    time_cols, asset_cols = _split_features(df)
    mask = np.array(chromosome.get('feature_mask', []))

    if len(mask) != len(asset_cols):
        mask = np.ones(len(asset_cols), dtype=int)

    asset_values = df[asset_cols].values[:, mask == 1]
    time_values = df[time_cols].values
    combined_data = np.hstack([asset_values, time_values])

    lookback = int(chromosome.get('lookback_window', 60))
    forecast = int(chromosome.get('forecast_horizon', 30))

    num_samples = len(combined_data) - lookback - forecast
    if num_samples > 0:
        X, y = [], []
        for i in range(num_samples):
            X.append(combined_data[i : (i + lookback)])
            y.append(asset_values[i + lookback + forecast])
        return np.array(X, dtype=np.float32), np.array(y, dtype=np.float32), mask

    return np.array([]), np.array([]), mask


def _build_and_train_full_model(chromosome: dict, X: np.ndarray, y: np.ndarray):
    """
    Trains a full-horizon Keras LSTM model for plotting and deployment exports.
    """
    import tensorflow as tf
    tf.get_logger().setLevel('ERROR')

    num_timesteps = X.shape[1]
    num_features = X.shape[2]
    num_targets = y.shape[1]

    num_layers = chromosome['lstm_layers']
    nodes = chromosome['nodes_per_layer']
    lr = chromosome['learning_rate']
    dropout_rate = chromosome['dropout_rate']
    batch_size = chromosome['batch_size']

    model = tf.keras.Sequential()
    model.add(tf.keras.Input(shape=(num_timesteps, num_features)))
    
    for i in range(num_layers):
        is_last = (i == num_layers - 1)
        node_count = nodes[i] if i < len(nodes) else nodes[0]
        model.add(tf.keras.layers.LSTM(node_count, return_sequences=not is_last))
        model.add(tf.keras.layers.Dropout(dropout_rate))

    model.add(tf.keras.layers.Dense(num_targets))

    optimizer = tf.keras.optimizers.Adam(learning_rate=lr, clipnorm=1.0)
    model.compile(optimizer=optimizer, loss='mse')

    model.fit(
        X, y,
        epochs=10,
        batch_size=batch_size,
        verbose=0
    )

    return model


# ==============================================================================
# SUB-ROUTINES: GRAPH RENDERING & MODEL EXPORTS
# ==============================================================================

def _plot_validation_overlay(chromosome: dict, model, scaler: MinMaxScaler, master_df: pd.DataFrame, val_df: pd.DataFrame, plot_dir: str):
    """
    Generates dynamic autoregressive multi-step validation overlay prediction graphs.
    """
    horizon = int(chromosome.get('forecast_horizon', 30))
    X_train, y_train, mask = _prepare_lstm_tensors(chromosome, master_df)
    
    if len(X_train) == 0:
        return

    _, asset_cols = _split_features(master_df)
    selected_features = [asset_cols[i] for i, val in enumerate(mask) if val == 1]

    curr_window = X_train[-1:].copy()
    val_preds_list = []

    for step in range(horizon):
        step_pred = model.predict(curr_window, verbose=0)
        val_preds_list.append(step_pred[0])

        next_feature_row = curr_window[0, -1, :].copy()
        num_targets = step_pred.shape[1]
        next_feature_row[:num_targets] = step_pred[0]

        new_win = np.append(curr_window[0, 1:, :], [next_feature_row], axis=0)
        curr_window = np.expand_dims(new_win, axis=0)

    val_preds_matrix = np.array(val_preds_list)

    for f_idx, feat_name in enumerate(selected_features):
        if feat_name not in master_df.columns:
            continue

        fig, ax = plt.subplots(figsize=(12, 4))
        master_col_idx = master_df.columns.get_loc(feat_name)
        feat_min = scaler.min_[master_col_idx]
        feat_scale = scaler.scale_[master_col_idx]

        pred_unscaled = (val_preds_matrix[:, f_idx] - feat_min) / feat_scale

        asset_base = feat_name.replace('price_log_return_', '').replace('volume_log_change_', '')
        raw_close_col = f'close_{asset_base}'

        if raw_close_col in master_df.columns:
            last_known_price = float(master_df[raw_close_col].iloc[-1])
            pred_plot = last_known_price * np.exp(np.cumsum(pred_unscaled))
            history_plot = master_df[raw_close_col].values[-100:]
            
            if val_df is not None and raw_close_col in val_df.columns:
                actual_val_plot = val_df[raw_close_col].values[:horizon]
            else:
                actual_val_plot = last_known_price * np.exp(np.cumsum(val_df[feat_name].values[:horizon])) if val_df is not None and feat_name in val_df.columns else np.zeros(horizon)
                
            y_label = f"Price (USD) - {asset_base.upper()}"
            title = f"Validation Overlay: {asset_base.upper()} Price Projection"
        else:
            pred_plot = pred_unscaled
            actual_val_plot = val_df[feat_name].values[:horizon] if val_df is not None and feat_name in val_df.columns else np.zeros(horizon)
            history_plot = master_df[feat_name].values[-100:]
            y_label = "Log Return"
            title = f"Validation Target: {feat_name}"

        history_plot = np.array(history_plot).flatten()
        actual_val_plot = np.array(actual_val_plot).flatten()
        pred_plot = np.array(pred_plot).flatten()

        last_hist_val = history_plot[-1]
        connected_actual = np.insert(actual_val_plot, 0, last_hist_val)
        connected_pred = np.insert(pred_plot, 0, last_hist_val)

        n_hist = len(history_plot)
        x_hist = np.arange(n_hist)
        x_val_actual = np.arange(n_hist - 1, n_hist - 1 + len(connected_actual))
        x_val_pred = np.arange(n_hist - 1, n_hist - 1 + len(connected_pred))

        ax.plot(x_hist, history_plot, label='Training History', color='blue', linewidth=2)
        ax.plot(x_val_actual, connected_actual, label='Actual Validation Data', color='green', linewidth=2, marker='o', ms=3)
        ax.plot(x_val_pred, connected_pred, label=f'LSTM Forecast ({horizon}d)', color='red', linestyle='--', linewidth=2, marker='s', ms=3)

        ax.axvline(x=n_hist - 1, color='gold', linestyle=':')
        ax.axvspan(n_hist - 1, n_hist - 1 + len(connected_pred), color='yellow', alpha=0.1)

        ax.set_title(title, fontweight='bold')
        ax.set_ylabel(y_label)
        ax.legend(loc='upper left')
        ax.grid(True, alpha=0.3)

        plt.savefig(os.path.join(plot_dir, f"val_overlay_{chromosome['id']}_{feat_name}.png"))
        plt.close()


def _plot_future_projection(chromosome: dict, model, scaler: MinMaxScaler, combined_df: pd.DataFrame, plot_dir: str):
    """
    Generates dynamic true future projections beyond all known historical market data.
    """
    horizon = int(chromosome.get('forecast_horizon', 30))
    X_full, _, mask = _prepare_lstm_tensors(chromosome, combined_df)

    if len(X_full) == 0:
        return

    _, asset_cols = _split_features(combined_df)
    selected_features = [asset_cols[i] for i, val in enumerate(mask) if val == 1]

    curr_future_win = X_full[-1:].copy()
    fut_preds_list = []

    for step in range(horizon):
        step_pred = model.predict(curr_future_win, verbose=0)
        fut_preds_list.append(step_pred[0])

        next_feature_row = curr_future_win[0, -1, :].copy()
        num_targets = step_pred.shape[1]
        next_feature_row[:num_targets] = step_pred[0]

        new_win = np.append(curr_future_win[0, 1:, :], [next_feature_row], axis=0)
        curr_future_win = np.expand_dims(new_win, axis=0)

    fut_preds_matrix = np.array(fut_preds_list)

    for f_idx, feat_name in enumerate(selected_features):
        if feat_name not in combined_df.columns:
            continue

        fig, ax = plt.subplots(figsize=(12, 4))
        master_col_idx = combined_df.columns.get_loc(feat_name)
        feat_min = scaler.min_[master_col_idx]
        feat_scale = scaler.scale_[master_col_idx]

        fut_pred_unscaled = (fut_preds_matrix[:, f_idx] - feat_min) / feat_scale

        asset_base = feat_name.replace('price_log_return_', '').replace('volume_log_change_', '')
        raw_close_col = f'close_{asset_base}'

        if raw_close_col in combined_df.columns:
            last_known_price = float(combined_df[raw_close_col].iloc[-1])
            future_plot = last_known_price * np.exp(np.cumsum(fut_pred_unscaled))
            history_plot = combined_df[raw_close_col].values[-100:]
            y_label = f"Price (USD) - {asset_base.upper()}"
            title = f"True Future Projection: {asset_base.upper()} Price"
        else:
            future_plot = fut_pred_unscaled
            history_plot = combined_df[feat_name].values[-100:]
            y_label = "Log Return"
            title = f"True Future Projection: {feat_name}"

        history_plot = np.array(history_plot).flatten()
        future_plot = np.array(future_plot).flatten()

        n_hist = len(history_plot)
        last_hist_val = history_plot[-1]
        connected_future = np.insert(future_plot, 0, last_hist_val)

        x_hist = np.arange(n_hist)
        x_future = np.arange(n_hist - 1, n_hist - 1 + len(connected_future))

        ax.plot(x_hist, history_plot, label='Known Market Data', color='blue', linewidth=2)
        ax.plot(x_future, connected_future, label=f'True Future Forecast ({horizon}d)', color='magenta', linestyle='--', linewidth=2, marker='^', ms=3)

        ax.axvline(x=n_hist - 1, color='magenta', linestyle=':')
        ax.axvspan(n_hist - 1, n_hist - 1 + len(connected_future), color='purple', alpha=0.1)

        ax.set_title(title, fontweight='bold')
        ax.set_ylabel(y_label)
        ax.legend(loc='upper left')
        ax.grid(True, alpha=0.3)

        plt.savefig(os.path.join(plot_dir, f"future_forecast_{chromosome['id']}_{feat_name}.png"))
        plt.close()


def _export_candidate_model(chromosome: dict, model, scaler: MinMaxScaler, master_df: pd.DataFrame, export_dir: str, rank: int):
    """
    Serializes Keras model file (.keras), scaler object (.pkl), and json metadata.
    """
    import tensorflow as tf
    
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

def generate_pareto_graphs_and_exports(top_chromosomes_json: list, master_data_json: str, val_data_json: str, gen_num: int, run_id: str = None) -> dict:
    """
    Task entry point called by Celery. Reconstructs datasets, builds trained models, 
    renders Matplotlib overlay/future plots, and exports deployed Keras artifacts.
    """
    logger.info(f"🎨 [WORKER] Generating Plots & Models for Generation {gen_num} (Run ID: {run_id or 'LEGACY'})...")
    
    # 1. Resolve Target Output Directories from Shared utils.py
    log_dir, export_dir, plot_dir = resolve_target_directories(run_id)

    # 2. Reconstruct DataFrames & Scaler from JSON
    master_df = pd.read_json(master_data_json)
    val_df = pd.read_json(val_data_json) if val_data_json else None
    
    scaler = MinMaxScaler(feature_range=(-1, 1))
    scaler.fit(master_df)

    combined_df = pd.concat([master_df, val_df]) if val_df is not None else master_df

    # 3. Process Each Candidate
    for rank_idx, chromosome in enumerate(top_chromosomes_json):
        rank = rank_idx + 1
        logger.info(f"💾 [WORKER] Processing Candidate Rank {rank} (Model: {chromosome['id']})...")

        # Prepare Tensors & Train Full Model
        X_full, y_full, _ = _prepare_lstm_tensors(chromosome, combined_df)
        if len(X_full) == 0:
            continue

        model = _build_and_train_full_model(chromosome, X_full, y_full)

        # Plot Graphs
        _plot_validation_overlay(chromosome, model, scaler, master_df, val_df, plot_dir)
        _plot_future_projection(chromosome, model, scaler, combined_df, plot_dir)

        # Export Deployed Artifacts
        _export_candidate_model(chromosome, model, scaler, master_df, export_dir, rank)

        gc.collect()

    logger.info(f"✅ [WORKER] Completed Visualization & Export task for Generation {gen_num}.")
    return {"status": "success", "gen_num": gen_num, "run_id": run_id}