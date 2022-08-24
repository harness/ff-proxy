#!/bin/bash
if [ "$GENERATE_OFFLINE_CONFIG" = true ]
then
  ./proxy-config-fetcher
else
  ./app/ff-proxy &

  pushpin --port localhost:7000
fi


