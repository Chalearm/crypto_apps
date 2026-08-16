#!/usr/bin/env python3
#!/usr/bin/env python3
# ##############################################################################
# File Name        : visualization_worker.py
# File Path        : apps/school/visualization_worker.py
#
# Author           : Chalearm Saelim & Gemini
# Owner            : Chalearm Saelim
# Reviewer         : Chalearm Saelim
#
# Version          : 1.2.0
# Status           : Development
# Created Date     : 2026-07-26 08:00:00 (UTC+7)
# Modified Date    : 2026-08-13 14:17:20 (UTC+7)
#
# Description      :
#    Offloaded worker node engine that reconstructs datasets, trains full-horizon
#    Rust LSTM models for top Pareto front candidate chromosomes, renders multi-step
#    direct sequence-to-sequence validation overlays and future forecasting plots via
#    Matplotlib, and serializes deployed scaler pickles and JSON metadata.
#
#    DEPENDENCY TREE & STRUCTURAL MAP:
#    ───────────────────────────────────────────────────────────────────────────
#    [celery_tasks.py] (Celery Task Router)
#        └── Calls ──> [visualization_worker.py] (Offloaded Renderer & Serializer)
#                        │
#                        ├── Imports ──> [utils.py] (resolve_target_directories)
#                        ├── Reconstructs Pandas DataFrames from JSON strings
#                        ├── Applies Strict Feature Masking (Plots ONLY selected features)
#                        ├── Dynamically extracts actual base prices from datasets
#                        ├── Binds MinMaxScaler & Column Classifications
#                        ├── Re-trains Native Rust LSTM on Full Sequences
#                        ├── Generates Direct Sequence Multi-Step Forecasts
#                        │    ├── Validation Overlays  ──> [prediction_result/<run_id>/G<gen_idx>/<M_id>/prc/ & vol/]
#                        │    └── True Future Forecasts──> [prediction_result/<run_id>/G<gen_idx>/<M_id>/prc/ & vol/]
#                        │
#                        └── Serializes Scalers & JSON Metadata 
#                            ──> [deployed_models/<run_id>/G<gen_idx>/<M_id>/]
#
#    FUNCTION DEPENDENCY MATRIX (Internal Sub-Routines):
#    ───────────────────────────────────────────────────────────────────────────
#    generate_pareto_graphs_and_exports(...)
#     ├── utils.resolve_target_directories(run_id)
#     ├── _get_model_dirs(run_id, gen_idx, model_id)
#     ├── _render_standalone_pareto_plot(top_chromosomes, run_id, gen_idx)
#     ├── _split_features(df)
#     ├── _prepare_lstm_tensors(chromosome, df)
#     ├── _get_inference_window(df_scaled, absolute_end_idx, lookback, mask, time_cols, asset_cols)
#     ├── _audit_per_asset_prediction_variance(asset_names, y_true, y_pred, model_id)
#     ├── _plot_validation_overlay(chromosome, hist_preds_scaled, fut_preds_scaled, scaler, master_df, val_df, prc_dir, vol_dir)
#     ├── _plot_future_projection(chromosome, hist_preds_scaled, fut_preds_scaled, scaler, combined_df, prc_dir, vol_dir, val_len)
#     └── _export_candidate_model(chromosome, scaler, combined_df, export_dir, rank)
#
# Responsibilities :
#    - Reconstructs DataFrame structures and fits MinMaxScalers from serialized task payloads.
#    - Enforces Strict Feature Masking to render only AI-selected features.
#    - Dynamically derives actual starting asset prices from raw dataset columns.
#    - Fits candidate Native Rust LSTM models across complete training sequences.
#    - Executes direct sequence-to-sequence multi-step prediction loops.
#    - Renders Matplotlib validation overlays and true future projection graphs with date X-axis references.
#    - Exports scaler pickles (.pkl) and structured JSON metadata.
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
#      - rust_lstm_engine
#
#    External :
#      - numpy, pandas, matplotlib, scikit-learn
#
# Change History :
#    -------------------------------------------------------------------------
#    Version | Date Time (UTC+7)         | Author          | Description
#    -------------------------------------------------------------------------
#    1.0.0   | 2026-07-26 08:00:00       | Chalearm Saelim | Initial release
#    1.1.0   | 2026-07-31 15:00:00       | Chalearm Saelim | Added dynamic price extraction & date ticks
#    1.2.0   | 2026-08-13 14:17:20       | Chalearm Saelim | Fixed slicing bugs, strict masking, updated directory routing & Rust LSTM integration
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
try:
    import tensorflow as tf
    from tensorflow.keras.models import Sequential
    from tensorflow.keras.layers import LSTM, Dense, Dropout, Reshape, Input, Flatten
except ImportError as imp_err:
    tf = None
    Sequential = None
    LSTM = None
    Dense = None
    Dropout = None
    Reshape = None
    Input = None
    Flatten = None
    print(f"⚠️ [VISUALIZATION WARN] TensorFlow/Keras import unavailable ({imp_err}). Standalone Pareto scatter mode active.")
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


# ##############################################################################
# Function Name : _split_features
# Purpose       : Classifies raw DataFrame feature columns. 
#                 🛡️ CRITICAL FIX: Explicitly filters out all non-numeric columns 
#                 (like 'timestamp', 'date') using `select_dtypes` to prevent 
#                 downstream Timestamp-to-float conversion crashes.
# ##############################################################################
def _split_features(df: pd.DataFrame):
    import numpy as np
    temporal_patterns = ['day_wk_sin', 'day_wk_cos', 'day_yr_sin', 'day_yr_cos', 'hour_sin', 'hour_cos', 'min_sin', 'min_cos', 'fourier_']
    
    # 🛡️ Forcefully exclude non-numeric columns
    numeric_cols = df.select_dtypes(include=[np.number]).columns
    
    time_cols = [c for c in numeric_cols if any(p in c for p in temporal_patterns)]
    base_asset_cols = [c for c in numeric_cols if c not in time_cols and not c.startswith('close_')]
    
    USER_EXCLUDE_FEATURES = ['volume_log_change_fed']
    banned_lower = [banned.lower() for banned in USER_EXCLUDE_FEATURES]
    asset_cols = [c for c in base_asset_cols if c.lower() not in banned_lower]

    return time_cols, asset_cols


# ##############################################################################
# Function Name : _prepare_lstm_tensors
# Purpose       : Generates 3D sliding-window training tensors (X, y). 
#                 Safely casts the pre-filtered numeric matrices to float32 
#                 for compatibility with the Native Rust LSTM Engine.
# ##############################################################################
def _prepare_lstm_tensors(chromosome: dict, df: pd.DataFrame):
    import numpy as np
    
    time_cols, asset_cols = _split_features(df)
    mask = np.array(chromosome.get('feature_mask', []))

    # 🟢 ROOT FIX: Universally apply the strict mask, never fallback to np.ones()
    mask = _get_strict_mask(chromosome.get('feature_mask', []), len(asset_cols))

    # 🛡️ Extract only valid columns and force float32 cast
    asset_values = df[asset_cols].values[:, mask == 1].astype(np.float32)
    time_values = df[time_cols].values.astype(np.float32) if time_cols else np.zeros((len(df), 0), dtype=np.float32)
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

    return np.array([], dtype=np.float32), np.array([], dtype=np.float32), mask

def _build_and_train_full_model(chromosome: dict, X_train: np.ndarray, y_train: np.ndarray):
    global tf
    if tf is None:
        try:
            import tensorflow as tf
        except ImportError:
            raise ModuleNotFoundError("TensorFlow is required for model building but is not installed in this environment.")
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
# ##############################################################################
# Function Name : _check_available_memory
# Purpose       : Acts as an Out-Of-Memory (OOM) guard. Inspects system RAM using 
#                 psutil or Linux /proc/meminfo to ensure we have enough memory 
#                 to load Keras models and render high-DPI Matplotlib charts.
# ##############################################################################
def _check_available_memory(threshold_gb=3.0):
    try:
        import psutil
        mem = psutil.virtual_memory()
        avail_gb = mem.available / (1024 ** 3)
        return avail_gb, avail_gb >= threshold_gb
    except ImportError:
        # Fallback to pure Linux meminfo if psutil is not installed
        try:
            with open('/proc/meminfo', 'r') as f:
                lines = f.readlines()
            mem_avail_line = [l for l in lines if 'MemAvailable' in l]
            if mem_avail_line:
                avail_kb = int(mem_avail_line[0].split()[1])
                avail_gb = avail_kb / (1024 ** 2)
                return avail_gb, avail_gb >= threshold_gb
        except Exception:
            pass
    # If we absolutely cannot determine memory, fail-safe to True to avoid locking the cluster
    return 99.0, True
 
# ##############################################################################
# Function Name : _export_candidate_model
# Purpose       : Serializes fitted MinMaxScaler pickles and structured JSON metadata
#                 for top candidate chromosomes.
# Parameters    :
#   - chromosome : dict          Target candidate chromosome genes and metadata.
#   - scaler     : MinMaxScaler  Fitted feature scaler instance.
#   - master_df  : pd.DataFrame  Master training DataFrame structure.
#   - export_dir : str           Target export path for serialization artifacts.
#   - rank       : int           Pareto rank / candidate index position.
# ##############################################################################
def _export_candidate_model(chromosome: dict, scaler: MinMaxScaler, master_df: pd.DataFrame, export_dir: str, rank: int):
    import os
    import json
    import pickle

    model_id = chromosome.get('id', 'UNKNOWN')
    prefix = f"rank_{rank}_" if rank is not None else "final_"

    # 1. Save MinMaxScaler Pickle
    scaler_path = os.path.join(export_dir, f"{prefix}scaler.pkl")
    with open(scaler_path, 'wb') as f:
        pickle.dump(scaler, f)

    # 2. Extract selected asset features from mask
    _, asset_cols = _split_features(master_df)
    mask = chromosome.get('feature_mask', [])
    selected_features = [
        col for col, mask_val in zip(asset_cols, mask) if mask_val == 1
    ]

    # 3. Save Structured Metadata JSON
    metadata = {
        "chromosome_id": model_id,
        "rank": rank,
        "engine": "rust_lstm_engine",
        "lookback_window": chromosome.get('lookback_window', 60),
        "forecast_horizon": chromosome.get('forecast_horizon', 30),
        "batch_size": chromosome.get('batch_size', 32),
        "learning_rate": chromosome.get('learning_rate', 0.001),
        "dropout_rate": chromosome.get('dropout_rate', 0.2),
        "selected_features": selected_features,
        "perf_vector": chromosome.get('perf_vector', [])
    }

    metadata_path = os.path.join(export_dir, f"{prefix}metadata.json")
    with open(metadata_path, 'w') as f:
        json.dump(metadata, f, indent=4)

    print(f"      [EXPORT SUCCESS] Rank {rank} ({model_id}) serialized ➔ {export_dir}")
# ##############################################################################
# Function Name : _get_model_dirs
#
# Path          : apps/school/visualization_worker.py
# Author        : Chalearm Saelim & Gemini
#
# Purpose :
#    Constructs hierarchical target directory structures for candidate models:
#    - Base Gen Dir   : prediction_result/{run_id}/G{gen_idx}/
#    - Model Main Dir : prediction_result/{run_id}/G{gen_idx}/M{x}/
#    - Price Plot Dir : prediction_result/{run_id}/G{gen_idx}/M{x}/prc/
#    - Volume Plot Dir: prediction_result/{run_id}/G{gen_idx}/M{x}/vol/
#
# Inputs :
#    run_id   : str  Active Run Identifier (e.g. '2885A21C')
#    gen_idx  : int  Generation index (e.g. 1)
#    model_id : str  Model string (e.g. 'G1-M3' -> short model 'M3')
#
# Return :
#    tuple : (gen_dir, model_dir, prc_dir, vol_dir)
# ##############################################################################
def _get_model_dirs(run_id: str, gen_idx: int, model_id: str = None) -> tuple:
    app_root = os.path.dirname(os.path.abspath(__file__))
    
    # 1. Base Generation Directory: prediction_result/{run_id}/G{gen_idx}/
    gen_dir = os.path.join(app_root, "prediction_result", run_id, f"G{gen_idx}")
    os.makedirs(gen_dir, exist_ok=True)
    
    if not model_id:
        return gen_dir, None, None, None

    # 2. Extract Short Model Name (e.g., 'G1-M3' -> 'M3')
    short_model = model_id.split("-")[-1] if "-" in model_id else model_id
    
    # 3. Model Subdirectories: prediction_result/{run_id}/G{gen_idx}/M{x}/prc & vol
    model_dir = os.path.join(gen_dir, short_model)
    prc_dir = os.path.join(model_dir, "prc")
    vol_dir = os.path.join(model_dir, "vol")

    os.makedirs(prc_dir, exist_ok=True)
    os.makedirs(vol_dir, exist_ok=True)

    print("📂 [DIRECTORY RESOLVED]")
    print(f"   ├── Generation Directory : {gen_dir}")
    print(f"   ├── Model Directory      : {model_dir}")
    print(f"   ├── Price Plot Dir (prc) : {prc_dir}")
    print(f"   └── Volume Plot Dir (vol): {vol_dir}")

    return gen_dir, model_dir, prc_dir, vol_dir
# ##############################################################################
# Function Name : _render_standalone_pareto_plot
#
# Path          : apps/school/visualization_worker.py
# Author        : Chalearm Saelim & Gemini
#
# Purpose :
#    Renders a standalone Pareto Efficiency Frontier scatter plot using Matplotlib
#    and saves it inside the generation directory:
#    `prediction_result/{run_id}/G{gen_idx}/pareto_frontier.png`
#
# Inputs :
#    top_chromosomes : list[dict] Evaluated candidate model chromosomes.
#    run_id          : str        Active Run ID (e.g. '2885A21C').
#    gen_idx         : int        Generation index (e.g. 1).
# ##############################################################################
def _render_standalone_pareto_plot(top_chromosomes: list, run_id: str, gen_idx: int):
    import matplotlib
    matplotlib.use('Agg')
    import matplotlib.pyplot as plt

    print("\n" + "📈" * 40)
    print(f"📈 [VISUALIZATION ENGINE] Rendering Pareto Efficiency Frontier Chart...")
    print(f"   ├── Target Run ID    : {run_id}")
    print(f"   ├── Generation Index : {gen_idx}")
    print(f"   └── Candidates Count : {len(top_chromosomes)}")
    print("📈" * 40)

    if not top_chromosomes:
        print("⚠️ [PLOT WARN] No evaluated candidate models provided for Pareto scatter rendering.")
        return

    # Route directory directly to prediction_result/{run_id}/G{gen_idx}/
    gen_dir, _, _, _ = _get_model_dirs(run_id=run_id, gen_idx=gen_idx)

    losses = []
    skills = []
    labels = []

    for i, c in enumerate(top_chromosomes):
        perf = c.get("perf_vector", [])
        rmse = perf[3] if len(perf) > 3 else 0.05
        skill_da = (perf[0] * 100) if len(perf) > 0 else 0.0
        c_id = c.get("id", f"M{i}")

        losses.append(rmse)
        skills.append(skill_da)
        labels.append(c_id)

    try:
        plt.figure(figsize=(10, 6))
        
        plt.scatter(losses, skills, color="#1f77b4", s=130, edgecolors="k", alpha=0.85, zorder=3)
        for i, txt in enumerate(labels):
            plt.annotate(
                txt, 
                (losses[i], skills[i]), 
                fontsize=9, 
                xytext=(6, 3), 
                textcoords="offset points", 
                fontweight="bold"
            )

        plt.title(f"GA-LSTM Pareto Efficiency Frontier - Run {run_id} (Gen {gen_idx})", fontsize=12, fontweight="bold")
        plt.xlabel("Validation Loss (RMSE)", fontsize=10)
        plt.ylabel("Skill Directional Accuracy (%)", fontsize=10)
        plt.grid(True, linestyle="--", alpha=0.5, zorder=0)
        plt.tight_layout()

        out_path = os.path.join(gen_dir, "pareto_frontier.png")
        plt.savefig(out_path, dpi=300)
        plt.close()
        
        print("\n" + "✅" * 40)
        print(f"✅ [PLOT SUCCESS] Written Pareto Efficiency Frontier ➔ {out_path}")
        print("✅" * 40 + "\n")

    except Exception as render_err:
        print(f"❌ [PLOT ERROR] Failed rendering Matplotlib Pareto chart: {render_err}")
# ##############################################################################
# Function Name : _render_standalone_pareto_plot
#
# Path          : apps/school/visualization_worker.py
# Author        : Chalearm Saelim
#
# Purpose :
#    Renders a standalone Pareto Efficiency Frontier scatter plot using Matplotlib
#    and saves it inside the generation-specific directory:
#    `prediction_result/{run_id}/G{gen_idx}/pareto_frontier.png`
#
# Inputs :
#    top_chromosomes : list[dict] List of evaluated chromosome dictionaries.
#    run_id          : str        Active Run ID (e.g., '2885A21C').
#    gen_idx         : int        Generation index (e.g., 1).
# ##############################################################################
def _render_standalone_pareto_plot(top_chromosomes: list, run_id: str, gen_idx: int):
    import matplotlib
    matplotlib.use('Agg')
    import matplotlib.pyplot as plt
    from utils import resolve_target_directories

    print("\n" + "📈" * 40)
    print(f"📈 [VISUALIZATION ENGINE] Rendering Pareto Efficiency Frontier Chart...")
    print(f"   ├── Target Run ID    : {run_id}")
    print(f"   ├── Generation Index : {gen_idx}")
    print(f"   └── Candidates Count : {len(top_chromosomes)}")
    print("📈" * 40)

    if not top_chromosomes:
        print("⚠️ [PLOT WARN] No evaluated candidate models provided for Pareto scatter rendering.")
        return

    # 1. Resolve Target Generation Directory: prediction_result/{run_id}/G{gen_idx}/
    _, _, target_plot_dir = resolve_target_directories(run_id=run_id, gen_idx=gen_idx)

    # 2. Extract Objective Metrics
    losses = []
    skills = []
    labels = []

    for i, c in enumerate(top_chromosomes):
        perf = c.get("perf_vector", [])
        rmse = perf[3] if len(perf) > 3 else 0.05
        skill_da = (perf[0] * 100) if len(perf) > 0 else 0.0
        c_id = c.get("id", f"M{i}")

        losses.append(rmse)
        skills.append(skill_da)
        labels.append(c_id)

    # 3. Construct Plot
    try:
        plt.figure(figsize=(10, 6))
        
        plt.scatter(losses, skills, color="#1f77b4", s=130, edgecolors="k", alpha=0.85, zorder=3)
        for i, txt in enumerate(labels):
            plt.annotate(
                txt, 
                (losses[i], skills[i]), 
                fontsize=9, 
                xytext=(6, 3), 
                textcoords="offset points", 
                fontweight="bold"
            )

        plt.title(f"GA-LSTM Pareto Efficiency Frontier - Run {run_id} (Gen {gen_idx})", fontsize=12, fontweight="bold")
        plt.xlabel("Validation Loss (RMSE)", fontsize=10)
        plt.ylabel("Skill Directional Accuracy (%)", fontsize=10)
        plt.grid(True, linestyle="--", alpha=0.5, zorder=0)
        plt.tight_layout()

        # Save PNG to prediction_result/{run_id}/G{gen_idx}/pareto_frontier.png
        out_path = os.path.join(target_plot_dir, "pareto_frontier.png")
        plt.savefig(out_path, dpi=300)
        plt.close()
        
        print("\n" + "✅" * 40)
        print(f"✅ [PLOT SUCCESS] Written Pareto Efficiency Frontier ➔ {out_path}")
        print("✅" * 40 + "\n")

    except Exception as render_err:
        print(f"❌ [PLOT ERROR] Failed rendering Matplotlib Pareto chart: {render_err}")
def _extract_real_validation_prices(val_data_raw, target_asset_column):
    """
    Extracts the unscaled actual price sequence for the validation window 
    so the green line reflects real market movements.
    """
    if val_data_raw is not None and target_asset_column in val_data_raw.columns:
        return val_data_raw[target_asset_column].values
    return None

# ##############################################################################
# Function Name : _extract_asset_name & _get_raw_actual_series
# Purpose       : Safely rips prefixes/suffixes to find the core asset name, 
#                 then extracts the exact absolute price/volume column from the DF.
# ##############################################################################
def _extract_asset_name(target_col: str) -> str:
    import re
    clean = target_col.lower()
    clean = re.sub(r'^(price_log_return_|volume_log_change_|price_|close_|volume_raw_|volume_)', '', clean)
    clean = re.sub(r'_(log_return|price|close|usd)$', '', clean)
    return clean.strip()

def _reconstruct_series(returns: np.ndarray, initial_val: float) -> np.ndarray:
    """Reconstructs actual price/volume series from log returns/changes"""
    import numpy as np
    return initial_val * np.exp(np.cumsum(np.nan_to_num(returns, nan=0.0)))
 
# ##############################################################################
# Function Name : _get_raw_actual_series
# Purpose       : Extracts the raw price/volume array. If the raw column is 
#                 missing from the DataFrame, it automatically builds a synthetic 
#                 price line starting at 100.0 using the log returns to guarantee 
#                 the graph renders.
# ##############################################################################
def _get_raw_actual_series(df, asset_name, is_price):
    import numpy as np
    if df is None or df.empty:
        return None
        
    cols_lower = {c.lower(): c for c in df.columns}
    expected_col = f"close_{asset_name}" if is_price else f"volume_raw_{asset_name}"
    target_col = None
    
    if expected_col in cols_lower:
        target_col = cols_lower[expected_col]
    else:
        for c_low, c_real in cols_lower.items():
            if asset_name in c_low:
                if is_price and ('close' in c_low or 'price' in c_low) and 'return' not in c_low and 'change' not in c_low:
                    target_col = c_real
                    break
                if not is_price and ('volume' in c_low or 'vol' in c_low) and 'change' not in c_low and 'return' not in c_low:
                    target_col = c_real
                    break

    # 🎯 TARGETED ASSET DEBUG TELEMETRY & FALLBACK RECONSTRUCTION
    if target_col:
        series = df[target_col].values
        print(f"      │    ├── 🎯 [ASSET DEBUG] SUCCESS: Found {'Price' if is_price else 'Volume'} col '{target_col}' for '{asset_name.upper()}'. Last value: {series[-1]:.4f}")
        return series
    else:
        available_matches = [c for c in df.columns if asset_name in c.lower()]
        print(f"      │    ├── ❌ [ASSET DEBUG] FAILED: Missing {'Price' if is_price else 'Volume'} for '{asset_name.upper()}'. Available matches in DF: {available_matches}")
        
        # 🛡️ SYNTHETIC FALLBACK RECONSTRUCTION
        # If the raw data is entirely missing from the payload, rebuild it mathematically from log returns
        log_col = None
        for c in available_matches:
            if is_price and 'return' in c.lower(): log_col = c
            elif not is_price and 'change' in c.lower(): log_col = c
        
        if log_col:
            print(f"      │    ├── ⚠️ [ASSET DEBUG] SYNTHETIC FALLBACK: Reconstructing '{asset_name.upper()}' series starting at 100.0 from '{log_col}'")
            return 100.0 * np.exp(np.cumsum(np.nan_to_num(df[log_col].values, nan=0.0)))
            
    return None
# ##############################################################################
# Function Name : _get_strict_mask
# Purpose       : Ensures the feature mask always matches the dataframe column 
#                 count exactly, preventing index misalignment during inference.
# ##############################################################################
def _get_strict_mask(raw_mask: list, expected_len: int) -> np.ndarray:
    import numpy as np
    clean = np.zeros(expected_len, dtype=int)
    for i, val in enumerate(raw_mask):
        if i < expected_len:
            clean[i] = int(val)
    return clean
# ##############################################################################
# Function Name : _unscale_feature_values
# Purpose       : Correctly reverses MinMaxScaler (-1, 1) normalization using 
#                 exact feature index lookup without squashing non-log features.
# Inputs        : scaled_values - Normalized NumPy prediction array
#                 feat_name     - Target feature column name
#                 scaler        - Fitted MinMaxScaler instance
#                 df            - Reference DataFrame
# Return        : Unscaled float NumPy array in original feature units
# ##############################################################################
def _unscale_feature_values(scaled_values: np.ndarray, feat_name: str, scaler, df: pd.DataFrame) -> np.ndarray:
    try:
        if hasattr(scaler, 'feature_names_in_') and feat_name in scaler.feature_names_in_:
            idx = list(scaler.feature_names_in_).index(feat_name)
            scale = scaler.scale_[idx]
            min_val = scaler.min_[idx]
            # Exact MinMaxScaler inverse transform: x = (x_scaled - min) / scale
            unscaled = (scaled_values - min_val) / scale
            return np.nan_to_num(unscaled, nan=0.0)
        elif feat_name in df.columns:
            f_min = df[feat_name].min()
            f_max = df[feat_name].max()
            if f_max > f_min:
                unscaled = (scaled_values + 1.0) / 2.0 * (f_max - f_min) + f_min
                return np.nan_to_num(unscaled, nan=f_min)
    except Exception as e:
        print(f"      ⚠️ [UNSCALE WARN] Failed to unscale '{feat_name}': {e}")
    return scaled_values


# ##############################################################################
# Function Name : _get_inference_window
# Purpose       : Extracts 100% real historical lookback tensors from full dataset,
#                 pulling true preceding historical rows from the timeline to 
#                 eliminate zero-padding flat lines on historical predictions.
# Inputs        : df_scaled        - Feature-scaled DataFrame
#                 absolute_end_idx - Target row index in the master timeline
#                 lookback         - Required historical window length
#                 mask             - Chromosome feature mask selection array
#                 time_cols        - Extracted temporal feature names
#                 asset_cols       - Extracted base asset feature names
# Return        : 3D float32 NumPy tensor of shape (1, lookback, selected_features)
# ##############################################################################
def _get_inference_window(df_scaled: pd.DataFrame, absolute_end_idx: int, lookback: int, mask: np.ndarray, time_cols: list, asset_cols: list) -> np.ndarray:
    absolute_start_idx = absolute_end_idx - lookback

    if absolute_start_idx < 0:
        real_start = 0
        pad_len = abs(absolute_start_idx)
        a_vals = df_scaled[asset_cols].values[real_start:absolute_end_idx, mask == 1].astype(np.float32)
        t_vals = df_scaled[time_cols].values[real_start:absolute_end_idx].astype(np.float32) if time_cols else np.zeros((absolute_end_idx, 0), dtype=np.float32)
        win = np.hstack([a_vals, t_vals]) if time_cols else a_vals
        pad = np.zeros((pad_len, win.shape[1]), dtype=np.float32)
        win = np.vstack([pad, win])
    else:
        a_vals = df_scaled[asset_cols].values[absolute_start_idx:absolute_end_idx, mask == 1].astype(np.float32)
        t_vals = df_scaled[time_cols].values[absolute_start_idx:absolute_end_idx].astype(np.float32) if time_cols else np.zeros((lookback, 0), dtype=np.float32)
        win = np.hstack([a_vals, t_vals]) if time_cols else a_vals

    return np.expand_dims(np.nan_to_num(win, nan=0.0).astype(np.float32), axis=0)

# ##############################################################################
# Function Name : _audit_per_asset_prediction_variance
# Purpose       : Executive-style tabular diagnostic audit. Computes prediction 
#                 mean and variance across 100 historical sample windows to flag 
#                 Mean Predictor Collapse before plot rendering.
# ##############################################################################
def _audit_per_asset_prediction_variance(asset_names: list, y_true: np.ndarray, y_pred: np.ndarray, model_id: str = "UNKNOWN"):
    print("\n" + "🔍" * 40)
    print(f"🔍 [ASSET PREDICTION DIAGNOSTIC AUDIT - MODEL: {model_id}]")
    print("─" * 85)
    print(f"{'ASSET NAME':<28} | {'TRUE MEAN':<10} | {'PRED MEAN':<10} | {'PRED STD (VARIANCE)':<20} | {'STATUS'}")
    print("─" * 85)

    pred_2d = y_pred.reshape(-1, y_pred.shape[-1])
    true_2d = y_true.reshape(-1, y_true.shape[-1])

    num_assets = min(len(asset_names), pred_2d.shape[-1])
    min_true_len = min(len(true_2d), len(pred_2d))

    flat_count = 0
    dynamic_count = 0

    for col_idx in range(num_assets):
        asset_name = asset_names[col_idx]
        t_mean = float(np.mean(true_2d[-min_true_len:, col_idx]))
        p_mean = float(np.mean(pred_2d[-min_true_len:, col_idx]))
        p_std  = float(np.std(pred_2d[-min_true_len:, col_idx]))

        if p_std < 1e-4:
            status = "🔴 FLAT LINE (Mean Predictor Collapse)"
            flat_count += 1
        elif p_std < 1e-2:
            status = "🟡 LOW VARIANCE (Weak Signal)"
            dynamic_count += 1
        else:
            status = "🟢 DYNAMIC (Active Oscillations)"
            dynamic_count += 1

        print(f"{asset_name:<28} | {t_mean:<+10.4f} | {p_mean:<+10.4f} | {p_std:<20.6f} | {status}")

    print("─" * 85)
    print(f"📊 [AUDIT SUMMARY] Active Features: {num_assets} | Dynamic: {dynamic_count} | Collapsed: {flat_count}")
    print("🔍" * 40 + "\n")

 # ##############################################################################
# Function Name : _format_xaxis_dates
# Purpose       : Formats X-axis date milestones with staggered vertical offsets 
#                 (-0.12, -0.22) to prevent text collisions. Dynamically pads 
#                 future dates using Timedelta to guarantee calendar ISO formats.
# ##############################################################################
def _format_xaxis_dates(ax, dates_list: list, n_hist: int, n_horizon: int, val_len: int = 0):
    n_total = n_hist + n_horizon
    
    def safe_date(idx, fallback_text):
        if not dates_list or len(dates_list) <= idx:
            return fallback_text
        try:
            val = dates_list[idx]
            return str(val).split()[0] if pd.notna(val) else fallback_text
        except (IndexError, TypeError):
            return fallback_text
            
    d_start = safe_date(0, "Start")
    d_end_hist = safe_date(n_hist - 1, "Current")
    d_horiz = safe_date(n_total - 1, f"End (+{n_horizon}d)")

    train_end_idx = n_hist - 1 - val_len if val_len > 0 else -1

    tick_indices = [0, n_hist - 1, n_total - 1]
    if train_end_idx > 0 and train_end_idx not in tick_indices:
        tick_indices.append(train_end_idx)

    tick_indices = sorted(list(set(tick_indices)))
    ax.set_xticks(tick_indices)
    ax.set_xticklabels([])

    # Staggered vertical offsets prevent text collisions
    ax.text(0, -0.12, f"0\n(Start: {d_start})", transform=ax.get_xaxis_transform(), ha='center', va='top', fontsize=8, clip_on=False)
    
    if train_end_idx > 0:
        d_train_end = safe_date(train_end_idx, "End Train")
        ax.text(train_end_idx, -0.22, f"{train_end_idx}\n(End Train: {d_train_end})", transform=ax.get_xaxis_transform(), ha='center', va='top', fontsize=8, clip_on=False, color='green', fontweight='bold')
        ax.axvline(x=train_end_idx, color='green', linestyle=':', alpha=0.8)

    ax.text(n_hist - 1, -0.12, f"{n_hist - 1}\n({d_end_hist})", transform=ax.get_xaxis_transform(), ha='center', va='top', fontsize=8, clip_on=False)
    
    # Renders the exact extrapolated ISO Date
    ax.text(n_total - 1, -0.22, f"{n_total - 1}\n(+{n_horizon}d: {d_horiz})", transform=ax.get_xaxis_transform(), ha='center', va='top', fontsize=8, clip_on=False, color='darkorange', fontweight='bold')

# ##############################################################################
# Function Name : _plot_validation_overlay
# Purpose       : Plots Validation Overlay with a seamless historical fit line.
#                 Strictly masks features and skips absolute metrics.
# ##############################################################################
def _plot_validation_overlay(chromosome: dict, hist_preds_scaled: np.ndarray, fut_preds_scaled: np.ndarray, scaler, master_df: pd.DataFrame, val_df: pd.DataFrame, prc_dir: str, vol_dir: str):
    try:
        c_id = chromosome.get('id', 'UNKNOWN_MODEL')
        horizon = int(chromosome.get('forecast_horizon', 30))
        lookback = int(chromosome.get('lookback_window', 60))
        
        logger.info(f"      │   ├── 🎨 [RENDER 1/2] Generating Validation Overlays...")

        time_candidates = [c for c in master_df.columns if c.lower() in ['timestamp', 'date', 'time', 'datetime']]
        t_col = time_candidates[0] if time_candidates else None
        
        n_hist = min(100, len(master_df))
        plot_dates = []

        if t_col:
            dates_master = pd.to_datetime(master_df[t_col]).dt.strftime('%Y-%m-%d').tolist()
            plot_dates = dates_master[-n_hist:]
            if val_df is not None and t_col in val_df.columns and not val_df.empty:
                val_dates = pd.to_datetime(val_df[t_col]).dt.strftime('%Y-%m-%d').tolist()
                plot_dates.extend(val_dates[:horizon])
        
        while len(plot_dates) < n_hist + horizon:
            last_dt = pd.to_datetime(plot_dates[-1]) if plot_dates else pd.Timestamp('2026-01-01')
            plot_dates.append((last_dt + pd.Timedelta(days=1)).strftime('%Y-%m-%d'))

        _, asset_cols = _split_features(master_df)
        mask = chromosome.get('feature_mask', [])
        
        # 🟢 STRICT MASKING
        clean_mask = np.zeros(len(asset_cols), dtype=int)
        for i, val in enumerate(mask):
            if i < len(asset_cols): clean_mask[i] = int(val)
                
        selected_features = [asset_cols[i] for i, val in enumerate(clean_mask) if val == 1]
        logger.info(f"      │   │   ├── 🎯 Strict Masking: Plotting {len(selected_features)} selected features.")

        val_len = len(val_df) if val_df is not None else 0
        lstm_label = f"Rust LSTM ({lookback}d look back, {horizon}d horizon)"
        rendered_count = 0

        for f_idx, feat_name in enumerate(selected_features):
            if feat_name not in master_df.columns or f_idx >= fut_preds_scaled.shape[1]:
                continue

            is_volume = 'volume' in feat_name.lower() or 'change' in feat_name.lower()
            is_price = not is_volume and ('price' in feat_name.lower() or 'close' in feat_name.lower() or 'return' in feat_name.lower())
            is_log_target = 'return' in feat_name.lower() or 'change' in feat_name.lower()
            
            target_dir = prc_dir if is_price else vol_dir
            asset_name = _extract_asset_name(feat_name)
            
            history_plot = np.nan_to_num(master_df[feat_name].values[-n_hist:], nan=0.0)
            hist_pred_unscaled = _unscale_feature_values(hist_preds_scaled[:, f_idx], feat_name, scaler, master_df)
            fut_pred_unscaled = _unscale_feature_values(fut_preds_scaled[:, f_idx], feat_name, scaler, master_df)
            
            actual_val_plot = None
            if val_df is not None and not val_df.empty and feat_name in val_df.columns:
                actual_val_plot = np.nan_to_num(val_df[feat_name].values[:horizon], nan=0.0)

            # --- PLOT 1: STANDARD TARGETS ---
            fig, ax = plt.subplots(figsize=(10, 4.5), dpi=100)
            fig.subplots_adjust(bottom=0.25) 
            
            ax.plot(np.arange(n_hist), history_plot, label='Known Market Data', color='blue', linewidth=1.5)

            x_hist_fit = np.arange(n_hist)
            y_hist_fit = hist_pred_unscaled[:n_hist]
            ax.plot(x_hist_fit, y_hist_fit, label='Rust LSTM (Historical Fit)', color='magenta', linestyle='--', linewidth=1.5)

            if actual_val_plot is not None:
                connected_actual = np.insert(actual_val_plot, 0, history_plot[-1] if len(history_plot) > 0 else 0.0)
                ax.plot(np.arange(n_hist - 1, n_hist - 1 + len(connected_actual)), connected_actual, label='Actual Validation Data', color='green', linewidth=1.8)

            x_val_pred = np.arange(n_hist - 1, n_hist + horizon)
            y_val_pred = np.insert(fut_pred_unscaled, 0, y_hist_fit[-1])
            ax.plot(x_val_pred, y_val_pred, label=lstm_label, color='red', linestyle='--', linewidth=1.5)

            ax.axvline(x=n_hist - 1, color='gold', linestyle=':')
            ax.axvspan(n_hist - 1, n_hist - 1 + horizon, color='yellow', alpha=0.1)

            ax.set_title(f"Validation Overlay: {feat_name.upper()}", fontweight='bold')
            ax.set_ylabel("Target Value (Log Return)" if is_log_target else "Raw Target Value")
            _format_xaxis_dates(ax, plot_dates, n_hist, horizon, val_len=val_len)
            ax.legend(loc='upper left')
            ax.grid(True, alpha=0.3)
            plt.savefig(os.path.join(target_dir, f"val_overlay_{feat_name}.png"), bbox_inches="tight")
            fig.clf()
            plt.close('all')
            rendered_count += 1

            # --- PLOT 2: RECONSTRUCTED ACTUAL PRICE / VOLUME ---
            if not is_log_target:
                gc.collect()
                continue

            raw_hist_series = _get_raw_actual_series(master_df, asset_name, is_price)
            if raw_hist_series is not None:
                raw_hist_actual = np.nan_to_num(raw_hist_series[-n_hist:], nan=0.0)
                raw_val_series = _get_raw_actual_series(val_df, asset_name, is_price)
                
                p_prev = np.clip(raw_hist_actual[:-1], -1e9, 1e9)
                recon_hist_pred = p_prev * np.exp(np.clip(y_hist_fit[1:], -5.0, 5.0))
                recon_hist_pred = np.insert(recon_hist_pred, 0, raw_hist_actual[0])
                
                accumulated_returns = np.clip(np.cumsum(np.nan_to_num(fut_pred_unscaled, nan=0.0)), -10.0, 10.0)
                recon_fut_pred = recon_hist_pred[-1] * np.exp(accumulated_returns)
                
                fig2, ax2 = plt.subplots(figsize=(10, 4.5), dpi=100)
                fig2.subplots_adjust(bottom=0.25)
                
                ax2.plot(np.arange(n_hist), raw_hist_actual, label='Known Market Data (Reconstructed)', color='blue', linewidth=1.5)
                ax2.plot(x_hist_fit, recon_hist_pred, color='magenta', linestyle='--', label='Rust LSTM (Historical Fit)')

                if raw_val_series is not None:
                    raw_val_actual = np.nan_to_num(raw_val_series[:horizon], nan=0.0)
                    connected_raw_val = np.insert(raw_val_actual, 0, raw_hist_actual[-1])
                    ax2.plot(np.arange(n_hist - 1, n_hist - 1 + len(connected_raw_val)), connected_raw_val, label='Actual Validation Data', color='green', linewidth=1.8)

                recon_fut_line = np.insert(recon_fut_pred, 0, recon_hist_pred[-1])
                ax2.plot(x_val_pred, recon_fut_line, label=lstm_label, color='red', linestyle='--', linewidth=1.5)

                ax2.axvline(x=n_hist - 1, color='gold', linestyle=':')
                ax2.axvspan(n_hist - 1, n_hist - 1 + horizon, color='yellow', alpha=0.1)

                ax2.set_title(f"Reconstructed Validation Overlay: {asset_name.upper()}", fontweight='bold')
                ax2.set_ylabel("Actual Price (USD)" if is_price else "Raw Volume")
                _format_xaxis_dates(ax2, plot_dates, n_hist, horizon, val_len=val_len)
                ax2.legend(loc='upper left')
                ax2.grid(True, alpha=0.3)
                plt.savefig(os.path.join(target_dir, f"val_overlay_reconstructed_{feat_name}.png"), bbox_inches="tight")
                fig2.clf()
                plt.close('all')
                rendered_count += 1
            gc.collect()

        logger.info(f"      │   │   └── ✅ Exported {rendered_count} Validation overlay plots.")

    except Exception as e:
        logger.error(f"❌ [VIS-STATE ERROR] Validation Overlay Crashed: {e}")


# ##############################################################################
# Function Name : _plot_future_projection
# Purpose       : Renders True Future Forecasts using `combined_df`. 
#                 BUG FIXED: Now correctly extracts features from `combined_df`.
# ##############################################################################
def _plot_future_projection(chromosome: dict, hist_preds_scaled: np.ndarray, fut_preds_scaled: np.ndarray, scaler, combined_df: pd.DataFrame, prc_dir: str, vol_dir: str, val_len: int = 0):
    try:
        c_id = chromosome.get('id', 'UNKNOWN_MODEL')
        horizon = int(chromosome.get('forecast_horizon', 30))
        lookback = int(chromosome.get('lookback_window', 60))

        logger.info(f"      │   ├── 🔮 [RENDER 2/2] Generating True Future Projections...")

        time_candidates = [c for c in combined_df.columns if c.lower() in ['timestamp', 'date', 'time', 'datetime']]
        t_col = time_candidates[0] if time_candidates else None
        
        n_hist = min(100, len(combined_df))
        plot_dates = []

        if t_col:
            dates = pd.to_datetime(combined_df[t_col]).dt.strftime('%Y-%m-%d').tolist()
            plot_dates = dates[-n_hist:]
        
        while len(plot_dates) < n_hist + horizon:
            last_dt = pd.to_datetime(plot_dates[-1]) if plot_dates else pd.Timestamp('2026-01-01')
            plot_dates.append((last_dt + pd.Timedelta(days=1)).strftime('%Y-%m-%d'))

        future_end_date = plot_dates[-1]

        # 🟢 BUG FIX: Use combined_df, NOT master_df
        _, asset_cols = _split_features(combined_df) 
        mask = chromosome.get('feature_mask', [])
        
        clean_mask = np.zeros(len(asset_cols), dtype=int)
        for i, val in enumerate(mask):
            if i < len(asset_cols): clean_mask[i] = int(val)
                
        selected_features = [asset_cols[i] for i, val in enumerate(clean_mask) if val == 1]
        
        lstm_label = f"Rust LSTM ({lookback}d look back, {horizon}d horizon {future_end_date})"
        rendered_count = 0

        for f_idx, feat_name in enumerate(selected_features):
            if feat_name not in combined_df.columns or f_idx >= fut_preds_scaled.shape[1]:
                continue

            is_volume = 'volume' in feat_name.lower() or 'change' in feat_name.lower()
            is_price = not is_volume and ('price' in feat_name.lower() or 'close' in feat_name.lower() or 'return' in feat_name.lower())
            is_log_target = 'return' in feat_name.lower() or 'change' in feat_name.lower()
            
            target_dir = prc_dir if is_price else vol_dir
            asset_name = _extract_asset_name(feat_name)
            
            history_plot = np.nan_to_num(combined_df[feat_name].values[-n_hist:], nan=0.0)
            hist_pred_unscaled = _unscale_feature_values(hist_preds_scaled[:, f_idx], feat_name, scaler, combined_df)
            fut_pred_unscaled = _unscale_feature_values(fut_preds_scaled[:, f_idx], feat_name, scaler, combined_df)

            # --- PLOT 1: STANDARD TARGETS ---
            fig, ax = plt.subplots(figsize=(10, 4.5), dpi=100)
            fig.subplots_adjust(bottom=0.25)
            
            ax.plot(np.arange(n_hist), history_plot, label='Known Market Data (Hist + Val)', color='blue', linewidth=1.5)

            x_hist_fit = np.arange(n_hist)
            ax.plot(x_hist_fit, hist_pred_unscaled[:n_hist], color='magenta', linestyle='--', label='Rust LSTM (Historical Fit)')
            
            x_fut_pred = np.arange(n_hist - 1, n_hist + horizon)
            fut_line = np.insert(fut_pred_unscaled, 0, hist_pred_unscaled[-1])
            fut_line = np.nan_to_num(fut_line, nan=0.0, posinf=0.0, neginf=0.0)
            
            ax.plot(x_fut_pred, fut_line, color='darkorange', linestyle='--', label=lstm_label)

            ax.axvline(x=n_hist - 1, color='magenta', linestyle=':')
            ax.axvspan(n_hist - 1, n_hist - 1 + horizon, color='purple', alpha=0.1)

            ax.set_title(f"True Future Projection: {feat_name.upper()}", fontweight='bold')
            ax.set_ylabel("Target Value (Log Return)" if is_log_target else "Raw Target Value")
            _format_xaxis_dates(ax, plot_dates, n_hist, horizon, val_len)
            ax.legend(loc='upper left')
            ax.grid(True, alpha=0.3)
            plt.savefig(os.path.join(target_dir, f"future_forecast_{feat_name}.png"), bbox_inches="tight")
            fig.clf()
            plt.close('all')
            rendered_count += 1

            # --- PLOT 2: RECONSTRUCTED ACTUAL PRICE / VOLUME ---
            if not is_log_target:
                gc.collect()
                continue 

            raw_hist_series = _get_raw_actual_series(combined_df, asset_name, is_price)
            if raw_hist_series is not None:
                raw_hist_actual = np.nan_to_num(raw_hist_series[-n_hist:], nan=0.0)
                
                p_prev = np.clip(raw_hist_actual[:-1], -1e9, 1e9)
                recon_hist_pred = p_prev * np.exp(np.clip(hist_pred_unscaled[1:n_hist], -5.0, 5.0))
                recon_hist_pred = np.insert(recon_hist_pred, 0, raw_hist_actual[0])
                
                accumulated_returns = np.clip(np.cumsum(np.nan_to_num(fut_pred_unscaled, nan=0.0)), -10.0, 10.0)
                recon_fut_pred = recon_hist_pred[-1] * np.exp(accumulated_returns)
                
                fig2, ax2 = plt.subplots(figsize=(10, 4.5), dpi=100)
                fig2.subplots_adjust(bottom=0.25)
                
                ax2.plot(np.arange(n_hist), raw_hist_actual, label='Known Market Data (Reconstructed)', color='blue', linewidth=1.5)
                ax2.plot(x_hist_fit, recon_hist_pred, color='magenta', linestyle='--', label='Rust LSTM (Historical Fit)')
                
                recon_fut_line = np.insert(recon_fut_pred, 0, recon_hist_pred[-1])
                recon_fut_line = np.nan_to_num(recon_fut_line, nan=recon_hist_pred[-1], posinf=recon_hist_pred[-1], neginf=recon_hist_pred[-1])
                
                ax2.plot(x_fut_pred, recon_fut_line, color='darkorange', linestyle='--', label=lstm_label)

                ax2.axvline(x=n_hist - 1, color='magenta', linestyle=':')
                ax2.axvspan(n_hist - 1, n_hist - 1 + horizon, color='purple', alpha=0.1)

                ax2.set_title(f"Reconstructed True Future: {asset_name.upper()}", fontweight='bold')
                ax2.set_ylabel("Actual Price (USD)" if is_price else "Raw Volume")
                _format_xaxis_dates(ax2, plot_dates, n_hist, horizon, val_len)
                ax2.legend(loc='upper left')
                ax2.grid(True, alpha=0.3)
                
                plt.savefig(os.path.join(target_dir, f"future_forecast_reconstructed_{feat_name}.png"), bbox_inches="tight")
                fig2.clf()
                plt.close('all')
                rendered_count += 1
            gc.collect()
            
        logger.info(f"      │   │   └── ✅ Exported {rendered_count} Future Projection plots.")

    except Exception as e:
        logger.error(f"❌ [VIS-STATE ERROR] Future Projection Crashed: {e}")
# ##############################################################################
# Function Name : generate_pareto_graphs_and_exports
# Purpose       : Single-pass whole-dataset visualization consumer engine.
#                 Maintains full [-1.0, 1.0] MinMaxScaler prediction bounds to 
#                 prevent prediction clamping and flat line artifacts.
# Inputs        : payload - Celery dictionary containing master_data, val_data, 
#                           top_chromosomes, run_id, and gen_idx.
# Return        : Execution status dictionary.
# ##############################################################################
def generate_pareto_graphs_and_exports(payload: dict, **kwargs) -> dict:
    import os, io, gc, json, traceback
    import numpy as np
    import pandas as pd
    from sklearn.preprocessing import MinMaxScaler
    import rust_lstm_engine

    try:
        print("\n" + "═" * 80)
        print("📊 [CONSUMER TELEMETRY 1.0] Ingesting Visualization Task Payload")
        print("═" * 80)

        if isinstance(payload, str):
            payload = json.loads(payload)
        if not isinstance(payload, dict):
            payload = kwargs.get("payload", {}) if isinstance(kwargs.get("payload"), dict) else kwargs

        run_id = payload.get("run_id", "DEFAULT_RUN")
        gen_idx = payload.get("gen_idx", 1)
        
        log_dir, export_dir, _ = resolve_target_directories(run_id)
        gen_dir, _, _, _ = _get_model_dirs(run_id, gen_idx)

        top_chromosomes = payload.get("top_chromosomes", [])

        # 1. Render Pareto Efficiency Frontier
        _render_standalone_pareto_plot(top_chromosomes, run_id, gen_idx)

        # 2. Unpack DataFrames
        master_raw = payload.get("master_data", None)
        if master_raw:
            try:
                master_df = pd.read_json(io.StringIO(master_raw), orient='split')
            except Exception:
                master_df = pd.read_json(io.StringIO(master_raw))
        else:
            return {"status": "error", "message": "Empty Master DataFrame"}

        val_raw = payload.get("val_data", None)
        if val_raw:
            try:
                val_df = pd.read_json(io.StringIO(val_raw), orient='split')
            except Exception:
                val_df = pd.read_json(io.StringIO(val_raw))
            val_length = len(val_df)
        else:
            val_df = None
            val_length = 0

        # 3. Combine into 100% Full Dataset Matrix
        unscaled_combined_df = pd.concat([master_df, val_df], ignore_index=True) if val_df is not None else master_df.copy()
        
        scaler = MinMaxScaler(feature_range=(-1, 1))
        numeric_cols = master_df.select_dtypes(include=[np.number]).columns
        scaler.fit(unscaled_combined_df[numeric_cols])

        scaled_combined_df = unscaled_combined_df.copy()
        scaled_combined_df[numeric_cols] = scaler.transform(scaled_combined_df[numeric_cols])

        print(f"   ├── 📂 Target Run Directory  : prediction_result/{run_id}/G{gen_idx}")
        print(f"   ├── 🕒 Master Train DF       : {len(master_df)} rows")
        print(f"   ├── 🕒 Validation DF         : {val_length} rows")
        print(f"   ├── 📈 Combined Timeline     : {len(unscaled_combined_df)} total rows")
        print(f"   └── 📐 MinMaxScaler Scope    : {len(numeric_cols)} numeric feature columns")
        print(f"   └── 🧬 Candidate Pareto Models: {len(top_chromosomes)} model(s) packed for export\n")

        processed_count = 0

        for rank_idx, chromosome in enumerate(top_chromosomes):
            rank = rank_idx + 1
            c_id = chromosome.get("id", f"G{gen_idx}-M{rank_idx}")
            gen_dir, model_dir, prc_dir, vol_dir = _get_model_dirs(run_id, gen_idx, c_id)
            
            print("─" * 80)
            print(f"🚀 [WHOLE-DATASET RENDER {rank}/{len(top_chromosomes)}] Model ID: {c_id}")
            print("─" * 80)

            try:
                lookback = int(chromosome.get('lookback_window', 60))
                horizon = int(chromosome.get('forecast_horizon', 30))
                
                time_cols, asset_cols = _split_features(unscaled_combined_df)
                # 🟢 APPLY STRICT MASK HERE TOO
                mask = _get_strict_mask(chromosome.get('feature_mask', []), len(asset_cols))
                
                # 1. Prepare Training Tensors on Full Dataset
                X_train_full, y_train_full, _ = _prepare_lstm_tensors(chromosome, scaled_combined_df)
                
                # 2. Construct Inference Windows (100 Historical Days + 1 Future Horizon)
                n_hist = min(100, len(unscaled_combined_df))
                total_rows = len(scaled_combined_df)
                
                infer_windows = [
                    _get_inference_window(scaled_combined_df, abs_i, lookback, mask, time_cols, asset_cols)
                    for abs_i in range(total_rows - n_hist + 1, total_rows + 1)
                ]
                X_infer_full = np.concatenate(infer_windows, axis=0)

                nodes_gene = chromosome.get('nodes_per_layer', [64])
                lstm_units = int(nodes_gene[0]) if isinstance(nodes_gene, list) and len(nodes_gene) > 0 else int(chromosome.get('lstm_units', 64))
                lr = float(chromosome.get('learning_rate', 0.001))
                epochs = int(chromosome.get('epochs', 25))
                batch_size = int(chromosome.get('batch_size', 32))

                print(f"   ├── 🏋️ Training Native Rust LSTM on Full Dataset ({len(unscaled_combined_df)} rows)")
                print(f"   ├── 📐 Tensor Specs : Train {X_train_full.shape} ➔ Infer Tensor {X_infer_full.shape}")
                print(f"   └── ⚙️ Hyperparams  : Units={lstm_units} | LR={lr} | Epochs={epochs} | Batch={batch_size}")

                full_preds = rust_lstm_engine.train_and_predict(
                    x_train_py=X_train_full.astype(np.float32),
                    y_train_py=y_train_full.astype(np.float32),
                    x_val_py=X_infer_full.astype(np.float32),
                    lstm_units=lstm_units,
                    learning_rate=lr,
                    epochs=epochs,
                    _batch_size=batch_size
                )
                
                num_selected = full_preds.shape[2]
                
                # 🟢 FIX: Retain full MinMaxScaler bounds [-1.0, 1.0] (no -0.15 clamping)
                hist_preds_scaled = np.clip(full_preds[:n_hist, 0, :], -1.0, 1.0)
                fut_preds_scaled = np.clip(full_preds[-1].reshape((horizon, num_selected)), -1.0, 1.0)

                selected_features = [asset_cols[i] for i, val in enumerate(mask) if val == 1 and i < len(asset_cols)]
                _audit_per_asset_prediction_variance(
                    asset_names=selected_features,
                    y_true=y_train_full,
                    y_pred=full_preds[:n_hist],
                    model_id=c_id
                )

                _plot_validation_overlay(chromosome, hist_preds_scaled, fut_preds_scaled, scaler, master_df, val_df, prc_dir, vol_dir)
                _plot_future_projection(chromosome, hist_preds_scaled, fut_preds_scaled, scaler, unscaled_combined_df, prc_dir, vol_dir, val_length)

                short_model = c_id.split("-")[-1] if "-" in c_id else c_id
                candidate_export_dir = os.path.join(export_dir, f"G{gen_idx}", short_model)
                os.makedirs(candidate_export_dir, exist_ok=True)
                _export_candidate_model(chromosome, scaler, unscaled_combined_df, candidate_export_dir, rank)

                processed_count += 1
                del X_train_full, y_train_full, X_infer_full, full_preds
                gc.collect()

            except Exception as cand_err:
                print(f"❌ [CONSUMER ERROR] Failed processing candidate {c_id}: {cand_err}")
                with open(os.path.join(prc_dir, "CRASH_REPORT_CANDIDATE.txt"), "w") as f:
                    f.write(traceback.format_exc())
                continue

        print(f"\n🎉 [CONSUMER COMPLETE] Finished Generation {gen_idx}. Successfully processed {processed_count} model(s).\n")
        return {"status": "success", "gen_idx": gen_idx, "run_id": run_id}

    except Exception as master_err:
        print(f"❌ [CONSUMER CRITICAL] Worker crashed: {master_err}")
        return {"status": "error", "message": str(master_err)}