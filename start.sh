#!/bin/bash
{ ./app/ff-proxy; } &
{ pushpin; } &
wait -n
pkill -P $$