#!/bin/bash

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

./gnmic-rc1 generate path --file srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models
./gnmic-rc1 generate path --file srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models --path-type gnmi
./gnmic-rc1 generate path --file srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models --config-only
./gnmic-rc1 generate path --file srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models --with-prefix
./gnmic-rc1 generate path --file srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models --types
./gnmic-rc1 generate path --file srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models --json
./gnmic-rc1 generate path --file srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models --json --config-only
./gnmic-rc1 generate path --file srl-latest-yang-models/srl_nokia/models --dir srl-latest-yang-models --json --path-type gnmi
