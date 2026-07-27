#!/usr/bin/env python3
# ##############################################################################
# File Name        : visualization_worker.py
# File Path        : apps/school/visualization_worker.py
#
# Author           : Chalearm Saelim & Gemini
# Owner            : Chalearm Saelim
# Reviewer         : Chalearm Saelim
#
# Version          : 1.0.0
# Status           : Development
# Created Date     : 2026-07-26 08:00:00 (UTC+7)
# Modified Date    : 2026-07-27 02:30:00 (UTC+7)
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
#     ├── _split_features(df)
#     ├── _prepare_lstm_tensors(chromosome, df)
#     ├── _build_and_train_full_model(chromosome, X, y)
#     ├── _plot_validation_overlay(chromosome, model, scaler, master_df, val_df, plot_dir)
#     ├── _plot_future_projection(chromosome, model, scaler, combined_df, plot_dir)
#     └── _export_candidate_model(chromosome, model, scaler, master_df, export_dir, rank)
#
# Responsibilities :
#    - Reconstructs DataFrame structures and fits MinMaxScalers from serialized task payloads.
#    - Fits candidate LSTM models across complete training sequences.
#    - Executes direct sequence-to-sequence multi-step prediction loops.
#    - Renders Matplotlib validation overlays and true future projection graphs into prc/ and vol/ subdirectories.
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
# Updated Parts :
#    [Function]
#      - _compile_lstm_architecture() (Added direct Seq2Seq horizon output layer)
#      - _plot_validation_overlay() (Updated to separate prc/vol paths and added telemetry audit)
#      - _plot_future_projection() (Updated to separate prc/vol paths and added telemetry audit)
#
# New Parts :
#    None
#
# Change History :
#    -------------------------------------------------------------------------
#    Version | Date Time (UTC+7)        | Author          | Description
#    -------------------------------------------------------------------------
#    1.0.0   | 2026-07-26 08:00:00      | Chalearm Saelim | Initial release
#    -------------------------------------------------------------------------
#
# TODO :
#    - Add interactive HTML plot exports alongside PNG image generation.
#
# Notes :
#    - Per regulator coding standard rules.
# ##############################################################################

import io
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
# HELPER UTILITIES: DATA RECONSTRUCTION & TENSOR SPLITTING
# ==============================================================================

# ##############################################################################
# Function Name : _compile_lstm_architecture
#
# Purpose :
#    Constructs and compiles a multi-layer Keras LSTM sequential model using
#    hyperparameters defined inside the candidate chromosome dictionary.
# ##############################################################################
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


# ##############################################################################
# Function Name : _split_features
#
# Purpose :
#    Separates global cyclical temporal channels from GA asset columns while
#    filtering out user-excluded features.
#
# Inputs :
#    df
#        Type        : pandas.DataFrame
#        Description : Target DataFrame to classify columns from.
#
# Return :
#    Type        : tuple (list, list)
#    Description : (time_cols, asset_cols)
#
# Complexity :
#    Time  : O(C) where C is column count.
#    Space : O(C)
#
# Error Cases :
#    - None
#
# Number Of Lines :
#    10
# ##############################################################################
def _split_features(df: pd.DataFrame):
    temporal_patterns = ['day_wk_sin', 'day_wk_cos', 'day_yr_sin', 'day_yr_cos', 'hour_sin', 'hour_cos', 'min_sin', 'min_cos']
    time_cols = [c for c in df.columns if any(p in c for p in temporal_patterns)]
    base_asset_cols = [c for c in df.columns if c not in time_cols and not c.startswith('close_')]
    
    USER_EXCLUDE_FEATURES = ['volume_log_change_fed']
    banned_lower = [banned.lower() for banned in USER_EXCLUDE_FEATURES]
    asset_cols = [c for c in base_asset_cols if c.lower() not in banned_lower]

    return time_cols, asset_cols


# ##############################################################################
# Function Name : _prepare_lstm_tensors
#
# Purpose :
#    Converts input Pandas DataFrame into 3D sliding-window tensor arrays (X, y)
#    based on chromosome lookback window and feature mask.
#
# Inputs :
#    chromosome
#        Type        : dict
#        Description : Target chromosome hyperparameter dictionary.
#    df
#        Type        : pandas.DataFrame
#        Description : Source dataset DataFrame.
#
# Return :
#    Type        : tuple (numpy.ndarray, numpy.ndarray, numpy.ndarray)
#    Description : (X_3d_array, y_2d_array, mask_array)
#
# Complexity :
#    Time  : O(S * L) where S=samples, L=lookback window.
#    Space : O(S * L * F) where F=active feature count.
#
# Error Cases :
#    - Returns empty arrays if sample count is less than 1.
#
# Number Of Lines :
#    22
# ##############################################################################
def _prepare_lstm_tensors(chromosome: dict, df: pd.DataFrame):
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
            y.append(asset_values[i + lookback : i + lookback + forecast])
        return np.array(X, dtype=np.float32), np.array(y, dtype=np.float32), mask

    return np.array([]), np.array([]), mask


# ##############################################################################
# Function Name : _build_and_train_full_model
#
# Purpose :
#    Constructs and fits a full-sequence Keras LSTM neural network model on the
#    entire dataset for plotting multi-step forecasts and deployment exports.
#
# Inputs :
#    chromosome
#        Type        : dict
#        Description : Chromosome containing architecture hyperparameters.
#    X
#        Type        : numpy.ndarray
#        Description : 3D feature tensor array.
#    y
#        Type        : numpy.ndarray
#        Description : 2D/3D target matrix array.
#
# Return :
#    Type        : tensorflow.keras.Sequential
#    Description : Trained Keras Sequential LSTM model.
#
# Complexity :
#    Time  : O(E * B) where E=10 epochs, B=batch count.
#    Space : O(W) where W is network parameter count.
#
# Error Cases :
#    - None
#
# Number Of Lines :
#    32
# ##############################################################################
def _build_and_train_full_model(chromosome: dict, X_train: np.ndarray, y_train: np.ndarray):
    # 1. Eradicate NaNs / Infs from training arrays
    X_train = np.nan_to_num(X_train, nan=0.0, posinf=1.0, neginf=-1.0)
    y_train = np.nan_to_num(y_train, nan=0.0, posinf=1.0, neginf=-1.0)

    # 2. Derive num_targets dynamically from y_train shape
    num_targets = y_train.shape[-1] if len(y_train.shape) > 1 else 1
    forecast_horizon = int(chromosome.get('forecast_horizon', 52))

    # 3. Compile architecture with correct output dimensions
    model = _compile_lstm_architecture(
        chromosome, 
        input_shape=(X_train.shape[1], X_train.shape[2]), 
        num_targets=num_targets,
        forecast_horizon=forecast_horizon
    )

    # 4. Compile optimizer with Keras 3 safe single clipvalue parameter
    lr = min(float(chromosome.get('learning_rate', 0.001)), 0.001)
    optimizer = tf.keras.optimizers.Adam(learning_rate=lr, clipvalue=0.5)
    model.compile(optimizer=optimizer, loss='mse')

    # 5. Fit model safely
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

# ##############################################################################
# Function Name : _plot_validation_overlay
#
# Purpose :
#    Executes multi-step forecasts on validation periods and renders Matplotlib
#    overlay graphs into prc/ and vol/ subdirectories comparing predicted close
#    prices against actual validation ground truth data with diagnostic logging.
#
# Inputs :
#    chromosome
#        Type        : dict
#        Description : Candidate model chromosome.
#    model
#        Type        : tensorflow.keras.Model
#        Description : Fitted Keras LSTM model.
#    scaler
#        Type        : sklearn.preprocessing.MinMaxScaler
#        Description : Fitted MinMaxScaler object.
#    master_df
#        Type        : pandas.DataFrame
#        Description : Training master DataFrame.
#    val_df
#        Type        : pandas.DataFrame or None
#        Description : Validation dataset DataFrame.
#    plot_dir
#        Type        : str
#        Description : Target directory to save rendered PNG plot files.
#
# Return :
#    Type        : None
#    Description : Saves validation plot images to plot_dir.
#
# Complexity :
#    Time  : O(H * M + F * S) where H=horizon, M=model inference, F=features.
#    Space : O(H * F)
#
# Error Cases :
#    - Silently returns if input training tensors are empty.
# ##############################################################################
def _plot_validation_overlay(chromosome: dict, model, scaler: MinMaxScaler, master_df: pd.DataFrame, val_df: pd.DataFrame, plot_dir: str):
    horizon = int(chromosome.get('forecast_horizon', 30))
    lookback = int(chromosome.get('lookback_window', 60))
    
    raw_combined = pd.concat([master_df, val_df], ignore_index=True) if val_df is not None else master_df.copy()
    raw_combined = raw_combined.ffill().bfill().fillna(0.0)

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

    # ==========================================================================
    # 🔍 FULL DIAGNOSTIC TELEMETRY HEADER
    # ==========================================================================
    print("\n" + "=" * 80)
    print(f"🐞 [DIAGNOSTIC START] Validation Rollout Audit for Chromosome: {chromosome.get('id')}")
    print(f"   ├── Target Forecast Horizon : {horizon} steps (days)")
    print(f"   ├── Lookback Window Size    : {lookback} steps")
    print(f"   ├── Input Window Shape      : {curr_window.shape} (Batch, Lookback, Features)")
    print(f"   ├── Initial Input Min / Max : Min = {curr_window.min():.6f}, Max = {curr_window.max():.6f}")
    print(f"   ├── Keras Input Spec        : {model.input_shape}")
    print(f"   └── Keras Output Spec       : {model.output_shape}")
    print("=" * 80)

    # Direct Sequence-to-Sequence Multi-Horizon Forecast
    seq_pred = model.predict(curr_window, verbose=0)[0]  # Shape: (horizon, num_targets)
    if np.isnan(seq_pred).any() or np.isinf(seq_pred).any():
        print("⚠️ [WARN] Validation output contained NaN/Inf! Applying nan_to_num recovery.")
        seq_pred = np.nan_to_num(seq_pred, nan=0.0)

    val_preds_matrix = np.clip(seq_pred, -0.05, 0.05)

    print("=" * 80)
    print(f"📊 [DIAGNOSTIC SUMMARY] Validation Rollout Completed ({len(val_preds_matrix)} / {horizon} steps)")
    print(f"   ├── Prediction Matrix Range : Min = {val_preds_matrix.min():.6f}, Max = {val_preds_matrix.max():.6f}")
    print(f"   └── Prediction Std Dev      : {val_preds_matrix.std():.6f}")
    print("=" * 80 + "\n")

    # Base model output directory: prediction_result/<RUN_ID>/<CHROMOSOME_ID>
    model_base_dir = os.path.join(plot_dir, chromosome.get('id', 'model'))

    for f_idx, feat_name in enumerate(selected_features):
        if feat_name not in master_df.columns or f_idx >= val_preds_matrix.shape[1]:
            continue

        fig, ax = plt.subplots(figsize=(10, 4))
        pred_returns = val_preds_matrix[:, f_idx]

        asset_base = feat_name.replace('price_log_return_', '').replace('volume_log_change_', '')
        raw_close_col = f'close_{asset_base}'

        # Route to prc/ or vol/ sub-folder
        is_price = 'price' in feat_name.lower() or 'close' in feat_name.lower()
        sub_folder = "prc" if is_price else "vol"
        target_dir = os.path.join(model_base_dir, sub_folder)
        os.makedirs(target_dir, exist_ok=True)

        if is_price and raw_close_col in master_df.columns:
            valid_prices = master_df[raw_close_col].dropna()
            last_known_price = float(valid_prices.iloc[-1]) if not valid_prices.empty else 100.0
            pred_plot = last_known_price * np.exp(np.cumsum(pred_returns))
            history_plot = master_df[raw_close_col].values[-100:]
            
            if val_df is not None and raw_close_col in val_df.columns:
                actual_val_plot = val_df[raw_close_col].values[:horizon]
            else:
                actual_val_plot = np.full(horizon, last_known_price)

            print(f"📈 [VALIDATION AUDIT] {feat_name.upper()}")
            print(f"   ├── Baseline Last Price : ${last_known_price:.2f}")
            print(f"   ├── Unscaled Return Range: Min = {pred_returns.min():.6f}, Max = {pred_returns.max():.6f}")
            print(f"   └── Projected Price Range: Start = ${pred_plot[0]:.2f}, End = ${pred_plot[-1]:.2f}")

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
        ax.legend(loc='upper left')
        ax.grid(True, alpha=0.3)

        plt.savefig(os.path.join(target_dir, f"val_overlay_{feat_name}.png"))
        plt.close()


# ##############################################################################
# Function Name : _plot_future_projection
#
# Purpose :
#    Generates true future market forecasts beyond all dataset dates using
#    direct sequence multi-horizon predictions into prc/ and vol/ directories.
# ##############################################################################
def _plot_future_projection(chromosome: dict, model, scaler: MinMaxScaler, combined_df: pd.DataFrame, plot_dir: str):
    horizon = int(chromosome.get('forecast_horizon', 30))
    lookback = int(chromosome.get('lookback_window', 60))

    raw_combined = combined_df.ffill().bfill().fillna(0.0)

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

    # ==========================================================================
    # 🔮 FULL DIAGNOSTIC TELEMETRY HEADER
    # ==========================================================================
    print("\n" + "=" * 80)
    print(f"🔮 [FUTURE DIAGNOSTIC START] Future Projection Audit for Chromosome: {chromosome.get('id')}")
    print(f"   ├── Target Forecast Horizon : {horizon} steps (days)")
    print(f"   ├── Lookback Window Size    : {lookback} steps")
    print(f"   ├── Future Window Shape     : {curr_future_win.shape} (Batch, Lookback, Features)")
    print(f"   ├── Initial Input Min / Max : Min = {curr_future_win.min():.6f}, Max = {curr_future_win.max():.6f}")
    print(f"   ├── Keras Input Spec        : {model.input_shape}")
    print(f"   └── Keras Output Spec       : {model.output_shape}")
    print("=" * 80)

    # Direct Sequence-to-Sequence Multi-Horizon Forecast
    seq_pred = model.predict(curr_future_win, verbose=0)[0]  # Shape: (horizon, num_targets)
    if np.isnan(seq_pred).any() or np.isinf(seq_pred).any():
        print("⚠️ [FUTURE WARN] Model output contained NaN/Inf! Applying nan_to_num recovery.")
        seq_pred = np.nan_to_num(seq_pred, nan=0.0)

    fut_preds_matrix = np.clip(seq_pred, -0.05, 0.05)

    print("=" * 80)
    print(f"📊 [FUTURE DIAGNOSTIC SUMMARY] Future Rollout Completed ({len(fut_preds_matrix)} / {horizon} steps)")
    print(f"   ├── Prediction Matrix Range : Min = {fut_preds_matrix.min():.6f}, Max = {fut_preds_matrix.max():.6f}")
    print(f"   └── Prediction Std Dev      : {fut_preds_matrix.std():.6f}")
    print("=" * 80 + "\n")

    # Base model output directory: prediction_result/<RUN_ID>/<CHROMOSOME_ID>
    model_base_dir = os.path.join(plot_dir, chromosome.get('id', 'model'))

    for f_idx, feat_name in enumerate(selected_features):
        if feat_name not in combined_df.columns or f_idx >= fut_preds_matrix.shape[1]:
            continue

        fig, ax = plt.subplots(figsize=(10, 4))
        fut_returns = fut_preds_matrix[:, f_idx]

        asset_base = feat_name.replace('price_log_return_', '').replace('volume_log_change_', '')
        raw_close_col = f'close_{asset_base}'

        # Route to prc/ or vol/ sub-folder
        is_price = 'price' in feat_name.lower() or 'close' in feat_name.lower()
        sub_folder = "prc" if is_price else "vol"
        target_dir = os.path.join(model_base_dir, sub_folder)
        os.makedirs(target_dir, exist_ok=True)

        if is_price and raw_close_col in combined_df.columns:
            valid_prices = combined_df[raw_close_col].dropna()
            last_known_price = float(valid_prices.iloc[-1]) if not valid_prices.empty else 100.0
            
            future_plot = last_known_price * np.exp(np.cumsum(fut_returns))
            history_plot = combined_df[raw_close_col].values[-100:]

            print(f"📈 [FUTURE FEATURE AUDIT] {feat_name.upper()}")
            print(f"   ├── Baseline Last Price : ${last_known_price:.2f}")
            print(f"   ├── Unscaled Return Range: Min = {fut_returns.min():.6f}, Max = {fut_returns.max():.6f}")
            print(f"   └── Projected Price Range: Start = ${future_plot[0]:.2f}, End = ${future_plot[-1]:.2f}")

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
        ax.legend(loc='upper left')
        ax.grid(True, alpha=0.3)

        plt.savefig(os.path.join(target_dir, f"future_forecast_{feat_name}.png"))
        plt.close()


# ##############################################################################
# Function Name : _export_candidate_model
#
# Purpose :
#    Serializes Keras binary model (.keras), MinMaxScaler pickle (.pkl), and
#    structured metadata (.json) into the deployed_models/<run_id>/ directory.
#
# Inputs :
#    chromosome
#        Type        : dict
#        Description : Chromosome dictionary containing model genes.
#    model
#        Type        : tensorflow.keras.Model
#        Description : Fitted Keras LSTM model object.
#    scaler
#        Type        : sklearn.preprocessing.MinMaxScaler
#        Description : Fitted MinMaxScaler object.
#    master_df
#        Type        : pandas.DataFrame
#        Description : Training master DataFrame.
#    export_dir
#        Type        : str
#        Description : Target deployment export directory path.
#    rank
#        Type        : int
#        Description : Pareto front rank index.
#
# Return :
#    Type        : None
#    Description : Saves serialized model files to export_dir.
#
# Complexity :
#    Time  : O(W) where W is network weight count.
#    Space : O(W)
#
# Error Cases :
#    - None
#
# Number Of Lines :
#    35
# ##############################################################################
def _export_candidate_model(chromosome: dict, model, scaler: MinMaxScaler, master_df: pd.DataFrame, export_dir: str, rank: int):
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

# ##############################################################################
# Function Name : generate_pareto_graphs_and_exports
#
# Purpose :
#    Celery task execution entry point. Reconstructs dataset DataFrames from
#    payloads, fits candidate models, renders plot graphics, and packages deployment
#    artifacts.
#
# Inputs :
#    top_chromosomes_json
#        Type        : list
#        Description : List of elite Pareto candidate chromosome dictionaries.
#    master_data_json
#        Type        : str or pandas.DataFrame
#        Description : Master training dataset passed as JSON string or DataFrame.
#    val_data_json
#        Type        : str or pandas.DataFrame or None
#        Description : Validation dataset passed as JSON string or DataFrame.
#    gen_num
#        Type        : int
#        Description : Current generation number index.
#    run_id
#        Type        : str or None
#        Description : Active run hex ID string.
#
# Return :
#    Type        : dict
#    Description : Dictionary containing execution status, gen_num, and run_id.
#
# Complexity :
#    Time  : O(C * (M + P)) where C=candidates, M=model training, P=plotting.
#    Space : O(D + W) where D=dataset size, W=model memory.
#
# Error Cases :
#    - Catches dataset parsing errors and logs detailed error reports.
#
# Number Of Lines :
#    40
# ##############################################################################
def generate_pareto_graphs_and_exports(top_chromosomes_json: list, master_data_json, val_data_json=None, gen_num: int = 1, run_id: str = None) -> dict:
    logger.info(f"🎨 [WORKER] Generating Plots & Models for Generation {gen_num} (Run ID: {run_id or 'LEGACY'})...")
    
    # 1. Resolve Target Output Directories from Shared utils.py
    log_dir, export_dir, plot_dir = resolve_target_directories(run_id)

    # 2. Reconstruct DataFrames & Scaler from input (handles both DataFrame and raw JSON string)
    if isinstance(master_data_json, pd.DataFrame):
        master_df = master_data_json.copy()
    elif isinstance(master_data_json, str):
        master_df = pd.read_json(io.StringIO(master_data_json))
    else:
        master_df = pd.DataFrame(master_data_json)

    if val_data_json is not None:
        if isinstance(val_data_json, pd.DataFrame):
            val_df = val_data_json.copy()
        elif isinstance(val_data_json, str):
            val_df = pd.read_json(io.StringIO(val_data_json))
        else:
            val_df = pd.DataFrame(val_data_json)
    else:
        val_df = None
    
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