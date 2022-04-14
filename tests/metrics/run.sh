#!/bin/bash

case "$1" in
  "build")
     docker build -t gnmic:0.0.0-rc1 ../../
esac

sudo clab dep -t metrics.clab.yaml --reconfigure

sleep 60

curl http://clab-metrics-gnmic1:7890/metrics
curl http://clab-metrics-gnmic2:7891/metrics
curl http://clab-metrics-gnmic3:7892/metrics