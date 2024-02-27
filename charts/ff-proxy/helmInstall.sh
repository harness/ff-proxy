#!/bin/bash


echo "installing proxy"


proxyKey=""
secret="foobar"
clientAddress="https://config.feature-flags.qa.harness.io/api/1.0"
metricsAddress="https://event.feature-flags.qa.harness.io/api/1.0"

helm upgrade -i ff-proxy --namespace ff-proxy --create-namespace \
  feature-flag-relay-proxy/ff-proxy \
  --set proxyKey=${proxyKey} \
  --set authSecret=${secret} \
  --set clientService=${clientAddress} \
  --set metricsService=${metricsAddress} \
  --set gcpProfilerEnabled=true \
  --set logLeve="INFO" \
  --set redis.address="10.91.97.4:6379" \
  --set image.tag="dev-latest-v2" \
  --set bypassAuth=true \
  --set readReplica.service.type=NodePort