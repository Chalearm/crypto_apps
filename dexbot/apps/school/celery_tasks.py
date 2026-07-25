#!/usr/bin/env python3
#/******************************************************************************
#* File Name        : celery_tasks.py
#* Path             : apps/school/celery_tasks.py
#* Author           : Chalearm Saelim & Gemini
#* System Role      : Distributed Task Router & Broker Interface
#* Architecture     : Distributed Client-Server / Master-Worker (Celery + Redis)
#* 
#* DEPENDENCY TREE & STRUCTURAL MAP:
#* ─────────────────────────────────────────────────────────────────────────────
#* [ga_master.py] (Central Orchestrator)
#*    │
#*    ├── Invokes Async Tasks ──> [celery_tasks.py]
#*    │                                │
#*    │                   (Redis Message Broker Queue)
#*    │                                │
#*    │  ┌─────────────────────────────┴──────────────────────────────┐
#*    │  ▼                                                            ▼
#* [tasks.run_fold_training_task]                         [tasks.export_and_plot_task]
#*    │                                                               │
#*    └── Calls ──> [train_worker.py]                                 └── Calls ──> [visualization_worker.py]
#*          - Deserializes JSON arrays                                      - Reconstructs Pandas DataFrames
#*          - Trains Keras LSTM model                                       - Autoregressive multi-step forecasts
#*          - Logs fold metrics to                                          - Matplotlib overlay rendering
#*            logs/<run_id>/folds_lifecycle.log                             - Exports .keras model & scaler to
#*          - Returns JSON metrics payload                                    deployed_models/<run_id>/
#*
#* FUNCTION DEPENDENCY MATRIX (Tasks & Exports):
#* ─────────────────────────────────────────────────────────────────────────────
#* run_fold_training_task(self, payload_json)
#*  └── train_worker.execute_fold_training(payload_json)
#*
#* export_and_plot_task(top_chromosomes_json, master_data_json, val_data_json, gen_num, run_id)
#*  └── visualization_worker.generate_pareto_graphs_and_exports(...)
#******************************************************************************/
import os
import sys
from celery import Celery

# Ensure current script directory is on sys.path
CURRENT_DIR = os.path.dirname(os.path.abspath(__file__))
if CURRENT_DIR not in sys.path:
    sys.path.insert(0, CURRENT_DIR)

# ------------------------------------------------------------------------------
# REDIS BROKER & RESULT BACKEND CONFIGURATION
# ------------------------------------------------------------------------------
REDIS_URL = os.getenv("REDIS_URL", "redis://localhost:6379/0")

# 1. Instantiate 'app' FIRST
app = Celery(
    "ga_lstm_tasks",
    broker=REDIS_URL,
    backend=REDIS_URL
)

# 2. THEN update conf
app.conf.update(
    task_serializer='json',
    result_serializer='json',
    accept_content=['json'],
    result_expires=3600,
    broker_transport_options={
        'visibility_timeout': 3600,
        'socket_timeout': 30,
        'socket_connect_timeout': 30,
        'socket_keepalive': True,
    },
    redis_backend_health_check_interval=10,
    task_compression='gzip',
)

# ==============================================================================
# CELERY TASK DEFINITIONS
# ==============================================================================

@app.task(name="tasks.run_fold_training_task", bind=True, max_retries=2)
def run_fold_training_task(self, payload_json: dict) -> dict:
    from train_worker import execute_fold_training
    return execute_fold_training(payload_json)

# TO THIS:
@app.task(name="tasks.export_and_plot_task", bind=True, max_retries=1)
def export_and_plot_task(self, payload_json: dict) -> dict:
    """
    Offloaded Celery task running on a dedicated worker node/VM to generate Matplotlib 
    overlay prediction graphs and package top Pareto candidate Keras models into 
    dedicated run_id sub-directories (or fallback root directories for legacy runs).
    """
    from visualization_worker import generate_pareto_graphs_and_exports
    try:
        return generate_pareto_graphs_and_exports(
            top_chromosomes_json=top_chromosomes_json,
            master_data_json=master_data_json,
            val_data_json=val_data_json,
            gen_num=gen_num,
            run_id=run_id  # 🎯 Passes run_id for sub-directory image/model saving
        )
    except Exception as exc:
        raise self.retry(exc=exc, countdown=5)