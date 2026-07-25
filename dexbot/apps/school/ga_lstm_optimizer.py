#!/usr/bin/env python3
#/******************************************************************************
#* File Name        : ga_lstm_optimizer.py
#* File Path        : apps/school/ga_lstm_optimizer.py
#* Author           : Chalearm Saelim & Gemini
#* Description      : Parallel Multi-Process GA-LSTM Optimizer Engine.
#*                    Features continuous rolling execution, auto-recreation of 
#*                    OOM-Killed tasks, completely muted C++ logging, and 
#*                    zero-leak IPC queue management.
#******************************************************************************/

import os
import sys
import logging

os.makedirs("logs", exist_ok=True)

# Shared Log Formatters
log_formatter = logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')

# ------------------------------------------------------------------------------
# 1. SPECIALIZED LOGGER: FOLD LIFECYCLE (logs/folds_lifecycle.log ONLY)
# ------------------------------------------------------------------------------
f_fold_handler = logging.FileHandler("logs/folds_lifecycle.log")
f_fold_handler.setFormatter(log_formatter)

fold_logger = logging.getLogger("FoldLifecycle")
fold_logger.setLevel(logging.INFO)
fold_logger.propagate = False  # Prevent bubbling up to main STDOUT / lstm_engine.log
if not fold_logger.handlers:
    fold_logger.addHandler(f_fold_handler)

# ------------------------------------------------------------------------------
# 2. SPECIALIZED LOGGER: CHROMOSOME SUMMARY (logs/chromosome_summary.log ONLY)
# ------------------------------------------------------------------------------
f_summary_handler = logging.FileHandler("logs/chromosome_summary.log")
f_summary_handler.setFormatter(log_formatter)

summary_logger = logging.getLogger("ChromosomeSummary")
summary_logger.setLevel(logging.INFO)
summary_logger.propagate = False  # Prevent bubbling up to main STDOUT / lstm_engine.log
if not summary_logger.handlers:
    summary_logger.addHandler(f_summary_handler)

# ------------------------------------------------------------------------------
# 3. MAIN ENGINE LOGGER: OUTER LOOP ONLY (STDOUT -> logs/lstm_engine.log)
# ------------------------------------------------------------------------------
logger = logging.getLogger("GA-LSTM-Optimizer")
logger.setLevel(logging.INFO)
logger.propagate = False

if not logger.handlers:
    c_handler = logging.StreamHandler(sys.stdout)
    c_handler.setFormatter(log_formatter)
    logger.addHandler(c_handler)
# ==============================================================================
# 1. ABSOLUTE LOW-LEVEL C++ & STDERR SUPPRESSION (MUST RUN FIRST BEFORE TF)
# ==============================================================================

os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'      # Mute TF C++ Info/Warning logs
os.environ['CUDA_VISIBLE_DEVICES'] = '-1'      # Force CPU execution & skip CUDA driver checks
os.environ['GLOG_minloglevel'] = '3'          # Mute Google Logging
os.environ['ABSL_LOG_LEVEL'] = '3'            # Silence Abseil C++ warnings completely
os.environ['TF_ENABLE_ONEDNN_OPTS'] = '0'

# 🛡️ Zombie Process Sweeper
if "-action=terminate" in sys.argv:
    print("🧹 [PYTHON] Terminate signal received. Sweeping zombie workers...")
    os.system("pkill -9 -f ga_lstm_optimizer")
    sys.exit(0)

import json
import random
import subprocess
import datetime
import logging
import glob
import socket
import time
import signal
import argparse
import pickle
import multiprocessing as mp
import multiprocessing
import os
import numpy as np
import pandas as pd
from sklearn.preprocessing import MinMaxScaler
from sklearn.metrics import root_mean_squared_error

import warnings
import queue  # <-- Make sure queue is imported for the rolling worker loop
# Ignore multiprocessing resource tracker semaphore leaks on Ctrl+C
warnings.filterwarnings("ignore", category=UserWarning, module="multiprocessing.resource_tracker")

# Force Logger to print specifically to STDOUT (FD 1) so it never gets muted
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[logging.StreamHandler(sys.stdout)]
)
logger = logging.getLogger("GA-LSTM-Optimizer")

# ==============================================================================
# 2. GLOBAL HYPERPARAMETER BOUNDS & SYSTEM CONFIGURATION
# ==============================================================================
# Data Directories
RAW_DATA_DIR = "../../data_set/daily/2022_07_01_2026_06_30"
TRANSFORMED_DATA_DIR = "../../data_set/daily/2022_07_01_2026_06_30"
VAL_RAW_DATA_DIR = "../../data_set/daily/2026_07_01_2026_07_21"
VAL_TRANSFORMED_DATA_DIR = "../../data_set/daily/2026_07_01_2026_07_21"

POPULATION_SIZE = 32
GENERATIONS = 12
MUTATION_RATE = 0.3
TOP_N_EXPORTS = 5  # <--- NEW: Number of top-ranking models to export for external apps
# Chromosome Range Constraints for LSTM Neural Architecture
MIN_LOOKBACK_DAYS = 60
MAX_LOOKBACK_DAYS = 150
MIN_FORECAST_DAYS = 30
MAX_FORECAST_DAYS = 60
MIN_HIDDEN_LAYERS = 1
MAX_HIDDEN_LAYERS = 8
MIN_NODES_PER_LAYER = 32
MAX_NODES_PER_LAYER = 512

# Optimizer & Batch Hyperparameter Constraints
MIN_LR, MAX_LR = 0.0001, 0.01
MIN_DROPOUT, MAX_DROPOUT = 0.0, 0.6
BATCH_SIZE_CHOICES = [16, 32, 64, 128]

# Walk-Forward Validation & IPC Multiprocessing Settings
NUM_FOLDS = 7 
MAX_PARALLEL_FOLDS = 4          # Maximum concurrent fold execution processes
UDP_BASE_PORT = 40001           # Dynamic IPC Port allocation base (40001, 40002, ...)

# User-Defined Feature Exclusions
USER_EXCLUDE_FEATURES = ['volume_log_change_fed']


# ==============================================================================
# 3. DATA ANCHOR UTILITY CLASS
# ==============================================================================
class DataAnchor:
    """
    Handles mapping and reversing transformations between normalized log returns
    and raw closing prices for visualization and real-world evaluation.
    """
    def __init__(self, original_path, transformed_path):
        self.original = pd.read_csv(original_path, parse_dates=['timestamp'], index_col='timestamp')
        self.transformed = pd.read_csv(transformed_path, parse_dates=['timestamp'], index_col='timestamp')
        
    def log_return_to_price(self, log_returns, last_known_price):
        """Reverses log return array back to absolute price series."""
        prices = [last_known_price]
        for r in log_returns:
            prices.append(prices[-1] * np.exp(r))
        return np.array(prices[1:])

    def get_last_price(self, timestamp):
        """Finds the raw closing price immediately preceding the target prediction window."""
        return self.original.loc[:timestamp].iloc[-1]['close']
# ==============================================================================
# 4. ISOLATED SUBPROCESS FOLD WORKER (UDP IPC SOCKET ENGINE)
# ==============================================================================
def _reap_dead_children():
    """Reaps finished child processes to prevent Linux zombie accumulation."""
    try:
        # Non-blocking OS wait to harvest any exited child processes
        while True:
            pid, status = os.waitpid(-1, os.WNOHANG)
            if pid == 0:
                break
    except ChildProcessError:
        pass # No child processes left to wait for
def _parallel_fold_worker_udp(port, payload, result_queue):
    import os
    import sys

    # Mute C++ STDERR
    try:
        devnull_fd = os.open(os.devnull, os.O_WRONLY)
        os.dup2(devnull_fd, 2)
        os.close(devnull_fd)
    except Exception:
        pass

    os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'
    os.environ['CUDA_VISIBLE_DEVICES'] = '-1'
    os.environ['ABSL_LOG_LEVEL'] = '3'

    import signal
    import socket
    import json
    import time
    import logging
    import gc
    import numpy as np

    signal.signal(signal.SIGINT, signal.SIG_IGN)
    signal.signal(signal.SIGTERM, signal.SIG_IGN)

    fold_idx = payload.get('fold_idx', 0)
    chrom_id = payload.get('chrom_id', 'UNKNOWN')

    sock = None
    try:
        import tensorflow as tf
        tf.get_logger().setLevel('ERROR')
        from sklearn.metrics import root_mean_squared_error

        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        
        # Target loop port
        parent_address = ("127.0.0.1", port)

        X_train = np.array(payload['x_train'], dtype=np.float32)
        y_train = np.array(payload['y_train'], dtype=np.float32)
        X_val = np.array(payload['x_val'], dtype=np.float32)
        y_val = np.array(payload['y_val'], dtype=np.float32)
        horizon = payload['horizon']
        chromosome = payload['chromosome']
        price_return_indices = payload['price_return_indices']
        target_cols = payload['target_cols']
        
        num_timesteps, num_features = X_train.shape[1], X_train.shape[2]
        num_targets = y_train.shape[1]

        fold_logger.info(
            f"⏳ [TRAIN START] Model {chrom_id} | FOLD {fold_idx}/{NUM_FOLDS} | "
            f"Matrix: ({num_timesteps}d, {num_features}f) | Batch: {chromosome['batch_size']}"
        )

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
        
        # Clipnorm added to prevent gradient explosions / NaN losses
        optimizer = tf.keras.optimizers.Adam(
            learning_rate=chromosome['learning_rate'],
            clipnorm=1.0
        )
        model.compile(optimizer=optimizer, loss='mse')

        start_time = time.perf_counter()

        # Keepalive Heartbeat Callback
        class HeartbeatCallback(tf.keras.callbacks.Callback):
            def __init__(self, socket_obj, target_addr, chrom_id, fold_idx):
                super().__init__()
                self.sock = socket_obj
                self.addr = target_addr
                self.chrom_id = chrom_id
                self.fold_idx = fold_idx
                self.last_beat = 0

            def _send_beat(self, stage):
                now = time.time()
                if now - self.last_beat >= 1.0:
                    self.last_beat = now
                    hb_payload = {
                        "type": "heartbeat",
                        "chrom_id": self.chrom_id,
                        "fold_idx": self.fold_idx,
                        "stage": stage,
                        "timestamp": now
                    }
                    try:
                        msg = json.dumps(hb_payload).encode('utf-8')
                        self.sock.sendto(msg, self.addr)
                    except Exception:
                        pass

            def on_batch_end(self, batch, logs=None):
                self._send_beat("training_batch")

            def on_epoch_end(self, epoch, logs=None):
                self._send_beat(f"epoch_{epoch+1}")

        hb_callback = HeartbeatCallback(sock, parent_address, chrom_id, fold_idx)

        # EarlyStopping callback stops when loss flattens
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
            callbacks=[hb_callback, early_stop]
        )

        duration = time.perf_counter() - start_time
        final_loss = float(history.history['loss'][-1])
        
        # Keepalive check before inference
        hb_callback._send_beat("predicting")
        predictions = model.predict(X_val, verbose=0)

        rmse = float(root_mean_squared_error(y_val, predictions))
        actual_signs = np.sign(y_val)
        pred_signs = np.sign(predictions)

        # 1. ACCURACY EVALUATION
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

        # 2. PORTFOLIO BACKTEST
        price_indices = [idx for idx, col in enumerate(target_cols) if 'price_log_return' in col]

        if not price_indices:
            portfolio_returns = np.zeros(y_val.shape[0])
            clean_pred_signs = np.zeros((y_val.shape[0], 1))
            clean_y_val = np.zeros((y_val.shape[0], 1))
            net_strategy_returns = np.zeros((y_val.shape[0], 1))
            price_return_indices = [0]
        else:
            price_return_indices = price_indices
            clean_pred_signs = pred_signs[:, price_indices]
            clean_y_val = y_val[:, price_indices]
            
            raw_strategy_returns = clean_pred_signs * clean_y_val
            position_changes = np.abs(np.diff(clean_pred_signs, axis=0, prepend=clean_pred_signs[:1]))
            TRADING_FEE_RATE = 0.0005
            friction_costs = position_changes * TRADING_FEE_RATE
            net_strategy_returns = raw_strategy_returns - friction_costs
            portfolio_returns = np.mean(net_strategy_returns, axis=1)

        portfolio_returns = np.clip(portfolio_returns, -0.15, 0.15)

        winning_days = int(np.sum(portfolio_returns > 0))
        losing_days = int(np.sum(portfolio_returns < 0))
        flat_days = int(np.sum(portfolio_returns == 0))
        total_days = len(portfolio_returns)
        win_ratio = (winning_days / total_days * 100) if total_days > 0 else 0.0

        gross_profits = float(np.sum(portfolio_returns[portfolio_returns > 0]))
        gross_losses = float(np.sum(np.abs(portfolio_returns[portfolio_returns < 0])))
        profit_factor = (gross_profits / gross_losses) if gross_losses > 1e-6 else (gross_profits if gross_profits > 0 else 1.0)

        daily_std = float(np.std(portfolio_returns))
        mean_ret = float(np.mean(portfolio_returns))
        crypto_ann_factor = float(np.sqrt(365.0 / horizon))
        
        sharpe = float((mean_ret / daily_std * crypto_ann_factor)) if daily_std > 1e-6 else -5.0

        equity_curve = np.exp(np.cumsum(portfolio_returns))
        running_max = np.maximum.accumulate(equity_curve)
        drawdowns = (equity_curve - running_max) / running_max
        max_dd = float(abs(np.min(drawdowns))) if len(drawdowns) > 0 else 1.0

        fractional_years = horizon / 365.0
        ending_wealth = float(equity_curve[-1])
        raw_cagr = (ending_wealth ** (1.0 / fractional_years)) - 1.0 if ending_wealth > 0 else -1.0
        cagr = float(np.clip(raw_cagr, -0.99, 5.00))
        calmar = (cagr / max_dd) if max_dd > 1e-5 else (cagr / 0.00001)

        row_traces = []
        for r_i in range(min(5, y_val.shape[0])):
            row_traces.append({
                "row_idx": r_i,
                "target_val": float(clean_y_val[r_i, 0]),
                "pred_raw": float(predictions[r_i, price_return_indices[0]]),
                "pred_sign": float(clean_pred_signs[r_i, 0]),
                "strategy_return": float(net_strategy_returns[r_i, 0])
            })

        fold_logger.info(
            f"📈 [TRAIN COMPLETE] Model {chrom_id} | FOLD {fold_idx}/{NUM_FOLDS} | "
            f"Loss: {final_loss:.6f} | Time: {duration:.2f}s"
        )

        result_payload = {
            "type": "result",
            "status": "success",
            "chrom_id": chrom_id,
            "fold_idx": fold_idx,
            "port": port,
            "execution_duration": duration,
            "skill_da": avg_skill,
            "sharpe": sharpe,
            "max_dd": max_dd,
            "rmse": rmse,
            "cagr": cagr,
            "profit_factor": profit_factor,
            "calmar": calmar,
            "win_ratio": win_ratio,
            "winning_days": winning_days,
            "losing_days": losing_days,
            "flat_days": flat_days,
            "worst_return": float(portfolio_returns.min()) if total_days > 0 else 0.0,
            "best_return": float(portfolio_returns.max()) if total_days > 0 else 0.0,
            "asset_skills": asset_skills,
            "row_traces": row_traces,
            "loss": final_loss
        }

        result_queue.put(result_payload)
        time.sleep(0.5)

    except Exception as e:
        fold_logger.error(f"❌ [WORKER ERROR] Model {chrom_id} | Fold {fold_idx} crashed: {e}")
        err_res = {"type": "result", "status": "error", "chrom_id": chrom_id, "fold_idx": fold_idx, "error": str(e), "execution_duration": 0.0}
        result_queue.put(err_res)
    finally:
        try:
            import tensorflow as tf
            tf.keras.backend.clear_session()
        except Exception:
            pass
        gc.collect()
        if sock:
            sock.close()
# ==============================================================================
# 5. MAIN GA-LSTM OPTIMIZER ENGINE
# ==============================================================================
class LSTMOptimizerEngine:
    """
    Main Genetic Algorithm Engine that manages dataset ingestion, chromosome populations,
    parallel walk-forward cross-validation dispatching, Pareto frontier ranking, and model export.
    """
    def __init__(self, data_directory=".", checkpoint_file="lstm_ga_checkpoint.json", verbose=False):
        self.data_directory = data_directory
        self.checkpoint_file = checkpoint_file
        self.verbose = verbose
        self.running = True
        self.active_tasks = {}
        
        signal.signal(signal.SIGINT, self._handle_exit)
        signal.signal(signal.SIGTERM, self._handle_exit)
        
        self.chromosome_population = []
        self.current_generation = 0  # <--- NEW: Track generation for resumption
        self.scaler = MinMaxScaler(feature_range=(-1, 1))
        self.master_data = None
        self.master_data_raw = None
        self._first_split_done = False
        self.global_max_fold_time = 0.0
        
        logger.info(f"🚀 [INIT] LSTMOptimizerEngine initialized (Verbose: {self.verbose}).")
    def _process_data(self):
        """Standardizes master DataFrame using MinMaxScaler."""
        if self.master_data is None or self.master_data.empty:
            logger.error("❌ [PROCESS] Cannot scale empty master_data.")
            return

        logger.info(f"🛠️ [PROCESS] Normalizing data with MinMaxScaler (Shape: {self.master_data.shape})...")
        self.master_data_raw = self.master_data.copy()
        self.master_data = pd.DataFrame(
            self.scaler.fit_transform(self.master_data),
            columns=self.master_data.columns,
            index=self.master_data.index
        )
        logger.info("✅ [PROCESS] Normalization complete.")

    def _handle_exit(self, signum, frame):
        logger.warning(f"⚠️ [SIGNAL] Received signal {signum}. Cleaning child processes and saving state...")
        self.running = False

        # Kill all tracked active children
        for task in getattr(self, 'active_tasks', {}).values():
            p = task['process']
            if p.is_alive():
                p.kill()
        if hasattr(self, 'active_tasks'):
            self.active_tasks.clear()

        self._save_checkpoint()
        logger.info("👋 Exiting cleanly.")
        os._exit(0)

    def _clean_data(self, df):
        """Filters non-use or null data rows."""
        df = df.replace(0, np.nan)
        df = df.dropna()
        return df

    def _inverse_transform(self, log_returns, last_price):
        """Converts normalized log returns back to original price scale."""
        return last_price * np.exp(np.cumsum(log_returns))

    def _clear_state(self):
        """Resets the state checkpoint file."""
        if os.path.exists(self.checkpoint_file):
            os.remove(self.checkpoint_file)
            logger.info("🧹 [CLEAR] State checkpoint file deleted.")
        self.chromosome_population = []

    def execute_pipeline(self):
        """Sequential coordinator for data ingestion, population seeding, and evolution."""
        logger.info("🚀 [PIPELINE] Starting Parallel Multi-Process LSTM-GA Evolution sequence...")
        
        if not self._ingest_data_layers():
            logger.error("❌ [PIPELINE] Data Ingestion failed. Terminating pipeline.")
            return

        if not self._load_checkpoint():
            logger.info("🌱 [PIPELINE] No checkpoint found. Initializing fresh chromosome population.")
            self._initialize_random_population()

        self._process_data()
        self._evolve_generations()
        self._save_checkpoint()
        logger.info("🏁 [PIPELINE] Evolution pipeline sequence terminated cleanly.")

    def _split_features(self, df):
        """
        Splits columns into global temporal features, predictive assets, and target trackers.
        Enforces exact USER_EXCLUDE_FEATURES bans from entering the GA search mask.
        """
        temporal_patterns = ['day_wk_sin', 'day_wk_cos', 'day_yr_sin', 'day_yr_cos', 'hour_sin', 'hour_cos', 'min_sin', 'min_cos']
        time_cols = [c for c in df.columns if any(p in c for p in temporal_patterns)]
        base_asset_cols = [c for c in df.columns if c not in time_cols and not c.startswith('close_')]
        close_trackers = [c for c in df.columns if c.startswith('close_')]

        banned_lower = [banned.lower() for banned in USER_EXCLUDE_FEATURES]
        asset_cols = [c for c in base_asset_cols if c.lower() not in banned_lower]
        excluded_dropped = [c for c in base_asset_cols if c not in asset_cols]

        if getattr(self, '_first_split_done', False) is False:
            logger.info("-" * 60)
            logger.info("📐 [FEATURE SPLIT] Executing structural column classification...")
            logger.info(f"📊 [FEATURE SPLIT] Total DataFrame Width   : {len(df.columns)} columns")
            logger.info(f"⏳ [FEATURE SPLIT] Global Time Inputs      : {len(time_cols)} channels")
            logger.info(f"🎯 [FEATURE SPLIT] Target Validation Sets   : {len(close_trackers)} channels")
            logger.info(f"🚫 [FEATURE SPLIT] User-Excluded Features   : {len(excluded_dropped)} dropped {excluded_dropped}")
            logger.info(f"🧬 [FEATURE SPLIT] Evolutionary GA Pool     : {len(asset_cols)} elements remaining")
            if self.verbose:
                logger.info(f"🔍 [VERBOSE] Available Optimization Mask Features:\n{asset_cols}")
            logger.info("-" * 60)
            self._first_split_done = True

        return time_cols, asset_cols
    def _load_directory_to_df(self, transform_dir, raw_dir):
        """Helper to load and merge CSVs from specific directories cleanly."""
        import re

        all_files = glob.glob(os.path.join(transform_dir, "*_transformed.csv"))
        if not all_files:
            return None

        master_df = None
        global_time_df = None

        for f in all_files:
            try:
                df = pd.read_csv(f)
                df['timestamp'] = pd.to_datetime(df['timestamp'])
                df.set_index('timestamp', inplace=True)

                if global_time_df is None:
                    time_cols = [c for c in df.columns if any(x in c for x in ['day_', 'hour_', 'min_'])]
                    global_time_df = df[time_cols]

                # 🧼 CLEAN ASSET NAME: Extract core token name (e.g., 'uniswap' from 'uniswap_2022-07-01_2026-06-30_1d_transformed.csv')
                filename = os.path.basename(f)
                asset_name = filename.split('_')[0].lower() # Grabs 'uniswap', 'solana', 'bitcoin', etc.
                
                # Check for raw file in raw_dir
                raw_files = glob.glob(os.path.join(raw_dir, f"{asset_name}*.csv"))
                if raw_files:
                    orig_df = pd.read_csv(raw_files[0])
                    orig_df['timestamp'] = pd.to_datetime(orig_df['timestamp'])
                    orig_df.set_index('timestamp', inplace=True)
                    
                    if 'close' in orig_df.columns:
                        df['close'] = orig_df['close']
                    if 'volume' in orig_df.columns:
                        df['volume_raw'] = orig_df['volume']

                df = df.drop(columns=[c for c in df.columns if any(x in c for x in ['day_', 'hour_', 'min_'])])
                df = df[~df.index.duplicated(keep='first')]
                
                # Apply clean suffix without date range noise
                df = df.add_suffix(f'_{asset_name}')

                if master_df is None:
                    master_df = df
                else:
                    master_df = master_df.join(df, how='outer')

            except Exception as e:
                logger.error(f"❌ [INGEST] Failed to process {f}: {e}")

        if master_df is not None:
            final_df = pd.concat([master_df, global_time_df], axis=1)
            final_df = final_df.interpolate(method='linear').bfill().ffill().fillna(0)
            return final_df.dropna()
        
        return None
    def _ingest_data_layers(self) -> bool:
        """Loads Train and Validation DataFrames from dedicated directories."""
        logger.info(f"🔍 [INGEST] Loading Train Data from '{TRANSFORMED_DATA_DIR}'...")
        self.master_data = self._load_directory_to_df(TRANSFORMED_DATA_DIR, RAW_DATA_DIR)
        
        logger.info(f"🔍 [INGEST] Loading Validation Data from '{VAL_TRANSFORMED_DATA_DIR}'...")
        self.val_master_data = self._load_directory_to_df(VAL_TRANSFORMED_DATA_DIR, VAL_RAW_DATA_DIR)

        if self.master_data is None or self.master_data.empty:
            logger.error("❌ [INGEST] Training Master Data is empty. Check data directories.")
            return False

        self.feature_cols = self.master_data.columns.tolist()
        logger.info(f"🏁 [INGEST] Train Data Shape: {self.master_data.shape}")
        
        if self.val_master_data is not None and not self.val_master_data.empty:
            logger.info(f"🏁 [INGEST] Val Data Shape: {self.val_master_data.shape}")
        else:
            logger.warning("⚠️ [INGEST] No validation data loaded. True Future overlay will be skipped.")

        logger.info(f"🔍 [INGEST] Final Feature Count: {len(self.feature_cols)}")
        return True
    def _prepare_lstm_dataset(self, chromosome, data_source=None):
        """Constructs sliding window tensors (X, y) based on lookback and horizon genes."""
        if data_source is None:
            data_source = self.master_data

        time_cols, asset_cols = self._split_features(data_source)
        mask = np.array(chromosome['feature_mask'])
        if len(mask) != len(asset_cols):
            mask = np.ones(len(asset_cols), dtype=int)
            chromosome['feature_mask'] = mask.tolist()

        asset_values = data_source[asset_cols].values[:, mask == 1]
        time_values = data_source[time_cols].values
        combined_data = np.hstack([asset_values, time_values])

        lookback = int(chromosome.get('lookback_window', 30))
        forecast = int(chromosome.get('forecast_horizon', 1))

        num_samples = len(combined_data) - lookback - forecast
        if num_samples > 0:
            X, y = [], []
            for i in range(num_samples):
                X.append(combined_data[i : (i + lookback)])
                y.append(asset_values[i + lookback + forecast])
            return np.array(X), np.array(y), chromosome['feature_mask']
        
        logger.warning(f"⚠️ [DATA] Dataset too small for lookback:{lookback} horizon:{forecast}")
        return np.array([]), np.array([]), chromosome['feature_mask']

    def _build_and_train_lstm(self, chromosome, X, y):
        """Builds and trains a single-process Keras LSTM model for export and plotting."""
        import tensorflow as tf

        num_timesteps = X.shape[1]
        num_features = X.shape[2]
        num_targets = y.shape[1]

        num_layers = chromosome['lstm_layers']
        nodes = chromosome['nodes_per_layer']
        lr = chromosome['learning_rate']
        dropout_rate = chromosome['dropout_rate']
        batch_size = chromosome['batch_size']
        fold_num = chromosome.get('current_fold_running', 1)

        model = tf.keras.Sequential()
        model.add(tf.keras.Input(shape=(num_timesteps, num_features)))
        for i in range(num_layers):
            is_last = (i == num_layers - 1)
            node_count = nodes[i] if i < len(nodes) else nodes[0]
            model.add(tf.keras.layers.LSTM(node_count, return_sequences=not is_last))
            model.add(tf.keras.layers.Dropout(dropout_rate))
        model.add(tf.keras.layers.Dense(num_targets))

        optimizer = tf.keras.optimizers.Adam(learning_rate=lr)
        model.compile(optimizer=optimizer, loss='mse')

        logger.info(
            f"⏳ [TRAIN EXPORT] Model {chromosome['id']} | "
            f"Samples: {X.shape[0]} | Matrix: ({num_timesteps}d, {num_features}f) | "
            f"Batch: {batch_size} | LR: {lr:.5f}"
        )

        start_time = time.perf_counter()

        history = model.fit(
            X, y,
            epochs=5,
            batch_size=batch_size,
            verbose=0
        )

        duration = time.perf_counter() - start_time
        self.global_max_fold_time = max(self.global_max_fold_time, duration)
        final_loss = history.history['loss'][-1]

        logger.info(f"📈 [TRAIN COMPLETE] Model {chromosome['id']} | Loss: {final_loss:.6f} | Duration: {duration:.4f} secs")

        return model, history
    def _summarize_chromosome_evaluation(self, chromosome, fold_results, gen):
        """
        Processes cross-validation results, outputs the full detailed audit breakdown for each fold,
        and logs the final executive summary to chromosome_summary.log.
        """
        fold_results.sort(key=lambda x: x.get('fold_idx', 0))

        fold_skill_da, fold_sharpe, fold_maxdd, fold_rmse = [], [], [], []
        fold_cagr, fold_profit_factor, fold_calmar = [], [], []
        asset_history_metrics = {}

        # ----------------------------------------------------------------------
        # RESTORED: Detailed per-fold audit diagnostics
        # ----------------------------------------------------------------------
        for res in fold_results:
            if res.get("status") == "error":
                summary_logger.error(f"❌ [FOLD ERROR] Model {chromosome['id']} | Fold {res.get('fold_idx')} failed: {res.get('error')}")
                continue

            if res.get("status") == "success":
                f_idx = res['fold_idx']
                fold_skill_da.append(res['skill_da'])
                fold_sharpe.append(res['sharpe'])
                fold_maxdd.append(res['max_dd'])
                fold_rmse.append(res['rmse'])
                fold_cagr.append(res['cagr'])
                fold_profit_factor.append(res['profit_factor'])
                fold_calmar.append(res['calmar'])

                # Full Diagnostic Audit Block
                summary_logger.info("=" * 65)
                summary_logger.info(f"🕵️‍♂️ [TARGET BALANCE CHECK - FOLD {f_idx}/{NUM_FOLDS}]:")
                for a_name, a_info in res['asset_skills'].items():
                    base_pct = a_info['baseline'] * 100
                    summary_logger.info(f"   {a_name.ljust(16)} Target -> Baseline DA: {base_pct:.1f}%")

                summary_logger.info("-" * 65)
                summary_logger.info(f"🎯 [PER-ASSET FORECASTING ACCURACY - FOLD {f_idx}/{NUM_FOLDS}]:")
                for a_name, a_info in res['asset_skills'].items():
                    m_da = a_info['model'] * 100
                    summary_logger.info(f"   {a_name.ljust(16)} Model DA: {m_da:.2f}% | Skill DA: {a_info['skill']*100:+.2f}%")

                summary_logger.info("-" * 65)
                summary_logger.info(f"🕵️‍♂️ [SURGICAL AUDIT FOLD {f_idx}/{NUM_FOLDS}] Deep Vector Diagnostics:")
                summary_logger.info(f"   📦 Day Counts -> Wins: {res['winning_days']} | Losses: {res['losing_days']} | Flats: {res['flat_days']} [Win Rate: {res['win_ratio']:.2f}%]")
                summary_logger.info(f"   📉 Bounds     -> Worst Day: {res['worst_return']:.6f} | Best Day: {res['best_return']:.6f}")
                summary_logger.info(f"   ⚡ Risk Specs -> Annualized Sharpe: {res['sharpe']:.2f} | Max Drawdown: {res['max_dd']*100:.2f}%")
                
                summary_logger.info(f"   📋 [RAW RETURN MATRIX STRATEGY ALIGNMENT - TARGET CHANNEL 0]:")
                for row_t in res.get('row_traces', []):
                    summary_logger.info(
                        f"      Row {row_t['row_idx']} -> Target Asset 0: {row_t['target_val']:.6f} | "
                        f"Pred Sign: {row_t['pred_sign']:+.0f} | "
                        f"Product Strategy Return: {row_t['strategy_return']:.6f}"
                    )
                summary_logger.info("=" * 65)

                for asset_name, metrics in res['asset_skills'].items():
                    if asset_name not in asset_history_metrics:
                        asset_history_metrics[asset_name] = {'baseline': [], 'model': [], 'skill': []}
                    asset_history_metrics[asset_name]['baseline'].append(metrics['baseline'])
                    asset_history_metrics[asset_name]['model'].append(metrics['model'])
                    asset_history_metrics[asset_name]['skill'].append(metrics['skill'])

        if not fold_skill_da:
            return [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]

        skill_da_mean = float(np.mean(fold_skill_da))
        sharpe_mean = float(np.mean(fold_sharpe))
        maxdd_mean = float(np.mean(fold_maxdd))
        rmse_mean = float(np.mean(fold_rmse))
        skill_da_std = float(np.std(fold_skill_da))
        sharpe_std = float(np.std(fold_sharpe))

        objectives_vector = [skill_da_mean, sharpe_mean, maxdd_mean, rmse_mean, skill_da_std, sharpe_std]

        # Executive Summary Panel
        total_eval_time = sum(res.get('execution_duration', 0.0) for res in fold_results if res.get('status') == 'success')
        avg_fold_time = total_eval_time / len(fold_skill_da) if fold_skill_da else 0.0
        avg_loss = float(np.mean([res['loss'] for res in fold_results if 'loss' in res])) if fold_results else 0.0

        summary_logger.info("=" * 65)
        summary_logger.info(f"⚡ [EVALUATION COMPLETE] Model: {chromosome['id']} | Gen: {gen}")
        summary_logger.info(f"   ⏱️  Total Duration      : {total_eval_time:.2f}s (Avg Fold: {avg_fold_time:.2f}s)")
        summary_logger.info(f"   📉 Mean Training Loss   : {avg_loss:.6f}")
        summary_logger.info(f"   🎯 Mean Skill DA        : {skill_da_mean * 100:+.2f}% (std: {skill_da_std * 100:.2f}%)")
        summary_logger.info(f"   ⚡ Mean Sharpe Ratio    : {sharpe_mean:.2f} (std: {sharpe_std:.2f})")
        summary_logger.info(f"   📉 Mean Max Drawdown    : {maxdd_mean * 100:.2f}%")
        summary_logger.info(f"   📐 Mean RMSE            : {rmse_mean:.4f}")
        summary_logger.info("=" * 65)

        # Restored Global Generation Profile
        all_baselines, all_models, all_skills, asset_summary_lines = [], [], [], []
        for asset, metrics in asset_history_metrics.items():
            a_base = float(np.mean(metrics['baseline']))
            a_model = float(np.mean(metrics['model']))
            a_skill = float(np.mean(metrics['skill']))

            all_baselines.append(a_base)
            all_models.append(a_model)
            all_skills.append(a_skill)
            asset_summary_lines.append((asset, a_skill))

        asset_summary_lines.sort(key=lambda x: x[1], reverse=True)
        global_avg_baseline = float(np.mean(all_baselines)) if all_baselines else 0.0
        global_avg_model = float(np.mean(all_models)) if all_models else 0.0
        global_avg_skill = float(np.mean(all_skills)) if all_skills else 0.0

        summary_logger.info("=" * 65)
        summary_logger.info(f"🏆 [GLOBAL GEN CROSS-VALIDATION SUMMARY] - ID: {chromosome['id']}")
        summary_logger.info(f"   Average Baseline DA : {global_avg_baseline * 100:.1f}%")
        summary_logger.info(f"   Average Model DA    : {global_avg_model * 100:.1f}%")
        summary_logger.info(f"   Average Skill DA    : {global_avg_skill * 100:+.2f}%")
        summary_logger.info("-" * 65)
        summary_logger.info("🎯 [SKILL DA MATRICES PROFILE BY ASSET]:")
        for asset_name, skill_val in asset_summary_lines:
            summary_logger.info(f"   {asset_name.ljust(16)} : {skill_val * 100:+.2f}%")
        summary_logger.info("-" * 65)
        summary_logger.info("📊 [SKILL SUMMARY PROFILE MATRICES]")
        summary_logger.info(f"   Best Asset Skill DA  : {max(all_skills) * 100:+.2f}%" if all_skills else "   Best Asset Skill DA  : +0.00%")
        summary_logger.info(f"   Worst Asset Skill DA : {min(all_skills) * 100:+.2f}%" if all_skills else "   Worst Asset Skill DA : +0.00%")
        summary_logger.info(f"   Average Skill DA     : {global_avg_skill * 100:+.2f}%")
        summary_logger.info(f"   Median Skill DA      : {float(np.median(all_skills)) * 100:+.2f}%" if all_skills else "   Median Skill DA      : +0.00%")
        summary_logger.info(f"   Skill DA Std         : {skill_da_std * 100:.2f}%")
        summary_logger.info("-" * 65)
        summary_logger.info("📈 [TRADING & RISK METRICS SUMMARY]")
        summary_logger.info(f"   Averages  -> Sharpe: {sharpe_mean:.2f} | MaxDD: {maxdd_mean*100:.2f}% | RMSE: {rmse_mean:.4f}")
        summary_logger.info(f"   Stability -> Sharpe_std: {sharpe_std:.2f}")
        summary_logger.info(f"   Extended  -> Mean CAGR: {np.mean(fold_cagr)*100:.2f}% | Profit Factor: {np.mean(fold_profit_factor):.2f} | Calmar: {np.mean(fold_calmar):.2f}")
        summary_logger.info("=" * 65)

        return objectives_vector
    def _build_fold_payloads(self, chromosome):
        """Pre-constructs walk-forward fold payloads for a single chromosome."""
        X, y_all, _ = self._prepare_lstm_dataset(chromosome)
        total_samples = X.shape[0]

        lookback = chromosome['lookback_window']
        horizon = chromosome['forecast_horizon']

        _, all_asset_cols = self._split_features(self.master_data)
        target_cols = [col for col, m in zip(all_asset_cols, chromosome['feature_mask']) if m == 1]
        price_return_indices = [idx for idx, name in enumerate(target_cols) if 'price_log_return' in name]
        if not price_return_indices:
            price_return_indices = list(range(len(target_cols)))

        min_train_size = 20
        usable_samples = max(10, total_samples - min_train_size)
        val_size = max(5, min(horizon, usable_samples // (NUM_FOLDS + 1)))
        fold_step = max(5, (total_samples - val_size) // NUM_FOLDS)

        fold_payloads = []
        for fold in range(NUM_FOLDS):
            train_end_idx = fold_step * (fold + 1)
            val_end_idx = min(total_samples, train_end_idx + val_size)

            X_train, y_train = X[:train_end_idx], y_all[:train_end_idx]
            X_val, y_val = X[train_end_idx:val_end_idx], y_all[train_end_idx:val_end_idx]

            if X_train.shape[0] < 10 or X_val.shape[0] == 0:
                continue

            fold_payloads.append({
                "chrom_id": chromosome['id'],
                "fold_idx": fold + 1,
                "x_train": X_train,
                "y_train": y_train,
                "x_val": X_val,
                "y_val": y_val,
                "horizon": horizon,
                "chromosome": chromosome,
                "price_return_indices": price_return_indices,
                "target_cols": target_cols
            })

        return fold_payloads
    def _evaluate_population_pipelined(self, population, gen):
        import queue

        global_task_queue = []
        completed_results = {c['id']: [] for c in population}
        expected_folds_count = {}
        result_queue = mp.Queue()

        # 1. Load fold tasks ONLY for unevaluated models
        for chromosome in population:
            c_id = chromosome['id']
            perf = chromosome.get('perf_vector', [])
            
            # Check if this model is already evaluated
            is_done = chromosome.get('fitness_evaluated', False) and len(perf) == 6 and perf[3] < 990.0
            if is_done:
                continue

            summary_logger.info("-" * 60)
            summary_logger.info(f"🧬 [CONFIG] GEN:{gen} | ID:{c_id}")
            summary_logger.info(f"🖥️  Structure    : Layers: {chromosome['lstm_layers']} | Nodes: {chromosome['nodes_per_layer']}")
            summary_logger.info(f"⏳  Windows      : Lookback: {chromosome['lookback_window']}d | Horizon: {chromosome['forecast_horizon']}d")
            summary_logger.info(f"🎛️  Hyperparams  : LR: {chromosome['learning_rate']:.5f} | Dropout: {chromosome['dropout_rate']:.2f} | Batch: {chromosome['batch_size']}")

            payloads = self._build_fold_payloads(chromosome)
            expected_folds_count[c_id] = len(payloads)
            
            for p in payloads:
                global_task_queue.append((c_id, p))

        logger.info(f"🚀 [PIPELINE QUEUE] Loaded {len(global_task_queue)} total fold tasks for Generation {gen}.")

        # Rolling worker execution loop
        while global_task_queue or self.active_tasks:
            if not self.running:
                break

            # Step A: Fill open worker slots
            while len(self.active_tasks) < MAX_PARALLEL_FOLDS and global_task_queue:
                chrom_id, payload = global_task_queue.pop(0)
                fold_idx = payload['fold_idx']
                port = UDP_BASE_PORT + fold_idx

                fold_logger.info(f"🚀 [DISPATCH] Spawning Model {chrom_id} | Fold {fold_idx}/{NUM_FOLDS} (Port {port})...")

                p = mp.Process(target=_parallel_fold_worker_udp, args=(port, payload, result_queue))
                p.start()
                self.active_tasks[p.pid] = {
                    'process': p,
                    'chrom_id': chrom_id,
                    'payload': payload
                }

            # Step B: Read non-blocking IPC results
            while True:
                try:
                    res = result_queue.get_nowait()
                    c_id = res.get('chrom_id')
                    if c_id in completed_results:
                        completed_results[c_id].append(res)
                except queue.Empty:
                    break

            # Step C: Harvest finished/dead worker processes
            dead_pids = []
            for pid, task in list(self.active_tasks.items()):
                p = task['process']
                if not p.is_alive():
                    p.join(timeout=0.1)
                    dead_pids.append(pid)
                    
                    c_id = task['chrom_id']
                    f_idx = task['payload']['fold_idx']
                    
                    got_res = any(r.get('fold_idx') == f_idx for r in completed_results.get(c_id, []))
                    if not got_res:
                        fold_logger.error(f"❌ [WORKER DIE/CRASH] Model {c_id} | Fold {f_idx} died! Re-queueing task...")
                        global_task_queue.insert(0, (c_id, task['payload']))

            for pid in dead_pids:
                del self.active_tasks[pid]

            # Step D: Finalize metrics for models receiving all expected fold results
            for chromosome in population:
                c_id = chromosome['id']
                target_count = expected_folds_count.get(c_id, NUM_FOLDS)
                if target_count > 0 and c_id in completed_results and len(completed_results[c_id]) == target_count:
                    if not chromosome.get('fitness_evaluated', False):
                        objectives = self._summarize_chromosome_evaluation(chromosome, completed_results[c_id], gen)
                        chromosome['perf_vector'] = objectives
                        chromosome['fitness_evaluated'] = True

            time.sleep(0.2)
    def _log_population_stats(self):
        """Logs feature selection frequency across the entire population."""
        _, asset_cols = self._split_features(self.master_data)
        # ✅ FIXED SAFE VERSION:
        masks_list = [c['feature_mask'] for c in self.chromosome_population]

        # Find the maximum mask length across the population
        max_len = max(len(m) for m in masks_list) if masks_list else 0

        # Pad shorter masks with zeros so they form a homogeneous 2D array
        padded_masks = [
        list(m) + [0] * (max_len - len(m)) for m in masks_list
        ]

        all_masks = np.array(padded_masks)
        selection_counts = np.sum(all_masks, axis=0)
        total_pop = len(self.chromosome_population)

        logger.info("📊 [STATS] Population Feature Selection Frequency:")
        for col, count in zip(asset_cols, selection_counts):
            percentage = (count / total_pop) * 100
            if percentage > 10:
                logger.info(f"   - {col:30} : {percentage:5.1f}%")

    def _get_normal_random(self, min_val, max_val):
        """Generates a value from a normal distribution centered between min and max."""
        mu = (min_val + max_val) / 2
        sigma = (max_val - min_val) / 6
        val = random.gauss(mu, sigma)
        return int(max(min_val, min(max_val, val)))

    def _initialize_random_population(self):
        """Seeds the initial chromosome population with randomized architectures and feature masks."""
        logger.info(f"🔍 [DEBUG] All available master data columns: {list(self.master_data.columns)}")
        logger.info("🌱 [INIT] Seeding new LSTM chromosome population with expanded hyperparameter matrix...")

        max_rows = len(self.master_data)
        actual_max_lookback = max(MIN_LOOKBACK_DAYS + 1, min(MAX_LOOKBACK_DAYS, int(max_rows * 0.7)))
        actual_max_horizon = max(MIN_FORECAST_DAYS + 1, min(MAX_FORECAST_DAYS, int(max_rows * 0.7)))

        logger.info(f"🔍 [INIT] Data rows: {max_rows} | Safe Lookback range: {MIN_LOOKBACK_DAYS}-{actual_max_lookback} | Safe Horizon range: {MIN_FORECAST_DAYS}-{actual_max_horizon}")

        _, asset_cols = self._split_features(self.master_data)
        self.chromosome_population = []

        for i in range(POPULATION_SIZE):
            lookback = random.randint(MIN_LOOKBACK_DAYS, actual_max_lookback)
            horizon = random.randint(MIN_FORECAST_DAYS, actual_max_horizon)
            num_layers = self._get_normal_random(MIN_HIDDEN_LAYERS, MAX_HIDDEN_LAYERS)
            num_features_to_select = random.randint(5, len(asset_cols))

            mask = [1] * num_features_to_select + [0] * (len(asset_cols) - num_features_to_select)
            random.shuffle(mask)

            chromosome = {
                "id": f"G1-ID{i}",
                "lstm_layers": num_layers,
                "nodes_per_layer": [self._get_normal_random(MIN_NODES_PER_LAYER, MAX_NODES_PER_LAYER) for _ in range(num_layers)],
                "lookback_window": lookback,
                "forecast_horizon": horizon,
                "learning_rate": round(random.uniform(MIN_LR, MAX_LR), 5),
                "dropout_rate": round(random.uniform(MIN_DROPOUT, MAX_DROPOUT), 2),
                "batch_size": random.choice(BATCH_SIZE_CHOICES),
                "feature_mask": mask,
                "perf_vector": [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
            }
            self.chromosome_population.append(chromosome)

            logger.info(
                f"✨ [INIT SEED] Chromosome {chromosome['id']} created -> "
                f"Layers: {chromosome['lstm_layers']} | Lookback: {chromosome['lookback_window']}d | "
                f"Horizon: {chromosome['forecast_horizon']}d | LR: {chromosome['learning_rate']:.5f} | "
                f"Dropout: {chromosome['dropout_rate']:.2f} | Batch: {chromosome['batch_size']} | "
                f"Inputs: {num_features_to_select}/{len(asset_cols)}"
            )

        logger.info(f"✅ [INIT] Population seeding completed. {POPULATION_SIZE} active chromosomes loaded into memory.")

    def _create_random_chromosome(self, child_id="G1-ID0") -> dict:
        """Generates a single bounded randomized chromosome."""
        layers = random.randint(MIN_HIDDEN_LAYERS, MAX_HIDDEN_LAYERS)
        _, asset_cols = self._split_features(self.master_data)
        feature_mask = [random.choice([0, 1]) for _ in range(len(asset_cols))]
        if sum(feature_mask) == 0:
            feature_mask[random.randint(0, len(feature_mask) - 1)] = 1

        return {
            "id": child_id,
            "lstm_layers": layers,
            "nodes_per_layer": [random.randint(MIN_NODES_PER_LAYER, MAX_NODES_PER_LAYER) for _ in range(layers)],
            "lookback_window": random.randint(MIN_LOOKBACK_DAYS, MAX_LOOKBACK_DAYS),
            "forecast_horizon": random.randint(MIN_FORECAST_DAYS, MAX_FORECAST_DAYS),
            "learning_rate": round(random.uniform(MIN_LR, MAX_LR), 5),
            "dropout_rate": round(random.uniform(MIN_DROPOUT, MAX_DROPOUT), 2),
            "batch_size": random.choice(BATCH_SIZE_CHOICES),
            "feature_mask": feature_mask, 
            "perf_vector": [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]  # <--- FIXED: Initialized multi-objective vector template
        }

    def _evolve_generations(self):
        # 1. Resume from the exact generation where we left off
        for gen in range(self.current_generation, GENERATIONS):
            if not self.running:
                break

            self.current_generation = gen
            gen_num = gen + 1
            logger.info(f"🧬 [NSGA-II MULTI-REGIME] Starting Generation {gen_num}/{GENERATIONS}...")

            for i, chromo in enumerate(self.chromosome_population):
                if not self.running:
                    break
                    
                expected_id = f"G{gen_num}-ID{i}"
                chromo['id'] = expected_id

                perf = chromo.get('perf_vector', [])
                # A chromosome is completed if it has 6 objectives and RMSE is less than default 999.0
                is_already_evaluated = chromo.get('fitness_evaluated', False) and len(perf) == 6 and perf[3] < 990.0

                if is_already_evaluated:
                    logger.info(f"⏭️ [RESUME] Skipping {chromo['id']} — already evaluated (Skill DA: {perf[0]*100:+.2f}%, Sharpe: {perf[1]:.2f}).")
                    continue

                logger.info("-" * 60)
                logger.info(f"🧬 [CONFIG] GEN:{gen_num} | ID:{chromo['id']}")
                logger.info(f"🖥️  Structure    : Layers: {chromo['lstm_layers']} | Nodes: {chromo['nodes_per_layer']}")
                logger.info(f"⏳  Windows      : Lookback: {chromo['lookback_window']}d | Horizon: {chromo['forecast_horizon']}d")
                logger.info(f"🎛️  Hyperparams  : LR: {chromo['learning_rate']:.5f} | Dropout: {chromo['dropout_rate']:.2f} | Batch: {chromo['batch_size']}")

                if self.verbose:
                    _, asset_cols = self._split_features(self.master_data)
                    selected = [col for col, mask_val in zip(asset_cols, chromo['feature_mask']) if mask_val == 1]
                    logger.info(f"🔍 [VERBOSE] Active Inputs Selected ({len(selected)}):\n{selected}")

            # Run parallel fold cross-validation pipeline for unevaluated models
            self._evaluate_population_pipelined(self.chromosome_population, gen=gen_num)
            
            # Save checkpoint after evaluating population
            self._save_checkpoint()

            # Non-Dominated Pareto Sorting
            pareto_front = []
            for c_target in self.chromosome_population:
                dominated = False
                for c_competitor in self.chromosome_population:
                    if 'perf_vector' in c_competitor and 'perf_vector' in c_target:
                        if self._check_pareto_dominance(c_competitor['perf_vector'], c_target['perf_vector']):
                            dominated = True
                            break
                if not dominated:
                    pareto_front.append(c_target)

            pareto_front.sort(key=self._apply_priority_tie_breaker)

            logger.info("=" * 60)
            logger.info(f"🏆 [PARETO FRONT] Gen {gen_num} Non-Dominated Pool Size: {len(pareto_front)} models")
            for elite in pareto_front[:3]:
                v = elite['perf_vector']
                logger.info(
                    f"  ⭐ Robust Candidate {elite['id']} -> "
                    f"Skill_DA_m: {v[0]*100:+.2f}% (std: {v[4]*100:.2f}%) | "
                    f"Sharpe_m: {v[1]:.2f} (std: {v[5]:.2f}) | "
                    f"MaxDD_m: {v[2]*100:.1f}% | RMSE_m: {v[3]:.4f}"
                )
            logger.info("=" * 60)

            if len(pareto_front) > 0:
                self._plot_prediction_comparison(pareto_front[0], export_dir="prediction_result")
                
                # Export top models immediately
                logger.info(f"📦 [EXPORT] Packaging Top {TOP_N_EXPORTS} models from Gen {gen_num} for external apps...")
                combined_df = pd.concat([self.master_data, self.val_master_data]) if self.val_master_data is not None else self.master_data
                X_full, y_full, _ = self._prepare_lstm_dataset(pareto_front[0], data_source=combined_df)
                
                for rank_idx, elite_chromo in enumerate(pareto_front[:TOP_N_EXPORTS]):
                    self.export_trained_model(elite_chromo, X_full, y_full, export_dir="deployed_models", rank=rank_idx + 1)

            # ==================================================================
            # 🧬 REPRODUCTION, CROSSOVER & MUTATION WITH STATE RESET
            # ==================================================================
            new_pop = []
            
            # 1. Retain Pareto Elites (Keep their fitness_evaluated = True)
            for elite in pareto_front:
                elite_copy = dict(elite)
                elite_copy['fitness_evaluated'] = True
                new_pop.append(elite_copy)
                if len(new_pop) >= POPULATION_SIZE // 2:
                    break

            # 2. Produce Mutated Children (Reset fitness_evaluated & perf_vector)
            while len(new_pop) < POPULATION_SIZE and len(pareto_front) > 0:
                parent_a = random.choice(pareto_front)
                parent_b = random.choice(pareto_front)
                
                child = {
                    "id": f"G{gen_num + 1}-ID{len(new_pop)}",
                    "lstm_layers": parent_a['lstm_layers'],
                    "nodes_per_layer": list(parent_a['nodes_per_layer']),
                    "lookback_window": parent_a['lookback_window'],
                    "forecast_horizon": parent_a['forecast_horizon'],
                    "learning_rate": parent_a['learning_rate'],
                    "dropout_rate": parent_a['dropout_rate'],
                    "batch_size": parent_a['batch_size'],
                    "feature_mask": list(parent_b['feature_mask']),
                    "fitness_evaluated": False,  # 🎯 RESET: Force evaluation for offspring
                    "perf_vector": [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
                }
                
                child = self._mutate(child)
                
                # 🎯 RESET: Ensure mutation cleans any stale evaluation flag/metrics
                child['fitness_evaluated'] = False
                child['perf_vector'] = [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
                
                new_pop.append(child)

            self.chromosome_population = new_pop if len(new_pop) > 0 else self._initialize_random_population()
            
            # Save state for the new generation transition
            self.current_generation += 1
            self._save_checkpoint()
            self._log_population_stats()
            logger.info(f"✅ [NSGA-II] Generation {gen_num} execution cycle complete.\n")
    def _mutate(self, chromosome: dict) -> dict:
        """Mutates chromosome genes based on global MUTATION_RATE probability."""
        rate = MUTATION_RATE
        max_rows = len(self.master_data)

        if random.random() < rate:
            chromosome['lstm_layers'] = random.randint(MIN_HIDDEN_LAYERS, MAX_HIDDEN_LAYERS)
            chromosome['nodes_per_layer'] = [
                self._get_normal_random(MIN_NODES_PER_LAYER, MAX_NODES_PER_LAYER)
                for _ in range(chromosome['lstm_layers'])
            ]
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: depth & nodes.")

        if random.random() < rate:
            safe_max_lookback = min(MAX_LOOKBACK_DAYS, int(max_rows * 0.7))
            chromosome['lookback_window'] = random.randint(MIN_LOOKBACK_DAYS, max(MIN_LOOKBACK_DAYS + 1, safe_max_lookback))
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: lookback_window ({chromosome['lookback_window']}d).")

        if random.random() < rate:
            safe_max_horizon = min(MAX_FORECAST_DAYS, int(max_rows * 0.7))
            chromosome['forecast_horizon'] = random.randint(MIN_FORECAST_DAYS, max(MIN_FORECAST_DAYS + 1, safe_max_horizon))
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: forecast_horizon ({chromosome['forecast_horizon']}d).")

        if random.random() < rate:
            chromosome['learning_rate'] = round(random.uniform(MIN_LR, MAX_LR), 5)
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: learning_rate ({chromosome['learning_rate']:.5f}).")

        if random.random() < rate:
            chromosome['dropout_rate'] = round(random.uniform(MIN_DROPOUT, MAX_DROPOUT), 2)
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: dropout_rate ({chromosome['dropout_rate']:.2f}).")

        if random.random() < rate:
            chromosome['batch_size'] = random.choice(BATCH_SIZE_CHOICES)
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: batch_size ({chromosome['batch_size']}).")

        if random.random() < rate:
            mask = chromosome['feature_mask']
            mutate_idx = random.randint(0, len(mask) - 1)
            mask[mutate_idx] = 1 - mask[mutate_idx]
            if sum(mask) == 0:
                mask[mutate_idx] = 1
            chromosome['feature_mask'] = mask
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: feature mask vector.")

        return chromosome
    # ==========================================================================
    # 🧬 NSGA-II MULTI-OBJECTIVE PARETO FITNESS VECTOR OPTIMIZATION
    # ==========================================================================
    # Note: We do NOT use a single scalar 'fitness_score' anymore. Instead, we 
    # evaluate models using a comprehensive 6-Dimensional Fitness Vector 
    # (perf_vector) across walk-forward cross-validation folds:
    # 
    # Vector Layout: 
    #   [0] Skill_DA_mean  -> MAXIMIZE (True predictive edge over naive baseline)
    #   [1] Sharpe_mean    -> MAXIMIZE (Risk-adjusted return performance)
    #   [2] MaxDD_mean     -> MINIMIZE (Portfolio max drawdown constraint)
    #   [3] RMSE_mean      -> MINIMIZE (Raw regression error minimization)
    #   [4] Skill_DA_std   -> MINIMIZE (Edge stability across cross-validation folds)
    #   [5] Sharpe_std     -> MINIMIZE (Return consistency / stability)
    #
    # Pareto dominance checks whether a candidate model is superior across multiple 
    # competing goals simultaneously, rather than collapsing performance into 
    # a single weighted average or raw Skill DA metric.
    # ==========================================================================
    def _check_pareto_dominance(self, vector_a, vector_b):
        """
        Evaluates 6-Dimensional Pareto dominance.
        Vector layout: [Skill_DA_mean, Sharpe_mean, MaxDD_mean, RMSE_mean, Skill_DA_std, Sharpe_std]
        """
        cond1 = (
            vector_a[0] >= vector_b[0] and  # 1. Skill_DA_mean (Maximize)
            vector_a[1] >= vector_b[1] and  # 2. Sharpe_mean (Maximize)
            vector_a[2] <= vector_b[2] and  # 3. MaxDD_mean (Minimize)
            vector_a[3] <= vector_b[3] and  # 4. RMSE_mean (Minimize)
            vector_a[4] <= vector_b[4] and  # 5. Skill_DA_std (Minimize)
            vector_a[5] <= vector_b[5]      # 6. Sharpe_std (Minimize)
        )
        cond2 = (
            vector_a[0] > vector_b[0] or
            vector_a[1] > vector_b[1] or
            vector_a[2] < vector_b[2] or
            vector_a[3] < vector_b[3] or
            vector_a[4] < vector_b[4] or
            vector_a[5] < vector_b[5]
        )
        return cond1 and cond2

    def _apply_priority_tie_breaker(self, chromosome):
        """Positional comparison tuple for non-dominated ranking."""
        v = chromosome['perf_vector']
        return (-v[0], -v[1], v[4], v[5], v[2], v[3])

    def _save_checkpoint(self):
        """Persists exact population state and generation integer to disk."""
        state = {
            "generation": self.current_generation,
            "population": self.chromosome_population,
            "timestamp": datetime.datetime.now().isoformat()
        }
        try:
            with open(self.checkpoint_file, 'w') as f:
                json.dump(state, f, indent=4)
        except Exception:
            pass
    def _load_checkpoint(self):
        """Restores population state and active generation from checkpoint."""
        if not os.path.exists(self.checkpoint_file):
            return False
        try:
            with open(self.checkpoint_file, 'r') as f:
                data = json.load(f)
            
            self.current_generation = data.get("generation", 0)
            raw_pop = data.get("population", [])
            valid_pop = [item for item in raw_pop if isinstance(item, dict)]
            
            if not valid_pop:
                return False
                
            self.chromosome_population = valid_pop
            logger.info(f"♻️ [RESTORE] Resuming at Generation {self.current_generation + 1} ({len(self.chromosome_population)} chromosomes loaded).")
            return True
        except Exception:
            return False
    def _plot_prediction_comparison(self, chromosome, export_dir="prediction_result"):
        """Generates continuous Validation Overlay and True Future Forecast plots with real price unscaling."""
        import matplotlib.pyplot as plt
        import matplotlib.dates as mdates
        os.makedirs(export_dir, exist_ok=True)
        
        horizon = int(chromosome.get('forecast_horizon', 30))

        # --- PREPARATION: TRAIN vs VAL ---
        X_train, y_train_scaled, mask = self._prepare_lstm_dataset(chromosome, data_source=self.master_data)
        model, _ = self._build_and_train_lstm(chromosome, X_train, y_train_scaled)
        
        _, asset_cols = self._split_features(self.master_data)
        selected_features = [asset_cols[i] for i, val in enumerate(mask) if val == 1]

        # Safe DataFrame resolution for raw unscaled prices
        raw_train_df = getattr(self, 'master_data_raw', None)
        if raw_train_df is None or raw_train_df.empty:
            raw_train_df = self.master_data

        raw_val_df = getattr(self, 'val_master_data_raw', None)
        if raw_val_df is None or raw_val_df.empty:
            raw_val_df = self.val_master_data

        # --- GRAPH SET 1: VALIDATION OVERLAY ---
        if self.val_master_data is not None and not self.val_master_data.empty:
            logger.info(f"📊 [PLOT] Generating Validation Overlay Graph for {chromosome['id']}...")
            
            # Sequence prediction over validation horizon
            last_window = X_train[-1:]
            # --- DYNAMIC AUTOREGRESSIVE MULTI-STEP FORECAST ---
            lookback = int(chromosome.get('lookback_window', 60))
            curr_window = X_train[-1:].copy()  # Shape: (1, lookback, num_features)
            
            val_preds_list = []
            
            for step in range(horizon):
                # 1. Predict next step
                step_pred = model.predict(curr_window, verbose=0)  # Shape: (1, num_targets)
                val_preds_list.append(step_pred[0])
                
                # 2. Construct next input feature row
                next_feature_row = curr_window[0, -1, :].copy()
                num_targets = step_pred.shape[1]
                
                # Update predicted targets in feature slice
                next_feature_row[:num_targets] = step_pred[0]
                
                # 3. Slide lookback window forward by 1 step
                new_win = np.append(curr_window[0, 1:, :], [next_feature_row], axis=0)
                curr_window = np.expand_dims(new_win, axis=0)

            val_preds_matrix = np.array(val_preds_list)  # Shape: (31, num_targets)

            for f_idx, feat_name in enumerate(selected_features):
                if feat_name not in self.master_data.columns:
                    continue

                fig, ax = plt.subplots(figsize=(12, 4))
                master_col_idx = self.master_data.columns.get_loc(feat_name)
                feat_min = self.scaler.min_[master_col_idx]
                feat_scale = self.scaler.scale_[master_col_idx]

                # Unscale normalized model output back to log return scale
                pred_unscaled = (val_preds_matrix[:, f_idx] - feat_min) / feat_scale
                
                # Check if feature corresponds to an asset price
                asset_base = feat_name.replace('price_log_return_', '').replace('volume_log_change_', '')
                raw_close_col = f'close_{asset_base}'

                if raw_close_col in raw_train_df.columns:
                    last_known_price = float(raw_train_df[raw_close_col].iloc[-1])
                    
                    # Calculate absolute USD Price trajectory
                    pred_plot = last_known_price * np.exp(np.cumsum(pred_unscaled))
                    history_plot = raw_train_df[raw_close_col].values[-100:]
                    
                    if raw_val_df is not None and raw_close_col in raw_val_df.columns:
                        actual_val_plot = raw_val_df[raw_close_col].values[:horizon]
                    else:
                        val_log_returns = self.val_master_data[feat_name].values[:horizon] if feat_name in self.val_master_data.columns else np.zeros(horizon)
                        actual_val_plot = last_known_price * np.exp(np.cumsum(val_log_returns))
                        
                    y_label = f"Price (USD) - {asset_base.upper()}"
                    title = f"Validation Overlay: {asset_base.upper()} Price Projection"
                
                else:
                    pred_plot = pred_unscaled
                    actual_val_plot = self.val_master_data[feat_name].values[:horizon] if feat_name in self.val_master_data.columns else np.zeros(horizon)
                    history_plot = self.master_data[feat_name].values[-100:]
                    y_label = "Log Return"
                    title = f"Validation Target: {feat_name}"

                # Ensure 1D numpy arrays
                history_plot = np.array(history_plot).flatten()
                actual_val_plot = np.array(actual_val_plot).flatten()
                pred_plot = np.array(pred_plot).flatten()

                # Connect boundary seamlessly at index (N-1)
                last_hist_val = history_plot[-1]
                connected_actual = np.insert(actual_val_plot, 0, last_hist_val)
                connected_pred = np.insert(pred_plot, 0, last_hist_val)

                n_hist = len(history_plot)
                x_hist = np.arange(n_hist)
                
                x_val_actual = np.arange(n_hist - 1, n_hist - 1 + len(connected_actual))
                x_val_pred = np.arange(n_hist - 1, n_hist - 1 + len(connected_pred))
                n_act_len = min(len(x_val_actual), len(connected_actual))

                ax.plot(x_hist, history_plot, label='Training History', color='blue', linewidth=2)
                ax.plot(x_val_actual[:n_act_len], connected_actual[:n_act_len], label='Actual Validation Data', color='green', linewidth=2, marker='o', ms=3)
                ax.plot(x_val_pred, connected_pred, label=f'LSTM Forecast ({horizon}d)', color='red', linestyle='--', linewidth=2, marker='s', ms=3)

                ax.axvline(x=n_hist - 1, color='gold', linestyle=':')
                ax.axvspan(n_hist - 1, n_hist - 1 + len(connected_pred), color='yellow', alpha=0.1)

                ax.set_title(title, fontweight='bold')
                ax.set_ylabel(y_label)
                ax.legend(loc='upper left')
                ax.grid(True, alpha=0.3)

                plt.savefig(os.path.join(export_dir, f"val_overlay_{chromosome['id']}_{feat_name}.png"))
                plt.close()

        # --- GRAPH SET 2: TRUE FUTURE FORECAST ---
        logger.info(f"🔮 [PLOT] Generating True Future Forecast for {chromosome['id']}...")
        
        combined_df = pd.concat([self.master_data, self.val_master_data]) if self.val_master_data is not None else self.master_data
        X_full, y_full_scaled, _ = self._prepare_lstm_dataset(chromosome, data_source=combined_df)
        
        future_model, _ = self._build_and_train_lstm(chromosome, X_full, y_full_scaled)
        
        # 1. Autoregressive Loop over full historical sequence
        curr_future_win = X_full[-1:].copy()
        fut_preds_list = []

        for step in range(horizon):
            step_pred = future_model.predict(curr_future_win, verbose=0)
            fut_preds_list.append(step_pred[0])
            
            next_feature_row = curr_future_win[0, -1, :].copy()
            num_targets = step_pred.shape[1]
            next_feature_row[:num_targets] = step_pred[0]
            
            new_win = np.append(curr_future_win[0, 1:, :], [next_feature_row], axis=0)
            curr_future_win = np.expand_dims(new_win, axis=0)

        fut_preds_matrix = np.array(fut_preds_list)

        raw_df = getattr(self, 'master_data_raw', combined_df)

        for f_idx, feat_name in enumerate(selected_features):
            if feat_name not in combined_df.columns:
                continue

            fig, ax = plt.subplots(figsize=(12, 4))
            master_col_idx = combined_df.columns.get_loc(feat_name)
            feat_min = self.scaler.min_[master_col_idx]
            feat_scale = self.scaler.scale_[master_col_idx]

            fut_pred_unscaled = (fut_preds_matrix[:, f_idx] - feat_min) / feat_scale

            asset_base = feat_name.replace('price_log_return_', '').replace('volume_log_change_', '')
            raw_close_col = f'close_{asset_base}'

            # Convert to absolute USD prices if raw price column is available
            if raw_close_col in raw_df.columns:
                last_known_price = float(raw_df[raw_close_col].iloc[-1])
                future_plot = last_known_price * np.exp(np.cumsum(fut_pred_unscaled))
                history_plot = raw_df[raw_close_col].values[-100:]
                y_label = f"Price (USD) - {asset_base.upper()}"
                title = f"True Future Projection: {asset_base.upper()} Price"
            else:
                future_plot = fut_pred_unscaled
                history_plot = combined_df[feat_name].values[-100:]
                y_label = "Log Return"
                title = f"True Future Projection: {feat_name}"

            history_plot = np.array(history_plot).flatten()
            future_plot = np.array(future_plot).flatten()

            # Dynamic X-axis alignment (no fixed 100 offset gap)
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

            plt.savefig(os.path.join(export_dir, f"future_forecast_{chromosome['id']}_{feat_name}.png"))
            plt.close()
    def export_trained_model(self, chromosome, X_data, y_data, export_dir="deployed_models", rank=None):
        """Exports fully trained Keras model, scaler, and metadata with bounded feature selection."""
        os.makedirs(export_dir, exist_ok=True)
        model_id = chromosome['id']
        prefix = f"rank_{rank}_" if rank is not None else "final_"
        
        logger.info(f"💾 [EXPORT] Finalizing weights training for export: {prefix}{model_id}...")
        
        import tensorflow as tf
        model, _ = self._build_and_train_lstm(chromosome, X_data, y_data)
        
        model_save_path = os.path.join(export_dir, f"{prefix}lstm_model.keras")
        model.save(model_save_path)
        tf.keras.backend.clear_session()
        
        with open(os.path.join(export_dir, f"{prefix}scaler.pkl"), 'wb') as f:
            pickle.dump(self.scaler, f)
            
        _, asset_cols = self._split_features(self.master_data)
        
        # 🛡️ SAFE INDEXING: Zip feature mask with asset_cols to avoid IndexError
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
            "selected_features": selected_features
        }
        
        with open(os.path.join(export_dir, f"{prefix}metadata.json"), 'w') as f:
            json.dump(metadata, f, indent=4)

def terminate_all_engine_processes():
    """
    Scans Linux /proc directory for any instances of ga_lstm_optimizer.py 
    or orphan multiprocessing fold workers using only Python standard library.
    Issues SIGTERM followed by SIGKILL and reaps zombie processes cleanly.
    """
    current_pid = os.getpid()
    logger.warning("🚨 [TERMINATE] Initiating pure Linux /proc system process sweep...")

    target_keywords = ["ga_lstm_optimizer.py", "_parallel_fold_worker_udp"]
    found_pids = []

    try:
        for pid_str in os.listdir('/proc'):
            if not pid_str.isdigit():
                continue
            
            pid = int(pid_str)
            if pid == current_pid:
                continue

            try:
                cmdline_path = os.path.join('/proc', pid_str, 'cmdline')
                if os.path.exists(cmdline_path):
                    with open(cmdline_path, 'rb') as f:
                        cmdline = f.read().decode('utf-8', errors='ignore').replace('\x00', ' ')
                        
                    if any(kw in cmdline for kw in target_keywords):
                        found_pids.append((pid, cmdline[:60]))
            except (PermissionError, FileNotFoundError):
                continue
    except Exception as e:
        logger.error(f"❌ [/proc SCAN] Error reading system process table: {e}")

    if not found_pids:
        logger.info("✅ [TERMINATE] No active engine processes found running in background.")
        sys.exit(0)

    killed_count = 0
    for pid, cmd in found_pids:
        try:
            logger.info(f"🔨 [TERMINATE] Sending SIGKILL to PID {pid} -> {cmd}...")
            os.kill(pid, signal.SIGKILL)
            killed_count += 1
        except ProcessLookupError:
            pass

    logger.info(f"✅ [TERMINATE] Sweep complete. Terminated {killed_count} engine processes.")
    sys.exit(0)


# ==============================================================================
# 🚀 CLI ENTRY POINT & MULTIPROCESSING INITIALIZATION
# ==============================================================================
def main():
    mp.set_start_method('spawn', force=True)
    parser = argparse.ArgumentParser(description="Parallel Multi-Process GA-LSTM Optimizer Engine")
    parser.add_argument("-v", "--verbose", action="store_true", help="Show detailed feature list and per-fold diagnostic audit panels")
    parser.add_argument("-action", type=str, help="Action to perform: clear-state | terminate | start")
    args = parser.parse_args()

    if args.action == "terminate":
        terminate_all_engine_processes()

    optimizer = LSTMOptimizerEngine(verbose=args.verbose)
    if args.action == "clear-state":
        optimizer._clear_state()
    else:
        optimizer.execute_pipeline()


if __name__ == "__main__":
    main()