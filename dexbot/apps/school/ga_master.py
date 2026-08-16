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
from logging.handlers import TimedRotatingFileHandler

# Suppress harmless Celery control inspection warnings
warnings.filterwarnings("ignore", category=UserWarning)

log_formatter = logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')

def _init_single_logger(logger_name: str) -> logging.Logger:
    lg = logging.getLogger(logger_name)
    lg.setLevel(logging.INFO)
    lg.propagate = False
    
    # 🛡️ Clear pre-existing handlers
    lg.handlers.clear()
    
    # 1. Console Handler (for docker logs & terminal stdout)
    c_handler = logging.StreamHandler(sys.stdout)
    c_handler.setLevel(logging.INFO)
    c_handler.setFormatter(log_formatter)
    lg.addHandler(c_handler)
    
    # 2. Time-Based File Rotation Handler (Rotates every 110 minutes, keeps 5 backups)
    log_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)), "logs")
    os.makedirs(log_dir, exist_ok=True)
    log_file = os.path.join(log_dir, f"{logger_name.lower()}_rotated.log")
    
    f_handler = TimedRotatingFileHandler(log_file, when='M', interval=110, backupCount=5)
    f_handler.setLevel(logging.INFO)
    f_handler.setFormatter(log_formatter)
    lg.addHandler(f_handler)
    
    return lg

# Instantiate clean, non-duplicating, auto-rotating loggers
logger = _init_single_logger("GAMaster")
summary_logger = _init_single_logger("ChromosomeSummary")
fold_logger = _init_single_logger("FoldLifecycle")


# Global Hyperparameter Constraints
RAW_DATA_DIR = "../../data_set/daily/2022_01_01_2026_06_30"
TRANSFORMED_DATA_DIR = "../../data_set/daily/2022_01_01_2026_06_30"
VAL_RAW_DATA_DIR = "../../data_set/daily/2026_07_01_2026_07_27"
VAL_TRANSFORMED_DATA_DIR = "../../data_set/daily/2026_07_01_2026_07_27"

POPULATION_SIZE = 43
GENERATIONS = 47
MUTATION_RATE = 0.14563
TOP_N_EXPORTS = 5
NUM_FOLDS = 5

# Add to global constraints section near MIN_LR / MAX_LR
MIN_EPOCHS, MAX_EPOCHS = 15, 150

#MIN_LOOKBACK_DAYS, MAX_LOOKBACK_DAYS = 12, 44
#MIN_FORECAST_DAYS, MAX_FORECAST_DAYS = 5, 15


MIN_LOOKBACK_DAYS, MAX_LOOKBACK_DAYS = 80, 170
MIN_FORECAST_DAYS, MAX_FORECAST_DAYS = 33, 60

#MIN_HIDDEN_LAYERS, MAX_HIDDEN_LAYERS = 1, 2
#MIN_NODES_PER_LAYER, MAX_NODES_PER_LAYER = 15, 45


MIN_HIDDEN_LAYERS, MAX_HIDDEN_LAYERS = 1, 8
MIN_NODES_PER_LAYER, MAX_NODES_PER_LAYER = 32, 432


MIN_LR, MAX_LR = 0.0001, 0.0025
MIN_DROPOUT, MAX_DROPOUT = 0.0, 0.5
#BATCH_SIZE_CHOICES = [12,17]
BATCH_SIZE_CHOICES = [16, 32, 64, 128, 168, 216,268]
USER_EXCLUDE_FEATURES = ['volume_log_change_fed', 'volume_raw_fed','volume_historical_volatility_fed']

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
    #    restores checkpoint state or seeds Generation 1, runs the startup audit
    #    to render any missing completed generation plots immediately upon resume, 
    #    and dispatches asynchronous multi-fold training tasks to Redis.
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

            # ------------------------------------------------------------------
            # CRITICAL FIX: EXPLICIT STARTUP AUDIT INVOCATION ON CHECKPOINT RESTORE
            # ------------------------------------------------------------------
            print("🔍 [STARTUP AUDIT] Scanning restored population for completed generations missing graph plots...")
            evaluated_pool_restored = [c for c in self.chromosome_population if c.get("fitness_evaluated", False)]
            if evaluated_pool_restored:
                gen1_models = [c for c in self.chromosome_population if str(c.get("id", "")).startswith("G1-")]
                restored_pop_per_gen = len(gen1_models) if len(gen1_models) > 0 else POPULATION_SIZE
                print(f"   ├── Restored Evaluated Pool Size : {len(evaluated_pool_restored)}")
                print(f"   └── Computed Pop Size Per Gen    : {restored_pop_per_gen}")
                self._audit_and_trigger_missing_plots(evaluated_pool_restored, custom_pop_size=restored_pop_per_gen)
            else:
                print("   └── ℹ️ No evaluated models found in restored checkpoint pool yet.")

        # Ensure max_generations is explicitly registered on active instance
        self.max_generations = max_generations

        # ----------------------------------------------------------------------
        # STEP 4: LAUNCH ASYNCHRONOUS REDIS PIPELINE LOOP
        # ----------------------------------------------------------------------
        print("\n🔥 [PIPELINE STEP 4/4] Launching Synchronous Generational Barrier Engine...")
        logger.info(f"🔥 [PIPELINE] Launching generational evaluation for {len(self.chromosome_population)} models...")

        start_time = time.time()
        self._evaluate_population_sync(
            self.chromosome_population, 
            max_generations=max_generations
        )
        duration = time.time() - start_time

        self._save_checkpoint()

        print("\n" + "🏁" * 40)
        print(f"🏁 [PIPELINE COMPLETE] Evolution pipeline finished all generational runs!")
        print(f"   ├── Total Execution Time : {duration:.2f} seconds ({duration/60.0:.2f} minutes)")
        print(f"   ├── Active Run ID        : {self.run_id}")
        print(f"   └── Final Checkpoint     : {os.path.join(self.app_dir, self.checkpoint_file)}")
        print("🏁" * 40 + "\n")

        logger.info(f"🏁 [PIPELINE] Evolution pipeline finished all generational runs in {duration:.2f}s.")
    # ##########################################################################
    # Function Name : _get_chrom_generation
    #
    # Path          : apps/school/ga_master.py
    # Author        : Chalearm Saelim & Gemini
    #
    # Purpose :
    #    Safely resolves the integer generation index for any chromosome dictionary
    #    by checking explicit 'generation' or 'gen' keys first, and falling back
    #    to string ID parsing (e.g., 'G3-M2' -> Gen 3).
    # ##########################################################################
    def _get_chrom_generation(self, chrom: dict) -> int:
        logger.debug(f"=== state 1.1 === Entering _get_chrom_generation() | chrom_type: {type(chrom)}")
        if not isinstance(chrom, dict):
            logger.debug("=== state 1.1.1 === chrom is not dict, returning default gen 1")
            return 1

        if "generation" in chrom:
            try:
                g = int(chrom["generation"])
                logger.debug(f"=== state 1.1.2 === Found explicit 'generation' key: {g}")
                return g
            except (ValueError, TypeError):
                pass

        if "gen" in chrom:
            try:
                g = int(chrom["gen"])
                logger.debug(f"=== state 1.1.3 === Found explicit 'gen' key: {g}")
                return g
            except (ValueError, TypeError):
                pass

        c_id = str(chrom.get("id", ""))
        logger.debug(f"=== state 1.1.4 === Parsing generation from ID string: '{c_id}'")
        if c_id.startswith("G") and "-" in c_id:
            try:
                g = int(c_id.split("-")[0].replace("G", ""))
                logger.debug(f"=== state 1.1.4.1 === Successfully parsed Gen {g} from ID")
                return g
            except (ValueError, TypeError):
                pass

        logger.debug("=== state 1.1.5 === Fallback default gen 1")
        return 1

    # ##########################################################################
    # Function Name : _trigger_immediate_gen_plot
    #
    # Path          : apps/school/ga_master.py
    # Author        : Chalearm Saelim & Gemini
    #
    # Purpose :
    #    Renders Pareto and price prediction plots locally on Master disk and
    #    dispatches an async task to Celery's export_queue, featuring
    #    full step-by-step diagnostic print debugging.
    # ##########################################################################
    # ##########################################################################
    # Function Name : _trigger_immediate_gen_plot
    # Purpose       : Renders Pareto and price prediction plots locally on Master 
    #                 disk and dispatches an async task to Celery's export_queue, 
    #                 using pack_and_dispatch_visualization_task to preserve val_df.
    # ##########################################################################
    def _trigger_immediate_gen_plot(self, gen_idx: int, evaluated_models: list) -> bool:
        logger.debug(f"=== state 3.1 === Entering _trigger_immediate_gen_plot() for Gen {gen_idx} | evaluated_pool={len(evaluated_models)}")
        
        gen_specific = [
            c for c in evaluated_models 
            if c.get("fitness_evaluated", False) and self._get_chrom_generation(c) == gen_idx
        ]
        logger.debug(f"=== state 3.2 === Filtered gen_specific models count: {len(gen_specific)}")

        if not gen_specific:
            logger.debug(f"=== state 3.2.1 === No models found for Gen {gen_idx}. Aborting plot trigger.")
            return False

        top_k = sorted(gen_specific, key=self._apply_priority_tie_breaker)[:TOP_N_EXPORTS]
        dedup_key = f"GEN_PLOT_G{gen_idx}_{self.run_id}"
        logger.debug(f"=== state 3.3 === Dedup key generated: {dedup_key} | Top candidates count: {len(top_k)}")

        direct_success = False
        try:
            logger.debug("=== state 3.4.1 === Attempting direct local synchronous render...")
            from visualization_worker import _render_standalone_pareto_plot
            _render_standalone_pareto_plot(
                top_chromosomes=top_k,
                run_id=self.run_id,
                gen_idx=gen_idx
            )
            logger.debug("=== state 3.4.2 === Direct local render successful.")
            direct_success = True
        except Exception as local_err:
            logger.warning(f"=== state 3.4.3 === Direct local render warning: {local_err}")

        celery_success = False
        try:
            logger.debug("=== state 3.5.1 === Attempting async Celery task dispatch using pack_and_dispatch_visualization_task...")
            
            # 🛡️ FIX: Pack payload using helper to preserve val_df columns
            payload = pack_and_dispatch_visualization_task(
                run_id=self.run_id,
                gen_idx=gen_idx,
                top_chromosomes=top_k,
                master_df=self.master_data_raw,
                val_df=self.val_master_data_raw
            )
            payload["task_type"] = "GEN_PLOT"

            async_task = export_and_plot_task.apply_async(
                args=[payload],
                queue='export_queue',
                priority=9
            )
            logger.debug(f"=== state 3.5.2 === Celery plot task dispatched successfully | Task ID: {async_task.id}")
            celery_success = True
        except Exception as e:
            logger.warning(f"=== state 3.5.3 === Celery dispatch notice: {e}")

        logger.debug(f"=== state 3.6 === _trigger_immediate_gen_plot returning success={direct_success or celery_success}")
        return direct_success or celery_success
    # ##########################################################################
    # Function Name : _load_directory_to_df
    # Purpose       : Reads transformed CSV datasets and exactly matches the raw 
    #                 price file by stripping the '_transformed' suffix, preventing 
    #                 date-range mismatch bugs.
    # ##########################################################################
    def _load_directory_to_df(self, transform_dir, raw_dir):
        import glob
        import os
        import pandas as pd
        
        all_files = glob.glob(os.path.join(transform_dir, "*_transformed.csv"))
        if not all_files:
            return None

        master_df = None
        global_time_df = None

        for f in all_files:
            try:
                df = pd.read_csv(f)
                if 'timestamp' not in df.columns:
                    continue
                df['timestamp'] = pd.to_datetime(df['timestamp'])
                df.set_index('timestamp', inplace=True)

                if global_time_df is None:
                    time_cols = [c for c in df.columns if any(x in c for x in ['day_', 'hour_', 'min_', 'fourier_'])]
                    global_time_df = df[time_cols]

                filename = os.path.basename(f)
                asset_name = filename.split('_')[0].lower()
                
                # 🛡️ THE FIX: Exact 1:1 filename translation. No more random globbing!
                expected_raw_name = filename.replace("_transformed", "")
                raw_filepath = os.path.join(raw_dir, expected_raw_name)
                
                if os.path.exists(raw_filepath):
                    orig_df = pd.read_csv(raw_filepath)
                    orig_df['timestamp'] = pd.to_datetime(orig_df['timestamp'])
                    orig_df.set_index('timestamp', inplace=True)
                    
                    if 'close' in orig_df.columns:
                        df['close'] = orig_df['close']
                    if 'volume' in orig_df.columns:
                        df['volume_raw'] = orig_df['volume']
                    print(f"      [INGEST DEBUG] 🟢 SUCCESS: Aligned raw data for {asset_name.upper()} from {expected_raw_name}")
                else:
                    print(f"      [INGEST DEBUG] 🔴 FAILED: Missing exact raw file {expected_raw_name} in {raw_dir}")

                df = df.drop(columns=[c for c in df.columns if any(x in c for x in ['day_', 'hour_', 'min_', 'fourier_'])])
                df = df[~df.index.duplicated(keep='first')]
                df = df.add_suffix(f'_{asset_name}')

                if master_df is None:
                    master_df = df
                else:
                    master_df = master_df.join(df, how='outer')

            except Exception as e:
                self.logger.error(f"❌ [INGEST] Failed to parse file {f}: {e}")

        if master_df is not None:
            final_df = pd.concat([master_df, global_time_df], axis=1)
            
            # Forward/Back fill absolute raw prices to cover weekend gaps
            raw_cols = [c for c in final_df.columns if c.startswith('close_') or c.startswith('volume_raw_')]
            if raw_cols:
                final_df[raw_cols] = final_df[raw_cols].ffill().bfill()
            
            final_df = final_df.interpolate(method='linear').bfill().ffill().fillna(0)
            return final_df.dropna(axis=1, how='all')
        
        return None

    # ##############################################################################
    # Function Name : _ingest_data_layers
    #
    # Path          : apps/school/ga_master.py
    # Author        : Chalearm Saelim & Gemini
    #
    # Purpose :
    #    Loads transformed CSV datasets and raw price data from disk for both training 
    #    and validation directories. Integrates an automatic fallback split mechanism: 
    #    if no explicit validation dataset is found on disk, it automatically slices 
    #    the tail end of the master training dataset (85% Train / 15% Val) to ensure 
    #    val_master_data is always populated for validation plotting and evaluation.
    #
    # Inputs :
    #    None (Reads TRANSFORMED_DATA_DIR, RAW_DATA_DIR, VAL_TRANSFORMED_DATA_DIR, 
    #          and VAL_RAW_DATA_DIR global paths).
    #
    # Return :
    #    bool : Returns True if master training data is ingested successfully, False otherwise.
    # ##############################################################################
    def _ingest_data_layers(self) -> bool:
        logger.info("=" * 75)
        logger.info("🔍 [DATASET INGESTION & FEATURE AUDIT]")
        logger.info("=" * 75)

        # 1. Ingest Master Training Dataset
        train_transform_files = glob.glob(os.path.join(TRANSFORMED_DATA_DIR, "*_transformed.csv"))
        logger.info(f"📂 [TRAIN SET] Found {len(train_transform_files)} transformed CSV files in '{TRANSFORMED_DATA_DIR}'")
        self.master_data = self._load_directory_to_df(TRANSFORMED_DATA_DIR, RAW_DATA_DIR)

        if self.master_data is None or self.master_data.empty:
            logger.error("❌ [INGEST CRITICAL] Master training dataset is empty or failed to parse!")
            return False

        # 2. Ingest Explicit Validation Dataset
        val_transform_files = glob.glob(os.path.join(VAL_TRANSFORMED_DATA_DIR, "*_transformed.csv"))
        logger.info(f"📂 [VAL SET]   Found {len(val_transform_files)} transformed CSV files in '{VAL_TRANSFORMED_DATA_DIR}'")
        self.val_master_data = self._load_directory_to_df(VAL_TRANSFORMED_DATA_DIR, VAL_RAW_DATA_DIR)

        # 🛡️ 3. AUTOMATIC SPLIT FALLBACK GUARD
        # If no validation CSVs exist on disk, slice the final 15% of master_data as validation ground truth
        if self.val_master_data is None or self.val_master_data.empty:
            logger.warning("⚠️ [VAL SET WARN] No validation files loaded from disk. Initiating automatic 85/15 chronological split...")
            
            split_idx = int(len(self.master_data) * 0.85)
            self.val_master_data = self.master_data.iloc[split_idx:].copy()
            self.master_data = self.master_data.iloc[:split_idx].copy()
            
            logger.info(f"✅ [VAL AUTO-SPLIT] Sliced master dataset into:")
            logger.info(f"   ├── Master Train Matrix: {len(self.master_data)} rows ({self.master_data.index.min().strftime('%Y-%m-%d')} ➔ {self.master_data.index.max().strftime('%Y-%m-%d')})")
            logger.info(f"   └── Val Ground Truth   : {len(self.val_master_data)} rows ({self.val_master_data.index.min().strftime('%Y-%m-%d')} ➔ {self.val_master_data.index.max().strftime('%Y-%m-%d')})")

        # Store raw unscaled copies for visualization payload packing
        self.master_data_raw = self.master_data.copy()
        self.val_master_data_raw = self.val_master_data.copy()

        # 4. Feature Auditing & Matrix Telemetry
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

        val_start = self.val_master_data.index.min().strftime('%Y-%m-%d')
        val_end   = self.val_master_data.index.max().strftime('%Y-%m-%d')
        val_time_cols, val_asset_cols = self._split_features(self.val_master_data)

        logger.info("-" * 75)
        logger.info("📊 [VALIDATION DATASET MATRIX SUMMARY]")
        logger.info(f"   ├── Total Row Samples : {self.val_master_data.shape[0]} days ({val_start} ➔ {val_end})")
        logger.info(f"   ├── Raw Features Count: {self.val_master_data.shape[1]} columns")
        logger.info(f"   ├── Global Time Feats : {len(val_time_cols)} channels")
        logger.info(f"   └── Active GA Pool    : {len(val_asset_cols)} channels")
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

        logger.info("\n" + "🎲" * 40)
        logger.info(f"🎲 [NEW RUN INITIALIZATION] Hex Run ID: {self.run_id}")
        logger.info(f"   ├── Log Directory   : {log_dir}")
        logger.info(f"   ├── Model Exports   : {export_dir}")
        logger.info(f"   ├── Plot Directory  : {plot_dir}")
        logger.info(f"   └── Warm Start Flag : {use_warm_start}")
        logger.info("🎲" * 40)

        logger.info(f"🎲 [NEW RUN] Generated Random Hex Run ID: {self.run_id}")
        logger.info(f"📂 [PATHS] Logs: {log_dir} | Models: {export_dir} | Plots: {plot_dir}")

        _, asset_cols = self._split_features(self.master_data)
        self.chromosome_population = []

        # ----------------------------------------------------------------------
        # 1. WARM-START SEEDING STEP
        # ----------------------------------------------------------------------
        if use_warm_start:
            logger.info("\n🔥 [WARM-START] Attempting candidate seed ingestion from prior run checkpoints...")
            self.chromosome_population = self._seed_warm_start_candidates()

        existing_count = len(self.chromosome_population)
        needed_count = max(0, POPULATION_SIZE - existing_count)

        if existing_count > 0:
            logger.info(f"🌱 [POPULATION SEEDING] Ingested {existing_count} warm-start candidate(s). Generating {needed_count} randomized model(s)...")
            logger.info(f"🌱 [INIT] Ingested {existing_count} warm-start seeds. Seeding {needed_count} random models...")
        else:
            logger.info(f"🌱 [POPULATION SEEDING] Seeding {POPULATION_SIZE} randomized hyperparameter matrices for Generation 1...")
            logger.info(f"🌱 [INIT] Seeding randomized hyperparameter matrices for Generation 1...")

        max_rows = len(self.master_data)
        actual_max_lookback = max(MIN_LOOKBACK_DAYS + 1, min(MAX_LOOKBACK_DAYS, int(max_rows * 0.7)))
        actual_max_horizon = max(MIN_FORECAST_DAYS + 1, min(MAX_FORECAST_DAYS, int(max_rows * 0.7)))

        logger.info(f"📐 [FEATURE & CONSTRAINTS MATRIX] Data Rows: {max_rows} | Asset Features: {len(asset_cols)}")
        logger.info(f"   ├── Lookback Range : {MIN_LOOKBACK_DAYS} -> {actual_max_lookback} days")
        logger.info(f"   └── Horizon Range  : {MIN_FORECAST_DAYS} -> {actual_max_horizon} days\n")

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
            # Inside the randomized chromosome generation loop:
            epochs_val = random.randint(MIN_EPOCHS, MAX_EPOCHS)

            chromosome = {
                "id": f"G1-M{idx}",
                "lstm_layers": num_layers,
                "nodes_per_layer": nodes_list,
                "lookback_window": lookback,
                "forecast_horizon": horizon,
                "learning_rate": lr_val,
                "dropout_rate": dropout_val,
                "batch_size": batch_val,
                "epochs": epochs_val,  # 🟢 ADDED EPOCHS GENE
                "feature_mask": mask,
                "fitness_evaluated": False,
                "perf_vector": [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
            }
            self.chromosome_population.append(chromosome)

            # 🟢 Added 'Epochs' telemetry tag to match your desired format log output
            logger.info(f"   ├── 🧬 [MODEL CREATED] ID: G1-M{idx:<2} | Layers: {num_layers} {nodes_list} | Lookback: {lookback}d | Horizon: {horizon}d | LR: {lr_val} | Dropout: {dropout_val} | Epochs: {epochs_val} | Batch: {batch_val} | Active Features: {sum(mask)}/{len(asset_cols)}")

        logger.info("\n" + "✅" * 40)
        logger.info(f"✅ [INIT COMPLETE] Generation 1 seeding complete ({len(self.chromosome_population)} models total). Flushing checkpoint to disk...")
        logger.info("✅" * 40 + "\n")

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
            # 1. Sort the pool from Best to Worst
            evaluated_pool.sort(key=self._apply_priority_tie_breaker)
            
            # 2. RANK-BASED SELECTION (Natural Selection)
            # The #1 model gets the most "raffle tickets", the worst model gets 1 ticket.
            # This ensures the best models breed the most, but weaker models still pass on diverse genes.
            pool_size = len(evaluated_pool)
            weights = [pool_size - i for i in range(pool_size)] 
            
            # Draw 2 parents from the lottery
            parents = random.choices(evaluated_pool, weights=weights, k=2)
            parent_a = parents[0]
            parent_b = parents[1]

            # 3. UNIFORM CROSSOVER (True Biological Inheritance)
            # For every single trait, flip a 50/50 coin to decide if it comes from Parent A or Parent B
            child = {
                "id": f"G{next_gen_idx}-M{child_idx}",
                
                # Architecture Genes
                "lstm_layers": parent_a['lstm_layers'] if random.random() < 0.5 else parent_b['lstm_layers'],
                "nodes_per_layer": list(parent_a['nodes_per_layer']) if random.random() < 0.5 else list(parent_b['nodes_per_layer']),
                
                # Time-Series Genes
                "lookback_window": parent_a['lookback_window'] if random.random() < 0.5 else parent_b['lookback_window'],
                "forecast_horizon": parent_a['forecast_horizon'] if random.random() < 0.5 else parent_b['forecast_horizon'],
                
                # Optimization Genes
                "learning_rate": parent_a['learning_rate'] if random.random() < 0.5 else parent_b['learning_rate'],
                "dropout_rate": parent_a['dropout_rate'] if random.random() < 0.5 else parent_b['dropout_rate'],
                "batch_size": parent_a['batch_size'] if random.random() < 0.5 else parent_b['batch_size'],
                "epochs": parent_a.get('epochs', 25) if random.random() < 0.5 else parent_b.get('epochs', 25),
                
                # Data Feature Gene
                "feature_mask": list(parent_a['feature_mask']) if random.random() < 0.5 else list(parent_b['feature_mask']),
                
                # State trackers
                "fitness_evaluated": False,
                "perf_vector": [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
            }
            # 4. MUTATION
            # Pass the child to the mutation function to randomly alter a few genes
            child = self._mutate(child)
        else:

           # FALLBACK: If we don't have enough parents yet, generate a random "Adam/Eve" model
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
                "epochs": random.randint(MIN_EPOCHS, MAX_EPOCHS),  
                "feature_mask": mask,
                "fitness_evaluated": False,
                "perf_vector": [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
            }

        child['fitness_evaluated'] = False
        child['perf_vector'] = [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
        return child
    # ##############################################################################
    # Function Name : _format_active_task_telemetry
    #
    # Path          : apps/school/ga_master.py
    # Author        : Chalearm Saelim & Gemini
    #
    # Purpose :
    #    Parses Celery active task payload dictionaries and formats model ID, generation
    #    index, cross-validation fold indices, and candidate model details for worker
    #    telemetry status reports.
    #
    # Inputs :
    #    task_dict : dict Raw active task dictionary inspected from Celery broker.
    #
    # Return :
    #    str : Formatted human-readable target string for cluster status display.
    # ##############################################################################
    def _format_active_task_telemetry(task_dict: dict) -> str:
        task_name = task_dict.get("name", "")
        args = task_dict.get("args", [])
        
        payload = args[0] if len(args) > 0 and isinstance(args[0], (dict, str)) else {}
        if isinstance(payload, str):
            try:
                import json
                payload = json.loads(payload)
            except Exception:
                payload = {}

        # 1. Format export_and_plot_task (GEN_PLOT or POST_GA_OOS)
        if "export_and_plot_task" in task_name:
            gen_idx = payload.get("gen_idx", "?")
            chroms = payload.get("top_chromosomes", [])
            task_type = payload.get("task_type", "GEN_PLOT")
            
            if task_type == "POST_GA_OOS":
                return f"Target: Gen {gen_idx} Out-Of-Sample Verification"

            if chroms and isinstance(chroms, list):
                first_cand_id = chroms[0].get("id", f"G{gen_idx}-M0")
                num_cands = len(chroms)
                return f"Target: Gen {gen_idx} ({first_cand_id} + {num_cands - 1} top models)"
            else:
                return f"Target: Gen {gen_idx} Pareto Plot"

        # 2. Format run_fold_training_task (CV_FOLD)
        elif "run_fold_training_task" in task_name:
            model_id = payload.get("chrom_id", payload.get("chromosome_id", payload.get("model_id", "Model")))
            fold_idx = payload.get("fold_idx", "?")
            num_folds = payload.get("num_folds", payload.get("total_folds", 4))
            return f"Target: {model_id:<8} (Fold {fold_idx}/{num_folds})"

        # 3. Default Fallback
        return f"Target: {payload.get('chrom_id', 'Model')}"
    # ##########################################################################
    # Function Name : _breed_new_generation
    # Path          : apps/school/ga_master.py
    # ##########################################################################
    def _breed_new_generation(self, parent_pool: list, target_gen: int, pop_size: int) -> list:
        logger.debug(f"=== state 5.1 === Entering _breed_new_generation() for Gen {target_gen} | parents={len(parent_pool)} | pop_size={pop_size}")
        
        # Fallback to existing engine mutation/crossover logic if available
        if hasattr(self, "_generate_next_generation"):
            return self._generate_next_generation(parent_pool, target_gen)
        
        # If the engine uses speculatively generated children per index:
        new_gen_chroms = []
        for i in range(pop_size):
            child = self._generate_speculative_child(parent_pool, target_gen, i)
            child['generation'] = target_gen
            new_gen_chroms.append(child)
            
        logger.debug(f"=== state 5.2 === Successfully bred {len(new_gen_chroms)} models for Generation {target_gen}")
        return new_gen_chroms
    # ##############################################################################
    # Function Name : _print_on_demand_status
    # Purpose       : Generates a rich diagnostic report comparing evaluated models 
    #                 in RAM vs Disk, and prints fold-level details for all active 
    #                 and recently completed models.
    # ##############################################################################
    # ##############################################################################
    # Function Name : _print_on_demand_status
    # Purpose       : Generates a rich diagnostic report comparing evaluated models 
    #                 in RAM vs Disk, and prints the current Lazy Pruning Threshold.
    # ##############################################################################
    def _print_on_demand_status(self, completed_fold_results, expected_folds, unevaluated_in_gen, current_gen, lazy_prune_rank):
        logger.info("\n" + "📊" * 60)
        logger.info("📊 [ON-DEMAND STATUS REPORT] RAM vs Disk State & Fold Details")
        logger.info("📊" * 60)

        evaluated_unsaved = [c for c in self.chromosome_population if c.get("fitness_evaluated", False) and not c.get("_is_saved", False)]
        evaluated_saved = [c for c in self.chromosome_population if c.get("fitness_evaluated", False) and c.get("_is_saved", False)]
        
        valid_das = sorted([c.get('perf_vector', [0.0])[0] for c in self.chromosome_population if c.get("fitness_evaluated", False) and c.get('generation', 0) == current_gen], reverse=True)
        
        if len(valid_das) < lazy_prune_rank:
            threshold_str = f"FREE PASS (Needs {lazy_prune_rank - len(valid_das)} more completed models to set baseline)"
        else:
            threshold_str = f"{valid_das[lazy_prune_rank - 1] * 100:+.2f}%"

        logger.info(f"   ├── 🧠 Total Population in RAM : {len(self.chromosome_population)}")
        logger.info(f"   ├── 💾 Evaluated & Saved Disk  : {len(evaluated_saved)}")
        logger.info(f"   ├── ⏳ Evaluated & UNSAVED     : {len(evaluated_unsaved)} (Waiting for checkpoint trigger)")
        logger.info(f"   └── 📉 Current Lazy Threshold  : {threshold_str} (Targeting Rank {lazy_prune_rank})")
        
        if evaluated_unsaved:
            logger.info("\n   [RECENTLY EVALUATED MODELS (UNSAVED IN RAM)]")
            for chrom in evaluated_unsaved:
                c_id = chrom['id']
                perf = chrom.get('perf_vector', [])
                rmse = perf[3] if len(perf) > 3 else 999.0
                da = perf[0] if len(perf) > 0 else 0.0
                logger.info(f"   ✅ {c_id} | Mean RMSE: {rmse:.4f} | Mean Skill DA: {da*100:+.2f}%")

        logger.info("\n   [PENDING MODELS IN CURRENT GENERATION - FOLD DETAILS]")
        pending_count = 0
        for chrom in self.chromosome_population:
            c_id = chrom['id']
            if c_id in unevaluated_in_gen:
                pending_count += 1
                completed_folds = completed_fold_results.get(c_id, [])
                expected = expected_folds.get(c_id, 5)
                logger.info(f"   ⏳ {c_id} (Completed {len(completed_folds)}/{expected} folds)")
                for f in completed_folds:
                    f_idx = f.get('fold_idx', '?')
                    f_rmse = f.get('metrics', {}).get('rmse', 999.0)
                    f_da = f.get('metrics', {}).get('skill_da', 0.0)
                    logger.info(f"        └── Fold {f_idx} ✅: RMSE={f_rmse:.4f} | Skill DA={f_da*100:+.2f}%")

        if pending_count == 0: logger.info("   👉 (None. All models in this generation are fully evaluated.)")
        logger.info("📊" * 60 + "\n")
    # ##############################################################################
    # Function Name : _evaluate_population_sync
    # Purpose       : Synchronous generational barrier engine with fixed local 
    #                 save_pct calculations, File-Flag Manual Saving, and strict 
    #                 zombie task tracking.
    # ##############################################################################
    def _evaluate_population_sync(self, initial_population, max_generations=50):
        logger.info("═" * 80)
        logger.info("🚀 [PIPELINE START] Entering Synchronous Generational Barrier Evolution Engine")
        logger.info("═" * 80)
        
        import os
        import time
        import redis
        import json
        from celery_tasks import run_fold_training_task, export_and_plot_task

        redis_target = os.getenv("REDIS_URL") or os.getenv("CELERY_BROKER_URL") or "redis://redis:6379/0"
        try:
            redis_client = redis.Redis.from_url(redis_target)
            redis_client.ping()
        except Exception:
            redis_client = redis.Redis(host="redis", port=6379, db=0)

        gen1_models = [c for c in initial_population if str(c.get("id", "")).startswith("G1-")]
        actual_pop_per_gen = len(gen1_models) if len(gen1_models) > 0 else POPULATION_SIZE

        self.chromosome_population = list(initial_population)
        
        # Mark all loaded models as safely saved to disk
        for c in self.chromosome_population:
            if c.get("fitness_evaluated", False):
                c['_is_saved'] = True

        evaluated_pool = [c for c in initial_population if c.get("fitness_evaluated", False)]
        current_gen = 1

        save_pct_threshold = getattr(self, 'save_pct', 12.5)
        save_min_threshold = getattr(self, 'save_interval_min', 35.0)
        EARLY_STOP_RMSE_THRESHOLD = 5.0 

        # 🟢 DYNAMIC LAZY PRUNE RANK: Protects Genetic Diversity
        required_parents = max(2, actual_pop_per_gen // 2)
        try:
            lazy_prune_rank = max(TOP_N_EXPORTS, required_parents)
        except NameError:
            lazy_prune_rank = max(5, required_parents)
            
        # Flag Files
        FORCE_SAVE_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "force_save.flag")
        PRINT_STATUS_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "print_status.flag")

        while current_gen <= max_generations and self.running:
            logger.info("\n" + "█" * 80)
            logger.info(f"🧬 [GENERATION BARRIER] Starting Evaluation Cycle for Generation {current_gen} / {max_generations}")
            logger.info("█" * 80)

            gen_chromosomes = [c for c in self.chromosome_population if self._get_chrom_generation(c) == current_gen]

            if not gen_chromosomes and current_gen > 1:
                logger.info(f"🌱 [BREEDING] Generation {current_gen} is empty. Breeding via Crossover & Mutation from top parents...")
                parent_pool = sorted(evaluated_pool, key=self._apply_priority_tie_breaker)[:max(2, actual_pop_per_gen // 2)]
                gen_chromosomes = self._breed_new_generation(parent_pool, current_gen, actual_pop_per_gen)
                self.chromosome_population.extend(gen_chromosomes)

            task_mapping = {}
            expected_folds = {}
            completed_fold_results = {}

            # Count models that still need evaluation
            unevaluated_in_gen = {c['id'] for c in gen_chromosomes if not c.get("fitness_evaluated", False)}
            total_unevaluated = len(unevaluated_in_gen)

            # 🟢 FIX: Calculate exactly how many models equal the requested save_pct
            evals_needed_for_save = max(1, int((save_pct_threshold / 100.0) * len(gen_chromosomes)))
            
            logger.info(f"📋 [GEN TARGETS] {total_unevaluated} models left to evaluate. Triggering checkpoint every {evals_needed_for_save} models ({save_pct_threshold}%).")

            for chrom in gen_chromosomes:
                c_id = chrom['id']
                chrom['generation'] = current_gen
                if chrom.get("fitness_evaluated", False):
                    continue

                payloads = self._build_fold_payloads(chrom)
                expected_folds[c_id] = len(payloads)
                completed_fold_results[c_id] = []

                for p in payloads:
                    try:
                        async_task = run_fold_training_task.apply_async(args=[p], queue='training_queue', priority=3)
                        task_mapping[async_task.id] = (chrom, p['fold_idx'], c_id)
                        logger.info(f"  ├── 📤 [DISPATCH] {c_id} (Fold {p['fold_idx']}/{len(payloads)}) ➔ Task ID: {async_task.id}")
                    except Exception as e:
                        logger.error(f"  └── ❌ [DISPATCH ERROR] Failed to dispatch fold task for {c_id}: {e}")

            last_save_time = time.time()
            models_evaluated_since_save = 0
            
            while unevaluated_in_gen and self.running:
                finished_task_ids = []
                model_completed_this_loop = False  

                # 🟢 MANUAL SAVE OVERRIDE
                if os.path.exists(FORCE_SAVE_FILE):
                    logger.info("🚨 [MANUAL OVERRIDE] 'force_save.flag' detected! Flushing JSON checkpoint to disk...")
                    self._save_checkpoint(partial=True)
                    for c in self.chromosome_population:
                        if c.get("fitness_evaluated", False): c['_is_saved'] = True
                    try:
                        os.remove(FORCE_SAVE_FILE)
                    except OSError: pass
                    last_save_time = time.time()
                    models_evaluated_since_save = 0

                # 🟢 ON-DEMAND STATUS REPORT
                if os.path.exists(PRINT_STATUS_FILE):
                    self._print_on_demand_status(completed_fold_results, expected_folds, unevaluated_in_gen, current_gen)
                    try:
                        os.remove(PRINT_STATUS_FILE)
                    except OSError: pass

                for t_id, (chrom, f_idx, c_id) in list(task_mapping.items()):
                    if c_id not in unevaluated_in_gen:
                        continue

                    try:
                        res = redis_client.get(f"celery-task-meta-{t_id}")
                        if res:
                            parsed = json.loads(res.decode('utf-8'))
                            task_status = str(parsed.get("status", "")).upper()
                            
                            if task_status == "SUCCESS":
                                fold_result = parsed.get("result", {})
                                fold_rmse = fold_result.get('metrics', {}).get('rmse', 999.0)
                                
                                if fold_rmse > EARLY_STOP_RMSE_THRESHOLD:
                                    logger.warning(f"⚠️ [EARLY STOP] {c_id} (Fold {f_idx}) returned exploding RMSE ({fold_rmse:.4f}). Aborting model!")
                                    finished_task_ids.append(t_id)
                                    chrom['perf_vector'] = [0.0, -5.0, 1.0, 999.0, 0.0, 0.0]
                                    chrom['fitness_evaluated'] = True
                                    if chrom not in evaluated_pool: evaluated_pool.append(chrom)
                                    unevaluated_in_gen.remove(c_id)
                                    model_completed_this_loop = True
                                    
                                    pending_tasks = [k for k, v in task_mapping.items() if v[2] == c_id and k != t_id]
                                    for p_tid in pending_tasks:
                                        try:
                                            run_fold_training_task.app.control.revoke(p_tid, terminate=True, signal='SIGTERM')
                                            finished_task_ids.append(p_tid)
                                        except: pass
                                    continue 
                                
                                finished_task_ids.append(t_id)
                                completed_fold_results[c_id].append(fold_result)
                                logger.info(f"  ├── ✅ [FOLD SUCCESS] {c_id} (Fold {f_idx}) completed with RMSE: {fold_rmse:.4f}")
                                completed_count = len(completed_fold_results[c_id])
                                expected_count = expected_folds.get(c_id, NUM_FOLDS)

                                # --- 2. LAZY FOLD PROCESSING (Branch & Bound) ---
                                if 0 < completed_count < expected_count:
                                    current_da_sum = sum(f.get('metrics', {}).get('skill_da', 0.0) for f in completed_fold_results[c_id])
                                    remaining_folds = expected_count - completed_count
                                    
                                    # Assume all remaining folds achieve a perfect 100% (1.0) Skill DA
                                    max_theoretical_da = (current_da_sum + (remaining_folds * 1.0)) / expected_count
                                    
                                    # 🟢 STRICT CURRENT GEN CHECK
                                    valid_das = sorted(
                                        [c.get('perf_vector', [0.0])[0] for c in evaluated_pool 
                                         if len(c.get('perf_vector', [])) > 0 and c.get('generation', 0) == current_gen], 
                                        reverse=True
                                    )

                                    # 🟢 FREE PASS LOGIC: Do not prune if we lack a local baseline
                                    if len(valid_das) < lazy_prune_rank:
                                        survival_threshold = -999.0 # Guaranteed survival
                                    else:
                                        survival_threshold = valid_das[lazy_prune_rank - 1]

                                    if max_theoretical_da < survival_threshold:
                                        logger.warning(f"  ├── 📉 [LAZY PRUNE] {c_id} max theoretical DA ({max_theoretical_da*100:.2f}%) cannot beat Top-{lazy_prune_rank} threshold ({survival_threshold*100:.2f}%). Aborting remaining {remaining_folds} folds!")
                                        
                                        chrom['perf_vector'] = [0.0, -5.0, 1.0, 999.0, 0.0, 0.0] # Penalize
                                        chrom['fitness_evaluated'] = True
                                        chrom['_is_saved'] = False
                                        if chrom not in evaluated_pool: evaluated_pool.append(chrom)
                                        unevaluated_in_gen.remove(c_id)
                                        model_completed_this_loop = True
                                        
                                        pending_tasks = [k for k, v in task_mapping.items() if v[2] == c_id and k not in finished_task_ids]
                                        for p_tid in pending_tasks:
                                            try:
                                                run_fold_training_task.app.control.revoke(p_tid, terminate=True, signal='SIGTERM')
                                                finished_task_ids.append(p_tid)
                                                logger.info(f"  │   └── 🔪 Revoked pending lazy fold task.")
                                            except: pass
                                        continue

                                # --- 3. MODEL FULLY COMPLETED ---
                                if completed_count >= expected_count:
                                    objectives = self._summarize_chromosome_evaluation(chrom, completed_fold_results[c_id], gen=current_gen)
                                    chrom['perf_vector'] = objectives
                                    chrom['fitness_evaluated'] = True
                                    chrom['_is_saved'] = False
                                    if chrom not in evaluated_pool: evaluated_pool.append(chrom)
                                    unevaluated_in_gen.remove(c_id)
                                    model_completed_this_loop = True
                                    logger.info(f"  └── 🏆 [MODEL COMPLETE] {c_id} finished all folds safely. Remaining in Gen {current_gen}: {len(unevaluated_in_gen)}")

                                    redundant_tasks = [k for k, v in task_mapping.items() if v[2] == c_id and k not in finished_task_ids]
                                    for r_tid in redundant_tasks:
                                        try:
                                            run_fold_training_task.app.control.revoke(r_tid, terminate=True, signal='SIGTERM')
                                            finished_task_ids.append(r_tid)
                                        except: pass
                                            
                            elif task_status in ["FAILURE", "REVOKED"]:
                                finished_task_ids.append(t_id)
                                logger.error(f"❌ [POISON PILL DETECTED] {c_id} (Fold {f_idx}) failed. Penalizing and aborting.")
                                chrom['perf_vector'] = [0.0, -5.0, 1.0, 999.0, 0.0, 0.0]
                                chrom['fitness_evaluated'] = True
                                if chrom not in evaluated_pool: evaluated_pool.append(chrom)
                                if c_id in unevaluated_in_gen:
                                    unevaluated_in_gen.remove(c_id)
                                    model_completed_this_loop = True

                                pending_tasks = [k for k, v in task_mapping.items() if v[2] == c_id and k != t_id]
                                for p_tid in pending_tasks:
                                    try:
                                        run_fold_training_task.app.control.revoke(p_tid, terminate=True, signal='SIGTERM')
                                        finished_task_ids.append(p_tid)
                                    except: pass

                    except Exception:
                        pass 

                for t_id in finished_task_ids:
                    if t_id in task_mapping: del task_mapping[t_id]

                # 🟢 ISOLATED CHECKPOINT TRIGGER CHECK
                if model_completed_this_loop:
                    models_evaluated_since_save += 1
                    time_elapsed_min = (time.time() - last_save_time) / 60.0

                    if models_evaluated_since_save >= evals_needed_for_save or time_elapsed_min >= save_min_threshold:
                        logger.info(f"💾 [REAL-TIME FLUSH] Threshold met ({models_evaluated_since_save} models processed | {time_elapsed_min:.1f}m elapsed). Saving Checkpoint.")
                        self._save_checkpoint(partial=True)
                        for c in self.chromosome_population:
                            if c.get("fitness_evaluated", False): c['_is_saved'] = True
                        last_save_time = time.time()
                        models_evaluated_since_save = 0

                if unevaluated_in_gen:
                    time.sleep(2.0)

            # End of Generation Checkpoint
            self._save_checkpoint(partial=False)
            for c in self.chromosome_population:
                if c.get("fitness_evaluated", False): c['_is_saved'] = True

            logger.info("═" * 80)
            logger.info(f"🎉 [GENERATION {current_gen} DONE] 100% Completion Reached! Triggering Visualizations...")
            
            gen_specific_evaluated = [c for c in evaluated_pool if self._get_chrom_generation(c) == current_gen]
            top_k = sorted(gen_specific_evaluated, key=self._apply_priority_tie_breaker)[:TOP_N_EXPORTS]

            plot_payload = pack_and_dispatch_visualization_task(
                run_id=self.run_id,
                gen_idx=current_gen,
                top_chromosomes=top_k,
                master_df=self.master_data_raw,
                val_df=self.val_master_data_raw
            )
            plot_payload["task_type"] = "GEN_PLOT"
            
            try:
                export_and_plot_task.apply_async(args=[plot_payload], queue='export_queue', priority=9)
            except Exception as plot_err:
                logger.error(f"  └── ❌ [PLOT ERROR] Failed dispatching visualization task for Gen {current_gen}: {plot_err}")

            current_gen += 1

        logger.info("🛑 [PIPELINE STOP] Synchronous generational evolution loop finished or interrupted.")
        return evaluated_pool

    # ##############################################################################
    # Function Name : _audit_and_trigger_missing_plots
    #
    # Purpose :
    #    Audits disk storage for completed generation plot artifacts. Called both
    #    upon checkpoint restoration during startup AND dynamically inside the main
    #    pipeline loop whenever a candidate model completes all training folds.
    #    If a generation has all models evaluated but its Pareto chart is missing
    #    from `prediction_result/{run_id}/`, dispatches an immediate high-priority
    #    `GEN_PLOT` task to Celery's `export_queue`.
    #
    # Inputs :
    #    evaluated_pool
    #        Type        : list
    #        Description : List of currently evaluated chromosome dictionaries.
    #    active_redis_tasks
    #        Type        : dict
    #        Description : Active task map to prevent duplicate dispatch if a plot
    #                      task is already in-flight.
    #
    # Return :
    #    Type        : int
    #    Description : Count of newly dispatched visualization tasks.
    # ##############################################################################
    # ##############################################################################
    # Function Name : _audit_and_trigger_missing_plots
    # Purpose       : Startup safety mechanism. Scans the restored checkpoint pool 
    #                 to find fully completed generations. If a generation finished 
    #                 but its graph plotting task was killed/lost (e.g., worker crash), 
    #                 this actively re-dispatches the visualization payload.
    # ##############################################################################
    def _audit_and_trigger_missing_plots(self, evaluated_pool, custom_pop_size):
        import os
        from celery_tasks import export_and_plot_task

        if not evaluated_pool:
            return

        # 1. Tally how many models are evaluated per generation
        gen_counts = {}
        for c in evaluated_pool:
            g = self._get_chrom_generation(c)
            gen_counts[g] = gen_counts.get(g, 0) + 1

        # 🟢 FIX: Dynamically resolve the app's root directory and prediction_result folder
        app_root = os.path.dirname(os.path.abspath(__file__))
        base_plot_dir = os.path.join(app_root, "prediction_result")

        for g_idx, count in gen_counts.items():
            # If the generation reached 100% completion in the checkpoint
            if count >= custom_pop_size:
                target_plot_dir = os.path.join(base_plot_dir, str(self.run_id), f"G{g_idx}")
                pareto_path = os.path.join(target_plot_dir, "pareto_frontier.png")

                # 2. Check if the plots successfully rendered to disk
                if not os.path.exists(pareto_path):
                    logger.info(f"  ├── ⚠️ [AUDIT WARN] Generation {g_idx} is 100% complete but plots are missing! Initiating recovery...")
                    
                    gen_specific_evaluated = [c for c in evaluated_pool if self._get_chrom_generation(c) == g_idx]
                    top_k = sorted(gen_specific_evaluated, key=self._apply_priority_tie_breaker)[:TOP_N_EXPORTS]

                    plot_payload = pack_and_dispatch_visualization_task(
                        run_id=self.run_id,
                        gen_idx=g_idx,
                        top_chromosomes=top_k,
                        master_df=self.master_data_raw,
                        val_df=self.val_master_data_raw
                    )
                    plot_payload["task_type"] = "GEN_PLOT"
                    
                    try:
                        export_and_plot_task.apply_async(args=[plot_payload], queue='export_queue', priority=9)
                        logger.info(f"  └── 🚑 [RECOVERY SUCCESS] Dispatched recovery plot task for Gen {g_idx} to Celery workers.")
                    except Exception as plot_err:
                        logger.error(f"  └── ❌ [RECOVERY ERROR] Failed dispatching recovery plot task for Gen {g_idx}: {plot_err}")
                else:
                    # Plots exist, no action needed
                    logger.debug(f"  ├── ✅ [AUDIT OK] Generation {g_idx} plots verified on disk.")
    # ##############################################################################
    # Function Name : _summarize_chromosome_evaluation
    # Purpose       : Summarizes cross-validation fold metrics, computes total 
    #                 processing duration, and outputs single-pass telemetry logs.
    # ##############################################################################
    def _summarize_chromosome_evaluation(self, chromosome, fold_results, gen):
        fold_results.sort(key=lambda x: x.get('fold_idx', 0))

        # 🛡️ Case-insensitive status check ('SUCCESS' or 'success')
        valid_folds = [
            res for res in fold_results 
            if str(res.get('status', '')).upper() == 'SUCCESS'
        ]

        if not valid_folds:
            logger.warning(f"⚠️ [EVAL WARN] Model {chromosome['id']} has no successful fold results!")
            chromosome['perf_vector'] = [0.0, -5.0, 1.0, 999.0, 0.0, 0.0]
            chromosome['fitness_evaluated'] = True
            return chromosome['perf_vector']

        # Extract fold performance metrics
        fold_skill_da = [f.get('metrics', {}).get('skill_da', f.get('skill_da', 0.0)) for f in valid_folds]
        fold_sharpe   = [f.get('metrics', {}).get('sharpe', f.get('sharpe', -5.0)) for f in valid_folds]
        fold_rmse     = [f.get('metrics', {}).get('rmse', f.get('rmse', 999.0)) for f in valid_folds]
        fold_cagr     = [f.get('metrics', {}).get('cagr', f.get('cagr', 0.0)) for f in valid_folds]
        fold_maxdd    = [f.get('metrics', {}).get('max_dd', f.get('max_dd', 0.0)) for f in valid_folds]

        # Calculate processing time duration across valid folds
        total_exec_sec = sum(float(f.get('execution_time_sec', 0.0)) for f in valid_folds)
        num_folds = len(valid_folds)

        skill_da_mean = float(np.mean(fold_skill_da))
        skill_da_std  = float(np.std(fold_skill_da)) if len(fold_skill_da) > 1 else 0.0
        sharpe_mean   = float(np.mean(fold_sharpe))
        sharpe_std    = float(np.std(fold_sharpe)) if len(fold_sharpe) > 1 else 0.0
        rmse_mean     = float(np.mean(fold_rmse))
        cagr_mean     = float(np.mean(fold_cagr))
        maxdd_mean    = float(np.mean(fold_maxdd))

        objectives_vector = [skill_da_mean, sharpe_mean, maxdd_mean, rmse_mean, skill_da_std, sharpe_std]

        chromosome['metrics'] = {
            "skill_da": skill_da_mean,
            "skill_da_std": skill_da_std,
            "sharpe": sharpe_mean,
            "sharpe_std": sharpe_std,
            "rmse": rmse_mean,
            "cagr": cagr_mean,
            "max_dd": maxdd_mean,
            "total_execution_sec": total_exec_sec
        }
        chromosome['perf_vector'] = objectives_vector
        chromosome['fitness_evaluated'] = True

        eval_headers = [
            "======================================================================",
            f"⚡ [EVALUATION COMPLETE] Model: {chromosome['id']} | Generation: {gen} | ⏱️ Duration: {total_exec_sec:.2f}s",
            f"    🎯 Mean Skill DA        : {skill_da_mean * 100:+.2f}%  (std: {skill_da_std * 100:.2f}%)",
            f"    ⚡ Mean Sharpe Ratio    : {sharpe_mean:.2f}  (std: {sharpe_std:.2f})",
            f"    📈 Mean CAGR            : {cagr_mean * 100:+.2f}%",
            f"    📉 Mean Max Drawdown    : {maxdd_mean * 100:.2f}%",
            f"    📐 Mean RMSE            : {rmse_mean:.4f}",
            f"    ⏱️ Total Processing Time: {total_exec_sec:.2f}s ({num_folds} fold(s))",
            "======================================================================"
        ]

        # 🛡️ FIX DUPLICATION: Log exclusively to summary_logger
        for line in eval_headers:
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
            chromosome['epochs'] = random.randint(MIN_EPOCHS, MAX_EPOCHS)
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: epochs ({chromosome['epochs']}).")
        
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
    # Path          : apps/school/ga_master.py
    # Author        : Chalearm Saelim & Gemini
    #
    # Purpose :
    #    Performs atomic checkpoint persistence to disk. Filters and deduplicates
    #    the active chromosome population in memory by unique chromosome ID
    #    (retaining evaluated models over pending duplicates), dynamically resolves
    #    active generation indices, serializes JSON state payloads, and flushes
    #    atomic temporary writes across the application root and deployed run paths.
    #
    # Inputs :
    #    partial : bool (Default: False)
    #        Flag indicating whether the save was triggered asynchronously mid-generation
    #        or during an interim checkpoint interval.
    #
    # Return :
    #    None
    #
    # Complexity :
    #    Time  : O(N) where N is the total population size across generations.
    #    Space : O(N) for dictionary deduplication maps and JSON serialization buffers.
    #
    # Error Cases :
    #    - Safely catches and prints file system creation, permission, write, or fsync errors.
    # ##########################################################################
    def _save_checkpoint(self, partial=False):
        app_root = "/workspace/crypto_apps/dexbot/apps/school"
        run_id = getattr(self, "run_id", None) or getattr(self, "current_run_id", "DEFAULT_RUN")
        raw_population = getattr(self, "chromosome_population", [])

        # ----------------------------------------------------------------------
        # 1. IN-MEMORY POPULATION DEDUPLICATION
        # Map chromosome IDs to objects, prioritizing evaluated candidates over pending duplicates.
        # ----------------------------------------------------------------------
        dedup_map = {}
        duplicates_removed = 0

        for chrom in raw_population:
            c_id = chrom.get("id")
            if not c_id:
                continue

            if c_id not in dedup_map:
                dedup_map[c_id] = chrom
            else:
                existing_is_eval = dedup_map[c_id].get("fitness_evaluated", False)
                new_is_eval = chrom.get("fitness_evaluated", False)

                # Overwrite if new candidate is evaluated while existing is pending
                if new_is_eval and not existing_is_eval:
                    dedup_map[c_id] = chrom

                duplicates_removed += 1

        # Re-assign clean, unique chromosome population back to instance
        population = list(dedup_map.values())
        self.chromosome_population = population

        # ----------------------------------------------------------------------
        # 2. DYNAMIC GENERATION DERIVATION (UNBOUNDED)
        # Inspect unique model IDs (e.g., 'G2334-M3' -> Gen 2334) to resolve current generation
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
        # 3. DYNAMIC MAX GENERATIONS RESOLUTION
        # ----------------------------------------------------------------------
        max_gen = getattr(self, "max_generations", None)
        if max_gen is None or max_gen < gen:
            max_gen = gen  # Unbounded dynamic ceiling

        eval_count = sum(1 for c in population if c.get("fitness_evaluated"))
        eval_pct = (eval_count / len(population) * 100.0) if len(population) > 0 else 0.0

        # ----------------------------------------------------------------------
        # 4. VERBOSE CONSOLE DIAGNOSTIC TELEMETRY
        # ----------------------------------------------------------------------
        logger.info("\n" + "💾" * 40)
        logger.info(f"💾 [SAVE CHECKPOINT] Run ID: {run_id} | Active Gen: {gen}/{max_gen} | Partial Trigger: {partial}")
        logger.info(f"   ├── Total Population : {len(population)} unique models loaded in memory")
        if duplicates_removed > 0:
            logger.info(f"   ├── Deduplication    : 🧹 Stripped {duplicates_removed} duplicate chromosome entry(ies)")
        logger.info(f"   └── Evaluation Ratio : {eval_count} / {len(population)} evaluated ({eval_pct:.1f}%)")
        logger.info("💾" * 40)

        root_ckpt_path = os.path.join(app_root, "lstm_ga_checkpoint.json")
        target_paths = {root_ckpt_path}

        if run_id and run_id != "DEFAULT_RUN":
            models_dir = os.path.join(app_root, "deployed_models", run_id)
            os.makedirs(models_dir, exist_ok=True)
            target_paths.add(os.path.join(models_dir, "checkpoint.json"))

        # ----------------------------------------------------------------------
        # 5. STRUCTURED CHECKPOINT PAYLOAD SERIALIZATION
        # ----------------------------------------------------------------------
        checkpoint_data = {
            "run_id": run_id,
            "generation": gen,                  # Dynamic active generation
            "current_generation": gen,          # Dual key for CLI compatibility
            "max_generations": max_gen,          # Dynamic configurable ceiling
            "evaluated_count": eval_count,
            "total_population": len(population),
            "chromosome_population": population,
            "chromosomes": population,          # Dual key for Go CLI inspector
            "timestamp": time.time()
        }

        # ----------------------------------------------------------------------
        # 6. ATOMIC DISK FLUSH (TEMP WRITE -> OS SYNC -> REPLACE)
        # ----------------------------------------------------------------------
        for path in target_paths:
            try:
                tmp_path = f"{path}.tmp"
                with open(tmp_path, "w") as f:
                    json.dump(checkpoint_data, f, indent=2)
                    f.flush()
                    os.fsync(f.fileno())
                os.replace(tmp_path, path)
                logger.info(f"✅ [DISK SUCCESS] Written {len(population)} unique models (Gen {gen}/{max_gen}) ➔ {path}")
            except Exception as e:
                logger.error(f"❌ [DISK ERROR] Failed writing checkpoint payload to {path}: {e}")

        logger.info("")

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
        logger.info("✅ [REDIS] Redis Broker started & validated successfully.")

        try:
            subprocess.run(["redis-cli", "-h", "redis", "CONFIG", "SET", "maxmemory", "4gb"], check=False, stdout=subprocess.DEVNULL)
            subprocess.run(["redis-cli", "-h", "redis", "CONFIG", "SET", "maxmemory-policy", "volatile-lru"], check=False, stdout=subprocess.DEVNULL)
            subprocess.run(["redis-cli", "-h", "redis", "CONFIG", "SET", "maxclients", "10000"], check=False, stdout=subprocess.DEVNULL)
            subprocess.run(["redis-cli", "-h", "redis", "CONFIG", "SET", "timeout", "0"], check=False, stdout=subprocess.DEVNULL)
            logger.info("🔧 [REDIS] Configured limits: 4GB maxmemory | volatile-lru | 10000 maxclients.")
        except Exception as e:
            logger.error(f"⚠️ [REDIS] Failed applying advanced configurations: {e}")

    except Exception:
        print("🚀 [REDIS] Auto-starting Redis Broker daemon (0.0.0.0:6379)...")
        os.system("redis-server --daemonize yes --bind 0.0.0.0")
        time.sleep(2.0)

def purge_redis_queues():
    try:
        subprocess.run(["redis-cli", "-h", "127.0.0.1", "flushall"], capture_output=True, text=True)
        logger.info("🧹 [REDIS] Flushed all stale task queues from broker memory.")
    except Exception as e:
        logger.error(f"⚠️ [REDIS] Could not flush Redis queues: {e}")

def stop_redis_server():
    try:
        subprocess.run(["redis-cli", "-h", "127.0.0.1", "shutdown"], capture_output=True, text=True)
        logger.info("🧹 [REDIS] Redis server daemon stopped cleanly.")
    except Exception:
        pass

# ##############################################################################
# Function Name : get_running_master_pids
#
# Path          : apps/school/ga_master.py
# Author        : Chalearm Saelim & Gemini
#
# Purpose :
#    Inspects /proc command lines to identify active GA Master processes while
#    filtering out CLI status and utility action invocations.
# ##############################################################################
def get_running_master_pids():
    current_pid = os.getpid()
    parent_pid = os.getppid()
    running_pids = []
    
    try:
        for pid_str in os.listdir('/proc'):
            if not pid_str.isdigit():
                continue
            
            pid = int(pid_str)
            # Ignore self and parent subshell launcher PID
            if pid == current_pid or pid == parent_pid:
                continue

            try:
                cmdline_path = os.path.join('/proc', pid_str, 'cmdline')
                if os.path.exists(cmdline_path):
                    with open(cmdline_path, 'rb') as f:
                        cmdline = f.read().decode('utf-8', errors='ignore').replace('\x00', ' ').strip()
                    
                    # Ensure it is running ga_master.py in execution mode (not status/stop)
                    is_ga_master = "ga_master.py" in cmdline or "school" in cmdline
                    is_action = any(x in cmdline for x in ["-action=status", "-action=stop", "-action=terminate", "-action=clear-state", "inspect"])
                    
                    if is_ga_master and not is_action:
                        running_pids.append((pid, cmdline))
            except (PermissionError, FileNotFoundError):
                continue
    except Exception:
        pass
        
    return running_pids
def write_config_and_pid(save_min, save_pct, rotate_min, rotate_mb):
    existing_pids = get_running_master_pids()
    if existing_pids:
        logger.warning(f"⚠️ [WARNING] Active master process already running (PIDs: {existing_pids}). Refusing to spawn duplicate daemon.")
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
        logger.info("ℹ️ [STOP] No running GA Master daemon instance found.")
        return
    for pid, cmd in pids:
        logger.info(f"🛑 [STOP] Sending SIGINT (Graceful Shutdown) to Master PID {pid}...")
        try:
            os.kill(pid, signal.SIGINT)
            logger.info(f"✅ [STOP] Signal sent. Master process {pid} saving checkpoint and exiting.")
        except ProcessLookupError:
            logger.error(f"⚠️ [STOP] Process {pid} not found.")

def terminate_all_cluster_processes():
    pids = get_running_master_pids()
    for pid, cmd in pids:
        logger.info(f"🔨 [TERMINATE] Sending SIGKILL (-9) to Master PID {pid}...")
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
                        logger.info(f"🔨 [TERMINATE] Killing Go daemon wrapper PID {pid}...")
                        os.kill(pid, signal.SIGKILL)
            except (PermissionError, FileNotFoundError):
                continue
    except Exception:
        pass

    purge_redis_queues()
    stop_redis_server()
    logger.info("✅ [TERMINATE] Cluster sweep complete. All processes and Redis server stopped cleanly.")

# ##############################################################################
# Function Name : print_cluster_status
#
# Path          : apps/school/ga_master.py
# Author        : Chalearm Saelim & Gemini
#
# Purpose :
#    Queries local process locks, inspects active JSON checkpoint state, and
#    interrogates the Redis Celery broker to render real-time worker pool
#    telemetry, active fold training assignments, candidate model names, and
#    cluster performance metrics.
# ##############################################################################
def print_cluster_status():
    logger.info("🔍 [CLI ACTION] Interrogating cluster state and worker pool telemetry...")
    logger.info("=" * 75)
    logger.info("📊 DISTRIBUTED GA-LSTM CLUSTER TELEMETRY & STATUS REPORT")
    logger.info("=" * 75)

    # --------------------------------------------------------------------------
    # INTERNAL TELEMETRY FORMATTING HELPER
    # --------------------------------------------------------------------------
    def _format_active_task_telemetry(task_dict: dict) -> str:
        task_name = task_dict.get("name", "")
        args = task_dict.get("args", [])
        
        payload = args[0] if len(args) > 0 and isinstance(args[0], (dict, str)) else {}
        if isinstance(payload, str):
            try:
                import json
                payload = json.loads(payload)
            except Exception:
                payload = {}

        # 1. Format export_and_plot_task (GEN_PLOT or POST_GA_OOS)
        if "export_and_plot_task" in task_name:
            gen_idx = payload.get("gen_idx", "?")
            chroms = payload.get("top_chromosomes", [])
            task_type = payload.get("task_type", "GEN_PLOT")
            
            if task_type == "POST_GA_OOS":
                return f"Target: Gen {gen_idx} Out-Of-Sample Verification"

            if chroms and isinstance(chroms, list):
                first_cand_id = chroms[0].get("id", f"G{gen_idx}-M0")
                num_cands = len(chroms)
                return f"Target: Gen {gen_idx} ({first_cand_id} + {num_cands - 1} top models)"
            else:
                return f"Target: Gen {gen_idx} Pareto Plot"

        # 2. Format run_fold_training_task (CV_FOLD)
        elif "run_fold_training_task" in task_name:
            model_id = payload.get("chrom_id", payload.get("chromosome_id", payload.get("model_id", "Model")))
            fold_idx = payload.get("fold_idx", "?")
            num_folds = payload.get("num_folds", payload.get("total_folds", 4))
            return f"Target: {model_id:<8} (Fold {fold_idx}/{num_folds})"

        # 3. Default Fallback
        return f"Target: {payload.get('chrom_id', 'Model')}"

    # 1. Master Process Status
    master_pids = get_running_master_pids()
    if master_pids:
        logger.info("🧠 [GA MASTER DAEMON]: 🟢 ONLINE / RUNNING")
        for pid, cmd in master_pids:
            logger.info(f"   ├── Process PID : {pid}")
            logger.info(f"   └── Command Line: {cmd[:65]}")
    else:
        logger.info("🧠 [GA MASTER DAEMON]: 🔴 OFFLINE / STOPPED")

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

            logger.info(f"💾 [CHECKPOINT STATE]: 🟢 VALID")
            logger.info(f"   ├── Active Run ID : {run_id}")
            logger.info(f"   ├── Current Gen   : Generation {max_active_gen}/{max_gen_target}")
            logger.info(f"   ├── Chromosomes   : {len(pop)} total ({eval_count} evaluated | {eval_pct:.1f}%)")
            logger.info(f"   └── Last Saved    : {ts_str} ({ts_raw})")
        except Exception as e:
            logger.error(f"💾 [CHECKPOINT STATE]: ⚠️ CORRUPTED ({e})")
    else:
        logger.warning("💾 [CHECKPOINT STATE]: ⚪ NO CHECKPOINT FILE FOUND")

    # 3. Worker Pool Telemetry & Active Task Inspector
    logger.info("🏋️ [REGISTERED WORKER POOL TELEMETRY]:")
    try:
        from celery_tasks import app
        inspect = app.control.inspect(timeout=3.0)

        registered = inspect.registered()
        active_tasks = inspect.active()
        stats = inspect.stats()

        if registered:
            logger.info(f"   └── Total Connected Worker Nodes: {len(registered)}")
            for node_name in sorted(registered.keys()):
                node_stats = stats.get(node_name, {}) if stats else {}
                broker_transport = node_stats.get('broker', {}).get('transport', 'redis')
                pool_concurrency = node_stats.get('pool', {}).get('max-concurrency', 'N/A')

                active_list = active_tasks.get(node_name, []) if active_tasks else []
                active_count = len(active_list)
                status_icon = "🏋️ BUSY / TRAINING" if active_count > 0 else "🟢 IDLE / READY"

                logger.info(f"   🖥️  Node Hostname   : {node_name}")
                logger.info(f"      ├── Connection  : ONLINE ({status_icon})")
                logger.info(f"      ├── Broker Type : {broker_transport.upper()}")
                logger.info(f"      ├── Concurrency : {pool_concurrency} Workers")
                logger.info(f"      └── Active Tasks: {active_count} executing")

                if active_count > 0:
                    for task in active_list:
                        t_name = task.get('name', 'tasks.run_fold_training_task')
                        task_desc = _format_active_task_telemetry(task)
                        logger.info(f"         └── ⚡ Executing: {t_name:<28} | {task_desc}")
        else:
            logger.warning("   ⚠️  No active Celery workers detected listening on Redis broker!")
    except Exception as e:
        logger.error(f"   ❌ Error inspecting Redis worker pool: {e}")

    # 4. Historical Runs
    run_folders = glob.glob("logs/*/")
    logger.info(f"📂 [HISTORICAL RUN SUB-DIRECTORIES]: {len(run_folders)} found")
    for folder in run_folders[:5]:
        r_hex = os.path.basename(os.path.normpath(folder))
        logger.info(f"   └── Sub-Directory Run ID: {r_hex}")

    logger.info("=" * 75 + "\n")
def load_config():
    global RUNTIME_CONFIG, ACTIVE_OPTIMIZER
    if os.path.exists(CONFIG_FILE):
        try:
            with open(CONFIG_FILE, "r") as f:
                new_cfg = json.load(f)
                RUNTIME_CONFIG.update(new_cfg)
                logger.info(f"🔄 [GA MASTER] Reloaded Active Configuration: {RUNTIME_CONFIG}\n")

                if ACTIVE_OPTIMIZER is not None:
                    ACTIVE_OPTIMIZER.save_interval_min = RUNTIME_CONFIG.get("save_min", 20)
                    ACTIVE_OPTIMIZER.save_pct = RUNTIME_CONFIG.get("save_pct", 25.0)
                    logger.info("✅ [GA MASTER] Applied updated save intervals to active pipeline.")
        except Exception as e:
            logger.error(f"⚠️ [GA MASTER] Failed to load config: {e}")

def handle_reload_signal(signum, frame):
    logger.info("📩 [SIGNAL RECEIVED] SIGHUP caught! Applying updated runtime parameters...")
    load_config()
# ##############################################################################
# Function Name : pack_and_dispatch_visualization_task
#
# Path          : apps/school/ga_master.py
# Author        : Chalearm Saelim & Gemini
#
# Purpose :
#    Producer-side serializer for Celery visualization tasks. Extracts and slices 
#    the top N Pareto candidate chromosomes from the generation pool, serializes 
#    master training and validation DataFrames into JSON using `orient='split'` 
#    with ISO timestamp formatting, and outputs step-by-step diagnostic telemetry.
#
# Inputs :
#    run_id          : str          Active run identifier (e.g., '2D78AA68').
#    gen_idx         : int          Active generation index.
#    top_chromosomes : list[dict]   Evaluated candidate chromosomes sorted by fitness.
#    master_df       : pd.DataFrame Raw unscaled master training DataFrame.
#    val_df          : pd.DataFrame Raw unscaled validation ground truth DataFrame.
#
# Return :
#    dict : Complete task payload ready for Redis/Celery queue dispatch.
# ##############################################################################
def pack_and_dispatch_visualization_task(run_id: str, gen_idx: int, top_chromosomes: list, master_df, val_df):
    import json
    import pandas as pd
    
    logger.info( "📦" * 40)
    logger.info(f"📦 [PRODUCER TELEMETRY] Packing Data Payload for Celery Visualization Worker...")
    logger.info(f"   ├── Target Run ID     : {run_id}")
    logger.info(f"   ├── Generation Index  : Gen {gen_idx}")
    logger.info(f"   └── Candidate Models  : {len(top_chromosomes)} Pareto Models Packed (TOP_N_EXPORTS)")
    
    # --- PACK MASTER TRAINING DATA ---
    if master_df is not None and not master_df.empty:
        m_df_copy = master_df.copy()
        if isinstance(m_df_copy.index, pd.DatetimeIndex):
            m_df_copy.reset_index(inplace=True)
            
        master_payload = m_df_copy.to_json(orient='split', date_format='iso')
        m_start = str(m_df_copy['timestamp'].iloc[0]).split()[0] if 'timestamp' in m_df_copy else 'Start'
        m_end = str(m_df_copy['timestamp'].iloc[-1]).split()[0] if 'timestamp' in m_df_copy else 'End'
        logger.info(f"   ├── ✅ Master DF Encoded : {len(m_df_copy)} rows x {len(m_df_copy.columns)} cols")
        logger.info(f"   │    ├── 🕒 Master Dates : {m_start} ➔ {m_end}")
        logger.info(f"   │    └── 🧬 Master Cols  : {list(m_df_copy.columns)}")
    else:
        master_payload = None
        logger.error(f"   ├── ❌ [PRODUCER CRITICAL] Master DF is EMPTY!")

    # --- PACK VALIDATION GROUND TRUTH DATA ---
    if val_df is not None and not val_df.empty:
        v_df_copy = val_df.copy()
        if isinstance(v_df_copy.index, pd.DatetimeIndex):
            v_df_copy.reset_index(inplace=True)
            
        val_payload = v_df_copy.to_json(orient='split', date_format='iso')
        v_start = str(v_df_copy['timestamp'].iloc[0]).split()[0] if 'timestamp' in v_df_copy else 'Start'
        v_end = str(v_df_copy['timestamp'].iloc[-1]).split()[0] if 'timestamp' in v_df_copy else 'End'
        logger.info(f"   ├── ✅ Val DF Encoded    : {len(v_df_copy)} rows x {len(v_df_copy.columns)} cols")
        logger.info(f"   │    ├── 🕒 Val Dates    : {v_start} ➔ {v_end}")
        logger.info(f"   │    └── 🧬 Val Cols     : {list(v_df_copy.columns)}")
    else:
        val_payload = None
        logger.warning(f"   ├── ⚠️ [PRODUCER WARN] Validation DF is None/Empty! Green validation lines will be skipped.")

    cand_ids = [c.get('id', f'M{i}') for i, c in enumerate(top_chromosomes)]
    logger.info(f"   └── 🎯 Selected Pareto Candidates ({len(cand_ids)}): {cand_ids}")

    payload = {
        "run_id": run_id,
        "gen_idx": gen_idx,
        "top_chromosomes": top_chromosomes,
        "master_data": master_payload,
        "val_data": val_payload
    }
    
    print("📦" * 40 + "\n")
    return payload
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

    logger.info("\n" + "=" * 80)
    logger.info(f"🧬 [GA MASTER DAEMON v1.2.0] Action: {args.action.upper()}")
    logger.info("=" * 80)
    logger.info(f"   ├── Target Generations : {args.generations} (Configurable / Unbounded)")
    logger.info(f"   ├── Worker/Pop Count   : {args.num}")
    logger.info(f"   ├── Redis Task Buffer  : {args.buffer_size} lookahead tasks")
    logger.info(f"   ├── Warm Start Seeding : {use_warm_start}")
    logger.info(f"   ├── Checkpoint Rules   : Interval={args.save_min}m | Ratio={args.save_pct}%")
    logger.info(f"   └── Log Rotation Rules : Interval={args.rotate_min}m | Max Size={args.rotate_mb}MB")
    logger.info("=" * 80 + "\n")

    # --------------------------------------------------------------------------
    # 3. DIRECT SYNCHRONOUS COMMAND DISPATCHING
    # --------------------------------------------------------------------------
    if args.action == "status":
        logger.info("🔍 [CLI ACTION] Interrogating cluster state and worker pool telemetry...")
        print_cluster_status()
        sys.exit(0)

    elif args.action == "stop":
        logger.info("🛑 [CLI ACTION] Initiating graceful shutdown sequence...")
        stop_master_process()
        if os.path.exists(PID_FILE):
            os.remove(PID_FILE)
            print(f"🧹 [PID CLEANUP] Removed PID lock file: {PID_FILE}")
        sys.exit(0)

    elif args.action == "terminate":
        logger.info("🔨 [CLI ACTION] Initiating full cluster sweep and force-termination...")
        terminate_all_cluster_processes()
        if os.path.exists(PID_FILE):
            os.remove(PID_FILE)
            print(f"🧹 [PID CLEANUP] Removed PID lock file: {PID_FILE}")
        sys.exit(0)

    # Initialize log rotation daemon for operational pipeline modes
    if args.action in ["set-up", "create-work", "restart", "plot", "start"]:
        logger.info(f"🔄 [LOG ROTATOR] Spawning background rotator (Interval: {args.rotate_min}m | Limit: {args.rotate_mb}MB)...")
        start_log_rotation_daemon(rotation_minutes=args.rotate_min, max_size_mb=args.rotate_mb)

    # Register SIGHUP reload signal handler for live parameter updates
    signal.signal(signal.SIGHUP, handle_reload_signal)

    if args.action == "set-up":
        logger.info("🛠️ [SETUP ACTION] Validating Redis Broker & Network Infrastructure...")
        ensure_redis_server_running()
        purge_redis_queues()
        logger.info("✅ [SETUP COMPLETE] Infrastructure validated cleanly.")
        sys.exit(0)

    # --------------------------------------------------------------------------
    # 4. GA MASTER PIPELINE INITIALIZATION & EXECUTION
    # --------------------------------------------------------------------------
    if args.action == "start":
        logger.info("🔒 [PROCESS LOCK] Validating PID file lock state...")
        if not write_config_and_pid(args.save_min, args.save_pct, args.rotate_min, args.rotate_mb):
            logger.error("❌ [PROCESS LOCK ERROR] Active instance detected. Aborting launch.")
            sys.exit(1)

        logger.info("🚀 [PIPELINE INITIALIZATION] Instantiating LSTMOptimizerEngine...")
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

        logger.info(f"🏁 [PIPELINE START] Launching evolution loop up to Target Generation {args.generations}...")
        ACTIVE_OPTIMIZER.execute_pipeline(
            max_generations=args.generations, 
            target_buffer_limit=args.buffer_size
        )
    elif args.action in ["plot", "force-plot"]:
        logger.info("🎨 [CLI ACTION] Rendering missing generation plots from checkpoint...")
        ACTIVE_OPTIMIZER = LSTMOptimizerEngine(
            verbose=args.verbose,
            save_interval_min=args.save_min,
            save_pct=args.save_pct,
        )
        ACTIVE_OPTIMIZER._ingest_data_layers()
        ACTIVE_OPTIMIZER._process_data()
        ACTIVE_OPTIMIZER.render_checkpoint_plots()
        sys.exit(0)
if __name__ == "__main__":
    main()