#!/bin/bash


echo "scaling replicas"

kubectl patch deployment -n ff-proxy ff-proxy-read-replica --type='merge' -p '{"spec":{"replicas": 10}}'