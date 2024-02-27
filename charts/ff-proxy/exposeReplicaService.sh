#!/bin/bash


echo "exposing service"

kubectl port-forward -n ff-proxy svc/ff-proxy-read-replica 7000