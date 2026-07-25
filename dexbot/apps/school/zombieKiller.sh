#!/bin/bash
# 1. Tell all parent processes to harvest their completed children
kill -s SIGCHLD $(ps -A -o ppid,stat | awk '$2~/Z/ {print $1}' | sort -u) 2>/dev/null

# 2. Force-kill parents of zombies (EXCLUDING PID 1 so container stays up)
ps -A -o ppid,stat | awk '$2~/Z/ && $1!=1 {print $1}' | sort -u | xargs -r kill -9

echo "🧹 Zombie cleanup complete. Current zombie count: $(ps aux | awk '$8~"Z"' | wc -l)"