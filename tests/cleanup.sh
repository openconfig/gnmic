#!/bin/bash

# cleanup
rm -f gnmic-rc1
# delete downloaded yang files
rm -rf srl-latest-yang-models
# destroy lab
sudo clab destroy -t clab/$1.clab.yaml --cleanup
