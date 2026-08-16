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
#    Configurable background log rotation and purge daemon with null-byte
#    sanitization. Monitors active log files inside school/logs/ and triggers an
#    archival snapshot to school/old_logs/{timestamp}/ when either a time
#    interval threshold (N minutes) OR a size threshold (N MB) is reached.
#
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
import logging

DEFAULT_ROTATION_MINUTES = 30
DEFAULT_ROTATION_MB = 30.0
CHECK_INTERVAL_SECONDS = 15


def _get_logs_info(logs_dir: str):
    """Calculates total size in MB of all active log files in logs_dir."""
    total_bytes = 0
    log_files = []

    if os.path.exists(logs_dir):
        for root, _, files in os.walk(logs_dir):
            for f in files:
                if f.endswith(".log"):
                    fp = os.path.join(root, f)
                    try:
                        sz = os.path.getsize(fp)
                        if sz > 0:
                            total_bytes += sz
                            log_files.append(fp)
                    except OSError:
                        pass

    return total_bytes / (1024.0 * 1024.0), log_files


def _safe_truncate_log_file(file_path: str):
    """
    Safely copies and truncates active log files, resetting active Python logging
    handlers and OS file pointers to offset 0 to prevent NUL byte (0000 0000) padding.
    """
    try:
        abs_path = os.path.abspath(file_path)

        # 1. Flush, seek to 0, and truncate active Python FileHandlers
        for logger_name in list(logging.Logger.manager.loggerDict.keys()):
            lg = logging.getLogger(logger_name)
            if hasattr(lg, "handlers"):
                for h in lg.handlers:
                    if isinstance(h, logging.FileHandler) and h.baseFilename and os.path.abspath(h.baseFilename) == abs_path:
                        h.flush()
                        if h.stream and not h.stream.closed:
                            try:
                                h.stream.seek(0)
                                h.stream.truncate(0)
                                h.stream.flush()
                            except Exception:
                                h.close()
                                h.stream = open(h.baseFilename, "a")

        # 2. Reset low-level OS file descriptor for background process redirections
        if os.path.exists(file_path):
            with open(file_path, "r+") as f:
                f.seek(0)
                f.truncate(0)
                f.flush()

    except Exception as e:
        print(f"⚠️ [LOG ROTATOR WARN] Truncation error on {file_path}: {e}")


def _execute_rotation(logs_dir: str, old_logs_dir: str, trigger_reason: str, log_files: list):
    """Archives active logs to old_logs/{timestamp}/ and safely resets active log files to 0 bytes."""
    try:
        timestamp = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
        target_archive_dir = os.path.join(old_logs_dir, timestamp)
        os.makedirs(target_archive_dir, exist_ok=True)

        for src_file in log_files:
            rel_path = os.path.relpath(src_file, logs_dir)
            dest_file = os.path.join(target_archive_dir, rel_path)
            os.makedirs(os.path.dirname(dest_file), exist_ok=True)
            
            # Copy snapshot first, then safely truncate with pointer reset
            shutil.copy2(src_file, dest_file)
            _safe_truncate_log_file(src_file)

        print("\n" + "=" * 75)
        print(f"📦 [LOG ROTATION COMPLETE - REASON: {trigger_reason}]")
        print(f"   ├── Archived Destination : school/old_logs/{timestamp}/")
        print(f"   └── Active Logs Purged   : Cleared {len(log_files)} file(s) back to 0 bytes (NUL byte safe).")
        print("=" * 75 + "\n")

    except Exception as e:
        print(f"❌ [LOG ROTATION ERROR] Execution failed: {e}")


def _log_rotation_worker_loop(rotation_minutes: int, max_size_mb: float):
    base_dir = os.path.dirname(os.path.abspath(__file__))
    logs_dir = os.path.join(base_dir, "logs")
    old_logs_dir = os.path.join(base_dir, "old_logs")

    last_rotation_time = time.time()
    max_seconds = rotation_minutes * 60

    print(f"⏱️ [LOG ROTATOR DAEMON ACTIVE] Interval: {rotation_minutes}m ({max_seconds}s) | Limit: {max_size_mb:.1f} MB")

    while True:
        time.sleep(CHECK_INTERVAL_SECONDS)

        if not os.path.exists(logs_dir):
            continue

        current_time = time.time()
        elapsed_seconds = current_time - last_rotation_time
        total_size_mb, log_files = _get_logs_info(logs_dir)

        trigger_reason = None

        if elapsed_seconds >= max_seconds:
            trigger_reason = f"TIME THRESHOLD REACHED ({rotation_minutes} mins elapsed)"
        elif total_size_mb >= max_size_mb:
            trigger_reason = f"SIZE THRESHOLD EXCEEDED ({total_size_mb:.2f} MB >= {max_size_mb:.1f} MB limit)"

        if trigger_reason and log_files:
            _execute_rotation(logs_dir, old_logs_dir, trigger_reason, log_files)
            last_rotation_time = time.time()


def start_log_rotation_daemon(rotation_minutes: int = DEFAULT_ROTATION_MINUTES, max_size_mb: float = DEFAULT_ROTATION_MB):
    """Launches the configurable log rotation daemon as a background thread."""
    daemon_thread = threading.Thread(
        target=_log_rotation_worker_loop,
        args=(rotation_minutes, max_size_mb),
        daemon=True,
    )
    daemon_thread.start()

    print("\n" + "=" * 75)
    print("⏱️ [LOG ROTATOR DAEMON INITIALIZED]")
    print(f"   ├── Time Limit Threshold : Every {rotation_minutes} minutes")
    print(f"   ├── Size Limit Threshold : Every {max_size_mb:.1f} MB")
    print(f"   └── Target Directories   : school/logs/ ➔ school/old_logs/{{timestamp}}/")
    print("=" * 75 + "\n")