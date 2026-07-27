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
# Purpose :
#    Resolves target log, model deployment, and plot graph directory paths.
#    If a run_id hex string is provided, routes to isolated sub-directories
#    (logs/<run_id>, deployed_models/<run_id>, prediction_result/<run_id>).
#    If run_id is None, falls back to legacy root directories. Ensures target
#    folders exist on disk before returning.
#
# Inputs :
#    run_id
#       Type        : str or None
#       Description : Unique 8-character hexadecimal run identifier string.
#
# Return :
#    Type        : tuple (str, str, str)
#    Description : (log_dir, export_dir, plot_dir) absolute or relative paths.
#
# Complexity :
#    Time  : O(1)
#    Space : O(1)
#
# Error Cases :
#    - Handles PermissionError if filesystem write access is restricted.
#
# Number Of Lines :
#    18
# ##############################################################################
def resolve_target_directories(run_id: str = None):
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

    