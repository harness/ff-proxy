# Brings up redis nodes
redis: 
	docker-compose up -d redis-node1 redis-node2 redis-node3 redis-node4 redis-node5 redis-node6

# Creates a redis cluster
cluster: 
	docker exec redis-node1 sh -c "redis-cli --cluster create redis-node1:6379 redis-node2:6380 redis-node3:6381 redis-node4:6382 redis-node5:6383 redis-node6:6384 --cluster-replicas 1 --cluster-yes"

# Brings up a Primary and replica Proxy
proxy: 
	docker-compose up -d primary replica prometheus grafana
