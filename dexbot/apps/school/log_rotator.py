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


def _has_valid_non_null_content(file_path: str, sample_bytes: int = 1024) -> bool:
    """
    Checks whether a file contains actual printable log characters rather than
    sparse-file NUL padding (\x00\x00...).
    """
    try:
        if not os.path.exists(file_path) or os.path.getsize(file_path) == 0:
            return False

        with open(file_path, "rb") as f:
            chunk = f.read(sample_bytes)
            # Remove all NUL bytes and check if meaningful text remains
            cleaned = chunk.replace(b"\x00", b"")
            return len(cleaned) > 0
    except OSError:
        return False


def _get_valid_logs_info(logs_dir: str):
    """
    Scans logs directory and calculates total size of ONLY valid (non-null) log files.
    Returns: (total_valid_size_mb, valid_file_paths, corrupted_null_file_paths)
    """
    total_valid_bytes = 0
    valid_files = []
    corrupted_files = []

    if os.path.exists(logs_dir):
        for root, _, files in os.walk(logs_dir):
            for f in files:
                if f.endswith(".log"):
                    fp = os.path.join(root, f)
                    try:
                        file_size = os.path.getsize(fp)
                        if file_size > 0:
                            if _has_valid_non_null_content(fp):
                                total_valid_bytes += file_size
                                valid_files.append(fp)
                            else:
                                corrupted_files.append(fp)
                    except OSError:
                        pass

    return total_valid_bytes / (1024.0 * 1024.0), valid_files, corrupted_files


def _safe_truncate_log_file(file_path: str):
    """
    Safely truncates a log file to 0 bytes using OS descriptors.
    Flushes Python logging handlers in the current process if attached.
    """
    try:
        # 1. Flush Python log handlers attached to this file
        for logger_name in logging.Logger.manager.loggerDict:
            lg = logging.getLogger(logger_name)
            if hasattr(lg, "handlers"):
                for h in lg.handlers:
                    if isinstance(h, logging.FileHandler) and os.path.abspath(
                        h.baseFilename
                    ) == os.path.abspath(file_path):
                        h.flush()
                        h.close()
                        h.stream = open(h.baseFilename, "a")

        # 2. Truncate using low-level OS file descriptor
        fd = os.open(file_path, os.O_WRONLY | os.O_TRUNC)
        os.close(fd)
    except Exception as e:
        print(f"⚠️ [LOG ROTATOR WARNING] Truncation error on {file_path}: {e}")


def _execute_rotation_and_purge(
    logs_dir: str,
    old_logs_dir: str,
    trigger_reason: str,
    valid_files: list,
    corrupted_files: list,
):
    try:
        timestamp = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")

        # 1. Purge corrupted zero-filled files immediately without creating an archive folder
        if corrupted_files:
            for bad_file in corrupted_files:
                _safe_truncate_log_file(bad_file)
            print(
                f"🧹 [LOG ROTATOR CLEANUP] Sanitized {len(corrupted_files)} zero-filled (00000) file(s)."
            )

        # 2. If no valid files exist, skip archive creation entirely
        if not valid_files:
            print("ℹ️ [LOG ROTATOR INFO] No valid log data to archive. Skipping snapshot creation.")
            return

        # 3. Replicate directory structure and archive ONLY valid logs
        target_archive_dir = os.path.join(old_logs_dir, timestamp)
        os.makedirs(target_archive_dir, exist_ok=True)

        for src_file in valid_files:
            rel_path = os.path.relpath(src_file, logs_dir)
            dest_file = os.path.join(target_archive_dir, rel_path)

            os.makedirs(os.path.dirname(dest_file), exist_ok=True)
            shutil.copy2(src_file, dest_file)
            _safe_truncate_log_file(src_file)

        print("\n" + "=" * 75)
        print(f"📦 [LOG ROTATION COMPLETE - REASON: {trigger_reason}]")
        print(f"   ├── Archived Destination : school/old_logs/{timestamp}/")
        print(
            f"   └── Active Logs Purged   : Cleared {len(valid_files)} file(s) back to 0 bytes."
        )
        print("=" * 75 + "\n")

    except Exception as e:
        print(f"❌ [LOG ROTATION ERROR] Execution failed: {e}")


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
        valid_size_mb, valid_files, corrupted_files = _get_valid_logs_info(logs_dir)

        trigger_reason = None

        if elapsed_seconds >= max_seconds:
            trigger_reason = (
                f"TIME THRESHOLD REACHED ({rotation_minutes} mins elapsed)"
            )
        elif valid_size_mb >= max_size_mb:
            trigger_reason = f"SIZE THRESHOLD EXCEEDED ({valid_size_mb:.2f} MB >= {max_size_mb} MB limit)"

        # Trigger rotation if time/size threshold is met OR if corrupted files need clearing
        if (trigger_reason and (valid_files or corrupted_files)) or corrupted_files:
            actual_reason = (
                trigger_reason
                if trigger_reason
                else "SANITY CLEANUP (CORRUPTED LOGS DETECTED)"
            )
            _execute_rotation_and_purge(
                logs_dir,
                old_logs_dir,
                actual_reason,
                valid_files,
                corrupted_files,
            )
            last_rotation_time = time.time()


def start_log_rotation_daemon(
    rotation_minutes: int = DEFAULT_ROTATION_MINUTES,
    max_size_mb: float = DEFAULT_ROTATION_MB,
):
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
    print(
        f"   └── Target Directories   : school/logs/ ➔ school/old_logs/{{timestamp}}/"
    )
    print("=" * 75 + "\n")