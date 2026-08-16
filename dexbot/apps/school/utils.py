#!/usr/bin/env python3
# ##############################################################################
# File Name        : utils.py
# File Path        : apps/school/utils.py
#
# Author           : Chalearm Saelim & Gemini
# Owner            : Chalearm Saelim
# Reviewer         : Chalearm Saelim
#
# Version          : 1.0.0
# Status           : Development
# Created Date     : 2026-07-26 08:00:00 (UTC+7)
# Modified Date    : 2026-07-26 15:50:00 (UTC+7)
#
# Description      :
#    Shared path resolution helper module for the distributed GA-LSTM system.
#    Provides directory path routing and automatic directory creation for logs,
#    deployed models, and rendered visualization outputs.
#
#    DEPENDENCY TREE & STRUCTURAL MAP:
#    ───────────────────────────────────────────────────────────────────────────
#    [utils.py]
#        ├── Imported by ──> [ga_master.py]
#        ├── Imported by ──> [train_worker.py]
#        └── Imported by ──> [visualization_worker.py]
#
#    FUNCTION DEPENDENCY MATRIX:
#    ───────────────────────────────────────────────────────────────────────────
#    resolve_target_directories(run_id)
#     └── os.makedirs(..., exist_ok=True)
#
# Responsibilities :
#    - Resolves localized run_id directory paths for multi-session data isolation.
#    - Guarantees filesystem directory existence before worker write operations.
#
# Usage :
#    Directory : apps/school/
#
#    Build :
#      N/A (Interpreted Python Script)
#
#    Run :
#      Imported as utility package across master and worker components.
#
# Dependencies :
#    Internal :
#      - None
#
#    External :
#      - os (stdlib)
#
# Updated Parts :
#    None
#
# New Parts :
#    [Function]
#      - resolve_target_directories()
#
# Change History :
#    -------------------------------------------------------------------------
#    Version | Date Time (UTC+7)        | Author          | Description
#    -------------------------------------------------------------------------
#    1.0.0   | 2026-07-26 08:00:00      | Chalearm Saelim | Initial release
#    -------------------------------------------------------------------------
#
# TODO :
#    - Add disk space availability validation check prior to folder creation.
#
# Notes :
#    - Per regulator coding standard rules.
# ##############################################################################

import os


# ==============================================================================
# DIRECTORY RESOLUTION UTILITIES
# ==============================================================================

# ##############################################################################
# Function Name : resolve_target_directories
#
# Path          : apps/school/utils.py
# Author        : Chalearm Saelim
#
# Purpose :
#    Resolves base directories for logging, deployed model artifacts, and plot
#    outputs. Maintains strict backward compatibility by returning EXACTLY 3 values.
#
# Inputs :
#    run_id   : str  (Optional) Execution Run Identifier (e.g. '45097E20')
#    gen_idx  : int  (Optional) Generation Index (e.g. 1 -> 'prediction_result/45097E20/G1')
#
# Return :
#    tuple : (log_dir, export_dir, plot_dir) -> EXACTLY 3 VALUES
# ##############################################################################
def resolve_target_directories(run_id: str = None, gen_idx: int = None):
    if run_id:
        log_dir = os.path.join("logs", run_id)
        export_dir = os.path.join("deployed_models", run_id)
        plot_dir = os.path.join("prediction_result", run_id)
    else:
        log_dir = "logs"
        export_dir = "deployed_models"
        plot_dir = "prediction_result"

    # Route plot_dir into generation subdirectory if gen_idx is supplied
    if gen_idx is not None:
        plot_dir = os.path.join(plot_dir, f"G{gen_idx}")

    os.makedirs(log_dir, exist_ok=True)
    os.makedirs(export_dir, exist_ok=True)
    os.makedirs(plot_dir, exist_ok=True)

    return log_dir, export_dir, plot_dir

    # ##############################################################################
# Function Name : resolve_model_plot_directories
#
# Path          : apps/school/utils.py
# Author        : Chalearm Saelim
#
# Purpose :
#    Creates nested model prediction directories for price overlays ('prc')
#    and volume overlays ('vol'):
#    `prediction_result/{run_id}/G{gen_idx}/{short_model}/[prc|vol]`
#
# Inputs :
#    run_id   : str  Active Run ID (e.g., '45097E20')
#    gen_idx  : int  Generation index (e.g., 1)
#    model_id : str  Model name (e.g., 'G1-M0' or 'M0')
#
# Return :
#    tuple : (gen_plot_dir, prc_dir, vol_dir)
# ##############################################################################
def resolve_model_plot_directories(run_id: str, gen_idx: int, model_id: str):
    _, _, gen_plot_dir = resolve_target_directories(run_id=run_id, gen_idx=gen_idx)

    # Shorten model name (e.g., 'G1-M0' -> 'M0')
    short_model = model_id.split("-")[-1] if "-" in model_id else model_id

    prc_dir = os.path.join(gen_plot_dir, short_model, "prc")
    vol_dir = os.path.join(gen_plot_dir, short_model, "vol")

    os.makedirs(prc_dir, exist_ok=True)
    os.makedirs(vol_dir, exist_ok=True)

    print(f"📂 [MODEL PLOT DIRS RESOLVED] Model: {model_id}")
    print(f"   ├── Gen Directory   : {gen_plot_dir}")
    print(f"   ├── Price Plot Dir  : {prc_dir}")
    print(f"   └── Volume Plot Dir : {vol_dir}")

    return gen_plot_dir, prc_dir, vol_dir