#!/usr/bin/env python3
# ##############################################################################
# File Name        : celery_tasks.py
# File Path        : apps/school/celery_tasks.py
#
# Author           : Chalearm Saelim & Gemini
# Owner            : Chalearm Saelim
# Reviewer         : Chalearm Saelim
#
# Version          : 3.5.0
# Status           : Development
# Created Date     : 2026-07-26 08:00:00 (UTC+7)
# Modified Date    : 2026-07-30 17:00:00 (UTC+7)
#
# Description      :
#    Distributed Task Router & Broker Interface for Celery and Redis. Registers
#    asynchronous tasks for fold cross-validation training, visualization/export,
#    and Post-GA Out-of-Sample (OOS) Walk-Forward verification with fallback
#    cache path resolution across worker containers.
#
# DEPENDENCY TREE & STRUCTURAL MAP:
# ───────────────────────────────────────────────────────────────────────────
# [ga_master.py] (Central Orchestrator)
#     │
#     ├── Invokes Async Tasks ──> [celery_tasks.py]
#     │                                 │
#     │                    (Redis Message Broker Queue)
#     │                                 │
#     │  ┌─────────────────────────────┼──────────────────────────────┐
#     │  ▼                             ▼                              ▼
# [tasks.run_fold_training_task] [tasks.export_and_plot_task] [tasks.run_oos_verification_task]
#     │                                 │                              │
#     └── Calls ──> [train_worker.py]   └── Calls ──> [viz_worker.py]  └── Calls ──> [verify_worker.py]
#
# Responsibilities :
#    - Routes distributed task invocations through Redis broker using Celery.
#    - Handles task serialization, compression (gzip), and retry mechanics.
#    - Performs absolute path fallback resolution for missing tensor cache files.
#
# Usage :
#    Directory : apps/school/
#
# Build :
#    N/A (Interpreted Python Script)
#
# Run :
#    celery -A celery_tasks worker -n local_worker_1 --loglevel=info
#
# Dependencies :
#    Internal :
#      - train_worker (execute_fold_training)
#      - visualization_worker (generate_pareto_graphs_and_exports)
#      - verify_worker (execute_oos_verification)
#
#    External :
#      - celery, redis, numpy
#
# Change History :
#    -------------------------------------------------------------------------
#    Version | Date Time (UTC+7)         | Author          | Description
#    -------------------------------------------------------------------------
#    1.0.0   | 2026-07-26 08:00:00 (UTC+7) | Chalearm Saelim | Initial version
#    1.0.1   | 2026-07-27 16:35:00 (UTC+7) | Chalearm Saelim | Registered verify task
#    3.5.0   | 2026-07-30 17:00:00 (UTC+7) | Chalearm Saelim | Added path resolution fallback & OOS task routing
#    -------------------------------------------------------------------------
#
# TODO :
#    - Add task deadline and soft-time-limit safeguards.
#
# Notes :
#    - Per regulator coding standard rules.
# ##############################################################################

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

# 1. Instantiate Celery app object FIRST
app = Celery(
    "ga_lstm_tasks",
    broker=REDIS_URL,
    backend=REDIS_URL
)

# 2. Update worker transport and serialization parameters
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
# HELPER: CACHE PATH RESOLUTION FALLBACK
# ==============================================================================
def resolve_cache_file_path(payload_json: dict) -> dict:
    """
    Validates and resolves the absolute tensor cache file path across worker containers.
    If the provided path does not exist, attempts fallback path construction relative
    to application root directory.
    """
    cache_file = payload_json.get('cache_file', '')
    run_id = payload_json.get('run_id', '')
    chrom_id = payload_json.get('chrom_id', '')

    if not cache_file or not os.path.exists(cache_file):
        app_root = os.path.dirname(os.path.abspath(__file__))
        alt_path = os.path.join(app_root, "logs", run_id, "cache", f"chrom_{chrom_id}.npz")
        
        if os.path.exists(alt_path):
            payload_json['cache_file'] = alt_path
        else:
            # Secondary fallback check relative to working directory
            pwd_path = os.path.abspath(os.path.join("logs", run_id, "cache", f"chrom_{chrom_id}.npz"))
            if os.path.exists(pwd_path):
                payload_json['cache_file'] = pwd_path

    return payload_json


# ==============================================================================
# CELERY TASK DEFINITIONS
# ==============================================================================

# ##############################################################################
# Function Name : run_fold_training_task
#
# Purpose :
#    Asynchronous Celery task wrapper that delegates fold training and cross-
#    validation evaluation to the isolated train_worker module. Includes path
#    fallback resolution for missing tensor cache files.
# ##############################################################################
@app.task(name="tasks.run_fold_training_task", bind=True, max_retries=2)
def run_fold_training_task(self, payload_json: dict) -> dict:
    from train_worker import execute_fold_training
    
    # Resolve path fallbacks before executing fold training
    resolved_payload = resolve_cache_file_path(payload_json)
    return execute_fold_training(resolved_payload)


# ##############################################################################
# Function Name : export_and_plot_task
#
# Purpose :
#    Offloaded Celery task running on a worker node to generate Matplotlib
#    overlay prediction graphs and package top Pareto candidate Keras models.
#    Routes to POST_GA_OOS if payload task type dictates verification.
# ##############################################################################
@app.task(name="tasks.export_and_plot_task", bind=True, max_retries=1)
def export_and_plot_task(self, payload_json: dict) -> dict:
    task_type = payload_json.get("task_type", "GEN_PLOT")
    
    # Route Post-GA Out-of-Sample verification directly if dispatched via export task handler
    if task_type == "POST_GA_OOS":
        from verify_worker import execute_oos_verification
        try:
            return execute_oos_verification(payload_json)
        except Exception as exc:
            raise self.retry(exc=exc, countdown=5)

    from visualization_worker import generate_pareto_graphs_and_exports
    try:
        top_chromosomes_json = payload_json.get("top_chromosomes")
        master_data_json = payload_json.get("master_data")
        val_data_json = payload_json.get("val_data", None)
        gen_num = payload_json.get("gen_idx", 1)
        run_id = payload_json.get("run_id")

        return generate_pareto_graphs_and_exports(
            top_chromosomes_json=top_chromosomes_json,
            master_data_json=master_data_json,
            val_data_json=val_data_json,
            gen_num=gen_num,
            run_id=run_id
        )
    except Exception as exc:
        raise self.retry(exc=exc, countdown=5)


# ##############################################################################
# Function Name : run_oos_verification_task
#
# Purpose :
#    Offloaded Celery task executing Post-GA Out-of-Sample Walk-Forward
#    verification for the Rank 1 elite chromosome across expanded folds.
# ##############################################################################
@app.task(name="tasks.run_oos_verification_task", bind=True, max_retries=1)
def run_oos_verification_task(self, payload_json: dict) -> dict:
    from verify_worker import execute_oos_verification
    try:
        return execute_oos_verification(payload_json)
    except Exception as exc:
        raise self.retry(exc=exc, countdown=5)