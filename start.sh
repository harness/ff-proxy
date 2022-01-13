#!/bin/bash

./app/ff-proxy &

pushpin --merge-output --port localhost:7000
  
