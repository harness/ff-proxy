#!/bin/bash
{ ./app/ff-proxy; } &
{ pushpin --port localhost:7000; } &
wait -n
pkill -P $$