#!/bin/bash

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
. "$project_dir/versions.env"

case "$1" in
  "build")
     sh "$project_dir/scripts/docker-build.sh" -t "gnmic:$GNMIC_TEST_VERSION"
esac

sh "$project_dir/scripts/containerlab.sh" dep -t metrics.clab.yaml --reconfigure

sleep 60

curl http://clab-metrics-gnmic1:7890/metrics
curl http://clab-metrics-gnmic2:7891/metrics
curl http://clab-metrics-gnmic3:7892/metrics
