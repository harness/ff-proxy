# HA Mode Proxy with Redis Cluster 

This example spins up a Primary Proxy & Read Replica Proxy that connect to a Redis Cluster. This example also starts up Prometheus and Grafana to demonstrate what monitoring we have available for the Proxy.

## Configure Proxy

If you haven't already, create a Proxy Key in Harness and set it as the `PROXY_KEY` environment variable in the docker-compose file.

## Running

Before we can start running and using the Proxy we first need to create our redis nodes which can be done by running

`make redis`

We then need to link our redis nodes and create our redis cluster which can be done by running

`make cluster`

Finally we can bring up the Proxy by running 

`make proxy`

You can then run `docker ps` to verify that everything is up and running properly. The output for `docker ps` should look something like this

```
CONTAINER ID   IMAGE                          COMMAND                  CREATED          STATUS          PORTS                               NAMES
dcd41af03def   grafana/grafana:10.2.0         "/run.sh"                3 seconds ago    Up 2 seconds    0.0.0.0:3000->3000/tcp              redis_cluster_ha_mode_with_monitoring_grafana_1
e6e7fa5d8de0   prom/prometheus:v2.47.2        "/bin/prometheus --c…"   3 seconds ago    Up 2 seconds    0.0.0.0:9090->9090/tcp              redis_cluster_ha_mode_with_monitoring_prometheus_1
55aa8c276034   harness/ff-proxy:2.0.0-rc.20   "docker-entrypoint.s…"   3 seconds ago    Up 2 seconds    0.0.0.0:7002->7000/tcp              replica-proxy
71e6b5fb6b3c   harness/ff-proxy:2.0.0-rc.20   "docker-entrypoint.s…"   3 seconds ago    Up 2 seconds    0.0.0.0:7001->7000/tcp              primary-proxy
65ca3eaac2f5   bitnami/redis-cluster:7.2.4    "/opt/bitnami/script…"   33 seconds ago   Up 32 seconds   6379/tcp, 0.0.0.0:63784->6384/tcp   redis-node6
cbfb255c62da   bitnami/redis-cluster:7.2.4    "/opt/bitnami/script…"   33 seconds ago   Up 32 seconds   6379/tcp, 0.0.0.0:63782->6382/tcp   redis-node4
3e0c8a7190f7   bitnami/redis-cluster:7.2.4    "/opt/bitnami/script…"   33 seconds ago   Up 32 seconds   6379/tcp, 0.0.0.0:63780->6380/tcp   redis-node2
d5b520b1872f   bitnami/redis-cluster:7.2.4    "/opt/bitnami/script…"   33 seconds ago   Up 32 seconds   6379/tcp, 0.0.0.0:63783->6383/tcp   redis-node5
f117be5b937d   bitnami/redis-cluster:7.2.4    "/opt/bitnami/script…"   33 seconds ago   Up 32 seconds   6379/tcp                            redis-node1
5e4f89a2e558   bitnami/redis-cluster:7.2.4    "/opt/bitnami/script…"   33 seconds ago   Up 32 seconds   6379/tcp, 0.0.0.0:63781->6381/tcp   redis-node3
```

Note: It's important that every time you run this example you run the commands in this order as each one depends on the previous one.

## Configuring an SDK to use the Proxy

See [Configure SDKs to work with the Relay Proxy](https://developer.harness.io/docs/feature-flags/relay-proxy/deploy-relay-proxy#configure-sdks-to-work-with-the-relay-proxy)

## Monitoring the Proxy with Grafana

- Open [http://localhost:3000](http://localhost:3000) in your browser
- To log in to a local grafana instance for the first time use `admin` for the username and `admin` for the password. Grafana will then ask you to create a new password for the `admin` user, you can set this to any value you want it to be.
- Once you've logged in you should be able to navigate to the Harness FF Proxy dashboard. If you've pointed an SDK at your Proxy you should be able to see some metrics start to appear here. 



https://github.com/harness/ff-proxy/assets/16992818/abf8362a-0897-442a-a3d7-1f932cdd5906


