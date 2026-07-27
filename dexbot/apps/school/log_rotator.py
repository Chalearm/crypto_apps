#!/usr/bin/env python3
# ##############################################################################
# File Name        : log_rotator.py
# File Path        : apps/school/log_rotator.py
#
# Author           : Chalearm Saelim & Gemini
# Owner            : Chalearm Saelim
# Reviewer         : Chalearm Saelim
#
# Version          : 1.0.0
# Status           : Development
# Created Date     : 2026-07-27 17:00:00 (UTC+7)
# Modified Date    : 2026-07-27 17:00:00 (UTC+7)
#
# Description      :
#    Configurable background log rotation and purge daemon. Monitors active log
#    files inside school/logs/ and triggers an archival snapshot to school/old_logs/{timestamp}/
#    when either a time interval threshold (N minutes) OR a size threshold (N MB)
#    is reached. Safely truncates active logs back to 0 bytes without disrupting file handles.
#
# Responsibilities :
#    - Monitors active file sizes and elapsed time intervals.
#    - Replicates identical folder/file hierarchies into school/old_logs/{timestamp}/.
#    - Safely truncates active log files to 0 bytes without breaking running loggers.
#
# Usage :
#    Directory : apps/school/
#
# Dependencies :
#    Internal : None
#    External : standard library (os, time, shutil, datetime, threading)
# ##############################################################################

import os
import time
import shutil
import datetime
import threading

# Default configuration constants
DEFAULT_ROTATION_MINUTES = 45
DEFAULT_ROTATION_MB = 30.0
CHECK_INTERVAL_SECONDS = 15  # Polling interval to check size/time bounds


# ##############################################################################
# Function Name : _get_total_logs_size_mb
#
# Purpose :
#    Calculates the cumulative size of all .log files inside the target directory.
#
# Inputs :
#    logs_dir
#        Type        : str
#        Description : Path to target active log folder.
#
# Return :
#    Type        : float
#    Description : Total log directory size in Megabytes (MB).
# ##############################################################################
def _get_total_logs_size_mb(logs_dir: str) -> float:
    total_bytes = 0
    if os.path.exists(logs_dir):
        for root, _, files in os.walk(logs_dir):
            for f in files:
                if f.endswith('.log'):
                    fp = os.path.join(root, f)
                    try:
                        total_bytes += os.path.getsize(fp)
                    except OSError:
                        pass
    return total_bytes / (1024.0 * 1024.0)


# ##############################################################################
# Function Name : _execute_rotation_and_purge
#
# Purpose :
#    Copies all active log files from school/logs/ into a new timestamped folder
#    under school/old_logs/, preserving folder hierarchy, then truncates active logs.
#
# Inputs :
#    logs_dir
#        Type        : str
#        Description : Path to active logs directory.
#    old_logs_dir
#        Type        : str
#        Description : Path to archived logs destination root directory.
#    trigger_reason
#        Type        : str
#        Description : Human-readable explanation of what triggered rotation.
# ##############################################################################
def _execute_rotation_and_purge(logs_dir: str, old_logs_dir: str, trigger_reason: str):
    try:
        timestamp = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
        target_archive_dir = os.path.join(old_logs_dir, timestamp)
        os.makedirs(target_archive_dir, exist_ok=True)

        files_to_truncate = []

        # 1. Walk through school/logs/ and replicate structure to old_logs/{timestamp}/
        for root, _, files in os.walk(logs_dir):
            rel_path = os.path.relpath(root, logs_dir)
            target_root = os.path.join(target_archive_dir, rel_path) if rel_path != "." else target_archive_dir
            os.makedirs(target_root, exist_ok=True)

            for file_name in files:
                if not file_name.endswith('.log'):
                    continue

                src_file = os.path.join(root, file_name)
                dest_file = os.path.join(target_root, file_name)

                # Copy active contents to archive path
                shutil.copy2(src_file, dest_file)
                files_to_truncate.append(src_file)

        # 2. Safely truncate active log files back to 0 bytes
        for file_path in files_to_truncate:
            try:
                with open(file_path, 'r+') as f:
                    f.truncate(0)
            except Exception as trunc_err:
                print(f"⚠️ [LOG ROTATOR WARNING] Failed to truncate {file_path}: {trunc_err}")

        print("\n" + "=" * 75)
        print(f"📦 [LOG ROTATION COMPLETE - REASON: {trigger_reason}]")
        print(f"   ├── Archived Destination : school/old_logs/{timestamp}/")
        print(f"   └── Active Logs Purged   : Cleared {len(files_to_truncate)} file(s) back to 0 bytes.")
        print("=" * 75 + "\n")

    except Exception as e:
        print(f"❌ [LOG ROTATION ERROR] Execution failed: {e}")


# ##############################################################################
# Function Name : _log_rotation_worker_loop
#
# Purpose :
#    Internal daemon loop that polls elapsed time and cumulative file size against
#    configured thresholds, executing rotation when bounds are exceeded.
# ##############################################################################
def _log_rotation_worker_loop(rotation_minutes: int, max_size_mb: float):
    base_dir = os.path.dirname(os.path.abspath(__file__))
    logs_dir = os.path.join(base_dir, "logs")
    old_logs_dir = os.path.join(base_dir, "old_logs")

    last_rotation_time = time.time()
    max_seconds = rotation_minutes * 60

    while True:
        time.sleep(CHECK_INTERVAL_SECONDS)

        if not os.path.exists(logs_dir):
            continue

        current_time = time.time()
        elapsed_seconds = current_time - last_rotation_time
        current_size_mb = _get_total_logs_size_mb(logs_dir)

        trigger_reason = None

        # Check Threshold 1: Time Elapsed (N minutes)
        if elapsed_seconds >= max_seconds:
            trigger_reason = f"TIME THRESHOLD REACHED ({rotation_minutes} mins elapsed)"

        # Check Threshold 2: Size Exceeded (N MB)
        elif current_size_mb >= max_size_mb:
            trigger_reason = f"SIZE THRESHOLD EXCEEDED ({current_size_mb:.2f} MB >= {max_size_mb} MB limit)"

        # Execute rotation if either condition was met
        if trigger_reason and current_size_mb > 0:
            _execute_rotation_and_purge(logs_dir, old_logs_dir, trigger_reason)
            last_rotation_time = time.time()


# ##############################################################################
# Function Name : start_log_rotation_daemon
#
# Purpose :
#    Public API entry point to launch the configurable background daemon thread.
#
# Inputs :
#    rotation_minutes
#        Type        : int
#        Description : Maximum time in minutes before forced rotation (Default: 45).
#    max_size_mb
#        Type        : float
#        Description : Maximum size in MB before forced rotation (Default: 30.0 MB).
# ##############################################################################
def start_log_rotation_daemon(rotation_minutes: int = DEFAULT_ROTATION_MINUTES, max_size_mb: float = DEFAULT_ROTATION_MB):
    """Launches the configurable log rotation daemon as a background thread."""
    daemon_thread = threading.Thread(
        target=_log_rotation_worker_loop,
        args=(rotation_minutes, max_size_mb),
        daemon=True
    )
    daemon_thread.start()

    print("\n" + "=" * 75)
    print("⏱️ [LOG ROTATOR DAEMON INITIALIZED]")
    print(f"   ├── Time Limit Threshold : Every {rotation_minutes} minutes")
    print(f"   ├── Size Limit Threshold : Every {max_size_mb:.1f} MB")
    print("   └── Target Directories   : school/logs/ ➔ school/old_logs/{timestamp}/")
    print("=" * 75 + "\n")