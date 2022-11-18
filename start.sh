#!/bin/bash
{ ./app/ff-proxy; } &
{ pushpin; } &
wait -n
echo "condure logs"
cat /pushpin/log/condure.log
echo "pushpin-handler logs"
cat /pushpin/log/pushpin-handler.log
echo "pushpin-proxy logs"
cat /pushpin/log/pushpin-proxy.log
echo "zurl logs"
cat /pushpin/log/zurl.log
pkill -P $$