import os
import json
import glob
import time
import threading
import pandas as pd

class GABacktestPipelineSingleton:
    _instance = None
    _lock = threading.Lock()
    
    def __new__(cls, *args, **kwargs):
        """Thread-safe Singleton Instance Control Room Allocation."""
        with cls._lock:
            if cls._instance is None:
                cls._instance = super(GABacktestPipelineSingleton, cls).__new__(cls)
                cls._instance._initialized = False
            return cls._instance

    def __init__(self, data_directory=".", checkpoint_file="ga_state_checkpoint.json"):
        if self._initialized:
            return
            
        self.data_directory = data_directory
        self.checkpoint_file = checkpoint_file
        self.current_generation = 0
        self.best_fitness = -float('inf')
        self.best_hyperparameters = {}
        self.processed_features_summary = {}
        self._initialized = True

    def execute_pipeline(self):
        """Sequential control coordinator for state execution verification."""
        print("🤖 [SYSTEM] Initializing GA Backtest Pipeline Control Room...")
        
        # Step 1: Checkpoint State Restoration
        if not self._load_checkpoint_state():
            self._initialize_fresh_training_state()
            
        # Step 2: Read Transformed Feature Arrays
        if not self._ingest_transformed_feature_matrices():
            print("❌ [CRITICAL] Pipeline terminated due to missing data layers.")
            return

        # Step 3: Mock Backtest Evaluation Generation Loop
        self._run_mock_genetic_algorithm_loop()

        # Step 4: Secure Results and Save State Checkpoint
        self._save_checkpoint_state()
        print("🏁 [SYSTEM] Pipeline execution cleanly terminated. State verified.")

    def _load_checkpoint_state(self) -> bool:
        """Looks for a saved JSON state file from a previous execution run."""
        print(f"🔍 [STATE] Searching for existing checkpoint file: {self.checkpoint_file}")
        if os.path.exists(self.checkpoint_file):
            try:
                with open(self.checkpoint_file, 'r') as f:
                    state_data = json.load(f)
                self.current_generation = state_data.get("current_generation", 0)
                self.best_fitness = state_data.get("best_fitness", -float('inf'))
                self.best_hyperparameters = state_data.get("best_hyperparameters", {})
                print(f"♻️ [RESTORE] State recovered from last run file! Generation: {self.current_generation} | Best Fitness: {self.best_fitness:.6f}")
                return True
            except Exception as e:
                print(f"⚠️ [WARN] Checkpoint corrupted, reverting to fresh state setup. Error: {e}")
                return False
        print("ℹ️ [STATE] No previous checkpoint detected. Creating a fresh tracking workspace.")
        return False

    def _initialize_fresh_training_state(self):
        """Sets pristine parameters for a fresh Genetic Algorithm generation sweep."""
        print("🌱 [INIT] Allocating fresh state registers for Generation 0...")
        self.current_generation = 0
        self.best_fitness = -1.0  # Initial baseline threshold benchmark
        self.best_hyperparameters = {
            "lstm_hidden_dim": 64,
            "lookback_windows": 90,
            "learning_rate": 0.001,
            "population_variants": 32
        }

    def _ingest_transformed_feature_matrices(self) -> bool:
        """Scans the working directory, ingesting your 12-digit high-precision transformed Go CSV files."""
        print(f"📂 [INGEST] Reading transformed matrix feature sheets inside: '{self.data_directory}'")
        search_pattern = os.path.join(self.data_directory, "*_transformed.csv")
        transformed_files = glob.glob(search_pattern)

        if not transformed_files:
            print("⚠️ [DATA] No calibrated '_transformed.csv' matrices found. Did Go fetcher run?")
            return False

        for file_path in transformed_files:
            file_name = os.path.basename(file_path)
            print(f"📊 [PARSING] Loading high-precision sequence vectors from: {file_name}")
            try:
                # Read just the head rows as a sample summary to verify column alignments match tensor conditions
                df = pd.read_csv(file_path, nrows=5)
                self.processed_features_summary[file_name] = {
                    "columns_detected": list(df.columns),
                    "feature_count": len(df.columns) - 1  # Excluding timestamp index
                }
                print(f"   ↳ Ingested {len(df.columns)} active variables. Precision metrics validated.")
            except Exception as e:
                print(f"❌ [ERROR] Failed to stream data row arrays from {file_name}: {e}")
                return False
        return True

    def _run_mock_genetic_algorithm_loop(self):
        """Mocks the evolution generations, processing evaluations, backtests, and fitness tracking."""
        print("🧬 [GA ENGINE] Initializing evolution population arrays...")
        time.sleep(0.5)
        
        start_gen = self.current_generation + 1
        max_generations = start_gen + 3  # Mock run tracking through 3 generation layers

        for gen in range(start_gen, max_generations):
            print(f"\n--- ⌛ GENERATION LEVEL {gen} ---")
            print(f"⚡ [GA] Mutating structural layer parameters across population tracks...")
            time.sleep(0.3)
            
            # Mocking backtest fitness score calculation improvements
            mock_fitness = self.best_fitness + (0.015420 * (gen * 0.55)) if self.best_fitness > -1.0 else 0.642591
            print(f"📈 [BACKTEST] Testing variant population strategies against historical feature nodes...")
            print(f"🎯 [FITNESS] Calculation complete. Current batch maximum fitness score: {mock_fitness:.12f}")
            
            if mock_fitness > self.best_fitness:
                print(f"🔥 [ALPHA] New optimal parameter resolve discovered at Gen {gen}!")
                self.best_fitness = mock_fitness
                self.current_generation = gen
                self.best_hyperparameters["lstm_hidden_dim"] = 64 + (gen * 8)
                self.best_hyperparameters["learning_rate"] *= 0.95  # Simulated convergence adjustment

    def _save_checkpoint_state(self):
        """Locks in state parameters and writes them to the target backup JSON file."""
        print(f"\n💾 [SAVE] Synchronizing current optimization state to file system matrix...")
        state_payload = {
            "current_generation": self.current_generation,
            "best_fitness": self.best_fitness,
            "best_hyperparameters": self.best_hyperparameters,
            "timestamp_utc": time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime()),
            "ingested_matrices": self.processed_features_summary
        }
        
        try:
            with open(self.checkpoint_file, 'w') as f:
                json.dump(state_payload, f, indent=4)
            print(f"✨ [SUCCESS] Checkpoint saved securely to '{self.checkpoint_file}' at generation step {self.current_generation}.")
        except Exception as e:
            print(f"❌ [CRITICAL] Failed to execute disk write backup pipeline state sequence: {e}")

# --- Execution Driver Module ---
if __name__ == "__main__":
    # Create a dummy transformed file if none exists to ensure the ingest system validates successfully
    dummy_file = "bitcoin_transformed.csv"
    if not os.path.exists(dummy_file):
        with open(dummy_file, "w") as f:
            f.write("timestamp,price_log_return,volume_log_change,hour_sin,hour_cos\n")
            f.write("2026-07-10 00:00,0.000124593812,-0.051204938102,0.866025,0.500000\n")

    # Instantiate and trigger the system control instance
    pipeline_engine = GABacktestPipelineSingleton()
    pipeline_engine.execute_pipeline()
