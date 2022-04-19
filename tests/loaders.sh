#!/bin/bash

export SHELLOPTS
set -eET

failure() {
  local lineno=$1
  local msg=$2
  echo "Failed at line $lineno: $msg"
}

export -f failure

function cleanup() {
  echo "gnmic_config_file: gnmic-docker-loader.yaml" > clab/loaders/loaders.clab_vars.yaml
  sudo clab des --cleanup -t clab/loaders/loaders.clab.yaml
  docker image prune -f
}

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR
trap cleanup EXIT
trap cleanup SIGINT

# build docker image
docker build -t gnmic:0.0.0-rc1 ../

start=`date +%s`

# docker loader
echo "gnmic_config_file: gnmic-docker-loader.yaml" > clab/loaders/loaders.clab_vars.yaml
sudo clab dep -t clab/loaders/loaders.clab.yaml --reconfigure
sleep 45
sudo clab des -t clab/loaders/loaders.clab.yaml --cleanup

# file loader
# change gnmic config file
echo "gnmic_config_file: gnmic-file-loader.yaml" > clab/loaders/loaders.clab_vars.yaml

# deploy lab with file loader
echo "clab-loaders-srl1:" > ./clab/loaders/targets/targets.yaml
echo "clab-loaders-srl2:" >> ./clab/loaders/targets/targets.yaml
echo "clab-loaders-srl3:" >> ./clab/loaders/targets/targets.yaml

sudo clab dep -t clab/loaders/loaders.clab.yaml --reconfigure
sleep 45
./api.sh clab-loaders-gnmic1:7890
./api.sh clab-loaders-gnmic2:7891
./api.sh clab-loaders-gnmic3:7892
./api.sh clab-loaders-agg-gnmic1:7893
./api.sh clab-loaders-agg-gnmic2:7894
./api.sh clab-loaders-agg-gnmic3:7895

echo "clab-loaders-srl1:" > ./clab/loaders/targets/targets.yaml
echo "clab-loaders-srl2:" >> ./clab/loaders/targets/targets.yaml
sleep 45
./api.sh clab-loaders-gnmic1:7890
./api.sh clab-loaders-gnmic2:7891
./api.sh clab-loaders-gnmic3:7892
./api.sh clab-loaders-agg-gnmic1:7893
./api.sh clab-loaders-agg-gnmic2:7894
./api.sh clab-loaders-agg-gnmic3:7895

echo "clab-loaders-srl1:" > ./clab/loaders/targets/targets.yaml
echo "clab-loaders-srl3:" >> ./clab/loaders/targets/targets.yaml
sleep 45
./api.sh clab-loaders-gnmic1:7890
./api.sh clab-loaders-gnmic2:7891
./api.sh clab-loaders-gnmic3:7892
./api.sh clab-loaders-agg-gnmic1:7893
./api.sh clab-loaders-agg-gnmic2:7894
./api.sh clab-loaders-agg-gnmic3:7895

sudo clab des -t clab/loaders/loaders.clab.yaml --cleanup
