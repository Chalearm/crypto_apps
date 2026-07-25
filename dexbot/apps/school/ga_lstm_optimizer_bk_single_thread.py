import os
import json
import random
import subprocess
import datetime  # <--- ADD THIS LINE HERE GLOBALLY
import logging
import glob  # <--- Add this line here
import numpy as np
import tensorflow as tf # Assuming Keras/TensorFlow for the LSTM implementation
import pandas as pd  # <--- Add this line
import matplotlib.pyplot as plt
import signal     # Already there
import sys        # <--- ADD THIS
import argparse   # <--- ADD THIS
from sklearn.preprocessing import MinMaxScaler
# 1. Mute TensorFlow engine status logs (0 = all, 1 = no info, 2 = no warnings, 3 = no errors)
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

# 2. Force the standard python logger to suppress underlying framework retracing warnings
logging.getLogger('tensorflow').setLevel(logging.ERROR)
# ==========================================
# GA-LSTM HYPERPARAMETER CONFIGURATION
# ==========================================
POPULATION_SIZE = 32
GENERATIONS = 12
MUTATION_RATE = 0.3

# Chromosome Range Constraints for LSTM
MIN_LOOKBACK_DAYS = 60
MAX_LOOKBACK_DAYS = 150
MIN_FORECAST_DAYS = 30
MAX_FORECAST_DAYS = 60
MIN_HIDDEN_LAYERS = 1
MAX_HIDDEN_LAYERS = 8
MIN_NODES_PER_LAYER = 32
MAX_NODES_PER_LAYER = 512

MIN_LR, MAX_LR = 0.0001, 0.01
MIN_DROPOUT, MAX_DROPOUT = 0.0, 0.6
BATCH_SIZE_CHOICES = [16, 32, 64, 128]  # Best handled as choice blocks rather than random floats
# ==============================================================================
# 🎯 WALK-FORWARD VALIDATION CONFIGURATION
# ==============================================================================
NUM_FOLDS = 7  # Configurable global setting (e.g., set to 6, 7, or 8)
FOLD_TIMEOUT_SECONDS = 360.0  # Maximum time limit allowed per individual fold
# ==============================================================================
# 🚫 USER-DEFINED FEATURE EXCLUSIONS
# ==============================================================================
# Banning only specific volume columns from entering the GA optimization pool
USER_EXCLUDE_FEATURES = ['volume_log_change_fed']
# Setup Logger for "Nice Print Debug"
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger("GA-LSTM-Optimizer")
class DataAnchor:
    def __init__(self, original_path, transformed_path):
        self.original = pd.read_csv(original_path, parse_dates=['timestamp'], index_col='timestamp')
        self.transformed = pd.read_csv(transformed_path, parse_dates=['timestamp'], index_col='timestamp')
        
    def log_return_to_price(self, log_returns, last_known_price):
        """Reverses log return: Price = Price_prev * exp(log_return)"""
        # log_return = ln(Price_t / Price_{t-1})
        # Price_t = Price_{t-1} * exp(log_return)
        prices = [last_known_price]
        for r in log_returns:
            prices.append(prices[-1] * np.exp(r))
        return np.array(prices[1:])

    def get_last_price(self, timestamp):
        """Finds the price immediately preceding the prediction window."""
        return self.original.loc[:timestamp].iloc[-1]['close']

class LSTMOptimizerEngine:
    """
    Genetic Algorithm Engine designed specifically for evolving LSTM 
    hyperparameters and time-series feature selections.
    """
    def __init__(self, data_directory=".", checkpoint_file="lstm_ga_checkpoint.json", verbose=False):
        self.data_directory = data_directory
        self.checkpoint_file = checkpoint_file
        self.verbose = verbose
        self.running = True
        
        # Register signal handlers
        signal.signal(signal.SIGINT, self._handle_exit)
        signal.signal(signal.SIGTERM, self._handle_exit)
        
        self.chromosome_population = []
        self.scaler = MinMaxScaler(feature_range=(-1, 1))
        self.master_data = None  # Explicitly initialize as None
        
        logger.info(f"🚀 [INIT] LSTMOptimizerEngine initialized (Verbose: {self.verbose}).")

    def _process_data(self):
        """Standardizes data after ingestion."""
        if self.master_data is None or self.master_data.empty:
            logger.error("❌ [PROCESS] Cannot scale empty master_data.")
            return

        logger.info(f"🛠️ [PROCESS] Normalizing data with MinMaxScaler (Shape: {self.master_data.shape})...")
        
        # Store a raw copy for inverse transformations later
        self.master_data_raw = self.master_data.copy()
        
        # Scale
        self.master_data = pd.DataFrame(
            self.scaler.fit_transform(self.master_data), 
            columns=self.master_data.columns, 
            index=self.master_data.index
        )
        logger.info("✅ [PROCESS] Normalization complete.")
    def _handle_exit(self, signum, frame):
        """Saves state upon receiving a kill signal."""
        logger.warning(f"⚠️ [SIGNAL] Received signal {signum}. Saving state and exiting...")
        self.running = False
        self._save_checkpoint()
        sys.exit(0)
    def _clean_data(self, df):
        """Filters non-use or null data."""
        # Replace 0 with NaN if 0 is invalid for your price/volume data
        df = df.replace(0, np.nan) 
        # Drop rows where price or other critical columns are missing
        df = df.dropna()
        return df

    def _inverse_transform(self, log_returns, last_price):
        """Converts log returns back to original price scale."""
        return last_price * np.exp(np.cumsum(log_returns))
    def _clear_state(self):
        """Resets the checkpoint file."""
        if os.path.exists(self.checkpoint_file):
            os.remove(self.checkpoint_file)
            logger.info("🧹 [CLEAR] State file deleted.")
        self.chromosome_population = []
    def execute_pipeline(self):
        """Sequential control coordinator for the LSTM-GA optimization pipeline."""
        logger.info("🚀 [PIPELINE] Starting LSTM-GA Evolution sequence...")
        
        if not self._ingest_data_layers():
            logger.error("❌ [PIPELINE] Data Ingestion failed. Terminating.")
            return

        if not self._load_checkpoint():
            logger.info("🌱 [PIPELINE] No checkpoint found. Initializing fresh chromosome population.")
            self._initialize_random_population()
        # 2. Process (New step)
        self._process_data()
        self._evolve_generations()
        self._save_checkpoint()
        logger.info("🏁 [PIPELINE] Evolution pipeline sequence terminated cleanly.")
    def _split_features(self, df):
        """
        Partitions the merged DataFrame into global temporal structures, 
        evolutionary predictive features, and diagnostic tracking parameters.
        Filters out 'close_' columns and exact USER_EXCLUDE_FEATURES from the GA selection pool.
        """
        # 1. Identify patterns for global operational components
        temporal_patterns = ['day_wk_sin', 'day_wk_cos', 'day_yr_sin', 'day_yr_cos', 'hour_sin', 'hour_cos', 'min_sin', 'min_cos']
        
        # 2. Extract strictly temporal tracking attributes
        time_cols = [c for c in df.columns if any(p in c for p in temporal_patterns)]
        
        # 3. Separate functional features from target validation trackers ('close_')
        base_asset_cols = [c for c in df.columns if c not in time_cols and not c.startswith('close_')]
        close_trackers = [c for c in df.columns if c.startswith('close_')]
        
        # 4. Apply exact user-defined feature exclusions
        # Banned list elements must match column names exactly (case-insensitive check)
        banned_lower = [banned.lower() for banned in USER_EXCLUDE_FEATURES]
        asset_cols = [c for c in base_asset_cols if c.lower() not in banned_lower]
        
        # Track items dropped specifically by the user exclusion list
        excluded_dropped = [c for c in base_asset_cols if c not in asset_cols]
        
        # 5. Diagnostics Profiling Report (Only logs if verbose/first run to stop loop spam)
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
            
            # Set state flag to silence repetitive inner loop logs
            self._first_split_done = True
            
        elif self.verbose and False: 
            logger.debug(f"🧬 [FEATURE SPLIT] Quick check: {len(asset_cols)} GA assets active.")

        return time_cols, asset_cols
    def _ingest_data_layers(self) -> bool:
        """
        Scans for files, deduplicates global time features, and merges all assets
        into a single feature-dense DataFrame.
        """
        all_files = glob.glob(os.path.join(self.data_directory, "*_transformed.csv"))
        if not all_files:
            logger.error(f"❌ [INGEST] No files found in {self.data_directory}")
            return False

        logger.info(f"🔍 [INGEST] Found {len(all_files)} files. Starting optimized merge...")

        master_df = None
        global_time_df = None

        for f in all_files:
            try:
                # 1. Load transformed data
                df = pd.read_csv(f)
                df['timestamp'] = pd.to_datetime(df['timestamp'])
                df.set_index('timestamp', inplace=True)
                
                # 2. Extract Global Time Features (Only once)
                if global_time_df is None:
                    time_cols = [c for c in df.columns if any(x in c for x in ['day_', 'hour_', 'min_'])]
                    global_time_df = df[time_cols]
                    logger.info(f"⏳ [INGEST] Extracted {len(time_cols)} global time features.")

                # 3. Load original data for 'close' price
                orig_f = f.replace('_transformed.csv', '.csv')
                orig_df = pd.read_csv(orig_f)
                orig_df['timestamp'] = pd.to_datetime(orig_df['timestamp'])
                orig_df.set_index('timestamp', inplace=True)
                
                # Attach 'close' to transformed df and drop redundant time columns
                df['close'] = orig_df['close']
                df = df.drop(columns=[c for c in df.columns if any(x in c for x in ['day_', 'hour_', 'min_'])])
                
                # Deduplicate and format
                df = df[~df.index.duplicated(keep='first')]
                asset_name = os.path.basename(f).split('_')[0]
                df = df.add_suffix(f'_{asset_name}')
                
                # Merge
                if master_df is None:
                    master_df = df
                    logger.info(f"✅ [INGEST] Initialized master with {asset_name} | Shape: {df.shape}")
                else:
                    master_df = master_df.join(df, how='outer')
                    logger.info(f"✅ [INGEST] Merged {asset_name} | Master Shape: {master_df.shape}")
                    
            except Exception as e:
                logger.error(f"❌ [INGEST] Failed to process {f}: {e}")
                return False

        # 4. Final Merge: Concatenate Master Assets + Global Time Features
        self.master_data = pd.concat([master_df, global_time_df], axis=1)
        
        # 5. Clean up
        self.master_data = self.master_data.interpolate(method='linear').bfill().ffill().fillna(0)
        self.master_data = self.master_data.dropna()

        # 6. Final Report
        self.feature_cols = self.master_data.columns.tolist()
        logger.info(f"🏁 [INGEST] Final Master Data Shape: {self.master_data.shape}")
        logger.info(f"🔍 [INGEST] Final Feature Count: {len(self.feature_cols)}")
        
        # Log feature summary
        time_feats = [c for c in self.feature_cols if any(x in c for x in ['day_', 'hour_', 'min_'])]
        asset_feats = [c for c in self.feature_cols if c not in time_feats]
        logger.info(f"📊 [INGEST] Split: {len(asset_feats)} Asset Features | {len(time_feats)} Global Time Features.")
        
        if self.master_data.empty:
            logger.error("❌ [INGEST] Final master_data is EMPTY.")
            return False
            
        return True
    def _prepare_lstm_dataset(self, chromosome):
        # 1. Identify features
        time_cols, asset_cols = self._split_features(self.master_data)
        
        # 2. Safety Check: Handle Mask Mismatch
        mask = np.array(chromosome['feature_mask'])
        if len(mask) != len(asset_cols):
            mask = np.ones(len(asset_cols), dtype=int)
            chromosome['feature_mask'] = mask.tolist()

        # 3. Data Extraction
        asset_values = self.master_data[asset_cols].values[:, mask == 1]
        time_values = self.master_data[time_cols].values
        combined_data = np.hstack([asset_values, time_values])
        
        lookback = int(chromosome.get('lookback_window', 30))
        forecast = int(chromosome.get('forecast_horizon', 1))
        
        # 4. Initialize placeholders to prevent UnboundLocalError
        X_arr, y_arr = np.array([]), np.array([])
        
        # 5. Generate Sliding Window Sequences
        num_samples = len(combined_data) - lookback - forecast
        if num_samples > 0:
            X, y = [], []
            for i in range(num_samples):
                X.append(combined_data[i : (i + lookback)])
                y.append(asset_values[i + lookback + forecast])
            X_arr, y_arr = np.array(X), np.array(y)
        else:
            logger.warning(f"⚠️ [DATA] Dataset too small for lookback:{lookback} horizon:{forecast}")
            
        return X_arr, y_arr, chromosome['feature_mask']
    def _build_and_train_lstm(self, chromosome, X, y):
        """Dynamically builds and trains an LSTM network with integrated execution limits."""
        import tensorflow as tf
        import time
        
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
            f"⏳ [TRAIN] Model {chromosome['id']} | **FOLD {fold_num}/{NUM_FOLDS}** | "
            f"Samples: {X.shape[0]} | Matrix: ({num_timesteps}d, {num_features}f) | "
            f"Batch: {batch_size} | LR: {lr:.5f}"
        )
        
        # ⏱️ START REAL-TIME EXECUTION CLOCK COUNTER
        start_time = time.perf_counter()
        
        # Custom Callback loop to enforce the training timeout constraints per epoch
        class TimeoutCallback(tf.keras.callbacks.Callback):
            def on_epoch_end(self, epoch, logs=None):
                elapsed = time.perf_counter() - start_time
                if elapsed > FOLD_TIMEOUT_SECONDS:
                    self.model.stop_training = True
                    
        history = model.fit(
            X, y, 
            epochs=5, 
            batch_size=batch_size, 
            verbose=0,
            callbacks=[TimeoutCallback()]
        )
        
        # ⏱️ STOP CLOCK AND CALCULATE DIFFERENCES
        end_time = time.perf_counter()
        duration = end_time - start_time
        
        # Track the global maximum fold duration on the engine instance securely
        if not hasattr(self, 'global_max_fold_time'):
            self.global_max_fold_time = 0.0
        self.global_max_fold_time = max(self.global_max_fold_time, duration)
        
        final_loss = history.history['loss'][-1]
        
        if duration > FOLD_TIMEOUT_SECONDS:
            logger.warning(f"⚠️  [TIMEOUT ALERT] FOLD {fold_num}/{NUM_FOLDS} forced cutoff! Elapsed: {duration:.4f} secs.")
        else:
            logger.info(f"📈 [TRAIN] FOLD {fold_num}/{NUM_FOLDS} Complete for {chromosome['id']} | Loss: {final_loss:.6f} | Execution Duration: {duration:.4f} secs")
            
        return model, history
    def _create_random_chromosome(self, child_id="G1-ID0") -> dict:
        """
        Generates a randomized chromosome configuration constrained within the 
        global boundary ranges defined for the LSTM architecture and training parameters.
        """
        import random

        # 1. Determine structural hidden depth layers
        layers = random.randint(MIN_HIDDEN_LAYERS, MAX_HIDDEN_LAYERS)
        
        # 2. Extract structural column layout context for feature mask sizing
        # Separates asset predictors from time parameters and validation labels
        time_cols, asset_cols = self._split_features(self.master_data)
        
        # 3. Initialize a randomized bitmask for active training feature metrics
        feature_mask = [random.choice([0, 1]) for _ in range(len(asset_cols))]
        
        # Safety enforcement: At least one feature must be active to avoid empty matrix errors
        if sum(feature_mask) == 0:
            feature_mask[random.randint(0, len(feature_mask) - 1)] = 1

        # 4. Construct the complete bounded chromosome object profile
        return {
            "id": child_id,
            "lstm_layers": layers,
            # Assigns a unique node width range dynamically per layer index depth
            "nodes_per_layer": [random.randint(MIN_NODES_PER_LAYER, MAX_NODES_PER_LAYER) for _ in range(layers)],
            "lookback_window": random.randint(MIN_LOOKBACK_DAYS, MAX_LOOKBACK_DAYS),
            "forecast_horizon": random.randint(MIN_FORECAST_DAYS, MAX_FORECAST_DAYS),
            
            # Continuous training rate parameterization metrics
            "learning_rate": round(random.uniform(MIN_LR, MAX_LR), 5),
            "dropout_rate": round(random.uniform(MIN_DROPOUT, MAX_DROPOUT), 2),
            "batch_size": random.choice(BATCH_SIZE_CHOICES),
            
            "feature_mask": feature_mask,
            "fitness_score": -float('inf')
        }
    def _evaluate_fitness(self, chromosome, gen, verbose=False):
        """
        Executes a walk-forward cross-validation with forensic baseline auditing.
        Computes dynamic baseline DA per asset, extracts isolated Skill DA values,
        and targets Skill DA mean/stability inside the core NSGA-II optimization vector.
        
        Returns an expanded robust multi-objective performance vector for NSGA-II:
        [Skill_DA_mean, Sharpe_mean, MaxDD_mean, RMSE_mean, Skill_DA_std, Sharpe_std]
        """
        import numpy as np
        import tensorflow as tf
        from sklearn.metrics import root_mean_squared_error
        
        # 1. Ingest baseline dataset tensors
        X, y_all, _ = self._prepare_lstm_dataset(chromosome)
        total_samples = X.shape[0]
        
        lookback = chromosome['lookback_window']
        horizon = chromosome['forecast_horizon']
        val_size = horizon
        train_available = total_samples - val_size
        
        fold_step = train_available // NUM_FOLDS
        if fold_step < 35:
            fold_step = max(35, train_available // (NUM_FOLDS + 1))
            
        # Optimization tracker arrays
        fold_skill_da, fold_sharpe, fold_maxdd, fold_rmse = [], [], [], []
        fold_cagr, fold_profit_factor, fold_calmar = [], [], []
        
        # Dictionary to aggregate per-asset metrics across all executed folds
        # Structure: { ASSET_NAME: { 'baseline': [...], 'model': [...], 'skill': [...] } }
        asset_history_metrics = {}
        
        # Extract the target dataframe labels mapping sequence
        _, target_cols = self._split_features(self.master_data)
        
        # Isolate indices corresponding strictly to asset price return vectors
        price_return_indices = [
            idx for idx, name in enumerate(target_cols) 
            if 'price_log_return' in name and idx < y_all.shape[1]
        ]
        if len(price_return_indices) == 0:
            price_return_indices = list(range(y_all.shape[1]))
        
        # 2. Progressively Expanded Walk-Forward Cross Validation Sequence
        for fold in range(NUM_FOLDS):
            train_end_idx = fold_step * (fold + 1)
            val_end_idx = train_end_idx + val_size
            
            if val_end_idx > total_samples:
                break
                
            X_train, y_train = X[:train_end_idx], y_all[:train_end_idx]
            X_val, y_val = X[train_end_idx:val_end_idx], y_all[train_end_idx:val_end_idx]
            
            if X_train.shape[0] < 30 or X_val.shape[0] == 0:
                continue
                
            chromosome['current_fold_running'] = fold + 1
            model, _ = self._build_and_train_lstm(chromosome, X_train, y_train)
            predictions = model.predict(X_val, verbose=0)
            tf.keras.backend.clear_session()
            
            # Primary structural array sign mappings
            actual_signs = np.sign(y_val)
            pred_signs = np.sign(predictions)
            
            rmse = root_mean_squared_error(y_val, predictions)
            fold_rmse.append(rmse)
            
            fold_assets_skill = []
            
            # Process metrics per individual asset for this specific fold
            for pr_idx in price_return_indices:
                asset_label = target_cols[pr_idx].replace('price_log_return_', '').upper()
                if asset_label not in asset_history_metrics:
                    asset_history_metrics[asset_label] = {'baseline': [], 'model': [], 'skill': []}
                
                asset_targets = y_val[:, pr_idx]
                pos_count = np.sum(asset_targets > 0)
                neg_count = np.sum(asset_targets < 0)
                tot_count = len(asset_targets)
                
                # Calculate majority class baseline DA
                pos_pct = (pos_count / tot_count) if tot_count > 0 else 0.0
                neg_pct = (neg_count / tot_count) if tot_count > 0 else 0.0
                baseline_da = max(pos_pct, neg_pct)
                
                # Calculate actual model predictive DA
                asset_actual_s = np.sign(asset_targets)
                asset_pred_s = np.sign(predictions[:, pr_idx])
                model_da = np.sum(asset_actual_s == asset_pred_s) / tot_count if tot_count > 0 else 0.0
                
                # Derive Skill DA (Edge over naive majority predictor)
                skill_da = model_da - baseline_da
                fold_assets_skill.append(skill_da)
                
                # Store history to aggregate post-fold run tracking matrices
                asset_history_metrics[asset_label]['baseline'].append(baseline_da)
                asset_history_metrics[asset_label]['model'].append(model_da)
                asset_history_metrics[asset_label]['skill'].append(skill_da)
                
            # Aggregate average skill for this fold across all active price assets
            current_fold_avg_skill = float(np.mean(fold_assets_skill))
            fold_skill_da.append(current_fold_avg_skill)
            
            # ==============================================================================
            # 💸 365-DAY CLEAN PRICE-ONLY SIMULATION FRAMEWORK
            # ==============================================================================
            clean_pred_signs = pred_signs[:, price_return_indices]
            clean_y_val = y_val[:, price_return_indices]
            
            strategy_returns = clean_pred_signs * clean_y_val
            portfolio_daily_returns = np.mean(strategy_returns, axis=1)
            
            winning_days = len(portfolio_daily_returns[portfolio_daily_returns > 0])
            losing_days = len(portfolio_daily_returns[portfolio_daily_returns < 0])
            win_ratio = (winning_days / len(portfolio_daily_returns) * 100) if len(portfolio_daily_returns) > 0 else 0.0
            
            worst_portfolio_return = portfolio_daily_returns.min() if len(portfolio_daily_returns) > 0 else 0.0
            best_portfolio_return = portfolio_daily_returns.max() if len(portfolio_daily_returns) > 0 else 0.0
            
            gross_profits = float(np.sum(portfolio_daily_returns[portfolio_daily_returns > 0]))
            gross_losses = float(np.sum(np.abs(portfolio_daily_returns[portfolio_daily_returns < 0])))
            profit_factor = (gross_profits / gross_losses) if gross_losses > 1e-6 else (gross_profits if gross_profits > 0 else 1.0)
            fold_profit_factor.append(profit_factor)
            
            daily_std = np.std(portfolio_daily_returns)
            mean_return = np.mean(portfolio_daily_returns)
            crypto_annualization_factor = np.sqrt(365.0 / horizon)
            sharpe = (mean_return / daily_std * crypto_annualization_factor) if daily_std > 1e-6 else -5.0
            fold_sharpe.append(sharpe)
            
            equity_curve = np.exp(np.cumsum(portfolio_daily_returns))
            running_max = np.maximum.accumulate(equity_curve)
            drawdowns = (equity_curve - running_max) / running_max
            max_dd = np.min(drawdowns) if len(drawdowns) > 0 else -1.0
            abs_max_dd = abs(max_dd)
            fold_maxdd.append(abs_max_dd)
            
            fractional_years = val_size / 365.0
            ending_wealth = equity_curve[-1]
            cagr = (ending_wealth ** (1.0 / fractional_years)) - 1.0 if ending_wealth > 0 else -1.0
            fold_cagr.append(cagr)
            
            calmar = (cagr / abs_max_dd) if abs_max_dd > 1e-5 else (cagr / 0.00001)
            fold_calmar.append(calmar)

        # 3. Post-Loop Verification Check
        actual_executed_folds = len(fold_skill_da)
        if actual_executed_folds == 0:
            return [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]

        # 4. Calculate Objective Averages and Stability Deviations
        skill_da_mean = float(np.mean(fold_skill_da))   # Maximize Objective 1 (Genuine predictive edge)
        sharpe_mean = float(np.mean(fold_sharpe))       # Maximize Objective 2
        maxdd_mean = float(np.mean(fold_maxdd))         # Minimize Objective 3
        rmse_mean = float(np.mean(fold_rmse))           # Minimize Objective 4
        
        skill_da_std = float(np.std(fold_skill_da))     # Minimize Objective 5 (Stability of edge)
        sharpe_std = float(np.std(fold_sharpe))         # Minimize Objective 6
        
        # Consolidated NSGA-II fitness array vector
        objectives_vector = [skill_da_mean, sharpe_mean, maxdd_mean, rmse_mean, skill_da_std, sharpe_std]
        
        # ==============================================================================
        # 📊 DYNAMIC INTER-FOLD PERFORMANCE TRACKING PANEL WITH COMPLETE OVERALL SUMMARY
        # ==============================================================================
        if verbose:
            # Gather aggregate baseline/model metrics metrics across ALL folds and ALL assets combined
            all_baselines = []
            all_models = []
            all_skills = []
            asset_summary_lines = []
            
            for asset, metrics in asset_history_metrics.items():
                a_base = float(np.mean(metrics['baseline']))
                a_model = float(np.mean(metrics['model']))
                a_skill = float(np.mean(metrics['skill']))
                
                all_baselines.append(a_base)
                all_models.append(a_model)
                all_skills.append(a_skill)
                
                asset_summary_lines.append((asset, a_skill))
            
            # Sort assets by performance edge (highest skill first)
            asset_summary_lines.sort(key=lambda x: x[1], reverse=True)
            
            global_avg_baseline = float(np.mean(all_baselines))
            global_avg_model = float(np.mean(all_models))
            global_avg_skill = float(np.mean(all_skills))
            
            logger.info("=" * 65)
            logger.info(f"🏆 [GLOBAL GEN CROSS-VALIDATION SUMMARY] - ID: {chromosome['id']}")
            logger.info(f"   Average Baseline DA : {global_avg_baseline * 100:.1f}%")
            logger.info(f"   Average Model DA    : {global_avg_model * 100:.1f}%")
            logger.info(f"   Average Skill DA    : {global_avg_skill * 100:+.2f}%")
            logger.info("-" * 65)
            logger.info("🎯 [SKILL DA MATRICES PROFILE BY ASSET]:")
            for asset_name, skill_val in asset_summary_lines:
                logger.info(f"   {asset_name.ljust(8)} : {skill_val * 100:+.2f}%")
                
            logger.info("-" * 65)
            logger.info("📊 [SKILL SUMMARY PROFILE MATRICES]")
            logger.info(f"   Best Asset Skill DA  : {max(all_skills) * 100:+.2f}%")
            logger.info(f"   Worst Asset Skill DA : {min(all_skills) * 100:+.2f}%")
            logger.info(f"   Average Skill DA     : {global_avg_skill * 100:+.2f}%")
            logger.info(f"   Median Skill DA      : {float(np.median(all_skills)) * 100:+.2f}%")
            logger.info(f"   Skill DA Std         : {skill_da_std * 100:.2f}%")
            
            logger.info("-" * 65)
            logger.info("📈 [TRADING & RISK METRICS SUMMARY]")
            logger.info(f"   Averages  -> Sharpe: {sharpe_mean:.2f} | MaxDD: {maxdd_mean*100:.2f}% | RMSE: {rmse_mean:.4f}")
            logger.info(f"   Stability -> Sharpe_std: {sharpe_std:.2f}")
            logger.info(f"   Extended  -> Mean CAGR: {np.mean(fold_cagr)*100:.2f}% | Profit Factor: {np.mean(fold_profit_factor):.2f} | Calmar: {np.mean(fold_calmar):.2f}")
            logger.info("=" * 65)
            
        return objectives_vector
    def _log_population_stats(self):
        """Logs how frequently each asset is selected across the entire population."""
        # 1. Identify asset columns
        _, asset_cols = self._split_features(self.master_data)
        
        # 2. Extract all masks
        all_masks = np.array([c['feature_mask'] for c in self.chromosome_population])
        
        # 3. Calculate frequency (percentage)
        selection_counts = np.sum(all_masks, axis=0)
        total_pop = len(self.chromosome_population)
        
        logger.info("📊 [STATS] Population Feature Selection Frequency:")
        for col, count in zip(asset_cols, selection_counts):
            percentage = (count / total_pop) * 100
            # Only log assets that are selected at least 10% of the time to keep logs clean
            if percentage > 10:
                logger.info(f"   - {col:30} : {percentage:5.1f}%")
    def _get_normal_random(self, min_val, max_val):
        """Generates a value from a normal distribution centered between min and max."""
        mu = (min_val + max_val) / 2
        sigma = (max_val - min_val) / 6  # 6 sigma covers 99.7% of the range
        val = random.gauss(mu, sigma)
        # Clip to ensure we stay within legal bounds
        return int(max(min_val, min(max_val, val)))
    def _initialize_random_population(self):
        """Seeding initial LSTM structural, temporal, and continuous training parameters with dynamic constraints."""
        import random

        logger.info(f"🔍 [DEBUG] All available columns: {list(self.master_data.columns)}")
        logger.info("🌱 [INIT] Seeding new LSTM chromosome population with expanded hyperparameter matrix...")
        
        # 1. Dynamic constraint calculation to safeguard structural data limits
        max_rows = len(self.master_data)

        # Use the minimum of (your constant) or (the data limit) to avoid sequence sizing crashes
        safe_max_lookback = min(MAX_LOOKBACK_DAYS, int(max_rows * 0.7))
        safe_max_horizon = min(MAX_FORECAST_DAYS, int(max_rows * 0.7))
        
        # Ensure min < max ranges remain mathematically operational
        actual_max_lookback = max(MIN_LOOKBACK_DAYS + 1, safe_max_lookback)
        actual_max_horizon = max(MIN_FORECAST_DAYS + 1, safe_max_horizon)
        
        logger.info(f"🔍 [INIT] Data rows: {max_rows} | Safe Lookback range: {MIN_LOOKBACK_DAYS}-{actual_max_lookback} | Safe Horizon range: {MIN_FORECAST_DAYS}-{actual_max_horizon}")

        # 2. Separate asset optimization columns from time indexes
        time_cols, asset_cols = self._split_features(self.master_data)
        
        # 3. Seed the population array
        self.chromosome_population = []  # Reset active engine population
        for i in range(POPULATION_SIZE):
            # Generate random temporal context windows safely
            lookback = random.randint(MIN_LOOKBACK_DAYS, actual_max_lookback)
            horizon = random.randint(MIN_FORECAST_DAYS, actual_max_horizon)
            
            num_layers = self._get_normal_random(MIN_HIDDEN_LAYERS, MAX_HIDDEN_LAYERS)
            num_features_to_select = random.randint(5, len(asset_cols))
            
            # Create feature bitmask array matching functional constraints
            mask = [1] * num_features_to_select + [0] * (len(asset_cols) - num_features_to_select)
            random.shuffle(mask)
            
            # Generate the new dynamic hyperparameter metrics
            lr_val = round(random.uniform(MIN_LR, MAX_LR), 5)
            dropout_val = round(random.uniform(MIN_DROPOUT, MAX_DROPOUT), 2)
            batch_size_val = random.choice(BATCH_SIZE_CHOICES)
            
            # Construct the expanded chromosome schema profile
            chromosome = {
                "id": f"G1-ID{i}", 
                "lstm_layers": num_layers,
                "nodes_per_layer": [self._get_normal_random(MIN_NODES_PER_LAYER, MAX_NODES_PER_LAYER) for _ in range(num_layers)],
                "lookback_window": lookback,
                "forecast_horizon": horizon,
                
                # --- NEW INJECTED TRAINING METRICS ---
                "learning_rate": lr_val,
                "dropout_rate": dropout_val,
                "batch_size": batch_size_val,
                
                "feature_mask": mask,
                "fitness_score": -float('inf')
            }
            self.chromosome_population.append(chromosome)
            
            # Print explicit creation parameters per seeded individual
            logger.info(
                f"✨ [INIT SEED] Chromosome {chromosome['id']} created -> "
                f"Layers: {chromosome['lstm_layers']} | "
                f"Lookback: {chromosome['lookback_window']}d | Horizon: {chromosome['forecast_horizon']}d | "
                f"LR: {chromosome['learning_rate']:.5f} | Dropout: {chromosome['dropout_rate']:.2f} | "
                f"Batch: {chromosome['batch_size']} | Inputs: {num_features_to_select}/{len(asset_cols)}"
            )
            
        logger.info(f"✅ [INIT] Population seeding completed. {POPULATION_SIZE} active chromosomes loaded into memory.")
        logger.info(f"🔍 [INIT] Standardized optimization search array vector width: {len(asset_cols)} elements.")
    def _trigger_go_backtester(self, chromosome, training_loss) -> float:
        """Invokes Go Backtester with dynamic layer/node configurations."""
        try:
            # Convert node list [128, 64] to "128,64" string
            nodes_str = ",".join(map(str, chromosome['nodes_per_layer'][:chromosome['lstm_layers']]))
            args = [
                "./backtester",
                f"-lookback={chromosome['lookback_window']}",
                f"-forecast={chromosome['forecast_horizon']}",
                f"-loss={training_loss}",
                f"-layers={chromosome['lstm_layers']}",
                f"-nodes={nodes_str}"
            ]
            result = subprocess.check_output(args).decode().strip()
            return float(result)
        except Exception as e:
            logger.error(f"❌ [GO-BACKTESTER] Execution failed: {e}")
            return 0.0
    def _evolve_generations(self):
        """The NSGA-II Evolutionary Loop containing integrated priority sort metrics selectors and full structural layer diagnostics."""
        import random
        import time
        
        for gen in range(GENERATIONS):
            if not self.running: 
                break
            
            gen_num = gen + 1
            logger.info(f"🧬 [NSGA-II MULTI-REGIME] Starting Generation {gen_num}/{GENERATIONS}...")
            
            # 1. Ingest performance profiles
            for i, chromo in enumerate(self.chromosome_population):
                chromo['id'] = f"G{gen_num}-ID{i}"
                
                # ==============================================================================
                # 🖥️ STRICT ENGINE PROFILE DIAGNOSTIC PRINT PANEL
                # ==============================================================================
                logger.info("-" * 60)
                logger.info(f"🧬 [CONFIG] GEN:{gen_num} | ID:{chromo['id']}")
                logger.info(f"🖥️  Structure    : Layers: {chromo['lstm_layers']} | Nodes: {chromo['nodes_per_layer']}")
                logger.info(f"⏳  Windows      : Lookback: {chromo['lookback_window']}d | Horizon: {chromo['forecast_horizon']}d")
                logger.info(f"🎛️  Hyperparams  : LR: {chromo['learning_rate']:.5f} | Dropout: {chromo['dropout_rate']:.2f} | Batch: {chromo['batch_size']}")
                
                if self.verbose:
                    _, asset_cols = self._split_features(self.master_data)
                    selected = [asset_cols[idx] for idx, val in enumerate(chromo['feature_mask']) if val == 1]
                    logger.info(f"🔍 [VERBOSE] Active Inputs Selected ({len(selected)}):\n{selected}")
                
                # Run evaluation pipeline
                chromo['perf_vector'] = self._evaluate_fitness(chromo, gen=gen_num, verbose=self.verbose)
            
            # 2. Extract Non-Dominated Pareto Frontier Solutions
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
            
            # Apply your exact priority rules to rank the models sitting on the Pareto front
            pareto_front.sort(key=self._apply_priority_tie_breaker)
            
            logger.info("=" * 60)
            logger.info(f"🏆 [PARETO FRONT] Gen {gen_num} Non-Dominated Pool Size: {len(pareto_front)} models")
            for elite in pareto_front[:3]:
                v = elite['perf_vector']
                logger.info(
                    f"  ⭐ Robust Candidate {elite['id']} -> "
                    f"DA_m: {v[0]*100:.1f}% (std: {v[4]*100:.2f}%) | "
                    f"Sharpe_m: {v[1]:.2f} (std: {v[5]:.2f}) | "
                    f"MaxDD_m: {v[2]*100:.1f}% | RMSE_m: {v[3]:.4f}"
                )
            logger.info("=" * 60)

            if len(pareto_front) > 0:
                self._plot_prediction_comparison(pareto_front[0], file_path="prediction_result/master_data.csv")
            
            self._save_checkpoint()
            
            # ⏱️ Generation remaining worst-case timer estimator engine
            if hasattr(self, 'global_max_fold_time') and self.global_max_fold_time > 0:
                generations_remaining = GENERATIONS - gen_num
                total_remaining_folds = generations_remaining * POPULATION_SIZE * NUM_FOLDS
                worst_case_seconds = total_remaining_folds * self.global_max_fold_time
                hours = int(worst_case_seconds // 3600)
                minutes = int((worst_case_seconds % 3600) // 60)
                logger.info("-" * 60)
                logger.info(f"⏱️  [SEARCH STATS] Longest Observed Fold Time : {self.global_max_fold_time:.4f} secs")
                logger.info(f"⏱️  [TIME LEFT] Max Estimated Search Grid Time Remaining: {hours}h {minutes}m ({worst_case_seconds:.2f} total secs)")
                logger.info("-" * 60)
            
            # 3. Elite Seed Generation Step
            new_pop = list(pareto_front)
            if len(new_pop) > POPULATION_SIZE // 2:
                new_pop = new_pop[:POPULATION_SIZE // 2]
                
            while len(new_pop) < POPULATION_SIZE and len(pareto_front) > 0:
                parent_a = random.choice(pareto_front)
                parent_b = random.choice(pareto_front)
                
                child = {
                    "id": f"G{gen_num}-ID{len(new_pop)}",
                    "lstm_layers": parent_a['lstm_layers'],
                    "nodes_per_layer": parent_a['nodes_per_layer'],
                    "lookback_window": parent_a['lookback_window'],
                    "forecast_horizon": parent_a['forecast_horizon'],
                    "learning_rate": parent_a['learning_rate'],
                    "dropout_rate": parent_a['dropout_rate'],
                    "batch_size": parent_a['batch_size'],
                    "feature_mask": parent_b['feature_mask'], 
                    "perf_vector": [0.0, -5.0, 1.0, 999.0, 1.0, 99.0]
                }
                
                child = self._mutate(child)
                new_pop.append(child)
                
            self.chromosome_population = new_pop if len(new_pop) > 0 else self._initialize_random_population()
            self._log_population_stats()
            logger.info(f"✅ [NSGA-II] Generation {gen_num} execution cycle complete.\n")
            
        # ==============================================================================
        # 🚀 POST-EVOLUTION PRODUCTION DEPLOYMENT
        # ==============================================================================
        if self.running and len(self.chromosome_population) > 0:
            final_front = []
            for c_target in self.chromosome_population:
                dominated = False
                for c_competitor in self.chromosome_population:
                    if 'perf_vector' in c_competitor and 'perf_vector' in c_target:
                        if self._check_pareto_dominance(c_competitor['perf_vector'], c_target['perf_vector']):
                            dominated = True
                            break
                if not dominated:
                    final_front.append(c_target)
            
            final_front.sort(key=self._apply_priority_tie_breaker)
            ultimate_winner = final_front[0] if len(final_front) > 0 else self.chromosome_population[0]
            
            logger.info("=" * 60)
            logger.info(f"🏁 [DEPLOYMENT SUCCESS] Selected most stable, non-dominated candidate: {ultimate_winner['id']}")
            v = ultimate_winner['perf_vector']
            logger.info(f"📊 Retain Specs Summary -> DA: {v[0]*100:.2f}% (±{v[4]*100:.2f}%) | Sharpe: {v[1]:.2f} (±{v[5]:.2f})")
            logger.info("=" * 60)
            
            X_full, y_full, _ = self._prepare_lstm_dataset(ultimate_winner)
            self.export_trained_model(ultimate_winner, X_full, y_full, export_dir="deployed_models")
    def _mutate(self, chromosome: dict) -> dict:
        """
        Mutates the genes of a chromosome with a probability determined by MUTATION_RATE.
        Enforces exact global boundary constraints for all parameters.
        """
        import random

        # Use the global configuration mutation threshold
        rate = MUTATION_RATE  

        # 1. Mutate architectural hidden layers & node counts
        if random.random() < rate:
            chromosome['lstm_layers'] = random.randint(MIN_HIDDEN_LAYERS, MAX_HIDDEN_LAYERS)
            # Regenerate the node configuration matching the new depth profile
            chromosome['nodes_per_layer'] = [
                self._get_normal_random(MIN_NODES_PER_LAYER, MAX_NODES_PER_LAYER) 
                for _ in range(chromosome['lstm_layers'])
            ]
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: structural depth & node distribution.")

        # 2. Mutate lookback window size within safe constraints
        if random.random() < rate:
            # Recalculate safe operational boundaries based on current master dataset size
            max_rows = len(self.master_data)
            safe_max_lookback = min(MAX_LOOKBACK_DAYS, int(max_rows * 0.7))
            actual_max_lookback = max(MIN_LOOKBACK_DAYS + 1, safe_max_lookback)
            
            chromosome['lookback_window'] = random.randint(MIN_LOOKBACK_DAYS, actual_max_lookback)
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: lookback_window ({chromosome['lookback_window']}d).")

        # 3. Mutate forecast horizon target frame within safe constraints
        if random.random() < rate:
            max_rows = len(self.master_data)
            safe_max_horizon = min(MAX_FORECAST_DAYS, int(max_rows * 0.7))
            actual_max_horizon = max(MIN_FORECAST_DAYS + 1, safe_max_horizon)
            
            chromosome['forecast_horizon'] = random.randint(MIN_FORECAST_DAYS, actual_max_horizon)
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: forecast_horizon ({chromosome['forecast_horizon']}d).")

        # 4. Mutate Learning Rate
        if random.random() < rate:
            chromosome['learning_rate'] = round(random.uniform(MIN_LR, MAX_LR), 5)
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: learning_rate ({chromosome['learning_rate']:.5f}).")

        # 5. Mutate Dropout Rate
        if random.random() < rate:
            chromosome['dropout_rate'] = round(random.uniform(MIN_DROPOUT, MAX_DROPOUT), 2)
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: dropout_rate ({chromosome['dropout_rate']:.2f}).")

        # 6. Mutate Batch Size
        if random.random() < rate:
            chromosome['batch_size'] = random.choice(BATCH_SIZE_CHOICES)
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: batch_size ({chromosome['batch_size']}).")

        # 7. Mutate Evolutionary Feature Mask Vector Array
        if random.random() < rate:
            mask = chromosome['feature_mask']
            mutate_idx = random.randint(0, len(mask) - 1)
            # Execute standard single bit-flip operator
            mask[mutate_idx] = 1 - mask[mutate_idx]  
            
            # Critical Safety Check: Ensure the mask has not dropped to an empty feature profile
            if sum(mask) == 0:
                mask[mutate_idx] = 1
                
            chromosome['feature_mask'] = mask
            logger.info(f"🔧 [MUTATE] ID:{chromosome['id']} mutated gene: feature selection bitmask array vector.")

        return chromosome
    def _check_pareto_dominance(self, vector_a, vector_b):
        """
        Evaluates strict multi-objective Pareto dominance for 6 properties.
        Vector layout: [DA_mean, Sharpe_mean, MaxDD_mean, RMSE_mean, DA_std, Sharpe_std]
        """
        # cond1 checks if vector_a is at least as good as vector_b in all dimensions
        cond1 = (
            vector_a[0] >= vector_b[0] and  # 1. DA_mean (Maximize)
            vector_a[1] >= vector_b[1] and  # 2. Sharpe_mean (Maximize)
            vector_a[2] <= vector_b[2] and  # 5. MaxDD_mean (Minimize)
            vector_a[3] <= vector_b[3] and  # 6. RMSE_mean (Minimize)
            vector_a[4] <= vector_b[4] and  # 3. DA_std (Minimize)
            vector_a[5] <= vector_b[5]      # 4. Sharpe_std (Minimize)
        )
        # cond2 checks if vector_a is strictly superior in at least one dimension
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
        """
        Transforms the audited 6-D performance vector into a strict positional comparison tuple.
        1. High Skill_DA_mean (Desc) -> 2. High Sharpe_mean (Desc) -> 3. Low Skill_DA_std (Asc)
        -> 4. Low Sharpe_std (Asc) -> 5. Low MaxDD_mean (Asc) -> 6. Low RMSE_mean (Asc)
        """
        v = chromosome['perf_vector']
        # Invert maximized objectives for uniform ascending sort logic compatibility
        return (-v[0], -v[1], v[4], v[5], v[2], v[3])
    def _save_checkpoint(self):
        """Persists the current population state to a JSON file."""
        state = {
            "population": self.chromosome_population,
            "timestamp": datetime.datetime.now().isoformat()
        }
        try:
            with open(self.checkpoint_file, 'w') as f:
                json.dump(state, f, indent=4)
            logger.info(f"💾 [SAVE] Checkpoint written to disk: {self.checkpoint_file}")
        except Exception as e:
            logger.error(f"❌ [SAVE] Could not save checkpoint: {e}")
    def _load_checkpoint(self):
        """Loads previous evolutionary state safely and enforces dictionary types."""
        if not os.path.exists(self.checkpoint_file):
            logger.info("🌱 [RESTORE] No checkpoint found. Starting fresh.")
            return False

        try:
            with open(self.checkpoint_file, 'r') as f:
                data = json.load(f)
            
            raw_population = data.get("population", [])
            valid_population = []
            
            # CRITICAL SANITY CHECK: Ensure we only ingest well-formed dictionaries
            for item in raw_population:
                if isinstance(item, dict):
                    valid_population.append(item)
                elif isinstance(item, str):
                    try:
                        # Attempt rescue if it's a stringified JSON object
                        parsed_item = json.loads(item)
                        if isinstance(parsed_item, dict):
                            valid_population.append(parsed_item)
                    except json.JSONDecodeError:
                        continue
            
            if not valid_population:
                logger.warning("⚠️ [RESTORE] Checkpoint contained no valid dictionary structures. Starting fresh.")
                return False
                
            self.chromosome_population = valid_population
            logger.info(f"♻️ [RESTORE] Previous LSTM-GA state recovered successfully ({len(self.chromosome_population)} chromosomes loaded).")
            return True

        except Exception as e:
            logger.error(f"❌ [RESTORE] Failed to load checkpoint cleanly: {e}")
            return False
    def _plot_prediction_comparison(self, chromosome, file_path):
        import matplotlib.pyplot as plt
        import os
        import datetime
        import numpy as np

        logger.info(f"📊 [PLOT] Generating metric-aware visualization and data dump for {chromosome['id']}...")
        
        # 1. Prepare data
        X, y_actual_scaled, _ = self._prepare_lstm_dataset(chromosome)
        if X.size == 0 or y_actual_scaled.size == 0:
            logger.warning(f"⚠️ [PLOT] ID:{chromosome['id']} has no data. Skipping.")
            return

        model, _ = self._build_and_train_lstm(chromosome, X, y_actual_scaled)
        y_pred_scaled = model.predict(X, verbose=0)
        
        # Identify horizon context
        horizon = chromosome.get('forecast_horizon', 30)
        total_steps = len(y_actual_scaled)
        horizon_start_idx = max(0, total_steps - horizon)

        # Inline classification of optimization pool columns
        temporal_patterns = ['day_wk_sin', 'day_wk_cos', 'day_yr_sin', 'day_yr_cos', 'hour_sin', 'hour_cos', 'min_sin', 'min_cos']
        time_cols = [c for c in self.master_data.columns if any(p in c for p in temporal_patterns)]
        asset_cols = [c for c in self.master_data.columns if c not in time_cols and not c.startswith('close_')]
        
        active_indices = [i for i, val in enumerate(chromosome['feature_mask']) if val == 1]
        selected_features = [asset_cols[i] for i in active_indices]
        
        num_outputs = y_actual_scaled.shape[1]
        max_plots_per_file = 3
        chunks = [selected_features[i:i + max_plots_per_file] for i in range(0, num_outputs, max_plots_per_file)]
        
        timestamp = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
        output_dir = "prediction_result"
        os.makedirs(output_dir, exist_ok=True)

        # Initialize diagnostic text file logs
        txt_filename = f"metrics_{chromosome['id']}_{timestamp}.txt"
        txt_path = os.path.join(output_dir, txt_filename)
        
        with open(txt_path, "w") as txt_file:
            txt_file.write(f"============================================================\n")
            txt_file.write(f"LSTM-GA HYPERPARAMETER METRICS REPORT: {chromosome['id']}\n")
            txt_file.write(f"Timestamp: {timestamp} | Horizon Window: {horizon} Days\n")
            txt_file.write(f"============================================================\n\n")

            # Loop through plot image chunks
            for chunk_idx, feature_chunk in enumerate(chunks):
                chunk_size = len(feature_chunk)
                fig, axes = plt.subplots(chunk_size, 1, figsize=(12, 4 * chunk_size), constrained_layout=True)
                if chunk_size == 1: 
                    axes = [axes]
                    
                for sub_idx, feat_name in enumerate(feature_chunk):
                    ax = axes[sub_idx]
                    global_idx = chunk_idx * max_plots_per_file + sub_idx
                    
                    master_col_idx = self.master_data.columns.get_loc(feat_name)
                    
                    # Inverse scale the values
                    actual_scaled_feat = y_actual_scaled[:, global_idx]
                    pred_scaled_feat = y_pred_scaled[:, global_idx]
                    feat_min = self.scaler.min_[master_col_idx]
                    feat_scale = self.scaler.scale_[master_col_idx]
                    
                    actual_unscaled = (actual_scaled_feat - feat_min) / feat_scale
                    pred_unscaled = (pred_scaled_feat - feat_min) / feat_scale
                    
                    # Track string descriptors
                    ylabel_str = "Value"
                    title_str = f"Feature Tracking: {feat_name.upper()}"
                    
                    if 'price_log_return_' in feat_name:
                        asset_base = feat_name.replace('price_log_return_', '')
                        close_col = f'close_{asset_base}'
                        if close_col in self.master_data_raw.columns:
                            base_prices = self.master_data_raw[close_col].iloc[-(len(actual_unscaled) + 1):-1].values
                            actual_plot = base_prices * np.exp(actual_unscaled)
                            pred_plot = base_prices * np.exp(pred_unscaled)
                            ylabel_str = "Price (USD)"
                            title_str = f"1-Step-Ahead Price Forecast: {asset_base.upper()}"
                        else:
                            actual_plot = actual_unscaled
                            pred_plot = pred_unscaled
                    elif 'volume_log_change_' in feat_name:
                        asset_base = feat_name.replace('volume_log_change_', '')
                        actual_plot = actual_unscaled
                        pred_plot = pred_unscaled
                        ylabel_str = "Log Vol Change"
                        title_str = f"Volume Change Profile: {asset_base.upper()}"
                    else:
                        actual_plot = actual_unscaled
                        pred_plot = pred_unscaled

                    # Write raw unscaled numerical outputs to diagnostic text logs
                    txt_file.write(f"--- Feature Channel: {feat_name} ---\n")
                    txt_file.write(f"Step,Actual_Value,Predicted_Value,Abs_Error,Is_Horizon_Zone\n")
                    for step_idx in range(len(actual_plot)):
                        act_val = actual_plot[step_idx]
                        prd_val = pred_plot[step_idx]
                        abs_err = abs(act_val - prd_val)
                        in_horizon = 1 if step_idx >= horizon_start_idx else 0
                        txt_file.write(f"{step_idx},{act_val:.6f},{prd_val:.6f},{abs_err:.6f},{in_horizon}\n")
                    txt_file.write("\n")

                    # Generate visualization lines
                    ax.plot(actual_plot, label='Actual Data', color='blue', alpha=0.7, linewidth=2)
                    ax.plot(pred_plot, label='Model Prediction', color='red', linestyle='--', alpha=0.7, linewidth=2)
                    
                    # Draw a distinct shaded boundary box showing the out-of-sample forecast horizon limits
                    if horizon_start_idx < total_steps:
                        ax.axvspan(horizon_start_idx, total_steps - 1, color='yellow', alpha=0.15, label=f'Forecast Horizon ({horizon}d)')
                        ax.axvline(horizon_start_idx, color='gold', linestyle=':', alpha=0.8, linewidth=1.5)

                    ax.set_title(title_str, fontsize=11, fontweight='bold')
                    ax.set_ylabel(ylabel_str)
                    ax.set_xlabel("Time Step Index")
                    ax.legend(loc='upper left')
                    ax.grid(True, alpha=0.3)

                # Save sequential visual images _(N).png
                img_filename = f"prediction_{chromosome['id']}_{timestamp}_{chunk_idx + 1}.png"
                img_save_path = os.path.join(output_dir, img_filename)
                
                plt.suptitle(f"Prediction Diagnostics Chunk {chunk_idx + 1} | {chromosome['id']}", fontsize=13)
                plt.savefig(img_save_path)
                plt.close()
                logger.info(f"📸 [PLOT] Visual slice saved successfully to: {img_save_path}")

        logger.info(f"📄 [PLOT] Raw data tracking metrics successfully dumped to: {txt_path}")
    def export_trained_model(self, chromosome, X_data, y_data, export_dir="deployed_models"):
        """
        Trains the specific chromosome configuration on the complete dataset 
        and exports both the Keras LSTM model bundle and the fitted MinMaxScaler.
        """
        import pickle
        import os
        
        # 1. Ensure the export target path structure exists
        os.makedirs(export_dir, exist_ok=True)
        model_id = chromosome['id']
        
        # 2. Retrain the model config to verify fresh target state weights
        logger.info(f"💾 [EXPORT] Finalizing weights training for export: {model_id}...")
        model, _ = self._build_and_train_lstm(chromosome, X_data, y_data)
        
        # 3. Define clean tracking paths
        model_save_path = os.path.join(export_dir, f"lstm_model_{model_id}.keras")
        scaler_save_path = os.path.join(export_dir, f"scaler_{model_id}.pkl")
        meta_save_path = os.path.join(export_dir, f"metadata_{model_id}.json")
        
        try:
            # 4. Save the Keras model using the high-performance native format
            model.save(model_save_path)
            logger.info(f"✅ [EXPORT] Keras model file successfully saved to: {model_save_path}")
            
            # 5. Serialize the fitted MinMaxScaler object for live inference scaling
            with open(scaler_save_path, 'wb') as f:
                pickle.dump(self.scaler, f)
            logger.info(f"✅ [EXPORT] Data processing scaler successfully saved to: {scaler_save_path}")
            
            # 6. Save structural metadata (so your trading app knows lookback windows and feature index keys)
            time_cols, asset_cols = self._split_features(self.master_data)
            active_indices = [i for i, val in enumerate(chromosome['feature_mask']) if val == 1]
            selected_features = [asset_cols[i] for i in active_indices]
            
            metadata = {
                "chromosome_id": model_id,
                "lookback_window": chromosome['lookback_window'],
                "forecast_horizon": chromosome['forecast_horizon'],
                "batch_size": chromosome.get('batch_size', 32),
                "learning_rate": chromosome.get('learning_rate', 0.001),
                "dropout_rate": chromosome.get('dropout_rate', 0.2),
                "selected_features": selected_features
            }
            
            import json
            with open(meta_save_path, 'w') as f:
                json.dump(metadata, f, indent=4)
            logger.info(f"✅ [EXPORT] Structural metadata successfully saved to: {meta_save_path}")
            
        except Exception as e:
            logger.error(f"❌ [EXPORT] Failed to package deployment artifacts: {e}")
def main():
    parser = argparse.ArgumentParser(description="LSTM-GA Optimizer")
    parser.add_argument("-v", "--verbose", action="store_true", help="Show detailed feature list for each population")
    parser.add_argument("-action", type=str, help="Action to perform (e.g., clear-state)")
    args = parser.parse_args()
# Pass the verbose flag to the engine
    optimizer = LSTMOptimizerEngine(verbose=args.verbose)
    if args.action == "clear-state":
        optimizer._clear_state()
    else:
        optimizer.execute_pipeline()

if __name__ == "__main__":
    main()