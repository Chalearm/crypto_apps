#!/usr/bin/env python3
#/******************************************************************************
#* File Name        : ga_master.py
#* Path             : apps/school/ga_master.py
#* Author           : Chalearm Saelim & Gemini
#* System Role      : Central Orchestrator / Master Node (GA Engine)
#* Architecture     : Distributed Client-Server / Master-Worker (Celery + Redis)
#* 
#* DEPENDENCY TREE & STRUCTURAL MAP:
#* ─────────────────────────────────────────────────────────────────────────────
#* [ga_master.py] (Central Orchestrator)
#*    │
#*    ├── Imports ──> [utils.py] (resolve_target_directories)
#*    ├── Reads/Writes ──> [lstm_ga_checkpoint.json] (State Persistence + run_id)
#*    ├── Logs to     ──> [logs/<run_id>/lstm_engine.log] & [logs/<run_id>/chromosome_summary.log]
#*    │
#*    ├── Invokes Async Tasks ──> [celery_tasks.py]
#*    │                                │
#*    │                   (Redis IPC Queue / Message Broker)
#*    │                                │
#*    │  ┌─────────────────────────────┴──────────────────────────────┐
#*    │  ▼                                                            ▼
#* [train_worker.py] (Worker 1 / Worker 2 Docker)        [visualization_worker.py]
#*   - Executes LSTM Fold Cross-Validation                - Autoregressive Forecasting
#*   - Computes Skill DA, Sharpe, RMSE, MaxDD             - Matplotlib Overlay Plots
#*   - Logs to [logs/<run_id>/folds_lifecycle.log]        - Keras Model Exports
#*   - Returns JSON metrics payload                         [deployed_models/<run_id>/]
#*
#* FUNCTION DEPENDENCY MATRIX (Internal Methods):
#* ─────────────────────────────────────────────────────────────────────────────
#* main() 
#*  ├── print_cluster_status()                       [Triggered by -action=status]
#*  ├── stop_master_process()                        [Triggered by -action=stop]
#*  ├── terminate_all_cluster_processes()            [Triggered by -action=terminate]
#*  └── LSTMOptimizerEngine.execute_pipeline()        [Triggered by -action=start]
#*       ├── _ingest_data_layers()
#*       │    └── _load_directory_to_df()
#*       ├── _load_checkpoint() / _initialize_random_population()
#*       │    ├── utils.resolve_target_directories(run_id)
#*       │    └── _setup_run_loggers()
#*       ├── _process_data()
#*       └── _evolve_generations()
#*            ├── _evaluate_population_pipelined()
#*            │    ├── _build_fold_payloads()
#*            │    ├── celery_tasks.run_fold_training_task.delay(p)
#*            │    └── _summarize_chromosome_evaluation()
#*            └── celery_tasks.export_and_plot_task.delay(..., run_id)
#******************************************************************************/ 
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

POPULATION_SIZE = 5
GENERATIONS = 2
MUTATION_RATE = 0.3
TOP_N_EXPORTS = 5
NUM_FOLDS = 7

MIN_LOOKBACK_DAYS, MAX_LOOKBACK_DAYS = 60, 150
MIN_FORECAST_DAYS, MAX_FORECAST_DAYS = 30, 60
MIN_HIDDEN_LAYERS, MAX_HIDDEN_LAYERS = 1, 8
MIN_NODES_PER_LAYER, MAX_NODES_PER_LAYER = 32, 512

MIN_LR, MAX_LR = 0.0001, 0.01
MIN_DROPOUT, MAX_DROPOUT = 0.0, 0.6
BATCH_SIZE_CHOICES = [16, 32, 64, 128]
USER_EXCLUDE_FEATURES = ['volume_log_change_fed']


# ==============================================================================
# MASTER GA-LSTM OPTIMIZER ENGINE CLASS
# ==============================================================================
class LSTMOptimizerEngine:
    def __init__(self, data_directory=".", checkpoint_file="lstm_ga_checkpoint.json", verbose=False):
        self.data_directory = data_directory
        self.checkpoint_file = checkpoint_file
        self.verbose = verbose
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
        
        logger.info(f"🚀 [INIT] LSTMOptimizerEngine Master initialized (Verbose: {self.verbose}).")

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

    def _handle_exit(self, signum, frame):
        logger.warning(f"⚠️ [SIGNAL] Interrupt {signum} received. Persisting state before termination...")
        self.running = False
        self._save_checkpoint()
        logger.info("👋 Master node exited safely.")
        sys.exit(0)

    def _clear_state(self):
        if os.path.exists(self.checkpoint_file):
            os.remove(self.checkpoint_file)
            logger.info("🧹 [CLEAR] State checkpoint file deleted successfully.")
        self.chromosome_population = []

    def execute_pipeline(self):
        logger.info("🚀 [PIPELINE] Starting Distributed GA Evolution sequence...")
        if not self._ingest_data_layers():
            logger.error("❌ [PIPELINE] Ingestion failed. Halting pipeline execution.")
            return

        if not self._load_checkpoint():
            logger.info("🌱 [PIPELINE] No active checkpoint detected. Seeding fresh Population G1.")
            self._initialize_random_population()

        self._process_data()
        self._evolve_generations()
        self._save_checkpoint()
        logger.info("🏁 [PIPELINE] Evolution pipeline finished all generational runs.")

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
            final_df = final_df.interpolate(method='linear').bfill().ffill().fillna(0)
            return final_df.dropna()
        
        return None

    def _ingest_data_layers(self) -> bool:
        logger.info("=" * 75)
        logger.info("🔍 [DATASET INGESTION & FEATURE AUDIT]")
        logger.info("=" * 75)

        # 1. Load Training Set
        train_transform_files = glob.glob(os.path.join(TRANSFORMED_DATA_DIR, "*_transformed.csv"))
        logger.info(f"📂 [TRAIN SET] Found {len(train_transform_files)} transformed CSV files in '{TRANSFORMED_DATA_DIR}'")
        self.master_data = self._load_directory_to_df(TRANSFORMED_DATA_DIR, RAW_DATA_DIR)

        # 2. Load Validation Set
        val_transform_files = glob.glob(os.path.join(VAL_TRANSFORMED_DATA_DIR, "*_transformed.csv"))
        logger.info(f"📂 [VAL SET]   Found {len(val_transform_files)} transformed CSV files in '{VAL_TRANSFORMED_DATA_DIR}'")
        self.val_master_data = self._load_directory_to_df(VAL_TRANSFORMED_DATA_DIR, VAL_RAW_DATA_DIR)

        if self.master_data is None or self.master_data.empty:
            logger.error("❌ [INGEST] Master training dataset is empty or failed to parse!")
            return False

        # 3. Extract Feature Statistics
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

        # 4. Validation Set Audit
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

    def _split_features(self, df):
        temporal_patterns = ['day_wk_sin', 'day_wk_cos', 'day_yr_sin', 'day_yr_cos', 'hour_sin', 'hour_cos', 'min_sin', 'min_cos']
        time_cols = [c for c in df.columns if any(p in c for p in temporal_patterns)]
        base_asset_cols = [c for c in df.columns if c not in time_cols and not c.startswith('close_')]

        banned_lower = [banned.lower() for banned in USER_EXCLUDE_FEATURES]
        excluded_cols = [c for c in base_asset_cols if c.lower() in banned_lower]
        asset_cols = [c for c in base_asset_cols if c.lower() not in banned_lower]

        if getattr(self, '_first_split_done', False) is False:
            logger.info("=" * 70)
            logger.info("📐 [FEATURE CLASSIFICATION & DATASET ARCHITECTURE]")
            logger.info("=" * 70)
            logger.info(f"⏳ Global Time Features ({len(time_cols)}) : {time_cols}")
            logger.info(f"🚫 User Excluded Features ({len(excluded_cols)}) : {excluded_cols if excluded_cols else ['None']}")
            logger.info(f"🧬 GA Evolutionary Pool ({len(asset_cols)}) : {asset_cols}")
            logger.info("=" * 70)
            self._first_split_done = True

        return time_cols, asset_cols

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
        forecast = int(chromosome.get('forecast_horizon', 1))

        num_samples = len(combined_data) - lookback - forecast
        if num_samples > 0:
            X, y = [], []
            for i in range(num_samples):
                X.append(combined_data[i : (i + lookback)])
                y.append(asset_values[i + lookback + forecast])
            return np.array(X), np.array(y), chromosome['feature_mask']
        
        return np.array([]), np.array([]), chromosome['feature_mask']

    def _get_normal_random(self, min_val, max_val):
        mu = (min_val + max_val) / 2
        sigma = (max_val - min_val) / 6
        val = random.gauss(mu, sigma)
        return int(max(min_val, min(max_val, val)))

    def _initialize_random_population(self):
        self.run_id = uuid.uuid4().hex[:8].upper()
        self._setup_run_loggers()

        log_dir, export_dir, plot_dir = resolve_target_directories(self.run_id)

        logger.info(f"🎲 [NEW RUN] Generated Random Hex Run ID: {self.run_id}")
        logger.info(f"📂 [PATHS] Logs: {log_dir} | Models: {export_dir} | Plots: {plot_dir}")
        logger.info("🌱 [INIT] Seeding randomized hyperparameter matrices for Generation 1...")

        _, asset_cols = self._split_features(self.master_data)
        self.chromosome_population = []

        max_rows = len(self.master_data)
        actual_max_lookback = max(MIN_LOOKBACK_DAYS + 1, min(MAX_LOOKBACK_DAYS, int(max_rows * 0.7)))
        actual_max_horizon = max(MIN_FORECAST_DAYS + 1, min(MAX_FORECAST_DAYS, int(max_rows * 0.7)))

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
                "fitness_evaluated": False,
                "perf_vector": [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
            }
            self.chromosome_population.append(chromosome)

        logger.info(f"✅ [INIT] Population seeding complete. {POPULATION_SIZE} models created for Run {self.run_id}.")

    def _build_fold_payloads(self, chromosome):
        import numpy as np

        # 1. Generate full dataset tensors for this chromosome
        X, y_all, _ = self._prepare_lstm_dataset(chromosome)
        total_samples = X.shape[0]

        lookback = chromosome['lookback_window']
        horizon = chromosome['forecast_horizon']

        _, all_asset_cols = self._split_features(self.master_data)
        target_cols = [col for col, m in zip(all_asset_cols, chromosome['feature_mask']) if m == 1]

        # 2. Save scaled tensors to a local disk cache file (0 KB sent through Redis!)
        cache_dir = os.path.join("logs", self.run_id, "cache")
        os.makedirs(cache_dir, exist_ok=True)
        cache_file = os.path.join(cache_dir, f"chrom_{chromosome['id']}.npz")
        
        if not os.path.exists(cache_file):
            np.savez_compressed(cache_file, X=X, y=y_all)

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

            # Lightweight payload sent through Redis (~1 KB)
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

    def _evaluate_population_pipelined(self, population, gen):
        from celery_tasks import run_fold_training_task

        ensure_redis_server_running()

        async_results = {}
        completed_results = {c['id']: [] for c in population}
        expected_folds_count = {}

        logger.info("=" * 75)
        logger.info(f"🧬 [GA GENERATION {gen}] DISPATCHING FOLD TASKS TO CELERY WORKER POOL")
        logger.info("=" * 75)

        for chromosome in population:
            c_id = chromosome['id']
            perf = chromosome.get('perf_vector', [])
            
            if chromosome.get('fitness_evaluated', False) and len(perf) == 6 and perf[3] < 990.0:
                logger.info(f"⏭️ [RESUME] Skipping {c_id} — already fully evaluated.")
                continue

            # Rich hyperparameter logging directly to lstm_engine.log
            _, asset_cols = self._split_features(self.master_data)
            mask = chromosome['feature_mask']
            
            selected_features = [col for col, active in zip(asset_cols, mask) if active == 1]
            masked_off_features = [col for col, active in zip(asset_cols, mask) if active == 0]

            logger.info("-" * 70)
            logger.info(f"🧬 [CHROMOSOME SEED] GEN:{gen} | ID:{c_id}")
            logger.info(f"   ├── Architecture : Layers: {chromosome['lstm_layers']} | Nodes: {chromosome['nodes_per_layer']}")
            logger.info(f"   ├── Windows      : Lookback: {chromosome['lookback_window']}d | Horizon: {chromosome['forecast_horizon']}d")
            logger.info(f"   ├── Hyperparams  : LR: {chromosome['learning_rate']:.5f} | Dropout: {chromosome['dropout_rate']:.2f} | Batch: {chromosome['batch_size']}")
            logger.info(f"   ├── Active Channels ({len(selected_features)}) : {selected_features}")
            logger.info(f"   └── Masked Off ({len(masked_off_features)})      : {masked_off_features}")
            logger.info("-" * 70)

            # Also mirror to dedicated summary logger
            summary_logger.info("-" * 65)
            summary_logger.info(f"🧬 [CHROMOSOME CONFIG] GEN:{gen} | ID:{c_id}")
            summary_logger.info(f"🖥️  Structure    : Layers: {chromosome['lstm_layers']} | Nodes: {chromosome['nodes_per_layer']}")
            summary_logger.info(f"⏳  Windows      : Lookback: {chromosome['lookback_window']}d | Horizon: {chromosome['forecast_horizon']}d")
            summary_logger.info(f"🎛️  Hyperparams  : LR: {chromosome['learning_rate']:.5f} | Dropout: {chromosome['dropout_rate']:.2f} | Batch: {chromosome['batch_size']}")
            summary_logger.info(f"🎯  Features     : Selected {sum(chromosome['feature_mask'])} active channels")
            summary_logger.info("-" * 65)

            payloads = self._build_fold_payloads(chromosome)
            expected_folds_count[c_id] = len(payloads)
            async_results[c_id] = []

            for p in payloads:
                for attempt in range(3):
                    try:
                        async_task = run_fold_training_task.delay(p)
                        async_results[c_id].append((p['fold_idx'], async_task))
                        fold_msg = f"🚀 [TASK QUEUED] Model {c_id} | Fold {p['fold_idx']}/{NUM_FOLDS} sent to Celery queue."
                        fold_logger.info(fold_msg)
                        break
                    except Exception as e:
                        logger.warning(f"⚠️ [REDIS RETRY] Task dispatch retry {attempt+1}/3: {e}")
                        ensure_redis_server_running()
                        time.sleep(1.0)

        # Active poll & completion logging
        total_finished = 0
        total_expected = sum(expected_folds_count.values())

        while any(tasks for tasks in async_results.values()):
            if not self.running:
                break

            for c_id in list(async_results.keys()):
                tasks = async_results[c_id]
                finished_tasks = []

                for fold_idx, task in tasks:
                    if task.ready():
                        finished_tasks.append((fold_idx, task))
                        
                        if task.successful():
                            res = task.result
                            worker_node = res.get('worker_node', 'Unknown Machine')
                            duration = res.get('execution_duration', 0.0)
                            loss = res.get('loss', 0.0)
                            skill_da = res.get('skill_da', 0.0)

                            completion_msg = (
                                f"📈 [TASK COMPLETE] Model {c_id} | Fold {fold_idx}/{NUM_FOLDS} | "
                                f"Worker: {worker_node} | Loss: {loss:.6f} | Skill DA: {skill_da*100:+.2f}% | Time: {duration:.2f}s"
                            )
                            
                            # Print directly to BOTH lstm_engine.log and folds_lifecycle.log
                            logger.info(completion_msg)
                            fold_logger.info(completion_msg)
                            
                            completed_results[c_id].append(res)
                            total_finished += 1
                        else:
                            err_msg = f"💥 [TASK CRASH] Model {c_id} | Fold {fold_idx}/{NUM_FOLDS} failed: {task.result}"
                            logger.error(err_msg)
                            fold_logger.error(err_msg)

                for item in finished_tasks:
                    tasks.remove(item)

                target_count = expected_folds_count.get(c_id, NUM_FOLDS)
                if len(tasks) == 0 and len(completed_results[c_id]) == target_count:
                    target_chrom = next(c for c in population if c['id'] == c_id)
                    if not target_chrom.get('fitness_evaluated', False):
                        objectives = self._summarize_chromosome_evaluation(target_chrom, completed_results[c_id], gen)
                        target_chrom['perf_vector'] = objectives
                        target_chrom['fitness_evaluated'] = True

            time.sleep(1.0)


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

        # Detailed block printed to BOTH summary_logger and main engine logger
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

    def _apply_priority_tie_breaker(self, chromosome):
        v = chromosome['perf_vector']
        return (-v[0], -v[1], v[4], v[5], v[2], v[3])

    def _save_checkpoint(self):
        state = {
            "run_id": self.run_id,
            "generation": self.current_generation,
            "population": self.chromosome_population,
            "timestamp": datetime.datetime.now().isoformat()
        }
        try:
            with open(self.checkpoint_file, 'w') as f:
                json.dump(state, f, indent=4)
        except Exception as e:
            logger.error(f"❌ [CHECKPOINT] Failed to save state: {e}")

    def _load_checkpoint(self):
        if not os.path.exists(self.checkpoint_file):
            return False
        try:
            with open(self.checkpoint_file, 'r') as f:
                data = json.load(f)
            
            self.run_id = data.get("run_id", None)
            self.current_generation = data.get("generation", 0)
            self.chromosome_population = data.get("population", [])
            
            self._setup_run_loggers()
            log_dir, export_dir, plot_dir = resolve_target_directories(self.run_id)
            logger.info(f"♻️ [RESTORE] Resumed Run ID: {self.run_id or 'LEGACY (Root Dir)'}")
            return True
        except Exception:
            return False

    def _evolve_generations(self):
        from celery_tasks import export_and_plot_task

        for gen in range(self.current_generation, GENERATIONS):
            if not self.running:
                break

            self.current_generation = gen
            gen_num = gen + 1
            logger.info(f"🧬 [NSGA-II MULTI-REGIME] Starting Generation {gen_num}/{GENERATIONS}...")

            # --- ID INTEGRITY FIX: Normalize Chromosome IDs for the current active Generation ---
            for idx, chrom in enumerate(self.chromosome_population):
                chrom['id'] = f"G{gen_num}-M{idx}"

            self._evaluate_population_pipelined(self.chromosome_population, gen=gen_num)
            self._save_checkpoint()

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

            logger.info("=" * 70)
            logger.info(f"🏆 [PARETO FRONT SELECTION] Gen {gen_num} Non-Dominated Models: {len(pareto_front)}")
            for rank_idx, elite in enumerate(pareto_front[:5]):
                v = elite['perf_vector']
                logger.info(
                    f"   ⭐ Rank {rank_idx + 1} -> Model {elite['id']} | "
                    f"Skill_DA: {v[0]*100:+.2f}% (std: {v[4]*100:.2f}%) | "
                    f"Sharpe: {v[1]:.2f} (std: {v[5]:.2f}) | "
                    f"MaxDD: {v[2]*100:.1f}% | RMSE: {v[3]:.4f}"
                )
            logger.info("=" * 70)

            if len(pareto_front) > 0:
                logger.info(f"🎨 [OFFLOAD] Offloading Matplotlib Plotting & Model Serialization to Worker VM...")
                plot_payload = {
                "run_id": self.run_id,
                "top_chromosomes": pareto_front[:5],
                "master_data": self.master_data_json,  # or filepath
                "gen_idx": gen_idx
                }
                export_and_plot_task.delay(plot_payload)

            new_pop = []
            # Preserve Pareto Elites
            for elite in pareto_front:
                elite_copy = dict(elite)
                elite_copy['fitness_evaluated'] = True
                new_pop.append(elite_copy)
                if len(new_pop) >= POPULATION_SIZE // 2:
                    break

            # --- ID INTEGRITY FIX: Standardize Offspring Generation Labels ---
            next_gen_num = gen_num + 1
            while len(new_pop) < POPULATION_SIZE and len(pareto_front) > 0:
                parent_a = random.choice(pareto_front)
                parent_b = random.choice(pareto_front)
                
                child = {
                    "id": f"G{next_gen_num}-M{len(new_pop)}",
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
                child['fitness_evaluated'] = False
                child['perf_vector'] = [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
                
                new_pop.append(child)

            self.chromosome_population = new_pop if len(new_pop) > 0 else self._initialize_random_population()
            
            self.current_generation += 1
            self._save_checkpoint()
            logger.info(f"✅ [NSGA-II] Generation {gen_num} execution cycle complete.\n")


# ==============================================================================
# CLI DAEMON CONTROLLER, ENCAPSULATED REDIS, & WORKER MANAGEMENT HELPERS
# ==============================================================================
def ensure_redis_server_running() -> bool:
    """
    Encapsulated Redis Manager: Validates installation, verifies connectivity,
    auto-starts Redis daemon on 0.0.0.0:6379, and purges stale leftover queues.
    """
    if subprocess.call(["which", "redis-cli"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL) != 0:
        print("❌ [REDIS ERROR] redis-cli binary not found in PATH! Please install redis-server.")
        return False

    res = subprocess.run(["redis-cli", "-h", "127.0.0.1", "ping"], capture_output=True, text=True)
    if "PONG" in res.stdout:
        print("ℹ️ [REDIS] Redis Broker verified on 127.0.0.1:6379.")
        return True

    print("🚀 [REDIS] Auto-starting Redis Broker daemon (0.0.0.0:6379)...")
    cmd = ["redis-server", "--bind", "0.0.0.0", "--protected-mode", "no", "--daemonize", "yes"]
    subprocess.run(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(1.0)

    check = subprocess.run(["redis-cli", "-h", "127.0.0.1", "ping"], capture_output=True, text=True)
    if "PONG" in check.stdout:
        print("✅ [REDIS] Redis Broker started & validated successfully.")
        return True
    else:
        print("❌ [REDIS] Failed to auto-start Redis server daemon!")
        return False

def purge_redis_queues():
    """Flushes all stale queued Celery task payloads from Redis memory."""
    try:
        subprocess.run(["redis-cli", "-h", "127.0.0.1", "flushall"], capture_output=True, text=True)
        print("🧹 [REDIS] Flushed all stale task queues from broker memory.")
    except Exception as e:
        print(f"⚠️ [REDIS] Could not flush Redis queues: {e}")

def stop_redis_server():
    """Cleanly stops Redis broker daemon on terminate/shutdown."""
    try:
        subprocess.run(["redis-cli", "-h", "127.0.0.1", "shutdown"], capture_output=True, text=True)
        print("🧹 [REDIS] Redis server daemon stopped cleanly.")
    except Exception:
        pass


def get_running_master_pids():
    """Scans system processes for active ga_master processes excluding current PID."""
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
                    # Match ga_master.py when running pipeline/setup/start
                    if "ga_master.py" in cmdline and not any(x in cmdline for x in ["-action=status", "-action=terminate", "-action=create-work"]):
                        running_pids.append((pid, cmdline.strip()))
            except (PermissionError, FileNotFoundError):
                continue
    except Exception:
        pass
    return running_pids


def get_active_local_celery_workers():
    """Scans /proc for active local Celery worker processes and returns list of (PID, node_name)."""
    workers = []
    try:
        for pid_str in os.listdir('/proc'):
            if not pid_str.isdigit():
                continue
            pid = int(pid_str)
            try:
                cmdline_path = os.path.join('/proc', pid_str, 'cmdline')
                if os.path.exists(cmdline_path):
                    with open(cmdline_path, 'rb') as f:
                        cmdline = f.read().decode('utf-8', errors='ignore').replace('\x00', ' ')
                    if "celery" in cmdline and "worker" in cmdline and "-n" in cmdline:
                        parts = cmdline.split()
                        node_name = "celery_worker"
                        for idx, p in enumerate(parts):
                            if p == "-n" and idx + 1 < len(parts):
                                node_name = parts[idx + 1]
                                break
                        workers.append((pid, node_name))
            except (PermissionError, FileNotFoundError):
                continue
    except Exception:
        pass
    return workers

def resolve_celery_executable():
    """Resolves virtualenv or system celery binary path."""
    venv_celery = os.path.join("venv", "bin", "celery")
    if os.path.exists(venv_celery):
        return venv_celery
    
    alt_venv_celery = os.path.join("apps", "school", "venv", "bin", "celery")
    if os.path.exists(alt_venv_celery):
        return alt_venv_celery
        
    return "celery"
def manage_local_workers(target_count: int):
    """Dynamically scales local Celery worker nodes up or down to match target_count."""
    # 1. Read REDIS_URL if set in environment (e.g., passed from client container), fallback to 127.0.0.1
    redis_url = os.environ.get("REDIS_URL", "redis://127.0.0.1:6379/0")
    os.environ["REDIS_URL"] = redis_url
    
    env = os.environ.copy()
    cwd = os.getcwd()
    env["PYTHONPATH"] = f"{cwd}:{env.get('PYTHONPATH', '')}"
    env["REDIS_URL"] = redis_url  # Ensure sub-process workers inherit the target Redis URL
    
    # Disable GPU probing & silence Celery root warnings in sub-processes
    env["CUDA_VISIBLE_DEVICES"] = "-1"
    env["TF_CPP_MIN_LOG_LEVEL"] = "3"
    env["C_FORCE_ROOT"] = "true"

    active_workers = get_active_local_celery_workers()
    current_count = len(active_workers)

    print(f"⚙️ [WORKER MGMT] Current Active Workers: {current_count} | Target Count: {target_count}")

    if target_count == current_count:
        print(f"✅ [WORKER MGMT] Exactly {target_count} worker(s) running. No scaling required.")
        return

    # Scale Down / Kill All
    if target_count < current_count:
        if target_count == 0:
            print("🔨 [WORKER MGMT] Terminating ALL local worker nodes (-num=0)...")
            for pid, name in active_workers:
                try:
                    os.kill(pid, signal.SIGKILL)
                    print(f"   └── Killed worker node {name} (PID: {pid})")
                except ProcessLookupError:
                    pass
        else:
            num_to_kill = current_count - target_count
            print(f"📉 [WORKER MGMT] Scaling down: Stopping {num_to_kill} worker node(s)...")
            for pid, name in active_workers[:num_to_kill]:
                try:
                    os.kill(pid, signal.SIGTERM)
                    print(f"   └── Scaled down worker node {name} (PID: {pid})")
                except ProcessLookupError:
                    pass
        return

    # Scale Up
    num_to_add = target_count - current_count
    print(f"📈 [WORKER MGMT] Scaling up: Launching {num_to_add} new worker node(s)...")

    existing_names = [w[1] for w in active_workers]
    os.makedirs("logs", exist_ok=True)
    
    celery_bin = resolve_celery_executable()

    for i in range(num_to_add):
        worker_id = len(existing_names) + i + 1
        node_name = f"local_worker_{worker_id}"
        log_file_path = os.path.join("logs", f"{node_name}.log")

        cmd = [
            celery_bin, "-A", "celery_tasks", "worker",
            "-n", node_name,
            "-O", "fair",
            "--prefetch-multiplier=1",
            "--loglevel=info",
            "--concurrency=1"
        ]

        with open(log_file_path, "a") as log_f:
            p = subprocess.Popen(cmd, stdout=log_f, stderr=log_f, env=env)
            print(f"   🚀 Spawned worker node '{node_name}' (PID: {p.pid}) -> Log: {log_file_path}")
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
    """Completely terminates Master instances, Go wrappers, local Celery workers, and Redis daemon."""
    pids = get_running_master_pids()
    for pid, cmd in pids:
        print(f"🔨 [TERMINATE] Sending SIGKILL (-9) to Master PID {pid}...")
        try:
            os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            pass

    # Sweep Go wrapper binaries named 'school'
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

    manage_local_workers(0)
    purge_redis_queues()
    stop_redis_server()
    print("✅ [TERMINATE] Cluster sweep complete. All processes and Redis server stopped cleanly.")

def print_cluster_status():
    print("\n" + "=" * 75)
    print("📊 DISTRIBUTED GA-LSTM CLUSTER TELEMETRY & STATUS REPORT")
    print("=" * 75)

    master_pids = get_running_master_pids()
    if master_pids:
        print("🧠 [GA MASTER DAEMON]: 🟢 ONLINE / RUNNING")
        for pid, cmd in master_pids:
            print(f"   └── Process PID : {pid}")
            print(f"   └── Command Line: {cmd[:65]}")
    else:
        print("🧠 [GA MASTER DAEMON]: 🔴 OFFLINE / STOPPED")

    checkpoint_file = "lstm_ga_checkpoint.json"
    if os.path.exists(checkpoint_file):
        try:
            with open(checkpoint_file, 'r') as f:
                ckpt = json.load(f)
            run_id = ckpt.get("run_id", "LEGACY (Root Dir)")
            gen = ckpt.get("generation", 0) + 1
            pop_count = len(ckpt.get("population", []))
            ts = ckpt.get("timestamp", "N/A")
            print(f"\n💾 [CHECKPOINT STATE]: 🟢 VALID")
            print(f"   ├── Active Run ID : {run_id}")
            print(f"   ├── Current Gen   : Generation {gen}/{GENERATIONS}")
            print(f"   ├── Chromosomes   : {pop_count} loaded")
            print(f"   └── Last Saved    : {ts}")
        except Exception as e:
            print(f"\n💾 [CHECKPOINT STATE]: ⚠️ CORRUPTED ({e})")
    else:
        print("\n💾 [CHECKPOINT STATE]: ⚪ NO CHECKPOINT FILE FOUND")

    print("\n🏋️ [REGISTERED WORKER POOL TELEMETRY]:")
    try:
        from celery_tasks import app
        inspect = app.control.inspect(timeout=3.0)
        
        registered = inspect.registered()
        active_tasks = inspect.active()
        stats = inspect.stats()

        if registered:
            print(f"   └── Total Connected Worker Nodes: {len(registered)}")
            for node_name, tasks_list in registered.items():
                node_stats = stats.get(node_name, {}) if stats else {}
                broker_transport = node_stats.get('broker', {}).get('transport', 'redis')
                pool_concurrency = node_stats.get('pool', {}).get('max-concurrency', 'N/A')
                
                active_count = len(active_tasks.get(node_name, [])) if active_tasks else 0
                status_icon = "🏋️ BUSY / TRAINING" if active_count > 0 else "🟢 IDLE / READY"

                print(f"\n   🖥️  Node Hostname   : {node_name}")
                print(f"      ├── Connection  : ONLINE ({status_icon})")
                print(f"      ├── Broker Type : {broker_transport.upper()}")
                print(f"      ├── Concurrency : {pool_concurrency} Workers")
                print(f"      └── Active Tasks: {active_count} executing")

                if active_count > 0 and active_tasks and node_name in active_tasks:
                    for task in active_tasks[node_name]:
                        t_name = task.get('name', 'Task')
                        print(f"          └── ⚡ Executing: {t_name}")
        else:
            print("   ⚠️  No active Celery workers detected listening on Redis broker!")
    except Exception as e:
        print(f"   ❌ Error inspecting Redis worker pool: {e}")

    run_folders = glob.glob("logs/*/")
    print(f"\n📂 [HISTORICAL RUN SUB-DIRECTORIES]: {len(run_folders)} found")
    for folder in run_folders[:5]:
        r_hex = os.path.basename(os.path.normpath(folder))
        print(f"   └── Sub-Directory Run ID: {r_hex}")

    print("=" * 75 + "\n")


# ==============================================================================
# 🚀 MAIN ENTRY POINT & CLI PARSER
# ==============================================================================
def main():
    parser = argparse.ArgumentParser(description="Distributed GA-LSTM Master Orchestrator Engine")
    parser.add_argument("-v", "--verbose", action="store_true", help="Enable verbose feature classification logging")
    parser.add_argument(
        "-action", 
        type=str, 
        choices=["start", "stop", "status", "terminate", "clear-state", "set-up", "create-work", "restart"], 
        default="start", 
        help="Action: start | stop | status | terminate | clear-state | set-up | create-work | restart"
    )
    parser.add_argument("-num", type=int, default=1, help="Number of local Celery workers for create-work")
    args = parser.parse_args()

    os.environ["REDIS_URL"] = "redis://127.0.0.1:6379/0"

    if args.action == "status":
        print_cluster_status()
        sys.exit(0)

    elif args.action == "stop":
        stop_master_process()
        sys.exit(0)

    elif args.action == "terminate":
        terminate_all_cluster_processes()
        sys.exit(0)

    elif args.action == "create-work":
        manage_local_workers(args.num)
        sys.exit(0)

    elif args.action == "set-up":
        print("🛠️ [SETUP] Validating Environment & Redis Infrastructure...")
        if not ensure_redis_server_running():
            print("❌ [SETUP] Cannot proceed: Redis infrastructure setup failed.")
            sys.exit(1)

        purge_redis_queues()  # <--- Wipe orphan task messages from old runs!

        master_pids = get_running_master_pids()
        if master_pids:
            print("ℹ️ [SETUP] GA Master Engine is already active on system.")
        else:
            print("🚀 [SETUP] Spawning GA Master Engine...")
            optimizer = LSTMOptimizerEngine(verbose=args.verbose)
            optimizer.execute_pipeline()
        sys.exit(0)

    elif args.action == "restart":
        print("🔄 [RESTART] Restarting Cluster Services...")
        stop_master_process()
        time.sleep(1)
        ensure_redis_server_running()
        optimizer = LSTMOptimizerEngine(verbose=args.verbose)
        optimizer.execute_pipeline()
        sys.exit(0)

    optimizer = LSTMOptimizerEngine(verbose=args.verbose)

    if args.action == "clear-state":
        optimizer._clear_state()
    elif args.action == "start":
        optimizer.execute_pipeline()


if __name__ == "__main__":
    main()