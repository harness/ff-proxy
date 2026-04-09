#!/bin/bash
{ ./app/ff-proxy; } &

if [ "$OFFLINE" != "true" ]; then
    { pushpin; } &
fi

wait -n
pkill -P $$