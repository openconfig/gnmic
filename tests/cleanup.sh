#!/bin/bash

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

# cleanup
rm -f gnmic-rc1
# delete downloaded yang files
sudo rm -rf srl-latest-yang-models
# destroy lab
sh "$project_dir/scripts/containerlab.sh" destroy -t clab/$1.clab.yaml --cleanup
