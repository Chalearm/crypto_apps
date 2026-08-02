#!/usr/bin/env python3
# ##############################################################################
# File Name        : ga_master.py
# File Path        : apps/school/ga_master.py
#
# Author           : Chalearm Saelim & Gemini
# Owner            : Chalearm Saelim
# Reviewer         : Chalearm Saelim
#
# Version          : 1.2.0
# Status           : Development / Production-Ready
# Created Date     : 2026-07-26 08:00:00 (UTC+7)
# Modified Date    : 2026-07-31 15:30:00 (UTC+7)
#
# Description      :
#    Central orchestrator and Genetic Algorithm (GA) Master Daemon for the
#    distributed GA-LSTM forecasting system. Manages population seeding, NSGA-II
#    multi-objective evaluation, Redis task queuing, worker pool management,
#    intra-generation checkpointing (time/percentage triggers), configurable log
#    rotation daemons, and cluster control actions.
#
# DEPENDENCY TREE & STRUCTURAL MAP:
# ───────────────────────────────────────────────────────────────────────────
# [ga_master.py] (Distributed GA Master Engine)
#      │
#      ├── Imports Internal Module ──> [log_rotator.py] (Log Daemon Manager)
#      ├── Imports Internal Module ──> [utils.py] (resolve_target_directories)
#      ├── Imports Celery Router   ──> [celery_tasks.py] (run_fold_training_task, export_and_plot_task)
#      │
#      ├── Ingests Data Matrices  ──> Transformed CSV & Raw Prices dataset files
#      ├── Manages Redis Broker   ──> Dispatches sliding window fold training tasks
#      ├── Atomic Checkpoint Sync ──> Dual writes lstm_ga_checkpoint.json to root & run dirs
#      └── Evaluates NSGA-II      ──> Multi-objective Pareto frontiers & Skill DA metrics
#
# FUNCTION DEPENDENCY MATRIX (Internal Methods):
# ───────────────────────────────────────────────────────────────────────────
# main()
#  ├── print_cluster_status()
#  ├── stop_master_process()
#  ├── terminate_all_cluster_processes()
#  └── LSTMOptimizerEngine()
#        ├── execute_pipeline()
#        │     ├── _ingest_data_layers()
#        │     ├── _load_checkpoint()
#        │     ├── _initialize_random_population()
#        │     ├── _process_data()
#        │     ├── _evaluate_population_pipelined()
#        │     │     ├── _build_fold_payloads()
#        │     │     ├── refill_redis_pipeline()
#        │     │     ├── _summarize_chromosome_evaluation()
#        │     │     └── _save_checkpoint()
#        │     └── _save_checkpoint()
#        └── _save_checkpoint()
#
# Responsibilities :
#    - Manages distributed Redis execution queues and worker pool telemetry.
#    - Performs sliding-window dataset slicing and feature mask splitting.
#    - Executes NSGA-II multi-objective selection and speculative offspring breeding.
#    - Atomically persists checkpoint JSON files to prevent corruption during worker cycles.
#    - Offloads Pareto graph visualization and model deployment artifact tasks.
#
# Usage :
#    Directory : apps/school/
#    Run       : python3 ga_master.py -num=5 -save-min=35 -save-pct=25.0 -buffer-size=120
#
# Dependencies :
#    Internal  : log_rotator, utils, celery_tasks
#    External  : numpy, pandas, redis, celery, scikit-learn
#
# Change History :
#    -------------------------------------------------------------------------
#    Version | Date Time (UTC+7)         | Author          | Description
#    -------------------------------------------------------------------------
#    1.0.0   | 2026-07-26 08:00:00 (UTC+7) | Chalearm Saelim | Initial release
#    1.0.2   | 2026-07-30 15:30:00 (UTC+7) | Chalearm Saelim | Added asynchronous buffer limit
#    1.1.0   | 2026-07-31 15:30:00 (UTC+7) | Chalearm Saelim | Fixed dynamic generation tracking bug
#    1.2.0   | 2026-07-31 15:30:00 (UTC+7) | Chalearm Saelim | Full documentation & verbose telemetry logging
#    -------------------------------------------------------------------------
# ##############################################################################

import os
import sys
import json
import glob
import time
import uuid
import signal
import random
import logging
import argparse
import subprocess
import datetime
import warnings
import numpy as np
import pandas as pd
from log_rotator import start_log_rotation_daemon
from sklearn.preprocessing import MinMaxScaler
from utils import resolve_target_directories

# Suppress harmless Celery control inspection warnings
warnings.filterwarnings("ignore", category=UserWarning)

# Shared Log Formatter
log_formatter = logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')

# Default Engine Logger for STDOUT
logger = logging.getLogger("GA-LSTM-Master")
logger.setLevel(logging.INFO)
logger.propagate = False

if not logger.handlers:
    c_handler = logging.StreamHandler(sys.stdout)
    c_handler.setFormatter(log_formatter)
    logger.addHandler(c_handler)

summary_logger = logging.getLogger("ChromosomeSummary")
summary_logger.setLevel(logging.INFO)
summary_logger.propagate = False

fold_logger = logging.getLogger("FoldLifecycle")
fold_logger.setLevel(logging.INFO)
fold_logger.propagate = False

# Global Hyperparameter Constraints
RAW_DATA_DIR = "../../data_set/daily/2022_07_01_2026_06_30"
TRANSFORMED_DATA_DIR = "../../data_set/daily/2022_07_01_2026_06_30"
VAL_RAW_DATA_DIR = "../../data_set/daily/2026_07_01_2026_07_21"
VAL_TRANSFORMED_DATA_DIR = "../../data_set/daily/2026_07_01_2026_07_21"

POPULATION_SIZE = 4
GENERATIONS = 8
MUTATION_RATE = 0.25
TOP_N_EXPORTS = 5
NUM_FOLDS = 4

MIN_LOOKBACK_DAYS, MAX_LOOKBACK_DAYS = 80, 160
MIN_FORECAST_DAYS, MAX_FORECAST_DAYS = 30, 60
MIN_HIDDEN_LAYERS, MAX_HIDDEN_LAYERS = 1, 8
MIN_NODES_PER_LAYER, MAX_NODES_PER_LAYER = 32, 512

MIN_LR, MAX_LR = 0.0001, 0.01
MIN_DROPOUT, MAX_DROPOUT = 0.0, 0.6
BATCH_SIZE_CHOICES = [16, 32, 64, 128, 168]
USER_EXCLUDE_FEATURES = ['volume_log_change_fed']

CONFIG_FILE = "server_config.json"
PID_FILE = "ga_master.pid"

RUNTIME_CONFIG = {
    "save_min": 20,
    "save_pct": 25.0,
    "rotate_min": 30,
    "rotate_mb": 30.0,
}
# Global reference to active optimizer engine for live parameter updates
ACTIVE_OPTIMIZER = None

# Use 'redis' hostname when inside Docker network
redis_host = os.getenv("REDIS_HOST", "redis")
os.environ["REDIS_URL"] = f"redis://{redis_host}:6379/0"

# Guarantee Redis environment variables are explicitly set in Python runtime
redis_target = os.getenv("REDIS_URL") or os.getenv("CELERY_BROKER_URL") or "redis://redis:6379/0"
os.environ["REDIS_URL"] = redis_target
os.environ["CELERY_BROKER_URL"] = redis_target
os.environ["CELERY_RESULT_BACKEND"] = redis_target

from celery_tasks import run_fold_training_task, export_and_plot_task, app as celery_app

# Explicitly override Celery app configuration targets
celery_app.conf.update(
    broker_url=redis_target,
    result_backend=redis_target,
    result_backend_transport_options={'retry_policy': {'timeout': 5.0}}
)


# ==============================================================================
# MASTER GA-LSTM OPTIMIZER ENGINE CLASS
# ==============================================================================

class LSTMOptimizerEngine:
    # ##########################################################################
    # Function Name : __init__
    # Purpose       : Initializes LSTMOptimizerEngine, sets signal handlers,
    #                 and configures runtime save/checkpoint parameters.
    # ##########################################################################
    def __init__(self, data_directory=".", checkpoint_file="lstm_ga_checkpoint.json", verbose=False, save_interval_min=20, save_pct=25.0):
        self.data_directory = data_directory
        self.checkpoint_file = checkpoint_file
        self.app_dir = os.path.dirname(os.path.abspath(checkpoint_file)) or "."
        self.verbose = verbose
        self.save_interval_min = save_interval_min
        self.save_pct = save_pct
        self.running = True
        
        signal.signal(signal.SIGINT, self._handle_exit)
        signal.signal(signal.SIGTERM, self._handle_exit)
        
        self.run_id = None
        self.current_generation = 0
        self.chromosome_population = []
        self.scaler = MinMaxScaler(feature_range=(-1, 1))
        self.master_data = None
        self.master_data_raw = None
        self.val_master_data = None
        self.val_master_data_raw = None
        self._first_split_done = False
        
        logger.info(f"🚀 [INIT] LSTMOptimizerEngine Master initialized (Verbose: {self.verbose} | Save Min: {self.save_interval_min}m | Save Pct: {self.save_pct}%).")

    # ##########################################################################
    # Function Name : _setup_run_loggers
    # Purpose       : Creates file log handlers anchored to active run sub-directory.
    # ##########################################################################
    def _setup_run_loggers(self):
        log_dir, _, _ = resolve_target_directories(self.run_id)

        engine_log_file = os.path.join(log_dir, "lstm_engine.log")
        if not any(isinstance(h, logging.FileHandler) for h in logger.handlers):
            fh_engine = logging.FileHandler(engine_log_file)
            fh_engine.setFormatter(log_formatter)
            logger.addHandler(fh_engine)

        summary_log_file = os.path.join(log_dir, "chromosome_summary.log")
        if not summary_logger.handlers:
            fh_summary = logging.FileHandler(summary_log_file)
            fh_summary.setFormatter(log_formatter)
            summary_logger.addHandler(fh_summary)

        fold_log_file = os.path.join(log_dir, "folds_lifecycle.log")
        if not fold_logger.handlers:
            fh_fold = logging.FileHandler(fold_log_file)
            fh_fold.setFormatter(log_formatter)
            fold_logger.addHandler(fh_fold)

    # ##########################################################################
    # Function Name : _handle_exit
    # Purpose       : Intercepts interrupt signals (SIGINT/SIGTERM) and forces checkpointing.
    # ##########################################################################
    def _handle_exit(self, signum, frame):
        logger.warning(f"⚠️ [SIGNAL] Interrupt {signum} received. Persisting state before termination...")
        self.running = False
        self._save_checkpoint(partial=True)
        logger.info("👋 Master node exited safely.")
        sys.exit(0)

    # ##########################################################################
    # Function Name : _clear_state
    # Purpose       : Removes local state checkpoint file and flushes population array.
    # ##########################################################################
    def _clear_state(self):
        if os.path.exists(self.checkpoint_file):
            os.remove(self.checkpoint_file)
            logger.info("🧹 [CLEAR] State checkpoint file deleted successfully.")
        self.chromosome_population = []

    # ##########################################################################
    # Function Name : execute_pipeline
    #
    # Purpose :
    #    Master execution supervisor for the distributed GA-LSTM evolutionary sequence.
    #    Ingests multi-asset data matrices, fits feature scaling transformers,
    #    restores checkpoint state or seeds Generation 1 (with optional warm start),
    #    dispatches asynchronous multi-fold training tasks to Redis, and persists
    #    final state artifacts upon evolution completion.
    #
    # Inputs :
    #    max_generations
    #        Type        : int (Default: 50)
    #        Description : Target maximum generation limit for the evolutionary loop.
    #    target_buffer_limit
    #        Type        : int (Default: 25)
    #        Description : Maximum lookahead task buffer size maintained in Redis.
    #
    # Return :
    #    Type        : None
    #    Description : Supervises pipeline lifecycle to completion.
    #
    # Complexity :
    #    Time  : O(G * P * F) where G is generations, P is population size,
    #            and F is CV fold evaluation time.
    #    Space : O(D + P) for dataset matrices and chromosome population structures.
    #
    # Error Cases :
    #    - Early returns if dataset ingestion (_ingest_data_layers) fails or returns empty.
    # ##########################################################################
    def execute_pipeline(self, max_generations=50, target_buffer_limit=25):
        print("\n" + "🚀" * 40)
        print("🚀 [PIPELINE SUPERVISOR] Initiating Distributed GA Evolution Sequence")
        print(f"   ├── Target Max Generations : {max_generations} (Unbounded)")
        print(f"   ├── Redis Task Buffer Size : {target_buffer_limit} lookahead tasks")
        print(f"   └── Active Run Directory   : {os.path.abspath(self.app_dir)}")
        print("🚀" * 40 + "\n")

        logger.info(f"🚀 [PIPELINE] Starting Distributed GA Evolution sequence (Max Gen: {max_generations} | Buffer: {target_buffer_limit}).")

        # ----------------------------------------------------------------------
        # STEP 1: INGEST DATA MATRICES FIRST
        # ----------------------------------------------------------------------
        print("🔍 [PIPELINE STEP 1/4] Ingesting CSV dataset layers and raw price matrices...")
        if not self._ingest_data_layers():
            print("❌ [PIPELINE ERROR] Dataset ingestion failed or returned empty matrices. Halting pipeline execution.")
            logger.error("❌ [PIPELINE] Ingestion failed. Halting pipeline execution.")
            return
        print("✅ [PIPELINE STEP 1/4] Dataset layers ingested successfully.")

        # ----------------------------------------------------------------------
        # STEP 2: FIT SCALERS & PREPROCESS DATA BEFORE POPULATION INITIALIZATION
        # ----------------------------------------------------------------------
        print("\n🛠️ [PIPELINE STEP 2/4] Fitting MinMaxScaler (-1, 1) across master feature matrix...")
        self._process_data()
        print("✅ [PIPELINE STEP 2/4] Master data scaled and index alignment complete.")

        # ----------------------------------------------------------------------
        # STEP 3: RESTORE CHECKPOINT OR INITIALIZE POPULATION G1
        # ----------------------------------------------------------------------
        print("\n💾 [PIPELINE STEP 3/4] Inspecting checkpoint state...")
        if not self._load_checkpoint():
            use_warm = getattr(self, "use_warm_start", False)
            print(f"🌱 [PIPELINE STEP 3/4] No active checkpoint restored. Seeding Generation 1 (Warm Start: {use_warm})...")
            logger.info(f"🌱 [PIPELINE] No active checkpoint detected. Seeding fresh Population G1 (Warm Start: {use_warm}).")
            self._initialize_random_population(use_warm_start=use_warm)
        else:
            print(f"♻️ [PIPELINE STEP 3/4] Checkpoint restored cleanly. Resuming Run ID: {self.run_id} at Generation {self.current_generation + 1}/{max_generations}")

        # Ensure max_generations is explicitly registered on active instance
        self.max_generations = max_generations

        # ----------------------------------------------------------------------
        # STEP 4: LAUNCH ASYNCHRONOUS REDIS PIPELINE LOOP
        # ----------------------------------------------------------------------
        print("\n🔥 [PIPELINE STEP 4/4] Launching Pipelined Multi-Fold Redis Execution Engine...")
        logger.info(f"🔥 [PIPELINE] Launching pipelined evaluation for {len(self.chromosome_population)} models...")

        start_time = time.time()
        self._evaluate_population_pipelined(
            self.chromosome_population, 
            max_generations=max_generations, 
            target_buffer_limit=target_buffer_limit
        )
        duration = time.time() - start_time

        # Final checkpoint flush upon completion
        self._save_checkpoint()

        print("\n" + "🏁" * 40)
        print(f"🏁 [PIPELINE COMPLETE] Evolution pipeline finished all generational runs!")
        print(f"   ├── Total Execution Time : {duration:.2f} seconds ({duration/60.0:.2f} minutes)")
        print(f"   ├── Active Run ID        : {self.run_id}")
        print(f"   └── Final Checkpoint     : {os.path.join(self.app_dir, self.checkpoint_file)}")
        print("🏁" * 40 + "\n")

        logger.info(f"🏁 [PIPELINE] Evolution pipeline finished all generational runs in {duration:.2f}s.")

    # ##########################################################################
    # Function Name : _load_directory_to_df
    # Purpose       : Reads transformed CSV datasets and aligns raw price closing data.
    # ##########################################################################
    def _load_directory_to_df(self, transform_dir, raw_dir):
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

                filename = os.path.basename(f)
                asset_name = filename.split('_')[0].lower()
                
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
                df = df.add_suffix(f'_{asset_name}')

                if master_df is None:
                    master_df = df
                else:
                    master_df = master_df.join(df, how='outer')

            except Exception as e:
                logger.error(f"❌ [INGEST] Failed to parse file {f}: {e}")

        if master_df is not None:
            final_df = pd.concat([master_df, global_time_df], axis=1)
            price_cols = [c for c in final_df.columns if c.startswith('close_')]
            if price_cols:
                final_df[price_cols] = final_df[price_cols].ffill().bfill()
            
            final_df = final_df.interpolate(method='linear').bfill().ffill().fillna(0)
            return final_df.dropna()
        
        return None

    # ##########################################################################
    # Function Name : _ingest_data_layers
    # Purpose       : Loads training and validation datasets, performing feature audits.
    # ##########################################################################
    def _ingest_data_layers(self) -> bool:
        logger.info("=" * 75)
        logger.info("🔍 [DATASET INGESTION & FEATURE AUDIT]")
        logger.info("=" * 75)

        train_transform_files = glob.glob(os.path.join(TRANSFORMED_DATA_DIR, "*_transformed.csv"))
        logger.info(f"📂 [TRAIN SET] Found {len(train_transform_files)} transformed CSV files in '{TRANSFORMED_DATA_DIR}'")
        self.master_data = self._load_directory_to_df(TRANSFORMED_DATA_DIR, RAW_DATA_DIR)

        val_transform_files = glob.glob(os.path.join(VAL_TRANSFORMED_DATA_DIR, "*_transformed.csv"))
        logger.info(f"📂 [VAL SET]   Found {len(val_transform_files)} transformed CSV files in '{VAL_TRANSFORMED_DATA_DIR}'")
        self.val_master_data = self._load_directory_to_df(VAL_TRANSFORMED_DATA_DIR, VAL_RAW_DATA_DIR)

        if self.master_data is None or self.master_data.empty:
            logger.error("❌ [INGEST] Master training dataset is empty or failed to parse!")
            return False

        time_cols, asset_cols = self._split_features(self.master_data)
        banned_lower = [banned.lower() for banned in USER_EXCLUDE_FEATURES]
        excluded_cols = [c for c in self.master_data.columns if c.lower() in banned_lower]

        train_start = self.master_data.index.min().strftime('%Y-%m-%d')
        train_end   = self.master_data.index.max().strftime('%Y-%m-%d')

        logger.info("-" * 75)
        logger.info("📊 [TRAINING DATASET MATRIX SUMMARY]")
        logger.info(f"   ├── Total Row Samples : {self.master_data.shape[0]} days ({train_start} ➔ {train_end})")
        logger.info(f"   ├── Raw Features Count: {self.master_data.shape[1]} columns")
        logger.info(f"   ├── Global Time Feats : {len(time_cols)} channels {time_cols}")
        logger.info(f"   ├── Excluded Features : {len(excluded_cols)} channels {excluded_cols if excluded_cols else ['None']}")
        logger.info(f"   └── Active GA Pool    : {len(asset_cols)} channels")

        if self.val_master_data is not None and not self.val_master_data.empty:
            val_start = self.val_master_data.index.min().strftime('%Y-%m-%d')
            val_end   = self.val_master_data.index.max().strftime('%Y-%m-%d')
            val_time_cols, val_asset_cols = self._split_features(self.val_master_data)

            logger.info("-" * 75)
            logger.info("📊 [VALIDATION DATASET MATRIX SUMMARY]")
            logger.info(f"   ├── Total Row Samples : {self.val_master_data.shape[0]} days ({val_start} ➔ {val_end})")
            logger.info(f"   ├── Raw Features Count: {self.val_master_data.shape[1]} columns")
            logger.info(f"   ├── Global Time Feats : {len(val_time_cols)} channels")
            logger.info(f"   └── Active GA Pool    : {len(val_asset_cols)} channels")
        else:
            logger.warning("⚠️ [VAL SET] No validation dataset detected!")

        logger.info("=" * 75 + "\n")
        return True

    # ##########################################################################
    # Function Name : _process_data
    # Purpose       : Fits MinMaxScaler on master_data and scales features.
    # ##########################################################################
    def _process_data(self):
        if self.master_data is None or self.master_data.empty:
            return

        logger.info(f"🛠️ [PROCESS] Scaling master features using MinMaxScaler (-1, 1)...")
        self.master_data_raw = self.master_data.copy()
        self.master_data = pd.DataFrame(
            self.scaler.fit_transform(self.master_data),
            columns=self.master_data.columns,
            index=self.master_data.index
        )
        logger.info("✅ [PROCESS] Data scaling complete.")

    # ##########################################################################
    # Function Name : _split_features
    #
    # Purpose :
    #    Classifies raw DataFrame feature columns into global temporal channels
    #    (cyclical sine/cosine/fourier signals) and active asset evolutionary pools,
    #    filtering out user-banned/excluded feature vectors.
    #
    # Inputs :
    #    df
    #        Type        : pd.DataFrame
    #        Description : Input dataset DataFrame containing raw/scaled features.
    #
    # Return :
    #    Type        : tuple (list, list)
    #    Description : Tuple containing (time_cols, asset_cols). Returns ([], [])
    #                  if df is None or empty.
    #
    # Complexity :
    #    Time  : O(C) where C is the total number of columns in DataFrame.
    #    Space : O(C) for storing column name lists.
    #
    # Error Cases :
    #    - Safely returns empty lists `([], [])` if df is None or empty without crashing.
    # ##########################################################################
    def _split_features(self, df: pd.DataFrame):
        # Safety Guard: Ensure DataFrame is instantiated before inspecting columns
        if df is None or df.empty:
            print("⚠️ [FEATURE SPLIT WARN] DataFrame is None or empty. Ingestion must precede feature splitting.")
            logger.warning("⚠️ [FEATURE SPLIT WARN] Target DataFrame is None or empty. Returning empty feature lists.")
            return [], []

        temporal_patterns = [
            'day_wk_sin', 'day_wk_cos', 
            'day_yr_sin', 'day_yr_cos', 
            'hour_sin', 'hour_cos', 
            'min_sin', 'min_cos', 
            'fourier_'
        ]
        
        time_cols = [c for c in df.columns if any(p in c for p in temporal_patterns)]
        base_asset_cols = [c for c in df.columns if c not in time_cols and not c.startswith('close_')]

        banned_lower = [banned.lower() for banned in USER_EXCLUDE_FEATURES]
        excluded_cols = [c for c in base_asset_cols if c.lower() in banned_lower]
        asset_cols = [c for c in base_asset_cols if c.lower() not in banned_lower]

        if getattr(self, '_first_split_done', False) is False:
            print("\n" + "📐" * 40)
            print("📐 [FEATURE CLASSIFICATION & DATASET ARCHITECTURE AUDIT]")
            print("📐" * 40)
            print(f"   ├── Total Raw Columns         : {len(df.columns)}")
            print(f"   ├── ⏳ Global Temporal Feats  : ({len(time_cols)}) ➔ {time_cols}")
            print(f"   ├── 🚫 User Banned Features   : ({len(excluded_cols)}) ➔ {excluded_cols if excluded_cols else ['None']}")
            print(f"   └── 🧬 Active GA Pool         : ({len(asset_cols)}) ➔ {asset_cols}")
            print("📐" * 40 + "\n")

            logger.info("=" * 80)
            logger.info("📐 [FEATURE CLASSIFICATION & DATASET ARCHITECTURE]")
            logger.info("=" * 80)
            logger.info(f"⏳ Global Time Features ({len(time_cols)}) : {time_cols}")
            logger.info(f"🚫 User Excluded Features ({len(excluded_cols)}) : {excluded_cols if excluded_cols else ['None']}")
            logger.info(f"🧬 GA Evolutionary Pool ({len(asset_cols)}) : {asset_cols}")
            logger.info("=" * 80)
            
            self._first_split_done = True

        return time_cols, asset_cols

    # ##########################################################################
    # Function Name : _prepare_lstm_dataset
    # Purpose       : Generates sliding-window tensors based on chromosome genes.
    # ##########################################################################
    def _prepare_lstm_dataset(self, chromosome, data_source=None):
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
        forecast = int(chromosome.get('forecast_horizon', 52))

        num_samples = len(combined_data) - lookback - forecast
        if num_samples > 0:
            X, y = [], []
            for i in range(num_samples):
                X.append(combined_data[i : (i + lookback)])
                y.append(asset_values[i + lookback : i + lookback + forecast])
            return np.array(X, dtype=np.float32), np.array(y, dtype=np.float32), chromosome['feature_mask']
        
        return np.array([]), np.array([]), chromosome['feature_mask']

    # ##########################################################################
    # Function Name : _get_normal_random
    # Purpose       : Generates Gaussian bounded integer for node/layer allocation.
    # ##########################################################################
    def _get_normal_random(self, min_val, max_val):
        mu = (min_val + max_val) / 2
        sigma = (max_val - min_val) / 6
        val = random.gauss(mu, sigma)
        return int(max(min_val, min(max_val, val)))
    # ##########################################################################
    # Function Name : _seed_warm_start_candidates
    #
    # Purpose :
    #    Scans deployed_models/ subdirectories for the most recent valid JSON
    #    checkpoint, extracts the top-performing Pareto models, resets their
    #    evaluation status, and formats them for Generation 1 warm-start seeding.
    #
    # Inputs :
    #    None
    #
    # Return :
    #    Type        : list
    #    Description : Array of inherited chromosome candidate dictionaries.
    #
    # Complexity :
    #    Time  : O(N log N) file sorting + O(K) candidate extraction.
    #    Space : O(K) where K is the number of warm-start seeds.
    # ##########################################################################
    def _seed_warm_start_candidates(self) -> list:
        app_root = "/workspace/crypto_apps/dexbot/apps/school"
        deployed_dir = os.path.join(app_root, "deployed_models")
        
        if not os.path.exists(deployed_dir):
            logger.info("ℹ️ [WARM-START] No deployed_models/ directory found. Skipping warm-start.")
            return []

        run_checkpoints = glob.glob(os.path.join(deployed_dir, "*/checkpoint.json"))
        if not run_checkpoints:
            logger.info("ℹ️ [WARM-START] No prior checkpoints located. Skipping warm-start.")
            return []

        # Sort by modification time descending to select latest run checkpoint
        run_checkpoints.sort(key=os.path.getmtime, reverse=True)
        latest_ckpt_file = run_checkpoints[0]

        try:
            with open(latest_ckpt_file, 'r', encoding='utf-8') as f:
                data = json.load(f)

            pop = data.get("chromosome_population", []) or data.get("chromosomes", [])
            evaluated = [c for c in pop if c.get("fitness_evaluated")]
            
            if not evaluated:
                logger.info(f"ℹ️ [WARM-START] Checkpoint {latest_ckpt_file} contains no evaluated models.")
                return []

            # Sort using Pareto tie-breaker
            evaluated.sort(key=self._apply_priority_tie_breaker)
            top_seeds = evaluated[:min(5, len(evaluated))]

            print("\n" + "🔥" * 40)
            print(f"🔥 [WARM-START SEEDING] Ingested {len(top_seeds)} elite candidates from prior run checkpoint:")
            print(f"   └── Source Checkpoint: {latest_ckpt_file}")
            
            w_seeds = []
            for idx, seed in enumerate(top_seeds):
                child = dict(seed)
                child["id"] = f"G1-M{idx}"
                # Reset evaluation status for the new dataset execution period
                child["fitness_evaluated"] = False
                child["perf_vector"] = [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
                w_seeds.append(child)
                print(f"   │    [Seed {idx+1}] Inherited Architecture ID: {seed.get('id')} ➔ Re-tagged as G1-M{idx}")
            
            print("🔥" * 40 + "\n")
            return w_seeds

        except Exception as e:
            logger.warning(f"⚠️ [WARM-START WARN] Failed reading candidate seeds from {latest_ckpt_file}: {e}")
            return []
            
    # ##########################################################################
    # Function Name : _initialize_random_population
    #
    # Purpose :
    #    Initializes Population G1 for a new evolutionary run. Generates a unique 
    #    Run ID, configures target logging directories, and populates Generation 1.
    #    If `use_warm_start` is enabled, inherits top Pareto candidate architectures 
    #    from prior run checkpoints before filling remaining slots with randomized 
    #    hyperparameter matrices up to POPULATION_SIZE.
    #
    # Inputs :
    #    use_warm_start
    #        Type        : bool (Default: False)
    #        Description : Flag enabling warm-start candidate inheritance from disk.
    #
    # Return :
    #    Type        : list
    #    Description : Completed Generation 1 chromosome population array.
    #
    # Complexity :
    #    Time  : O(P * L) where P is POPULATION_SIZE and L is layer count.
    #    Space : O(P) for storing chromosome dictionary structures in memory.
    #
    # Error Cases :
    #    - Falls back to 100% random seeding if candidate extraction fails or
    #      deployed_models/ checkpoints are missing/corrupt.
    # ##########################################################################
    def _initialize_random_population(self, use_warm_start=False):
        self.run_id = uuid.uuid4().hex[:8].upper()
        self._setup_run_loggers()

        log_dir, export_dir, plot_dir = resolve_target_directories(self.run_id)

        print("\n" + "🎲" * 40)
        print(f"🎲 [NEW RUN INITIALIZATION] Hex Run ID: {self.run_id}")
        print(f"   ├── Log Directory   : {log_dir}")
        print(f"   ├── Model Exports   : {export_dir}")
        print(f"   ├── Plot Directory  : {plot_dir}")
        print(f"   └── Warm Start Flag : {use_warm_start}")
        print("🎲" * 40)

        logger.info(f"🎲 [NEW RUN] Generated Random Hex Run ID: {self.run_id}")
        logger.info(f"📂 [PATHS] Logs: {log_dir} | Models: {export_dir} | Plots: {plot_dir}")

        _, asset_cols = self._split_features(self.master_data)
        self.chromosome_population = []

        # ----------------------------------------------------------------------
        # 1. WARM-START SEEDING STEP
        # ----------------------------------------------------------------------
        if use_warm_start:
            print("\n🔥 [WARM-START] Attempting candidate seed ingestion from prior run checkpoints...")
            self.chromosome_population = self._seed_warm_start_candidates()

        existing_count = len(self.chromosome_population)
        needed_count = max(0, POPULATION_SIZE - existing_count)

        if existing_count > 0:
            print(f"🌱 [POPULATION SEEDING] Ingested {existing_count} warm-start candidate(s). Generating {needed_count} randomized model(s)...")
            logger.info(f"🌱 [INIT] Ingested {existing_count} warm-start seeds. Seeding {needed_count} random models...")
        else:
            print(f"🌱 [POPULATION SEEDING] Seeding {POPULATION_SIZE} randomized hyperparameter matrices for Generation 1...")
            logger.info(f"🌱 [INIT] Seeding randomized hyperparameter matrices for Generation 1...")

        max_rows = len(self.master_data)
        actual_max_lookback = max(MIN_LOOKBACK_DAYS + 1, min(MAX_LOOKBACK_DAYS, int(max_rows * 0.7)))
        actual_max_horizon = max(MIN_FORECAST_DAYS + 1, min(MAX_FORECAST_DAYS, int(max_rows * 0.7)))

        print(f"📐 [FEATURE & CONSTRAINTS MATRIX] Data Rows: {max_rows} | Asset Features: {len(asset_cols)}")
        print(f"   ├── Lookback Range : {MIN_LOOKBACK_DAYS} -> {actual_max_lookback} days")
        print(f"   └── Horizon Range  : {MIN_FORECAST_DAYS} -> {actual_max_horizon} days\n")

        # ----------------------------------------------------------------------
        # 2. RANDOM CHROMOSOME GENERATION STEP
        # ----------------------------------------------------------------------
        for i in range(needed_count):
            idx = existing_count + i
            lookback = random.randint(MIN_LOOKBACK_DAYS, actual_max_lookback)
            horizon = random.randint(MIN_FORECAST_DAYS, actual_max_horizon)
            num_layers = self._get_normal_random(MIN_HIDDEN_LAYERS, MAX_HIDDEN_LAYERS)
            num_features_to_select = random.randint(5, len(asset_cols))

            mask = [1] * num_features_to_select + [0] * (len(asset_cols) - num_features_to_select)
            random.shuffle(mask)

            nodes_list = [self._get_normal_random(MIN_NODES_PER_LAYER, MAX_NODES_PER_LAYER) for _ in range(num_layers)]
            lr_val = round(random.uniform(MIN_LR, MAX_LR), 5)
            dropout_val = round(random.uniform(MIN_DROPOUT, MAX_DROPOUT), 2)
            batch_val = random.choice(BATCH_SIZE_CHOICES)

            chromosome = {
                "id": f"G1-M{idx}",
                "lstm_layers": num_layers,
                "nodes_per_layer": nodes_list,
                "lookback_window": lookback,
                "forecast_horizon": horizon,
                "learning_rate": lr_val,
                "dropout_rate": dropout_val,
                "batch_size": batch_val,
                "feature_mask": mask,
                "fitness_evaluated": False,
                "perf_vector": [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
            }
            self.chromosome_population.append(chromosome)

            print(f"   ├── 🧬 [MODEL CREATED] ID: G1-M{idx:<2} | Layers: {num_layers} {nodes_list} | Lookback: {lookback}d | Horizon: {horizon}d | LR: {lr_val} | Dropout: {dropout_val} | Batch: {batch_val} | Active Features: {sum(mask)}/{len(asset_cols)}")

        print("\n" + "✅" * 40)
        print(f"✅ [INIT COMPLETE] Generation 1 seeding complete ({len(self.chromosome_population)} models total). Flushing checkpoint to disk...")
        print("✅" * 40 + "\n")

        logger.info(f"✅ [INIT] Population seeding complete. {len(self.chromosome_population)} models created for Run {self.run_id}.")
        self._save_checkpoint()
        return self.chromosome_population

    # ##########################################################################
    # Function Name : _build_fold_payloads
    # Purpose       : Creates cross-validation fold slices and caches compressed npz.
    # ##########################################################################
    def _build_fold_payloads(self, chromosome):
        X, y_all, _ = self._prepare_lstm_dataset(chromosome)
        total_samples = X.shape[0]

        lookback = chromosome['lookback_window']
        horizon = chromosome['forecast_horizon']

        _, all_asset_cols = self._split_features(self.master_data)
        target_cols = [col for col, m in zip(all_asset_cols, chromosome['feature_mask']) if m == 1]

        if not self.run_id:
            self.run_id = uuid.uuid4().hex[:8].upper()

        app_root = os.path.dirname(os.path.abspath(__file__))
        cache_dir = os.path.join(app_root, "logs", self.run_id, "cache")
        os.makedirs(cache_dir, exist_ok=True)
        cache_file = os.path.join(cache_dir, f"chrom_{chromosome['id']}.npz")

        if not os.path.exists(cache_file) and X.size > 0 and y_all.size > 0:
            try:
                np.savez_compressed(cache_file, X=X, y=y_all)
                if hasattr(os, 'sync'):
                    os.sync()
            except Exception as e:
                logger.error(f"💥 [CACHE ERROR] Failed creating tensor cache {cache_file}: {e}")

        val_size = max(5, min(horizon, total_samples // (NUM_FOLDS + 1)))
        fold_step = max(5, (total_samples - val_size) // NUM_FOLDS)

        payloads = []
        for fold in range(NUM_FOLDS):
            fold_idx = fold + 1
            train_start_idx = 0
            train_end_idx = fold_step * fold_idx
            val_start_idx = train_end_idx
            val_end_idx = min(total_samples, train_end_idx + val_size)

            if train_end_idx < 10 or (val_end_idx - val_start_idx) == 0:
                continue

            payloads.append({
                'run_id': self.run_id,
                'chrom_id': chromosome['id'],
                'fold_idx': fold_idx,
                'num_folds': NUM_FOLDS,
                'chromosome': chromosome,
                'horizon': horizon,
                'target_cols': target_cols,
                'cache_file': cache_file,
                'train_slice': (train_start_idx, train_end_idx),
                'val_slice': (val_start_idx, val_end_idx),
            })

        return payloads

    # ##########################################################################
    # Function Name : _verify_checkpoint_persistence
    # Purpose       : Reads JSON checkpoint from disk to verify model persistence.
    # ##########################################################################
    def _verify_checkpoint_persistence(self, chrom_id: str, gen: int) -> bool:
        checkpoint_path = os.path.join(self.app_dir, self.checkpoint_file)
        
        if not os.path.exists(checkpoint_path):
            logger.error(f"❌ [VERIFY FAILED] Checkpoint file missing on disk: {checkpoint_path}")
            return False

        try:
            with open(checkpoint_path, 'r', encoding='utf-8') as f:
                data = json.load(f)

            population = data.get("chromosome_population", []) or data.get("chromosomes", [])
            saved_chrom = next((c for c in population if c.get("id") == chrom_id), None)

            if not saved_chrom:
                logger.error(f"❌ [VERIFY FAILED] Model {chrom_id} NOT found inside {checkpoint_path}")
                return False

            evaluated = saved_chrom.get("fitness_evaluated", False)
            perf = saved_chrom.get("perf_vector", [])

            if evaluated and len(perf) in [4, 6] and perf[3] < 990.0:
                logger.info(
                    f"✅ [DISK VERIFIED] {chrom_id} confirmed on disk | "
                    f"Gen: {gen} | Objectives: [Skill DA: {perf[0]*100:+.2f}%, Sharpe: {perf[1]:.2f}, RMSE: {perf[3] if len(perf)==6 else perf[2]:.4f}]"
                )
                return True
            else:
                logger.warning(f"⚠️ [VERIFY WARNING] {chrom_id} found on disk but incomplete. Evaluated: {evaluated}, Perf len: {len(perf)}")
                return False

        except Exception as e:
            logger.error(f"💥 [VERIFY ERROR] Exception while verifying disk save for {chrom_id}: {e}")
            return False

    # ##########################################################################
    # Function Name : _generate_speculative_child
    # Purpose       : Breeds offspring from top evaluated Pareto candidates.
    # ##########################################################################
    def _generate_speculative_child(self, evaluated_pool, next_gen_idx, child_idx):
        if len(evaluated_pool) >= 2:
            evaluated_pool.sort(key=self._apply_priority_tie_breaker)
            parent_a = random.choice(evaluated_pool[:5])
            parent_b = random.choice(evaluated_pool[:5])
            
            child = {
                "id": f"G{next_gen_idx}-M{child_idx}",
                "lstm_layers": parent_a['lstm_layers'],
                "nodes_per_layer": list(parent_a['nodes_per_layer']),
                "lookback_window": parent_a['lookback_window'],
                "forecast_horizon": parent_a['forecast_horizon'],
                "learning_rate": parent_a['learning_rate'],
                "dropout_rate": parent_a['dropout_rate'],
                "batch_size": parent_a['batch_size'],
                "feature_mask": list(parent_b['feature_mask']),
                "fitness_evaluated": False,
                "perf_vector": [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
            }
            child = self._mutate(child)
        else:
            _, asset_cols = self._split_features(self.master_data)
            max_rows = len(self.master_data)
            num_layers = self._get_normal_random(MIN_HIDDEN_LAYERS, MAX_HIDDEN_LAYERS)
            num_features = random.randint(5, len(asset_cols))
            mask = [1] * num_features + [0] * (len(asset_cols) - num_features)
            random.shuffle(mask)

            child = {
                "id": f"G{next_gen_idx}-M{child_idx}",
                "lstm_layers": num_layers,
                "nodes_per_layer": [self._get_normal_random(MIN_NODES_PER_LAYER, MAX_NODES_PER_LAYER) for _ in range(num_layers)],
                "lookback_window": random.randint(MIN_LOOKBACK_DAYS, min(MAX_LOOKBACK_DAYS, int(max_rows * 0.7))),
                "forecast_horizon": random.randint(MIN_FORECAST_DAYS, min(MAX_FORECAST_DAYS, int(max_rows * 0.7))),
                "learning_rate": round(random.uniform(MIN_LR, MAX_LR), 5),
                "dropout_rate": round(random.uniform(MIN_DROPOUT, MAX_DROPOUT), 2),
                "batch_size": random.choice(BATCH_SIZE_CHOICES),
                "feature_mask": mask,
                "fitness_evaluated": False,
                "perf_vector": [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
            }

        child['fitness_evaluated'] = False
        child['perf_vector'] = [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
        return child

    # ##########################################################################
    # Function Name : _evaluate_population_pipelined
    # Purpose       : Asynchronous execution pipeline filling target Redis task
    #                 buffers and tracking live speculative generation expansion.
    # ##########################################################################
    def _evaluate_population_pipelined(self, initial_population, max_generations=37, target_buffer_limit=25):
        import redis
        import json
        from celery_tasks import run_fold_training_task, export_and_plot_task, app as celery_app

        redis_target = os.getenv("REDIS_URL") or os.getenv("CELERY_BROKER_URL") or "redis://redis:6379/0"
        
        try:
            redis_client = redis.Redis.from_url(redis_target)
            redis_client.ping()
            print(f"🔌 [REDIS CONNECTED] Successfully connected to Broker at {redis_target}")
        except Exception as e:
            print(f"⚠️ [REDIS WARNING] Failed connecting to primary target {redis_target}: {e}")
            try:
                redis_client = redis.Redis(host="redis", port=6379, db=0)
                redis_client.ping()
                print("🔌 [REDIS CONNECTED] Fallback connected to 'redis:6379'")
            except Exception:
                redis_client = redis.Redis(host="127.0.0.1", port=6379, db=0)

        active_redis_tasks = {}
        completed_results = {}
        expected_folds_count = {}
        evaluated_pool = []
        task_queue = []

        speculative_gen_counter = 1

        print("\n" + "🚀" * 40)
        print(f"🚀 [INIT POPULATION] Seeding Gen 1 into master array...")
        self.chromosome_population = list(initial_population)
        print(f"   ├── Population Size: {len(self.chromosome_population)}")
        print("🚀" * 40 + "\n")

        def enqueue_generation_suite(chromosomes, gen_idx):
            req_eval_target = len(evaluated_pool) + len(chromosomes)
            print(f"📥 [ENQUEUE SUITE] Adding Gen {gen_idx} ({len(chromosomes)} models) to task_queue...")
            
            for chromosome in chromosomes:
                c_id = chromosome['id']
                payloads = self._build_fold_payloads(chromosome)
                expected_folds_count[c_id] = len(payloads)
                completed_results[c_id] = []
                for p in payloads:
                    task_queue.append(("CV_FOLD", p, {'chrom_id': c_id, 'chrom_ref': chromosome, 'gen_idx': gen_idx}, None))
                    print(f"   ├── Enqueued FOLD task for model {c_id}")

            plot_payload = {"run_id": self.run_id, "gen_idx": gen_idx, "requires_eval_count": req_eval_target}
            gate_cond = {"min_evaluated_models": req_eval_target, "gen_idx": gen_idx}
            task_queue.append(("GEN_PLOT", plot_payload, {'gen_idx': gen_idx}, gate_cond))
            print(f"   └── Enqueued GEN_PLOT task for Gen {gen_idx} (Gated: awaits {req_eval_target} eval models)")

        def enqueue_final_post_ga_oos():
            total_expected = POPULATION_SIZE * max_generations
            oos_payload = {"run_id": self.run_id, "gen_idx": max_generations, "requires_eval_count": total_expected}
            gate_cond = {"min_evaluated_models": total_expected, "gen_idx": max_generations}
            task_queue.append(("POST_GA_OOS", oos_payload, {'gen_idx': max_generations}, gate_cond))
            print(f"📊 [ENQUEUE POST-GA OOS] Gated: awaits all {total_expected} models.")

        enqueue_generation_suite(initial_population, speculative_gen_counter)
        self._save_checkpoint(partial=True)

        def refill_redis_pipeline():
            nonlocal speculative_gen_counter
            
            queued_fold_tasks = [t for t in task_queue if t[0] == "CV_FOLD"]

            while len(queued_fold_tasks) == 0 and speculative_gen_counter < max_generations:
                speculative_gen_counter += 1
                print(f"\n🔮 [SPECULATIVE GENERATION {speculative_gen_counter}] Unlocking Gen {speculative_gen_counter}...")

                speculative_chroms = [
                    self._generate_speculative_child(evaluated_pool, speculative_gen_counter, i)
                    for i in range(POPULATION_SIZE)
                ]
                
                self.chromosome_population.extend(speculative_chroms)
                self._save_checkpoint(partial=True)

                enqueue_generation_suite(speculative_chroms, speculative_gen_counter)

                if speculative_gen_counter == max_generations:
                    enqueue_final_post_ga_oos()

                queued_fold_tasks = [t for t in task_queue if t[0] == "CV_FOLD"]

            while len(active_redis_tasks) < target_buffer_limit and len(task_queue) > 0:
                dispatched_any = False
                for idx in range(len(task_queue)):
                    task_type, payload, meta, gate = task_queue[idx]

                    if gate is not None and len(evaluated_pool) < gate["min_evaluated_models"]:
                        continue

                    task_type, payload, meta, gate = task_queue.pop(idx)
                    dispatched_any = True

                    if task_type == "CV_FOLD":
                        async_task = run_fold_training_task.delay(payload)
                        task_key = f"{meta['chrom_id']}_fold_{payload['fold_idx']}_{uuid.uuid4().hex[:4]}"
                        active_redis_tasks[task_key] = ("CV_FOLD", async_task, meta, payload)
                        print(f"🚀 [DISPATCH FOLD] Sent {meta['chrom_id']} to Redis | Celery ID: {async_task.id}")

                    elif task_type in ["GEN_PLOT", "POST_GA_OOS"]:
                        payload["top_chromosomes"] = sorted(evaluated_pool, key=self._apply_priority_tie_breaker)[:5]
                        payload["master_data"] = self.master_data_raw.to_json()
                        payload["val_data"] = self.val_master_data_raw.to_json() if self.val_master_data_raw is not None else None

                        async_task = export_and_plot_task.delay(payload)
                        task_key = f"{task_type}_gen_{meta['gen_idx']}_{uuid.uuid4().hex[:4]}"
                        active_redis_tasks[task_key] = (task_type, async_task, meta, payload)
                        print(f"🎨 [DISPATCH PLOT/OOS] Sent {task_type} Gen {meta['gen_idx']} to Redis | Celery ID: {async_task.id}")

                    break

                if not dispatched_any:
                    break

        refill_redis_pipeline()

        last_heartbeat = time.time()
        last_state_hash = ""

        while len(active_redis_tasks) > 0 or len(task_queue) > 0:
            if not self.running:
                print("🛑 [HALT] GA Engine stop signal received.")
                break

            current_active_keys = sorted(list(active_redis_tasks.keys()))
            current_state_hash = f"active:{len(active_redis_tasks)}_queue:{len(task_queue)}_keys:{','.join(current_active_keys)}"

            if current_state_hash != last_state_hash or (time.time() - last_heartbeat > 60.0):
                print(f"\n💓 [PIPELINE STATE] Active Celery Tasks: {len(active_redis_tasks)} | Queue: {len(task_queue)}")
                for k, v in list(active_redis_tasks.items()):
                    t_name = k.split('_')[0]
                    c_id = v[2].get('chrom_id', t_name)
                    print(f"   -> {k:<28} | Task Type: {v[0]:<12} | Target/Model: {c_id:<7} | Celery ID: {v[1].id}")
                last_state_hash = current_state_hash
                last_heartbeat = time.time()

            finished_keys = []

            for task_key, (t_type, async_task, meta, payload) in list(active_redis_tasks.items()):
                c_id = meta.get('chrom_id', 'UNKNOWN')
                task_id = async_task.id
                
                is_ready = False
                is_success = False
                res_payload = None

                if async_task.ready():
                    is_ready = True
                    if async_task.successful():
                        is_success = True
                        res_payload = async_task.result
                else:
                    try:
                        redis_key = f"celery-task-meta-{task_id}"
                        raw_meta = redis_client.get(redis_key)
                        if raw_meta:
                            parsed = json.loads(raw_meta.decode('utf-8'))
                            r_status = parsed.get("status")
                            if r_status == "SUCCESS":
                                is_ready = True
                                is_success = True
                                res_payload = parsed.get("result")
                            elif r_status in ["FAILURE", "REVOKED"]:
                                is_ready = True
                                is_success = False
                    except Exception:
                        pass

                if is_ready:
                    finished_keys.append(task_key)

                    if is_success and res_payload:
                        if t_type == "CV_FOLD":
                            completed_results[c_id].append(res_payload)

                            if len(completed_results[c_id]) == expected_folds_count.get(c_id, 1):
                                target_chrom = meta['chrom_ref']
                                objectives = self._summarize_chromosome_evaluation(target_chrom, completed_results[c_id], gen=meta['gen_idx'])
                                target_chrom['perf_vector'] = objectives
                                target_chrom['fitness_evaluated'] = True

                                evaluated_pool.append(target_chrom)

                                print("\n" + "═"*80)
                                print(f"🎯 [FITNESS EVALUATED] Model {c_id} completed all folds for Generation {meta['gen_idx']}!")
                                print(f"   ├── Skill Directional Accuracy : {objectives[0]*100:+.2f}%")
                                print(f"   ├── Annualized Sharpe Ratio    : {objectives[1]:.2f}")
                                print(f"   ├── Max Drawdown               : {objectives[2]*100:.2f}%")
                                print(f"   └── Validation RMSE            : {objectives[3]:.6f}")
                                print(f"📊 [GENERATION PROGRESS] Evaluated: {len(evaluated_pool)} / {POPULATION_SIZE * meta['gen_idx']} models")
                                print("═"*80 + "\n")

                                self._save_checkpoint(partial=True)

                        elif t_type in ["GEN_PLOT", "POST_GA_OOS"]:
                            print("\n" + "🎨"*40)
                            print(f"🎨 [PARETO STEP] Generation {meta['gen_idx']} plot task complete!")
                            print(f"   ├── Target Directory : /workspace/crypto_apps/dexbot/apps/school/prediction_result/{self.run_id}")
                            print(f"   └── Rendered Plots   : Pareto Frontiers & Price Predictions for BTC, UNI, SPY")
                            print("🎨"*40 + "\n")

                    else:
                        print(f"💥 [TASK CRASH] Task {task_key} failed or was revoked.")

            for key in finished_keys:
                del active_redis_tasks[key]

            refill_redis_pipeline()
            time.sleep(1.0)

    # ##########################################################################
    # Function Name : _summarize_chromosome_evaluation
    # Purpose       : Aggregates evaluation results across folds and prints stats.
    # ##########################################################################
    def _summarize_chromosome_evaluation(self, chromosome, fold_results, gen):
        fold_results.sort(key=lambda x: x.get('fold_idx', 0))

        fold_skill_da = [res['skill_da'] for res in fold_results if res.get('status') == 'success']
        fold_sharpe   = [res['sharpe'] for res in fold_results if res.get('status') == 'success']
        fold_maxdd    = [res['max_dd'] for res in fold_results if res.get('status') == 'success']
        fold_rmse     = [res['rmse'] for res in fold_results if res.get('status') == 'success']
        fold_cagr     = [res['cagr'] for res in fold_results if res.get('status') == 'success']
        fold_calmar   = [res['calmar'] for res in fold_results if res.get('status') == 'success']
        fold_pf       = [res['profit_factor'] for res in fold_results if res.get('status') == 'success']

        if not fold_skill_da:
            return [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]

        skill_da_mean = float(np.mean(fold_skill_da))
        skill_da_std  = float(np.std(fold_skill_da))
        sharpe_mean   = float(np.mean(fold_sharpe))
        sharpe_std    = float(np.std(fold_sharpe))
        maxdd_mean    = float(np.mean(fold_maxdd))
        rmse_mean     = float(np.mean(fold_rmse))

        objectives_vector = [skill_da_mean, sharpe_mean, maxdd_mean, rmse_mean, skill_da_std, sharpe_std]

        asset_history_metrics = {}
        for res in fold_results:
            for asset_name, metrics in res.get('asset_skills', {}).items():
                if asset_name not in asset_history_metrics:
                    asset_history_metrics[asset_name] = {'baseline': [], 'model': [], 'skill': []}
                asset_history_metrics[asset_name]['baseline'].append(metrics['baseline'])
                asset_history_metrics[asset_name]['model'].append(metrics['model'])
                asset_history_metrics[asset_name]['skill'].append(metrics['skill'])

        eval_headers = [
            "======================================================================",
            f"⚡ [EVALUATION COMPLETE] Model: {chromosome['id']} | Generation: {gen}",
            f"   🎯 Mean Skill DA        : {skill_da_mean * 100:+.2f}%  (std: {skill_da_std * 100:.2f}%)",
            f"   ⚡ Mean Sharpe Ratio    : {sharpe_mean:.2f}  (std: {sharpe_std:.2f})",
            f"   📉 Mean Max Drawdown    : {maxdd_mean * 100:.2f}%",
            f"   📐 Mean RMSE            : {rmse_mean:.4f}",
            "----------------------------------------------------------------------",
            "🎯 [SKILL DA MATRICES PROFILE BY ASSET]:"
        ]

        for line in eval_headers:
            logger.info(line)
            summary_logger.info(line)

        all_baselines, all_models, all_skills, asset_summary_lines = [], [], [], []
        for asset, metrics in asset_history_metrics.items():
            a_base  = float(np.mean(metrics['baseline']))
            a_model = float(np.mean(metrics['model']))
            a_skill = float(np.mean(metrics['skill']))

            all_baselines.append(a_base)
            all_models.append(a_model)
            all_skills.append(a_skill)
            asset_summary_lines.append((asset, a_base, a_model, a_skill))

        asset_summary_lines.sort(key=lambda x: x[3], reverse=True)

        for asset_name, b_val, m_val, s_val in asset_summary_lines:
            line = f"   {asset_name.ljust(22)} -> Baseline: {b_val*100:.1f}% | Model DA: {m_val*100:.2f}% | Skill DA: {s_val*100:+.2f}%"
            logger.info(line)
            summary_logger.info(line)

        stat_lines = [
            "----------------------------------------------------------------------",
            "📊 [SKILL SUMMARY STATISTICAL PROFILE]",
            f"   Best Asset Skill DA  : {max(all_skills) * 100:+.2f}%",
            f"   Worst Asset Skill DA : {min(all_skills) * 100:+.2f}%",
            f"   Average Skill DA     : {float(np.mean(all_skills)) * 100:+.2f}%",
            f"   Median Skill DA      : {float(np.median(all_skills)) * 100:+.2f}%",
            f"   Skill DA Std         : {skill_da_std * 100:.2f}%",
            "----------------------------------------------------------------------",
            "📈 [TRADING & RISK METRICS SUMMARY]",
            f"   Extended Metrics    -> Mean CAGR: {np.mean(fold_cagr)*100:.2f}% | Profit Factor: {np.mean(fold_pf):.2f} | Calmar: {np.mean(fold_calmar):.2f}",
            "======================================================================"
        ]

        for line in stat_lines:
            logger.info(line)
            summary_logger.info(line)

        return objectives_vector

    # ##########################################################################
    # Function Name : _mutate
    # Purpose       : Applies random mutations across hyperparameter genes.
    # ##########################################################################
    def _mutate(self, chromosome: dict) -> dict:
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

    # ##########################################################################
    # Function Name : _check_pareto_dominance
    # Purpose       : Evaluates Pareto dominance condition between candidate vectors.
    # ##########################################################################
    def _check_pareto_dominance(self, vector_a, vector_b):
        cond1 = (
            vector_a[0] >= vector_b[0] and
            vector_a[1] >= vector_b[1] and
            vector_a[2] <= vector_b[2] and
            vector_a[3] <= vector_b[3] and
            vector_a[4] <= vector_b[4] and
            vector_a[5] <= vector_b[5]
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

    # ##########################################################################
    # Function Name : _apply_priority_tie_breaker
    # Purpose       : Multi-objective sorting key for rank selection.
    # ##########################################################################
    def _apply_priority_tie_breaker(self, chromosome):
        v = chromosome['perf_vector']
        return (-v[0], -v[1], v[4], v[5], v[2], v[3])

    # ##########################################################################
    # Function Name : _save_checkpoint
    #
    # Purpose :
    #    Dynamically resolves active and target maximum generations, extracts
    #    evaluation progress metrics, and atomically serializes state checkpoints
    #    to JSON disk files across root and deployed model run sub-directories.
    #
    # Inputs :
    #    partial
    #        Type        : bool
    #        Description : Flag indicating mid-generation or asynchronous save trigger.
    #
    # Return :
    #    Type        : None
    #    Description : Saves JSON checkpoint payload files atomically.
    #
    # Complexity :
    #    Time  : O(N) where N is chromosome population size.
    #    Space : O(N) for JSON serialization payload buffer.
    #
    # Error Cases :
    #    - Catches and logs file system write, directory creation, or sync errors.
    # ##########################################################################
    def _save_checkpoint(self, partial=False):
        app_root = "/workspace/crypto_apps/dexbot/apps/school"
        run_id = getattr(self, "run_id", None) or getattr(self, "current_run_id", "DEFAULT_RUN")
        population = getattr(self, "chromosome_population", [])

        # ----------------------------------------------------------------------
        # 1. DYNAMIC GENERATION DERIVATION (UNBOUNDED)
        # Inspect model IDs (e.g., 'G2334-M3' -> Gen 2334) to resolve current generation
        # ----------------------------------------------------------------------
        max_active_gen = 1
        for chrom in population:
            c_id = chrom.get("id", "")
            if c_id.startswith("G") and "-" in c_id:
                try:
                    g_num = int(c_id.split("-")[0].replace("G", ""))
                    if g_num > max_active_gen:
                        max_active_gen = g_num
                except ValueError:
                    pass

        gen = max_active_gen

        # ----------------------------------------------------------------------
        # 2. DYNAMIC MAX GENERATIONS RESOLUTION (NO HARDCODED FALLBACKS)
        # ----------------------------------------------------------------------
        max_gen = getattr(self, "max_generations", None)
        if max_gen is None or max_gen < gen:
            max_gen = gen  # Unbounded dynamic ceiling

        eval_count = sum(1 for c in population if c.get("fitness_evaluated"))
        eval_pct = (eval_count / len(population) * 100.0) if len(population) > 0 else 0.0

        # ----------------------------------------------------------------------
        # 3. VERBOSE CONSOLE DIAGNOSTIC TELEMETRY
        # ----------------------------------------------------------------------
        print("\n" + "💾" * 40)
        print(f"💾 [SAVE CHECKPOINT] Run ID: {run_id} | Active Gen: {gen}/{max_gen} | Partial Trigger: {partial}")
        print(f"   ├── Total Population : {len(population)} models loaded in memory")
        print(f"   └── Evaluation Ratio : {eval_count} / {len(population)} evaluated ({eval_pct:.1f}%)")
        print("💾" * 40)

        root_ckpt_path = os.path.join(app_root, "lstm_ga_checkpoint.json")
        target_paths = {root_ckpt_path}

        if run_id and run_id != "DEFAULT_RUN":
            models_dir = os.path.join(app_root, "deployed_models", run_id)
            os.makedirs(models_dir, exist_ok=True)
            target_paths.add(os.path.join(models_dir, "checkpoint.json"))

        # ----------------------------------------------------------------------
        # 4. STRUCTURED CHECKPOINT PAYLOAD SERIALIZATION
        # ----------------------------------------------------------------------
        checkpoint_data = {
            "run_id": run_id,
            "generation": gen,                  # Dynamic active generation
            "current_generation": gen,          # Dual key for CLI compatibility
            "max_generations": max_gen,         # Dynamic configurable ceiling
            "evaluated_count": eval_count,
            "total_population": len(population),
            "chromosome_population": population,
            "chromosomes": population,          # Dual key for Go CLI inspector
            "timestamp": time.time()
        }

        # ----------------------------------------------------------------------
        # 5. ATOMIC DISK FLUSH (TEMP WRITE -> OS SYNC -> REPLACE)
        # ----------------------------------------------------------------------
        for path in target_paths:
            try:
                tmp_path = f"{path}.tmp"
                with open(tmp_path, "w") as f:
                    json.dump(checkpoint_data, f, indent=2)
                    f.flush()
                    os.fsync(f.fileno())
                os.replace(tmp_path, path)
                print(f"✅ [DISK SUCCESS] Written {len(population)} models (Gen {gen}/{max_gen}) ➔ {path}")
            except Exception as e:
                print(f"❌ [DISK ERROR] Failed writing checkpoint payload to {path}: {e}")
        
        print("")

    # ##########################################################################
    # Function Name : _load_checkpoint
    # Purpose       : Loads persisted checkpoint state from JSON disk files.
    # ##########################################################################
    def _load_checkpoint(self):
        if not os.path.exists(self.checkpoint_file):
            return False
        try:
            with open(self.checkpoint_file, 'r') as f:
                data = json.load(f)
            
            self.run_id = data.get("run_id", None)
            self.current_generation = data.get("generation", 0)
            self.chromosome_population = data.get("chromosome_population", []) or data.get("chromosomes", [])
            
            self._setup_run_loggers()
            log_dir, export_dir, plot_dir = resolve_target_directories(self.run_id)
            logger.info(f"♻️ [RESTORE] Resumed Run ID: {self.run_id or 'LEGACY'} at Generation {self.current_generation + 1}")
            return True
        except Exception:
            return False


# ==============================================================================
# CLI DAEMON CONTROLLER, ENCAPSULATED REDIS, & WORKER MANAGEMENT HELPERS
# ==============================================================================

def ensure_redis_server_running():
    """Validates Redis connection and enforces memory limits."""
    try:
        import redis
        client = redis.Redis.from_url(os.getenv("REDIS_URL", "redis://redis:6379/0"))
        client.ping()
        print("✅ [REDIS] Redis Broker started & validated successfully.")

        try:
            subprocess.run(["redis-cli", "-h", "redis", "CONFIG", "SET", "maxmemory", "4gb"], check=False, stdout=subprocess.DEVNULL)
            subprocess.run(["redis-cli", "-h", "redis", "CONFIG", "SET", "maxmemory-policy", "volatile-lru"], check=False, stdout=subprocess.DEVNULL)
            subprocess.run(["redis-cli", "-h", "redis", "CONFIG", "SET", "maxclients", "10000"], check=False, stdout=subprocess.DEVNULL)
            subprocess.run(["redis-cli", "-h", "redis", "CONFIG", "SET", "timeout", "0"], check=False, stdout=subprocess.DEVNULL)
            print("🔧 [REDIS] Configured limits: 4GB maxmemory | volatile-lru | 10000 maxclients.")
        except Exception as e:
            print(f"⚠️ [REDIS] Failed applying advanced configurations: {e}")

    except Exception:
        print("🚀 [REDIS] Auto-starting Redis Broker daemon (0.0.0.0:6379)...")
        os.system("redis-server --daemonize yes --bind 0.0.0.0")
        time.sleep(2.0)

def purge_redis_queues():
    try:
        subprocess.run(["redis-cli", "-h", "127.0.0.1", "flushall"], capture_output=True, text=True)
        print("🧹 [REDIS] Flushed all stale task queues from broker memory.")
    except Exception as e:
        print(f"⚠️ [REDIS] Could not flush Redis queues: {e}")

def stop_redis_server():
    try:
        subprocess.run(["redis-cli", "-h", "127.0.0.1", "shutdown"], capture_output=True, text=True)
        print("🧹 [REDIS] Redis server daemon stopped cleanly.")
    except Exception:
        pass

def get_running_master_pids():
    current_pid = os.getpid()
    running_pids = []
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
                    if "ga_master.py" in cmdline and not any(x in cmdline for x in ["-action=status", "-action=terminate", "-action=create-work"]):
                        running_pids.append((pid, cmdline.strip()))
            except (PermissionError, FileNotFoundError):
                continue
    except Exception:
        pass
    return running_pids

def write_config_and_pid(save_min, save_pct, rotate_min, rotate_mb):
    existing_pids = get_running_master_pids()
    if existing_pids:
        print(f"⚠️ [WARNING] Active master process already running (PIDs: {existing_pids}). Refusing to spawn duplicate daemon.")
        return False
        
    with open(PID_FILE, "w") as f:
        f.write(str(os.getpid()))
        
    config_data = {
        "save_min": save_min,
        "save_pct": save_pct,
        "rotate_min": rotate_min,
        "rotate_mb": rotate_mb,
    }
    with open(CONFIG_FILE, "w") as f:
        json.dump(config_data, f, indent=4)
    return True

def stop_master_process():
    pids = get_running_master_pids()
    if not pids:
        print("ℹ️ [STOP] No running GA Master daemon instance found.")
        return
    for pid, cmd in pids:
        print(f"🛑 [STOP] Sending SIGINT (Graceful Shutdown) to Master PID {pid}...")
        try:
            os.kill(pid, signal.SIGINT)
            print(f"✅ [STOP] Signal sent. Master process {pid} saving checkpoint and exiting.")
        except ProcessLookupError:
            print(f"⚠️ [STOP] Process {pid} not found.")

def terminate_all_cluster_processes():
    pids = get_running_master_pids()
    for pid, cmd in pids:
        print(f"🔨 [TERMINATE] Sending SIGKILL (-9) to Master PID {pid}...")
        try:
            os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            pass

    try:
        current_pid = os.getpid()
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
                    if "./school" in cmdline or "school -action" in cmdline:
                        print(f"🔨 [TERMINATE] Killing Go daemon wrapper PID {pid}...")
                        os.kill(pid, signal.SIGKILL)
            except (PermissionError, FileNotFoundError):
                continue
    except Exception:
        pass

    purge_redis_queues()
    stop_redis_server()
    print("✅ [TERMINATE] Cluster sweep complete. All processes and Redis server stopped cleanly.")

# ##############################################################################
# Function Name : print_cluster_status
#
# Purpose :
#    Queries local process locks, inspects the active JSON checkpoint state,
#    and interrogates the Redis Celery broker to render real-time worker pool
#    telemetry, active fold training assignments, and cluster metrics.
# ##############################################################################
def print_cluster_status():
    print("\n🔍 [CLI ACTION] Interrogating cluster state and worker pool telemetry...")
    print("=" * 75)
    print("📊 DISTRIBUTED GA-LSTM CLUSTER TELEMETRY & STATUS REPORT")
    print("=" * 75)

    # 1. Master Process Status
    master_pids = get_running_master_pids()
    if master_pids:
        print("🧠 [GA MASTER DAEMON]: 🟢 ONLINE / RUNNING")
        for pid, cmd in master_pids:
            print(f"   ├── Process PID : {pid}")
            print(f"   └── Command Line: {cmd[:65]}")
    else:
        print("🧠 [GA MASTER DAEMON]: 🔴 OFFLINE / STOPPED")

    # 2. Checkpoint State
    checkpoint_file = "/workspace/crypto_apps/dexbot/apps/school/lstm_ga_checkpoint.json"
    if not os.path.exists(checkpoint_file):
        checkpoint_file = "lstm_ga_checkpoint.json"

    if os.path.exists(checkpoint_file):
        try:
            with open(checkpoint_file, 'r', encoding='utf-8') as f:
                ckpt = json.load(f)

            run_id = ckpt.get("run_id", "UNKNOWN")
            pop = ckpt.get("chromosome_population", []) or ckpt.get("chromosomes", [])
            
            # Resolve dynamic active generation
            max_active_gen = ckpt.get("generation", 1)
            for chrom in pop:
                c_id = chrom.get("id", "")
                if c_id.startswith("G") and "-" in c_id:
                    try:
                        g_num = int(c_id.split("-")[0].replace("G", ""))
                        if g_num > max_active_gen:
                            max_active_gen = g_num
                    except ValueError:
                        pass

            max_gen_target = ckpt.get("max_generations", max_active_gen)
            if max_gen_target < max_active_gen:
                max_gen_target = max_active_gen

            eval_count = sum(1 for c in pop if c.get("fitness_evaluated"))
            eval_pct = (eval_count / len(pop) * 100.0) if pop else 0.0

            ts_raw = ckpt.get("timestamp", time.time())
            ts_str = datetime.datetime.fromtimestamp(ts_raw, datetime.timezone.utc).strftime('%Y-%m-%d %H:%M:%S UTC')

            print(f"\n💾 [CHECKPOINT STATE]: 🟢 VALID")
            print(f"   ├── Active Run ID : {run_id}")
            print(f"   ├── Current Gen   : Generation {max_active_gen}/{max_gen_target}")
            print(f"   ├── Chromosomes   : {len(pop)} total ({eval_count} evaluated | {eval_pct:.1f}%)")
            print(f"   └── Last Saved    : {ts_str} ({ts_raw})")
        except Exception as e:
            print(f"\n💾 [CHECKPOINT STATE]: ⚠️ CORRUPTED ({e})")
    else:
        print("\n💾 [CHECKPOINT STATE]: ⚪ NO CHECKPOINT FILE FOUND")

    # 3. Worker Pool Telemetry & Active Task Inspector
    print("\n🏋️ [REGISTERED WORKER POOL TELEMETRY]:")
    try:
        from celery_tasks import app
        inspect = app.control.inspect(timeout=3.0)

        registered = inspect.registered()
        active_tasks = inspect.active()
        stats = inspect.stats()

        if registered:
            print(f"   └── Total Connected Worker Nodes: {len(registered)}")
            for node_name in sorted(registered.keys()):
                node_stats = stats.get(node_name, {}) if stats else {}
                broker_transport = node_stats.get('broker', {}).get('transport', 'redis')
                pool_concurrency = node_stats.get('pool', {}).get('max-concurrency', 'N/A')

                active_list = active_tasks.get(node_name, []) if active_tasks else []
                active_count = len(active_list)
                status_icon = "🏋️ BUSY / TRAINING" if active_count > 0 else "🟢 IDLE / READY"

                print(f"\n   🖥️  Node Hostname   : {node_name}")
                print(f"      ├── Connection  : ONLINE ({status_icon})")
                print(f"      ├── Broker Type : {broker_transport.upper()}")
                print(f"      ├── Concurrency : {pool_concurrency} Workers")
                print(f"      └── Active Tasks: {active_count} executing")

                if active_count > 0:
                    for task in active_list:
                        t_name = task.get('name', 'tasks.run_fold_training_task')
                        args_payload = task.get('args', [{}])
                        p_data = args_payload[0] if isinstance(args_payload, list) and len(args_payload) > 0 else {}
                        
                        c_id = p_data.get('chrom_id', 'Model')
                        fold_idx = p_data.get('fold_idx', '?')
                        num_folds = p_data.get('num_folds', 5)
                        
                        print(f"         └── ⚡ Executing: {t_name:<28} | Target: {c_id:<7} (Fold {fold_idx}/{num_folds})")
        else:
            print("   ⚠️  No active Celery workers detected listening on Redis broker!")
    except Exception as e:
        print(f"   ❌ Error inspecting Redis worker pool: {e}")

    # 4. Historical Runs
    run_folders = glob.glob("logs/*/")
    print(f"\n📂 [HISTORICAL RUN SUB-DIRECTORIES]: {len(run_folders)} found")
    for folder in run_folders[:5]:
        r_hex = os.path.basename(os.path.normpath(folder))
        print(f"   └── Sub-Directory Run ID: {r_hex}")

    print("=" * 75 + "\n")

def load_config():
    global RUNTIME_CONFIG, ACTIVE_OPTIMIZER
    if os.path.exists(CONFIG_FILE):
        try:
            with open(CONFIG_FILE, "r") as f:
                new_cfg = json.load(f)
                RUNTIME_CONFIG.update(new_cfg)
                print(f"\n🔄 [GA MASTER] Reloaded Active Configuration: {RUNTIME_CONFIG}\n")

                if ACTIVE_OPTIMIZER is not None:
                    ACTIVE_OPTIMIZER.save_interval_min = RUNTIME_CONFIG.get("save_min", 20)
                    ACTIVE_OPTIMIZER.save_pct = RUNTIME_CONFIG.get("save_pct", 25.0)
                    print("✅ [GA MASTER] Applied updated save intervals to active pipeline.")
        except Exception as e:
            print(f"⚠️ [GA MASTER] Failed to load config: {e}")

def handle_reload_signal(signum, frame):
    print("\n📩 [SIGNAL RECEIVED] SIGHUP caught! Applying updated runtime parameters...")
    load_config()

# ==============================================================================
# MAIN ENTRY POINT & CLI PARSER
# ==============================================================================
# ##############################################################################
# Function Name : main
#
# Purpose :
#    Primary CLI entry point and process dispatcher for the Distributed GA-LSTM
#    Master Orchestrator Engine. Parses runtime arguments, configures Redis targets,
#    handles cluster lifecycle management actions (status, stop, terminate, set-up),
#    initializes log rotation daemons, and spawns the optimization engine with
#    unbounded generation and warm-start capabilities.
#
# Inputs :
#    None (Parses command-line arguments via sys.argv)
#
# Return :
#    Type        : None
#    Description : Dispatches requested actions or runs the optimization loop.
#
# Complexity :
#    Time  : O(1) for CLI parsing/dispatch; O(N * G) for pipeline startup loop.
#    Space : O(1) for CLI initialization state.
#
# Error Cases :
#    - Exits with code 1 if duplicate master PID process is detected on start.
#    - Exits with code 0 on status, stop, terminate, or set-up execution completion.
# ##############################################################################
def main():
    global ACTIVE_OPTIMIZER

    # --------------------------------------------------------------------------
    # 1. CLI ARGUMENT PARSER SETUP
    # --------------------------------------------------------------------------
    parser = argparse.ArgumentParser(description="Distributed GA-LSTM Master Orchestrator Engine")
    parser.add_argument("-v", "--verbose", action="store_true", help="Enable verbose diagnostic telemetry logging")
    parser.add_argument("-action", type=str, choices=["start", "update", "stop", "status", "terminate", "clear-state", "set-up", "create-work", "restart", "plot"], default="start", help="Cluster orchestration action trigger")
    parser.add_argument("-num", type=int, default=1, help="Population or worker count override")
    parser.add_argument("-gen", type=int, default=1, help="Target generation index for recovery plot action")
    parser.add_argument("-generations", type=int, default=50, help="Target maximum generations limit (Unbounded, e.g., 50, 100, 2334)")
    parser.add_argument("-save-min", type=int, default=20, help="Checkpoint interval threshold in minutes")
    parser.add_argument("-save-pct", type=float, default=25.0, help="Checkpoint interval threshold in % of population")
    parser.add_argument("-rotate-min", type=int, default=30, help="Log rotation time threshold in minutes")
    parser.add_argument("-rotate-mb", type=float, default=30.0, help="Log rotation size threshold in MB")
    parser.add_argument("-buffer-size", type=int, default=25, help="Maximum lookahead task buffer size in Redis (Default: 25)")
    parser.add_argument("-warm-start", type=str, choices=["true", "false", "True", "False"], default="false", help="Seed top candidate architectures from prior run checkpoint")
    args = parser.parse_args()

    use_warm_start = (args.warm_start.lower() == "true")

    # --------------------------------------------------------------------------
    # 2. ENVIRONMENT & REDIS BROKER CONFIGURATION
    # --------------------------------------------------------------------------
    os.environ["REDIS_URL"] = os.getenv("REDIS_URL", "redis://127.0.0.1:6379/0")

    print("\n" + "=" * 80)
    print(f"🧬 [GA MASTER DAEMON v1.2.0] Action: {args.action.upper()}")
    print("=" * 80)
    print(f"   ├── Target Generations : {args.generations} (Configurable / Unbounded)")
    print(f"   ├── Worker/Pop Count   : {args.num}")
    print(f"   ├── Redis Task Buffer  : {args.buffer_size} lookahead tasks")
    print(f"   ├── Warm Start Seeding : {use_warm_start}")
    print(f"   ├── Checkpoint Rules   : Interval={args.save_min}m | Ratio={args.save_pct}%")
    print(f"   └── Log Rotation Rules : Interval={args.rotate_min}m | Max Size={args.rotate_mb}MB")
    print("=" * 80 + "\n")

    # --------------------------------------------------------------------------
    # 3. DIRECT SYNCHRONOUS COMMAND DISPATCHING
    # --------------------------------------------------------------------------
    if args.action == "status":
        print("🔍 [CLI ACTION] Interrogating cluster state and worker pool telemetry...")
        print_cluster_status()
        sys.exit(0)

    elif args.action == "stop":
        print("🛑 [CLI ACTION] Initiating graceful shutdown sequence...")
        stop_master_process()
        if os.path.exists(PID_FILE):
            os.remove(PID_FILE)
            print(f"🧹 [PID CLEANUP] Removed PID lock file: {PID_FILE}")
        sys.exit(0)

    elif args.action == "terminate":
        print("🔨 [CLI ACTION] Initiating full cluster sweep and force-termination...")
        terminate_all_cluster_processes()
        if os.path.exists(PID_FILE):
            os.remove(PID_FILE)
            print(f"🧹 [PID CLEANUP] Removed PID lock file: {PID_FILE}")
        sys.exit(0)

    # Initialize log rotation daemon for operational pipeline modes
    if args.action in ["set-up", "create-work", "restart", "plot", "start"]:
        print(f"🔄 [LOG ROTATOR] Spawning background rotator (Interval: {args.rotate_min}m | Limit: {args.rotate_mb}MB)...")
        start_log_rotation_daemon(rotation_minutes=args.rotate_min, max_size_mb=args.rotate_mb)

    # Register SIGHUP reload signal handler for live parameter updates
    signal.signal(signal.SIGHUP, handle_reload_signal)

    if args.action == "set-up":
        print("🛠️ [SETUP ACTION] Validating Redis Broker & Network Infrastructure...")
        ensure_redis_server_running()
        purge_redis_queues()
        print("✅ [SETUP COMPLETE] Infrastructure validated cleanly.")
        sys.exit(0)

    # --------------------------------------------------------------------------
    # 4. GA MASTER PIPELINE INITIALIZATION & EXECUTION
    # --------------------------------------------------------------------------
    if args.action == "start":
        print("🔒 [PROCESS LOCK] Validating PID file lock state...")
        if not write_config_and_pid(args.save_min, args.save_pct, args.rotate_min, args.rotate_mb):
            print("❌ [PROCESS LOCK ERROR] Active instance detected. Aborting launch.")
            sys.exit(1)

        print("🚀 [PIPELINE INITIALIZATION] Instantiating LSTMOptimizerEngine...")
        ACTIVE_OPTIMIZER = LSTMOptimizerEngine(
            verbose=args.verbose,
            save_interval_min=args.save_min,
            save_pct=args.save_pct,
        )

        # Store parameters on engine instance
        ACTIVE_OPTIMIZER.max_generations = args.generations
        ACTIVE_OPTIMIZER.use_warm_start = use_warm_start

        # DO NOT CALL _initialize_random_population() HERE!
        # Let execute_pipeline() ingest data layers first, then seed the population cleanly.

        print(f"🏁 [PIPELINE START] Launching evolution loop up to Target Generation {args.generations}...")
        ACTIVE_OPTIMIZER.execute_pipeline(
            max_generations=args.generations, 
            target_buffer_limit=args.buffer_size
        )
if __name__ == "__main__":
    main()