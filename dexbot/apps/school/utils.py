#!/usr/bin/env python3
#/******************************************************************************
#* File Name        : utils.py
#* Path             : apps/school/utils.py
#* Author           : Chalearm Saelim & Gemini
#* System Role      : Shared Directory & Path Resolver Helper
#* Architecture     : Distributed Client-Server / Master-Worker
#*
#* DEPENDENCY TREE & STRUCTURAL MAP:
#* ─────────────────────────────────────────────────────────────────────────────
#* [utils.py]
#*    ├── Imported by ──> [ga_master.py]
#*    ├── Imported by ──> [train_worker.py]
#*    └── Imported by ──> [visualization_worker.py]
#******************************************************************************/

import os

def resolve_target_directories(run_id: str = None):
    """
    Resolves log, model deployment, and plot result directory paths.
    If run_id (8-digit hex) is provided, routes to localized sub-directories.
    If run_id is None (Legacy Checkpoints), falls back to root directories.
    """
    if run_id:
        log_dir = os.path.join("logs", run_id)
        export_dir = os.path.join("deployed_models", run_id)
        plot_dir = os.path.join("prediction_result", run_id)
    else:
        log_dir = "logs"
        export_dir = "deployed_models"
        plot_dir = "prediction_result"

    os.makedirs(log_dir, exist_ok=True)
    os.makedirs(export_dir, exist_ok=True)
    os.makedirs(plot_dir, exist_ok=True)

    return log_dir, export_dir, plot_dir