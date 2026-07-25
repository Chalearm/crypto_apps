# 1. Terminate running master/workers
./school -action=terminate

# 2. Wipe logs, models, results, AND all checkpoint files
rm -rf logs/* prediction_result/* deployed_models/*  *.json


# 4. Start Master fresh
nohup ./school -action=set-up > logs/setup.log 2>&1 &
# 3. Configure Redis
./redis-setup.sh
# 5. Check status
./school -action=status