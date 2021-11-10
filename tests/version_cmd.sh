#!/bin/bash

trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

# version
./gnmic-rc1 version
./gnmic-rc1 version --format json
./gnmic-rc1 version upgrade
./gnmic-rc1 version upgrade --use-pkg