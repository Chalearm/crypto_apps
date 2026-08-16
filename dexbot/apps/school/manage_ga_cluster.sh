#!/usr/bin/env bash

set -e

CONTAINER_NAME="llxprt"
APP_DIR="/workspace/crypto_apps/dexbot/apps/school"

echo "========================================================================"
echo "      🤖 GA-LSTM DISTRIBUTED CLUSTER ORCHESTRATION & MANAGER           "
echo "========================================================================"
echo "Select execution option:"
echo "  [1] Soft Restart Daemon : Stop ALL master processes, keep checkpoints."
echo "  [2] Total Cold Reset    : WIPE checkpoints/logs, flush Redis, restart."
echo "  [3] Reset Worker Pool   : Restart remote workers & inspect connectivity."
echo "  [4] Launch Master Engine: Start single master daemon instance."
echo "  [5] Stream Master Logs  : Stream real-time output from master_daemon.log."
echo "  [6] Inspect Cluster     : View active tasks & checkpoint status."
echo "  [7] Inspect Checkpoint  : Detailed tabular checkpoint analysis."
echo "  [8] Force Checkpoint Save : Instantly flush RAM models to JSON disk."
echo "  [9] Print Status Report : Dump detailed RAM vs Disk / Fold status to logs."
echo "  [10] Run Plot Unit Tests: Execute unit tests for event-driven plot triggers."
echo "========================================================================"

if [[ $# -ge 1 ]]; then
    CHOICE="$1"
    echo "Selected option: $CHOICE"
else
    read -rp "Enter option [1-6]: " CHOICE
fi

case $CHOICE in
  1)
    echo ""
    echo "🔄 --- [OPTION 1: SOFT RESTART MASTER] ---"
    docker exec ${CONTAINER_NAME} pkill -9 -f "ga_master.py" 2>/dev/null || true
    docker exec ${CONTAINER_NAME} rm -f ${APP_DIR}/ga_master.pid 2>/dev/null || true
    echo "✅ Master daemon stopped gracefully."
    ;;

  2)
    echo ""
    echo "🧹 --- [OPTION 2: TOTAL COLD RESET] ---"
    docker exec ${CONTAINER_NAME} pkill -9 -f "ga_master.py" 2>/dev/null || true
    docker exec worker1 pkill -9 -f celery 2>/dev/null || true
    docker exec worker2 pkill -9 -f celery 2>/dev/null || true
    docker exec redis_broker redis-cli flushall

    docker exec ${CONTAINER_NAME} bash -c \
      "cd ${APP_DIR} && \
       rm -f lstm_ga_checkpoint.json lstm_ga_checkpoint.json.tmp ga_master.pid && \
       rm -rf logs/* old_logs/* prediction_result/* deployed_models/* 2>/dev/null || true"

    docker restart worker1 worker2
    sleep 5

    REDIS_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' redis_broker 2>/dev/null || echo "172.18.0.4")
    
    docker exec worker1 mkdir -p /home/worker1/dexbot/apps/school/logs
    docker exec -d worker1 bash -c \
      "export PATH=/opt/venv/bin:\$PATH PYTHONPATH=/home/worker1/dexbot/apps/school:\$PYTHONPATH REDIS_HOST=${REDIS_IP} CELERY_BROKER_URL=redis://${REDIS_IP}:6379/0 REDIS_URL=redis://${REDIS_IP}:6379/0 && \
       cd /home/worker1/dexbot/apps/school && \
       /opt/venv/bin/celery -A celery_tasks worker -n local_worker_1@worker1 -Q export_queue,training_queue -O fair --prefetch-multiplier=1 --concurrency=6 --max-tasks-per-child=60 --loglevel=info --logfile=/home/worker1/dexbot/apps/school/logs/celery_worker1.log"

    docker exec worker2 mkdir -p /home/worker2/dexbot/apps/school/logs
    docker exec -d worker2 bash -c \
      "export PATH=/opt/venv/bin:\$PATH PYTHONPATH=/home/worker2/dexbot/apps/school:\$PYTHONPATH REDIS_HOST=${REDIS_IP} CELERY_BROKER_URL=redis://${REDIS_IP}:6379/0 REDIS_URL=redis://${REDIS_IP}:6379/0 && \
       cd /home/worker2/dexbot/apps/school && \
       /opt/venv/bin/celery -A celery_tasks worker -n local_worker_2@worker2 -Q export_queue,training_queue -O fair --prefetch-multiplier=1 --concurrency=6 --max-tasks-per-child=60 --loglevel=info --logfile=/home/worker2/dexbot/apps/school/logs/celery_worker2.log"

    echo "✅ Cold reset complete!"
    ;;

  3)
    echo ""
    echo "🔄 --- [OPTION 3: WORKER POOL RESTART] ---"
    docker exec worker1 pkill -9 -f celery 2>/dev/null || true
    docker exec worker2 pkill -9 -f celery 2>/dev/null || true
    docker restart worker1 worker2
    sleep 5

    REDIS_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' redis_broker 2>/dev/null || echo "172.18.0.4")

    docker exec worker1 mkdir -p /home/worker1/dexbot/apps/school/logs
    docker exec -d worker1 bash -c \
      "export PATH=/opt/venv/bin:\$PATH PYTHONPATH=/home/worker1/dexbot/apps/school:\$PYTHONPATH REDIS_HOST=${REDIS_IP} CELERY_BROKER_URL=redis://${REDIS_IP}:6379/0 REDIS_URL=redis://${REDIS_IP}:6379/0 && \
       cd /home/worker1/dexbot/apps/school && \
       /opt/venv/bin/celery -A celery_tasks worker -n local_worker_1@worker1 -Q export_queue,training_queue -O fair --prefetch-multiplier=1 --concurrency=6 --max-tasks-per-child=60 --loglevel=info --logfile=/home/worker1/dexbot/apps/school/logs/celery_worker1.log"

    docker exec worker2 mkdir -p /home/worker2/dexbot/apps/school/logs
    docker exec -d worker2 bash -c \
      "export PATH=/opt/venv/bin:\$PATH PYTHONPATH=/home/worker2/dexbot/apps/school:\$PYTHONPATH REDIS_HOST=${REDIS_IP} CELERY_BROKER_URL=redis://${REDIS_IP}:6379/0 REDIS_URL=redis://${REDIS_IP}:6379/0 && \
       cd /home/worker2/dexbot/apps/school && \
       /opt/venv/bin/celery -A celery_tasks worker -n local_worker_2@worker2 -Q export_queue,training_queue -O fair --prefetch-multiplier=1 --concurrency=6 --max-tasks-per-child=60 --loglevel=info --logfile=/home/worker2/dexbot/apps/school/logs/celery_worker2.log"

    sleep 8
    docker exec ${CONTAINER_NAME} bash -c \
      "export PYTHONPATH=${APP_DIR}:\$PYTHONPATH REDIS_HOST=${REDIS_IP} CELERY_BROKER_URL=redis://${REDIS_IP}:6379/0 REDIS_URL=redis://${REDIS_IP}:6379/0 && \
       cd ${APP_DIR} && \
       /opt/venv/bin/celery -A celery_tasks inspect ping --timeout=15"
    ;;

  4)
    echo ""
    echo "🚀 Launching Single GA Master Daemon (Immediate 1.0% Checkpoint Flushing)..."
    docker exec ${CONTAINER_NAME} pkill -9 -f "ga_master.py" 2>/dev/null || true
    docker exec ${CONTAINER_NAME} rm -f ${APP_DIR}/ga_master.pid 2>/dev/null || true
    docker exec ${CONTAINER_NAME} mkdir -p ${APP_DIR}/logs
    sleep 1

    docker exec -d ${CONTAINER_NAME} bash -c \
      "export PATH=/opt/venv/bin:\$PATH PYTHONPATH=${APP_DIR}:\$PYTHONPATH REDIS_HOST=redis CELERY_BROKER_URL=redis://redis:6379/0 REDIS_URL=redis://redis:6379/0 && \
       cd ${APP_DIR} && \
       python3 ga_master.py -generations=47 -num=5 -save-min=50 -save-pct=9.3 -rotate-min=293 -buffer-size=89 > ${APP_DIR}/logs/master_daemon.log 2>&1"
    
    echo "✅ GA Master process spawned."
    ;;

  5)
    echo ""
    echo "📜 --- [OPTION 5: LIVE MASTER LOG STREAM] ---"
    docker exec -it ${CONTAINER_NAME} tail -f -n 100 ${APP_DIR}/logs/master_daemon.log
    ;;

  6)
    echo ""
    docker exec ${CONTAINER_NAME} bash -c \
      "export PATH=/opt/venv/bin:\$PATH PYTHONPATH=${APP_DIR}:\$PYTHONPATH REDIS_HOST=redis CELERY_BROKER_URL=redis://redis:6379/0 REDIS_URL=redis://redis:6379/0 && \
       cd ${APP_DIR} && \
       python3 ga_master.py -action=status"
    ;;
  7)
    docker exec -it llxprt python3 /workspace/crypto_apps/dexbot/apps/school/inspect_checkpoint.py
    exit 0
    ;;
  8)
      echo ""
      echo "🚨 --- [OPTION 8: FORCE CHECKPOINT SAVE] ---"
      echo "Triggering manual override..."
      touch force_save.flag
      echo "✅ 'force_save.flag' created!"
      ls -alth force_save.flag && sleep 2
      ls -alth force_save.flag
      echo "👉 The GA Master will intercept this within 2 seconds, save the checkpoint, and delete the flag."
      ;;
      
  9)
      echo ""
      echo "📊 --- [OPTION 9: PRINT ON-DEMAND STATUS] ---"
      echo "Requesting rich telemetry report from Master Daemon..."
      touch print_status.flag
      echo "✅ 'print_status.flag' created!"
      ls -alth print_status.flag && sleep 2
      ls -alth print_status.flag
      echo "👉 The GA Master will print the detailed RAM vs Disk status to the logs."
      echo "💡 Run Option [5] immediately after this to view the output."
      ;;
      
  10)
    echo ""
    echo "🧪 --- [OPTION 10: EXECUTE PLOT EVENT UNIT TESTS] ---"
    echo "🔍 Running unit tests inside '${CONTAINER_NAME}' container..."
    echo "------------------------------------------------------------------------"
    
    docker exec -it ${CONTAINER_NAME} bash -c \
      "export PATH=/opt/venv/bin:\$PATH PYTHONPATH=/workspace/crypto_apps/dexbot/apps/school:\$PYTHONPATH && \
       cd /workspace/crypto_apps/dexbot/apps/school && \
       python3 -m unittest tests.test_plot_generation_events"
    
    echo "------------------------------------------------------------------------"
    echo "✅ Unit test execution finished."
    ;;

  *)
    echo "❌ Invalid option."
    exit 1
    ;;
esac
