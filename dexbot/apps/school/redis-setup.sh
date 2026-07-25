# Set max memory to 4GB and allow LRU key eviction
redis-cli CONFIG SET maxmemory 4gb
redis-cli CONFIG SET maxmemory-policy volatile-lru
redis-cli CONFIG SET maxclients 10000
redis-cli CONFIG SET timeout 0